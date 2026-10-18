package com.wcomsat.vpn

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.system.Os
import android.system.OsConstants
import android.util.Log
import androidx.core.app.NotificationCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import java.io.File
import java.net.InetSocketAddress
import java.net.Socket

class TunnelService : VpnService() {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    @Volatile private var xray: Process? = null
    @Volatile private var tun2socks: Process? = null
    private var tun: ParcelFileDescriptor? = null
    private var monitor: Job? = null

    // Свой порт на каждый запуск: старые процессы Xray не мешают новому.
    private val socksPort = 10808 + (android.os.Process.myPid() % 500) * 2
    private val httpPort = socksPort + 1

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            stopTunnel()
            stopSelf()
            return START_NOT_STICKY
        }

        val subscription = intent?.getStringExtra(EXTRA_SUBSCRIPTION).orEmpty()
        val channel = intent?.getStringExtra(EXTRA_CHANNEL) ?: "xhttp"

        startForegroundNow()

        if (subscription.isBlank()) {
            TunnelState.error.value = "Пустая подписка"
            stopTunnel()
            stopSelf()
            return START_NOT_STICKY
        }

        scope.launch { connect(subscription, channel) }
        return START_NOT_STICKY
    }

    private fun connect(subscription: String, channel: String) {
        try {
            stopTunnel()
            TunnelState.error.value = null
            TunnelState.running.value = true
            TunnelState.message.value = "Подключение…"
            TunnelState.channel.value = channel

            val dir = File(filesDir, "xray").apply { mkdirs() }
            copyAsset("xray/geoip.dat", File(dir, "geoip.dat"))
            copyAsset("xray/geosite.dat", File(dir, "geosite.dat"))

            val ch = Subscription.pick(Subscription.parse(subscription), channel)
            val cfgFile = File(dir, "config.json")
            cfgFile.writeText(
                Subscription.buildXrayConfig(ch, socksPort, httpPort).toString(),
            )

            val native = File(applicationInfo.nativeLibraryDir)
            val xrayBin = File(native, "libxray.so")
            val t2sBin = File(native, "libtun2socks.so")
            require(xrayBin.exists()) { "нет libxray.so для этой архитектуры" }
            require(t2sBin.exists()) { "нет libtun2socks.so для этой архитектуры" }

            val xrayLog = File(dir, "xray.log")
            xray = ProcessBuilder(xrayBin.absolutePath, "run", "-c", cfgFile.absolutePath)
                .redirectErrorStream(true)
                .redirectOutput(xrayLog)
                .apply { environment()["XRAY_LOCATION_ASSET"] = dir.absolutePath }
                .start()

            if (!waitPort(socksPort, 12_000)) {
                throw IllegalStateException(
                    "Xray не поднял SOCKS.\n${logTail(xrayLog)}",
                )
            }

            val builder = Builder()
                .setSession("VPN")
                .setMtu(1500)
                .addAddress("198.18.0.1", 15)
                .addRoute("0.0.0.0", 0)
                .addAddress("fd00::1", 128)
                .addRoute("::", 0)
                .addDnsServer("1.1.1.1")
                .addDnsServer("8.8.8.8")

            // Свои соединения (Xray/tun2socks) не заворачиваем в туннель.
            try {
                builder.addDisallowedApplication(packageName)
            } catch (e: Exception) {
                Log.w(TAG, "addDisallowedApplication: ${e.message}")
            }

            val pfd = builder.establish() ?: error("не удалось создать TUN")
            tun = pfd

            // Ребёнок должен унаследовать fd TUN.
            try {
                Os.fcntlInt(pfd.fileDescriptor, OsConstants.F_SETFD, 0)
            } catch (e: Exception) {
                Log.w(TAG, "fcntl FD_CLOEXEC: ${e.message}")
            }

            tun2socks = ProcessBuilder(
                t2sBin.absolutePath,
                "--device",
                "fd://${pfd.fd}",
                "--proxy",
                "socks5://127.0.0.1:$socksPort",
                "--loglevel",
                "warn",
            )
                .redirectErrorStream(true)
                .redirectOutput(File(dir, "tun2socks.log"))
                .start()

            Thread.sleep(400)
            if (!alive(xray) || !alive(tun2socks)) {
                throw IllegalStateException(
                    "процесс туннеля завершился.\n${logTail(File(dir, "tun2socks.log"))}",
                )
            }

            TunnelState.message.value = "Подключено"
            monitor = scope.launch { watch(dir) }
        } catch (e: Exception) {
            stopTunnel()
            TunnelState.error.value = e.message ?: e.toString()
            TunnelState.message.value = "Ошибка"
            stopSelf()
        }
    }

    private fun stopTunnel() {
        monitor?.cancel()
        monitor = null

        try {
            tun2socks?.destroy()
        } catch (_: Exception) {
        }
        try {
            xray?.destroy()
        } catch (_: Exception) {
        }
        tun2socks = null
        xray = null

        try {
            tun?.close()
        } catch (_: Exception) {
        }
        tun = null

        TunnelState.running.value = false
        if (TunnelState.message.value != "Ошибка") {
            TunnelState.message.value = "Отключено"
        }
    }

    override fun onRevoke() {
        stopTunnel()
        stopSelf()
        super.onRevoke()
    }

    override fun onDestroy() {
        stopTunnel()
        scope.cancel()
        stopForeground(STOP_FOREGROUND_REMOVE)
        super.onDestroy()
    }

    private fun startForegroundNow() {
        val channelId = CHANNEL_ID
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val mgr = getSystemService(NotificationManager::class.java)
            if (mgr.getNotificationChannel(channelId) == null) {
                mgr.createNotificationChannel(
                    NotificationChannel(channelId, "VPN", NotificationManager.IMPORTANCE_LOW),
                )
            }
        }

        val open = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java).apply {
                flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP
            },
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )

        val notification = NotificationCompat.Builder(this, channelId)
            .setContentTitle("VPN")
            .setContentText("Подключено")
            .setSmallIcon(R.drawable.ic_notification)
            .setContentIntent(open)
            .setOngoing(true)
            .build()

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startForeground(
                NOTIF_ID,
                notification,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE,
            )
        } else {
            startForeground(NOTIF_ID, notification)
        }
    }

    private fun copyAsset(asset: String, dest: File) {
        if (dest.exists() && dest.length() > 0) return
        dest.parentFile?.mkdirs()
        assets.open(asset).use { input ->
            dest.outputStream().use { output -> input.copyTo(output) }
        }
    }

    private suspend fun watch(dir: File) {
        while (true) {
            delay(3_000)
            if (!TunnelState.running.value) return
            if (alive(xray) && alive(tun2socks)) continue

            stopTunnel()
            TunnelState.error.value =
                "Туннель остановился.\n${logTail(File(dir, "tun2socks.log"))}"
            TunnelState.message.value = "Ошибка"
            stopSelf()
            return
        }
    }

    private fun alive(p: Process?): Boolean {
        if (p == null) return false
        return try {
            p.exitValue()
            false
        } catch (_: IllegalThreadStateException) {
            true
        }
    }

    private fun waitPort(port: Int, timeoutMs: Long): Boolean {
        val deadline = System.currentTimeMillis() + timeoutMs
        while (System.currentTimeMillis() < deadline) {
            try {
                Socket().use {
                    it.connect(InetSocketAddress("127.0.0.1", port), 300)
                    return true
                }
            } catch (_: Exception) {
            }
            Thread.sleep(200)
        }
        return false
    }

    private fun logTail(file: File): String = runCatching {
        if (!file.exists()) "" else file.readText().takeLast(600)
    }.getOrDefault("")

    companion object {
        const val ACTION_START = "com.wcomsat.vpn.START"
        const val ACTION_STOP = "com.wcomsat.vpn.STOP"
        const val EXTRA_SUBSCRIPTION = "subscription"
        const val EXTRA_CHANNEL = "channel"

        private const val TAG = "VPN"
        private const val CHANNEL_ID = "vpn"
        private const val NOTIF_ID = 1
    }
}
