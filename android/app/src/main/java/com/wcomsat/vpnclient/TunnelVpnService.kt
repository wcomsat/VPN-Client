package com.wcomsat.vpnclient

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.Build

/**
 * Системный VPN через VpnService.
 *
 * Текущее состояние — каркас. На Фазе 2 здесь появится:
 *   1. чтение сохранённой подписки и выбранного канала;
 *   2. ClientConfig.build(channel) → конфиг ядра;
 *   3. запуск libXray (SOCKS 127.0.0.1:10808);
 *   4. establish() tun и его маршрутизация в SOCKS через tun2socks;
 *   5. маршруты/DNS, обработка переподключений.
 */
class TunnelVpnService : VpnService() {

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_DISCONNECT) {
            stopTunnel()
            stopSelf()
            return START_NOT_STICKY
        }
        startTunnel()
        return START_STICKY
    }

    private fun startTunnel() {
        startForeground(NOTIF_ID, buildNotification())
        // TODO(Фаза 2): см. описание класса.
    }

    private fun stopTunnel() {
        // TODO(Фаза 2): остановить libXray и tun2socks, закрыть tun.
    }

    private fun buildNotification(): Notification {
        val manager = getSystemService(NotificationManager::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            manager.createNotificationChannel(
                NotificationChannel(CHANNEL_ID, "VPN", NotificationManager.IMPORTANCE_LOW)
            )
        }
        val open = PendingIntent.getActivity(
            this, 0, Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE
        )
        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL_ID)
        } else {
            @Suppress("DEPRECATION")
            Notification.Builder(this)
        }
        return builder
            .setContentTitle("VPN подключён")
            .setContentText("Трафик идёт через ваш сервер")
            .setSmallIcon(android.R.drawable.stat_sys_vpn_ic)
            .setContentIntent(open)
            .setOngoing(true)
            .build()
    }

    companion object {
        const val ACTION_CONNECT = "com.wcomsat.vpnclient.CONNECT"
        const val ACTION_DISCONNECT = "com.wcomsat.vpnclient.DISCONNECT"
        private const val CHANNEL_ID = "vpn"
        private const val NOTIF_ID = 1
    }
}
