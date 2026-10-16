package com.wcomsat.vpnclient

import org.json.JSONArray
import org.json.JSONObject

/**
 * Сборка конфига Xray-core (SOCKS-инбаунд + VLESS-аутбаунд) для канала.
 * Должна совпадать с shared/fixtures/expected/*.json и Go-реализацией.
 */
object ClientConfig {

    fun build(ch: Channel, socksPort: Int = 10808): String {
        val user = JSONObject()
            .put("id", ch.uuid)
            .put("encryption", "none")
        if (!ch.flow.isNullOrEmpty()) user.put("flow", ch.flow)

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
        if (ch.transport == "xhttp" && ch.xhttp != null) {
            stream.put(
                "xhttpSettings",
                JSONObject()
                    .put("host", ch.xhttp.host)
                    .put("path", ch.xhttp.path)
                    .put("mode", ch.xhttp.mode)
            )
        }

        val sniffing = JSONObject()
            .put("enabled", true)
            .put("destOverride", JSONArray(listOf("http", "tls", "quic")))

        val inbound = JSONObject()
            .put("listen", "127.0.0.1")
            .put("port", socksPort)
            .put("protocol", "socks")
            .put("settings", JSONObject().put("udp", true))
            .put("sniffing", sniffing)

        val vnext = JSONObject()
            .put("address", ch.address)
            .put("port", ch.port)
            .put("users", JSONArray(listOf(user)))

        val outbound = JSONObject()
            .put("protocol", "vless")
            .put("settings", JSONObject().put("vnext", JSONArray(listOf(vnext))))
            .put("streamSettings", stream)

        val outbounds = JSONArray(
            listOf(outbound, JSONObject().put("protocol", "freedom").put("tag", "direct"))
        )

        val root = JSONObject()
            .put("log", JSONObject().put("loglevel", "warning"))
            .put("inbounds", JSONArray(listOf(inbound)))
            .put("outbounds", outbounds)

        return root.toString(2)
    }
}
