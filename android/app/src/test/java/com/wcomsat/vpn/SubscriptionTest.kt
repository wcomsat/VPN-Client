package com.wcomsat.vpn

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.json.JSONObject

class SubscriptionTest {

    private val sample = """
        vless://11111111-1111-4111-8111-111111111111@203.0.113.45:443?encryption=none&security=reality&sni=www.github.com&fp=firefox&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA&sid=6ef4f969d11e9751&type=xhttp&path=/api/v1/updates&mode=stream-one#ivan-phone-xhttp
        vless://11111111-1111-4111-8111-111111111111@203.0.113.45:8443?encryption=none&security=reality&sni=www.github.com&fp=firefox&pbk=BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB&sid=454435ab6d6ab48e&type=tcp&flow=xtls-rprx-vision#ivan-phone-vision
    """.trimIndent()

    @Test
    fun parsesBothChannels() {
        val channels = Subscription.parse(sample)
        assertEquals(2, channels.size)

        val xhttp = Subscription.pick(channels, "xhttp")
        assertEquals("xhttp", xhttp.transport)
        assertEquals(443, xhttp.port)
        assertEquals("/api/v1/updates", xhttp.xhttp?.path)
        assertEquals("www.github.com", xhttp.reality.serverName)

        val vision = Subscription.pick(channels, "vision")
        assertEquals("tcp", vision.transport)
        assertEquals(8443, vision.port)
        assertEquals("xtls-rprx-vision", vision.flow)
    }

    @Test
    fun buildsExpectedSocksAndOutbound() {
        val ch = Subscription.pick(Subscription.parse(sample), "xhttp")
        val cfg = Subscription.buildXrayConfig(ch)

        val inbounds = cfg.getJSONArray("inbounds")
        assertEquals(10808, inbounds.getJSONObject(0).getInt("port"))
        assertEquals(true, inbounds.getJSONObject(0).getJSONObject("settings").getBoolean("udp"))

        val out = cfg.getJSONArray("outbounds").getJSONObject(0)
        assertEquals("vless", out.getString("protocol"))
        val vnext = out.getJSONObject("settings").getJSONArray("vnext").getJSONObject(0)
        assertEquals("203.0.113.45", vnext.getString("address"))
        assertEquals(443, vnext.getInt("port"))

        val reality = out.getJSONObject("streamSettings").getJSONObject("realitySettings")
        assertEquals("www.github.com", reality.getString("serverName"))
        assertEquals("6ef4f969d11e9751", reality.getString("shortId"))
    }

    @Test
    fun rejectsUnknownTransport() {
        val bad = "vless://id@example.com:443?type=ws#x"
        val error = runCatching { Subscription.parse(bad) }.exceptionOrNull()
        assertTrue(error?.message?.contains("транспорт") == true)
    }

    @Test
    fun visionFlowIsWrittenToUser() {
        val ch = Subscription.pick(Subscription.parse(sample), "vision")
        val cfg = Subscription.buildXrayConfig(ch)
        val user = cfg.getJSONArray("outbounds")
            .getJSONObject(0)
            .getJSONObject("settings")
            .getJSONArray("vnext")
            .getJSONObject(0)
            .getJSONArray("users")
            .getJSONObject(0)
        assertEquals("xtls-rprx-vision", user.getString("flow"))
        assertEquals("tcp", cfg.getJSONArray("outbounds").getJSONObject(0)
            .getJSONObject("streamSettings").getString("network"))
    }

    @Test
    fun jsonRoundTrip() {
        val ch = Subscription.pick(Subscription.parse(sample), "xhttp")
        val cfg = Subscription.buildXrayConfig(ch)
        val parsed = JSONObject(cfg.toString())
        assertEquals(2, parsed.getJSONArray("inbounds").length())
        assertEquals(2, parsed.getJSONArray("outbounds").length())
    }
}
