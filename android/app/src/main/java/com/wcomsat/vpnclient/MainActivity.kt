package com.wcomsat.vpnclient

import android.app.Activity
import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import android.widget.ArrayAdapter
import android.widget.Button
import android.widget.EditText
import android.widget.Spinner
import android.widget.TextView
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat

/**
 * Минимальный UI: вставить подписку, выбрать канал, подключить.
 */
class MainActivity : AppCompatActivity() {

    private lateinit var subscription: EditText
    private lateinit var channel: Spinner
    private lateinit var status: TextView

    private val prefs by lazy { getSharedPreferences("vpn", MODE_PRIVATE) }

    private val prepareVpn = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            startTunnel(TunnelVpnService.ACTION_CONNECT)
            status.text = "Подключение…"
        } else {
            status.text = "Разрешение на VPN не выдано"
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        subscription = findViewById(R.id.subscription)
        channel = findViewById(R.id.channel)
        status = findViewById(R.id.status)

        channel.adapter = ArrayAdapter(
            this,
            android.R.layout.simple_spinner_dropdown_item,
            listOf("xhttp (основной)", "vision (резерв)")
        )
        subscription.setText(prefs.getString("subscription", ""))
        channel.setSelection(prefs.getInt("channel", 0))

        findViewById<Button>(R.id.save).setOnClickListener {
            saveSubscription()
            status.text = "Сохранено"
        }

        findViewById<Button>(R.id.connect).setOnClickListener {
            saveSubscription()

            val parsed = runCatching { SubscriptionParser.parse(subscription.text.toString()) }
            if (parsed.isFailure) {
                status.text = "Подписка некорректна: ${parsed.exceptionOrNull()?.message}"
                return@setOnClickListener
            }
            status.text = "Каналов в подписке: ${parsed.getOrThrow().size}"

            val intent = VpnService.prepare(this)
            if (intent != null) {
                prepareVpn.launch(intent)
            } else {
                startTunnel(TunnelVpnService.ACTION_CONNECT)
                status.text = "Подключение…"
            }
        }
    }

    private fun saveSubscription() {
        prefs.edit()
            .putString("subscription", subscription.text.toString())
            .putInt("channel", channel.selectedItemPosition)
            .apply()
    }

    private fun startTunnel(action: String) {
        val intent = Intent(this, TunnelVpnService::class.java).setAction(action)
        ContextCompat.startForegroundService(this, intent)
    }
}
