package com.wcomsat.vpn

import android.Manifest
import android.app.Activity
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.LocalTextStyle
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle

private val Bg = Color(0xFF0E1219)
private val Card = Color(0xFF161C26)
private val Line = Color(0xFF252D3A)
private val Muted = Color(0xFF93A0B4)
private val TextMain = Color(0xFFE9EDF3)
private val Blue = Color(0xFF3B82F6)
private val Green = Color(0xFF22C55E)
private val Red = Color(0xFFEF4444)
private val ErrorColor = Color(0xFFFCA5A5)

class MainActivity : ComponentActivity() {

    private var pendingSubscription = ""
    private var pendingChannel = "xhttp"

    private val vpnPermission =
        registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
            if (result.resultCode == Activity.RESULT_OK) {
                startTunnel(pendingSubscription, pendingChannel)
            }
        }

    private val notificationPermission =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) { }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        acceptIncoming(intent)
        setContent {
            VpnTheme {
                VpnApp(
                    onConnect = ::connect,
                    onDisconnect = ::disconnect,
                )
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        if (acceptIncoming(intent)) recreate()
    }

    /** Принять подписку, отправленную в приложение файлом или текстом. */
    private fun acceptIncoming(intent: Intent?): Boolean {
        if (intent == null) return false
        @Suppress("DEPRECATION")
        val stream = intent.getParcelableExtra(Intent.EXTRA_STREAM) as? Uri
        val text = when (intent.action) {
            Intent.ACTION_SEND ->
                intent.getCharSequenceExtra(Intent.EXTRA_TEXT)?.toString()
                    ?: stream?.let(::readUri)
            Intent.ACTION_VIEW -> intent.data?.let(::readUri)
            else -> null
        }?.trim()

        if (text.isNullOrBlank()) return false
        if (runCatching { Subscription.parse(text) }.isFailure) return false

        Prefs.save(this, text)
        return true
    }

    private fun readUri(uri: Uri): String? = runCatching {
        contentResolver.openInputStream(uri)?.bufferedReader()?.use { it.readText() }
    }.getOrNull()

    private fun connect(subscription: String, channel: String) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(
                this,
                Manifest.permission.POST_NOTIFICATIONS,
            ) != PackageManager.PERMISSION_GRANTED
        ) {
            notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
        }

        pendingSubscription = subscription
        pendingChannel = channel

        val prep = VpnService.prepare(this)
        if (prep != null) {
            vpnPermission.launch(prep)
        } else {
            startTunnel(subscription, channel)
        }
    }

    private fun startTunnel(subscription: String, channel: String) {
        val intent = Intent(this, TunnelService::class.java).apply {
            action = TunnelService.ACTION_START
            putExtra(TunnelService.EXTRA_SUBSCRIPTION, subscription)
            putExtra(TunnelService.EXTRA_CHANNEL, channel)
        }
        ContextCompat.startForegroundService(this, intent)
    }

    private fun disconnect() {
        stopService(Intent(this, TunnelService::class.java))
    }
}

@Composable
private fun VpnTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = darkColorScheme(
            primary = Blue,
            background = Bg,
            surface = Card,
            onPrimary = Color.White,
            onBackground = TextMain,
            onSurface = TextMain,
        ),
        content = content,
    )
}

@Composable
private fun VpnApp(
    onConnect: (String, String) -> Unit,
    onDisconnect: () -> Unit,
) {
    val context = LocalContext.current
    var subscription by remember { mutableStateOf(Prefs.load(context)) }
    var channel by remember { mutableStateOf(Prefs.channel(context)) }
    var editing by remember { mutableStateOf(subscription.isBlank()) }
    var error by remember { mutableStateOf<String?>(null) }

    val running by TunnelState.running.collectAsStateWithLifecycle()
    val message by TunnelState.message.collectAsStateWithLifecycle()
    val serviceError by TunnelState.error.collectAsStateWithLifecycle()

    val channels = remember(subscription) {
        runCatching {
            Subscription.parse(subscription).map(Subscription::label).distinct()
        }.getOrDefault(emptyList())
    }

    LaunchedEffect(channels) {
        if (channels.isNotEmpty() && channel !in channels) {
            channel = channels.first()
        }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(
                Brush.verticalGradient(
                    listOf(Color(0xFF141D2B), Bg, Color(0xFF0B0F15)),
                ),
            ),
        contentAlignment = Alignment.TopCenter,
    ) {
        Column(
            modifier = Modifier
                .widthIn(max = 520.dp)
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .imePadding()
                .safeDrawingPadding()
                .padding(horizontal = 20.dp, vertical = 18.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Header()
            Spacer(Modifier.height(16.dp))

            if (editing) {
                SetupScreen(
                    subscription = subscription,
                    onSubscriptionChange = {
                        subscription = it
                        error = null
                    },
                    error = error,
                    allowCancel = subscription.isNotBlank(),
                    onCancel = {
                        editing = false
                        error = null
                    },
                    onSave = {
                        try {
                            val text = subscription.trim()
                            require(text.isNotEmpty()) { "Вставьте подписку" }
                            val parsed = Subscription.parse(text)
                            val labels = parsed.map(Subscription::label).distinct()
                            Prefs.save(context, text)
                            subscription = text
                            if (channel !in labels) channel = labels.first()
                            Prefs.saveChannel(context, channel)
                            error = null
                            editing = false
                        } catch (e: Exception) {
                            error = e.message ?: e.toString()
                        }
                    },
                )
            } else {
                MainScreen(
                    subscription = subscription,
                    running = running,
                    message = message,
                    serviceError = serviceError,
                    channels = channels,
                    channel = channel,
                    onChannel = {
                        channel = it
                        Prefs.saveChannel(context, it)
                    },
                    onConnect = { onConnect(subscription, channel) },
                    onDisconnect = onDisconnect,
                    onEdit = {
                        editing = true
                        error = null
                    },
                )
            }
        }
    }
}

@Composable
private fun Header() {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            modifier = Modifier
                .size(44.dp)
                .clip(RoundedCornerShape(12.dp))
                .background(Brush.linearGradient(listOf(Color(0xFF2563EB), Blue))),
            contentAlignment = Alignment.Center,
        ) {
            ShieldIcon(Modifier.size(22.dp))
        }
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text("VPN", fontSize = 19.sp, fontWeight = FontWeight.Bold, color = TextMain)
            Text("Личный доступ", fontSize = 12.sp, color = Muted)
        }
    }
}

@Composable
private fun ShieldIcon(modifier: Modifier) {
    Canvas(modifier) {
        val path = Path().apply {
            moveTo(size.width / 2f, 0f)
            lineTo(size.width, size.height * 0.18f)
            lineTo(size.width, size.height * 0.55f)
            quadraticTo(
                size.width,
                size.height * 0.85f,
                size.width / 2f,
                size.height,
            )
            quadraticTo(0f, size.height * 0.85f, 0f, size.height * 0.55f)
            lineTo(0f, size.height * 0.18f)
            close()
        }
        drawPath(path, Color.White)
    }
}

@Composable
private fun SetupScreen(
    subscription: String,
    onSubscriptionChange: (String) -> Unit,
    error: String?,
    allowCancel: Boolean,
    onCancel: () -> Unit,
    onSave: () -> Unit,
) {
    val context = LocalContext.current
    val clipboard = context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager

    val filePicker = rememberLauncherForActivityResult(
        ActivityResultContracts.OpenDocument(),
    ) { uri: Uri? ->
        if (uri != null) {
            runCatching {
                context.contentResolver.openInputStream(uri)?.bufferedReader()?.use { it.readText() }
            }.onSuccess { text ->
                if (!text.isNullOrBlank()) onSubscriptionChange(text.trim())
            }.onFailure {
                // Текст ошибки покажет onSave/parse; здесь просто игнорируем.
            }
        }
    }

    Text(
        text = "Подписка",
        modifier = Modifier.fillMaxWidth(),
        color = TextMain,
        fontSize = 15.sp,
        fontWeight = FontWeight.SemiBold,
    )
    Spacer(Modifier.height(8.dp))

    OutlinedTextField(
        value = subscription,
        onValueChange = onSubscriptionChange,
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 150.dp),
        placeholder = { Text("vless://...", color = Muted) },
        textStyle = LocalTextStyle.current.copy(fontSize = 13.sp, color = TextMain),
        colors = OutlinedTextFieldDefaults.colors(
            focusedBorderColor = Blue,
            unfocusedBorderColor = Line,
            cursorColor = Blue,
        ),
    )

    Spacer(Modifier.height(6.dp))
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        TextButton(
            onClick = {
                val text = clipboard.primaryClip
                    ?.getItemAt(0)
                    ?.coerceToText(context)
                    ?.toString()
                if (!text.isNullOrBlank()) onSubscriptionChange(text.trim())
            },
        ) {
            Text("Вставить", color = Blue)
        }
        TextButton(onClick = { filePicker.launch(arrayOf("*/*")) }) {
            Text("Импорт из файла", color = Blue)
        }
    }

    if (error != null) {
        Text(
            text = error,
            modifier = Modifier.fillMaxWidth(),
            color = ErrorColor,
            fontSize = 12.5.sp,
        )
        Spacer(Modifier.height(6.dp))
    }

    Button(
        onClick = onSave,
        modifier = Modifier
            .fillMaxWidth()
            .height(50.dp),
        colors = ButtonDefaults.buttonColors(containerColor = Blue),
    ) {
        Text("Сохранить", fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
    }

    if (allowCancel) {
        TextButton(onClick = onCancel) {
            Text("Отмена", color = Muted)
        }
    }
}

@Composable
private fun MainScreen(
    subscription: String,
    running: Boolean,
    message: String,
    serviceError: String?,
    channels: List<String>,
    channel: String,
    onChannel: (String) -> Unit,
    onConnect: () -> Unit,
    onDisconnect: () -> Unit,
    onEdit: () -> Unit,
) {
    val detail = remember(subscription, channel) {
        runCatching {
            val ch = Subscription.pick(Subscription.parse(subscription), channel)
            "${Subscription.label(ch)} · ${ch.address}"
        }.getOrDefault("")
    }

    PowerButton(
        on = running,
        onClick = { if (running) onDisconnect() else onConnect() },
    )
    Spacer(Modifier.height(16.dp))

    Text(
        text = message,
        color = TextMain,
        fontSize = 24.sp,
        fontWeight = FontWeight.Bold,
        textAlign = TextAlign.Center,
    )
    if (detail.isNotEmpty()) {
        Text(detail, color = Muted, fontSize = 13.sp, textAlign = TextAlign.Center)
    }
    if (serviceError != null) {
        Spacer(Modifier.height(8.dp))
        Text(
            text = serviceError,
            color = ErrorColor,
            fontSize = 12.sp,
            textAlign = TextAlign.Center,
        )
    }

    Spacer(Modifier.height(22.dp))
    Text(
        text = "Канал",
        modifier = Modifier.fillMaxWidth(),
        color = Muted,
        fontSize = 12.sp,
    )
    Spacer(Modifier.height(6.dp))
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        channels.forEach { c ->
            SegmentButton(
                text = if (c == "vision") "Vision" else "xHTTP",
                selected = c == channel,
                onClick = { onChannel(c) },
                modifier = Modifier.weight(1f),
            )
        }
    }

    Spacer(Modifier.height(16.dp))
    Text(
        text = "TUN — весь трафик через VPN. Android один раз спросит разрешение.",
        modifier = Modifier.fillMaxWidth(),
        color = Muted,
        fontSize = 12.sp,
    )

    Spacer(Modifier.height(6.dp))
    TextButton(onClick = onEdit) {
        Text("Изменить подписку", color = Muted)
    }
}

@Composable
private fun PowerButton(on: Boolean, onClick: () -> Unit) {
    var focused by remember { mutableStateOf(false) }
    val active = if (on) Green else Red

    Box(
        modifier = Modifier
            .size(176.dp)
            .onFocusChanged { focused = it.isFocused }
            .clip(CircleShape)
            .background(
                Brush.radialGradient(
                    listOf(
                        active.copy(alpha = 0.95f),
                        active.copy(alpha = 0.45f),
                        Color(0xFF0C0708),
                    ),
                ),
            )
            .border(
                width = if (focused) 3.dp else 0.dp,
                color = Color.White,
                shape = CircleShape,
            )
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Canvas(Modifier.size(64.dp)) {
            val stroke = 6.dp.toPx()
            drawArc(
                color = Color.White,
                startAngle = -60f,
                sweepAngle = 300f,
                useCenter = false,
                style = Stroke(width = stroke, cap = StrokeCap.Round),
            )
            drawLine(
                color = Color.White,
                start = Offset(size.width / 2f, 0f),
                end = Offset(size.width / 2f, size.height * 0.46f),
                strokeWidth = stroke,
                cap = StrokeCap.Round,
            )
        }
    }
}

@Composable
private fun SegmentButton(
    text: String,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    var focused by remember { mutableStateOf(false) }
    val bg = when {
        selected -> Color(0xFF1E293B)
        focused -> Color(0xFF1A2433)
        else -> Color.Transparent
    }

    Box(
        modifier = modifier
            .height(46.dp)
            .onFocusChanged { focused = it.isFocused }
            .clip(RoundedCornerShape(10.dp))
            .background(bg)
            .border(
                width = if (focused) 2.dp else 0.dp,
                color = if (focused) Blue else Color.Transparent,
                shape = RoundedCornerShape(10.dp),
            )
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = text,
            color = if (selected) Color.White else Muted,
            fontSize = 14.sp,
            fontWeight = FontWeight.SemiBold,
        )
    }
}
