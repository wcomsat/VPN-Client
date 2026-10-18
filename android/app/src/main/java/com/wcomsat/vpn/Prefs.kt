package com.wcomsat.vpn

import android.content.Context

object Prefs {
    private const val NAME = "vpn"
    private const val KEY_SUB = "subscription"
    private const val KEY_CHANNEL = "channel"

    fun load(ctx: Context): String = sp(ctx).getString(KEY_SUB, "").orEmpty()

    fun save(ctx: Context, text: String) {
        sp(ctx).edit().putString(KEY_SUB, text.trim()).apply()
    }

    fun channel(ctx: Context): String =
        sp(ctx).getString(KEY_CHANNEL, "xhttp").orEmpty().ifEmpty { "xhttp" }

    fun saveChannel(ctx: Context, channel: String) {
        sp(ctx).edit().putString(KEY_CHANNEL, channel).apply()
    }

    private fun sp(ctx: Context) =
        ctx.getSharedPreferences(NAME, Context.MODE_PRIVATE)
}
