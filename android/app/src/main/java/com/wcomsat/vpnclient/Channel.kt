package com.wcomsat.vpnclient

import android.net.Uri

/**
 * Модель канала и парсер подписки (vless://).
 *
 * Контракт общий с Windows-клиентом — см. shared/README.md и фикстуры
 * shared/fixtures. Результат разбора должен совпадать с Go-реализацией.
 */
data class Reality(
    val serverName: String,
    val publicKey: String,
    val shortId: String,
    val fingerprint: String,
    val spiderX: String,
)

data class Xhttp(val host: String, val path: String, val mode: String)

data class Channel(
    val name: String,
    val uuid: String,
    val address: String,
    val port: Int,
    val transport: String, // "xhttp" | "tcp"
    val security: String,
    val reality: Reality,
    val xhttp: Xhttp?,
    val flow: String?,
)

object SubscriptionParser {

    fun parse(text: String): List<Channel> =
        text.lineSequence()
            .map { it.trim() }
            .filter { it.isNotEmpty() && !it.startsWith("#") }
            .map { parseLink(it) }
            .toList()

    fun parseLink(raw: String): Channel {
        val uri = Uri.parse(raw.trim())
        require(uri.scheme == "vless") { "ожидалась схема vless://" }
        val uuid = uri.userInfo?.takeIf { it.isNotEmpty() } ?: error("в ссылке нет UUID")
        val address = uri.host ?: error("в ссылке нет адреса")
        val port = uri.port
        require(port in 1..65535) { "некорректный порт: $port" }

        fun q(k: String): String = uri.getQueryParameter(k) ?: ""

        val transport = q("type").ifEmpty { "tcp" }
        require(transport == "xhttp" || transport == "tcp") { "неподдерживаемый транспорт: $transport" }

        val reality = Reality(
            serverName = q("sni"),
            publicKey = q("pbk"),
            shortId = q("sid"),
            fingerprint = q("fp").ifEmpty { "firefox" },
            spiderX = q("spx").ifEmpty { "/" },
        )
        val xhttp = if (transport == "xhttp") {
            Xhttp(
                host = q("host").ifEmpty { reality.serverName },
                path = q("path").ifEmpty { "/" },
                mode = q("mode").ifEmpty { "stream-one" },
            )
        } else null
        val flow = if (transport == "tcp") q("flow").ifEmpty { null } else null

        return Channel(
            name = uri.fragment ?: "",
            uuid = uuid,
            address = address,
            port = port,
            transport = transport,
            security = q("security").ifEmpty { "reality" },
            reality = reality,
            xhttp = xhttp,
            flow = flow,
        )
    }
}
