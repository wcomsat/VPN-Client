package com.wcomsat.vpn

import kotlinx.coroutines.flow.MutableStateFlow

/** Состояние туннеля: сервис пишет, UI читает (тот же процесс). */
object TunnelState {
    val running = MutableStateFlow(false)
    val message = MutableStateFlow("Отключено")
    val error = MutableStateFlow<String?>(null)
    val channel = MutableStateFlow("xhttp")
}
