package com.wcomsat.vpn

import org.json.JSONArray
import org.json.JSONObject

data class Reality(
    val serverName: String,
    val publicKey: String,
    val shortId: String,
    val fingerprint: String,
    val spiderX: String,
)

data class Xhttp(
    val host: String,
    val path: String,
    val mode: String,
)

data class Channel(
    val name: String,
    val uuid: String,
    val address: String,
    val port: Int,
    /** "xhttp" | "tcp" */
    val transport: String,
    val security: String,
    val reality: Reality,
    val xhttp: Xhttp?,
    val flow: String?,
)

/**
 * Разбор подписки `vless://` и сборка конфига Xray-core.
 * Контракт совпадает с core/ (Rust) и shared/fixtures.
 */
object Subscription {

    fun parseLink(raw: String): Channel {
        val line = raw.trim()
        require(line.isNotEmpty()) { "пустая ссылка" }
        require(line.startsWith("vless://")) { "ожидалась схема vless://" }
        var rest = line.removePrefix("vless://")

        val hash = rest.indexOf('#')
        val name = if (hash >= 0) rest.substring(hash + 1) else ""
        if (hash >= 0) rest = rest.substring(0, hash)

        val q = rest.indexOf('?')
        val authority = if (q >= 0) rest.substring(0, q) else rest
        val query = if (q >= 0) rest.substring(q + 1) else ""

        val at = authority.lastIndexOf('@')
        require(at > 0) { "в ссылке нет UUID (userinfo@host)" }
        val uuid = authority.substring(0, at)
        val (host, portStr) = splitHostPort(authority.substring(at + 1))
        val port = portStr.toIntOrNull() ?: error("некорректный порт $portStr")

        val params: Map<String, String> = if (query.isEmpty()) {
            emptyMap()
        } else {
            query.split('&')
                .filter { it.isNotEmpty() }
                .associate { pair ->
                    val i = pair.indexOf('=')
                    if (i >= 0) pair.substring(0, i) to pair.substring(i + 1) else pair to ""
                }
        }

        val transport = params["type"].orEmpty().ifEmpty { "tcp" }
        require(transport == "xhttp" || transport == "tcp") {
            "неподдерживаемый транспорт: $transport"
        }

        val sni = params["sni"].orEmpty()
        val fp = params["fp"].orEmpty().ifEmpty { "firefox" }
        val spiderX = params["spx"].orEmpty().ifEmpty { "/" }
        val security = params["security"].orEmpty().ifEmpty { "reality" }

        val reality = Reality(
            serverName = sni,
            publicKey = params["pbk"].orEmpty(),
            shortId = params["sid"].orEmpty(),
            fingerprint = fp,
            spiderX = spiderX,
        )

        val xhttp = if (transport == "xhttp") {
            Xhttp(
                host = params["host"].orEmpty().ifEmpty { sni },
                path = params["path"].orEmpty().ifEmpty { "/" },
                mode = params["mode"].orEmpty().ifEmpty { "stream-one" },
            )
        } else {
            null
        }

        val flow = if (transport == "tcp") {
            params["flow"]?.takeIf { it.isNotEmpty() }
        } else {
            null
        }

        return Channel(
            name = name,
            uuid = uuid,
            address = host,
            port = port,
            transport = transport,
            security = security,
            reality = reality,
            xhttp = xhttp,
            flow = flow,
        )
    }

    fun parse(text: String): List<Channel> {
        val out = text.lines()
            .map { it.trim() }
            .filter { it.isNotEmpty() && !it.startsWith("#") }
            .map { parseLink(it) }
        require(out.isNotEmpty()) { "в подписке нет ни одной ссылки" }
        return out
    }

    fun pick(channels: List<Channel>, want: String): Channel {
        val target = when (want) {
            "", "xhttp" -> "xhttp"
            "vision" -> "tcp"
            else -> want
        }
        return channels.firstOrNull { it.transport == target }
            ?: channels.firstOrNull()
            ?: error("в подписке нет ни одного канала")
    }

    fun label(channel: Channel): String =
        if (channel.transport == "tcp") "vision" else channel.transport

    fun buildXrayConfig(
        ch: Channel,
        socksPort: Int = 10808,
        httpPort: Int = 10809,
    ): JSONObject {
        val sniffing = JSONObject()
            .put("enabled", true)
            .put("destOverride", JSONArray(listOf("http", "tls", "quic")))

        val socks = JSONObject()
            .put("listen", "127.0.0.1")
            .put("port", socksPort)
            .put("protocol", "socks")
            .put("settings", JSONObject().put("udp", true))
            .put("sniffing", sniffing)

        val http = JSONObject()
            .put("listen", "127.0.0.1")
            .put("port", httpPort)
            .put("protocol", "http")
            .put("sniffing", sniffing)

        val user = JSONObject()
            .put("id", ch.uuid)
            .put("encryption", "none")
        ch.flow?.let { user.put("flow", it) }

        val vnext = JSONObject()
            .put("address", ch.address)
            .put("port", ch.port)
            .put("users", JSONArray().put(user))

        val reality = JSONObject()
            .put("serverName", ch.reality.serverName)
            .put("fingerprint", ch.reality.fingerprint)
            .put("publicKey", ch.reality.publicKey)
            .put("shortId", ch.reality.shortId)
            .put("spiderX", ch.reality.spiderX)

        val stream = JSONObject()
            .put("network", ch.transport)
            .put("security", "reality")
            .put("realitySettings", reality)
        ch.xhttp?.let {
            stream.put(
                "xhttpSettings",
                JSONObject()
                    .put("host", it.host)
                    .put("path", it.path)
                    .put("mode", it.mode),
            )
        }

        val vless = JSONObject()
            .put("protocol", "vless")
            .put("settings", JSONObject().put("vnext", JSONArray().put(vnext)))
            .put("streamSettings", stream)

        val direct = JSONObject()
            .put("protocol", "freedom")
            .put("tag", "direct")

        val rules = JSONArray()
            .put(
                JSONObject()
                    .put("type", "field")
                    .put("domain", JSONArray(listOf("geosite:private", "geosite:category-ru")))
                    .put("outboundTag", "direct"),
            )
            .put(
                JSONObject()
                    .put("type", "field")
                    .put("ip", JSONArray(listOf("geoip:private", "geoip:ru")))
                    .put("outboundTag", "direct"),
            )

        return JSONObject()
            .put("log", JSONObject().put("loglevel", "warning"))
            .put("inbounds", JSONArray().put(socks).put(http))
            .put("outbounds", JSONArray().put(vless).put(direct))
            .put(
                "routing",
                JSONObject()
                    .put("domainStrategy", "IPIfNonMatch")
                    .put("rules", rules),
            )
    }

    private fun splitHostPort(hostPort: String): Pair<String, String> {
        if (hostPort.startsWith("[")) {
            val end = hostPort.indexOf(']')
            require(end > 0) { "некорректный адрес $hostPort" }
            val host = hostPort.substring(1, end)
            val after = hostPort.substring(end + 1)
            require(after.startsWith(":")) { "нет порта в $hostPort" }
            return host to after.substring(1)
        }
        val i = hostPort.lastIndexOf(':')
        require(i > 0) { "нет host:port в $hostPort" }
        return hostPort.substring(0, i) to hostPort.substring(i + 1)
    }
}
