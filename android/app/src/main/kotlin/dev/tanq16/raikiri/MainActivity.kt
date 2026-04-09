package dev.tanq16.raikiri

import android.Manifest
import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.ServiceConnection
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.os.IBinder
import android.view.View
import android.webkit.JavascriptInterface
import android.webkit.WebChromeClient
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout

class MainActivity : android.app.Activity() {
    private lateinit var webView: WebView
    private lateinit var setupContainer: LinearLayout
    private var mediaService: MediaService? = null
    private var serviceBound = false

    private val jsCommandReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            val command = intent?.getStringExtra("command") ?: return
            executeJs(command)
        }
    }

    private val serviceConnection = object : ServiceConnection {
        override fun onServiceConnected(name: ComponentName?, binder: IBinder?) {
            mediaService = (binder as MediaService.LocalBinder).getService()
            serviceBound = true
        }
        override fun onServiceDisconnected(name: ComponentName?) {
            mediaService = null
            serviceBound = false
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 1)
            }
        }

        registerReceiver(
            jsCommandReceiver,
            IntentFilter("dev.tanq16.raikiri.JS_COMMAND"),
            RECEIVER_NOT_EXPORTED
        )

        webView = findViewById(R.id.webview)
        setupContainer = findViewById(R.id.setup_container)

        setupWebView()

        val prefs = getSharedPreferences("raikiri", MODE_PRIVATE)
        val savedUrl = prefs.getString("server_url", null)

        if (savedUrl != null) {
            startMediaService()
            showWebView(savedUrl)
        } else {
            showSetup()
        }
    }

    private fun setupWebView() {
        webView.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            mediaPlaybackRequiresUserGesture = false
            allowContentAccess = true
        }
        webView.webViewClient = WebViewClient()
        webView.webChromeClient = WebChromeClient()
        webView.addJavascriptInterface(WebBridge(), "Android")
    }

    private fun showSetup() {
        setupContainer.visibility = View.VISIBLE
        webView.visibility = View.GONE

        val urlInput = findViewById<EditText>(R.id.url_input)
        val connectBtn = findViewById<Button>(R.id.connect_btn)

        connectBtn.setOnClickListener {
            var url = urlInput.text.toString().trim()
            if (url.isEmpty()) return@setOnClickListener
            if (!url.startsWith("http://") && !url.startsWith("https://")) {
                url = "http://$url"
            }
            url = url.trimEnd('/')
            getSharedPreferences("raikiri", MODE_PRIVATE).edit()
                .putString("server_url", url).apply()
            startMediaService()
            showWebView(url)
        }
    }

    private fun showWebView(url: String) {
        setupContainer.visibility = View.GONE
        webView.visibility = View.VISIBLE
        webView.loadUrl(url)
    }

    private fun startMediaService() {
        val intent = Intent(this, MediaService::class.java)
        startForegroundService(intent)
        bindService(intent, serviceConnection, BIND_AUTO_CREATE)
    }

    fun executeJs(js: String) {
        runOnUiThread { webView.evaluateJavascript(js, null) }
    }

    inner class WebBridge {
        @JavascriptInterface
        fun updateMetadata(title: String, artist: String, album: String, thumbUrl: String) {
            mediaService?.updateMetadata(title, artist, album)
        }

        @JavascriptInterface
        fun updatePlaybackState(isPlaying: Boolean) {
            mediaService?.updatePlaybackState(isPlaying)
        }

        @JavascriptInterface
        fun resetServer() {
            getSharedPreferences("raikiri", MODE_PRIVATE).edit()
                .remove("server_url").apply()
            runOnUiThread { showSetup() }
        }
    }

    override fun onBackPressed() {
        if (webView.canGoBack()) webView.goBack()
        else super.onBackPressed()
    }

    override fun onDestroy() {
        unregisterReceiver(jsCommandReceiver)
        if (serviceBound) {
            unbindService(serviceConnection)
            serviceBound = false
        }
        super.onDestroy()
    }
}
