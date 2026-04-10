package dev.tanq16.raikiri

import android.Manifest
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.os.Bundle
import android.view.View
import android.view.ViewGroup
import android.webkit.JavascriptInterface
import android.webkit.WebChromeClient
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Button
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.LinearLayout
import androidx.activity.OnBackPressedCallback
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.view.ViewCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.core.view.updatePadding

class MainActivity : androidx.activity.ComponentActivity() {
    private lateinit var webView: WebView
    private lateinit var setupContainer: LinearLayout
    private lateinit var fullscreenContainer: FrameLayout
    private lateinit var rootView: View
    private var fullscreenView: View? = null
    private var fullscreenCallback: WebChromeClient.CustomViewCallback? = null
    private val notificationPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) {}

    private val jsCommandReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            val command = intent?.getStringExtra("command") ?: return
            runOnUiThread { webView.evaluateJavascript(command, null) }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContentView(R.layout.activity_main)

        notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)

        registerReceiver(
            jsCommandReceiver,
            IntentFilter("dev.tanq16.raikiri.JS_COMMAND"),
            RECEIVER_NOT_EXPORTED
        )

        rootView = findViewById<ViewGroup>(android.R.id.content).getChildAt(0)
        ViewCompat.setOnApplyWindowInsetsListener(rootView) { view, insets ->
            if (fullscreenView == null) {
                val bars = insets.getInsets(WindowInsetsCompat.Type.systemBars() or WindowInsetsCompat.Type.displayCutout())
                view.updatePadding(top = bars.top, bottom = bars.bottom, left = bars.left, right = bars.right)
            } else {
                view.updatePadding(top = 0, bottom = 0, left = 0, right = 0)
            }
            insets
        }

        webView = findViewById(R.id.webview)
        setupContainer = findViewById(R.id.setup_container)
        fullscreenContainer = findViewById(R.id.fullscreen_container)

        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                if (fullscreenView != null) {
                    webView.webChromeClient?.onHideCustomView()
                } else if (webView.canGoBack()) {
                    webView.goBack()
                } else {
                    finish()
                }
            }
        })

        setupWebView()

        val prefs = getSharedPreferences("raikiri", MODE_PRIVATE)
        val savedUrl = prefs.getString("server_url", null)

        if (savedUrl != null) {
            startService()
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
        webView.webChromeClient = object : WebChromeClient() {
            override fun onShowCustomView(view: View?, callback: CustomViewCallback?) {
                if (fullscreenView != null) { callback?.onCustomViewHidden(); return }
                fullscreenView = view
                fullscreenCallback = callback
                fullscreenContainer.addView(view)
                fullscreenContainer.visibility = View.VISIBLE
                webView.visibility = View.GONE
                val controller = WindowCompat.getInsetsController(window, rootView)
                controller.hide(WindowInsetsCompat.Type.systemBars())
                controller.systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
                rootView.requestApplyInsets()
            }
            override fun onHideCustomView() {
                fullscreenContainer.removeView(fullscreenView)
                fullscreenContainer.visibility = View.GONE
                webView.visibility = View.VISIBLE
                fullscreenCallback?.onCustomViewHidden()
                fullscreenView = null
                fullscreenCallback = null
                val controller = WindowCompat.getInsetsController(window, rootView)
                controller.show(WindowInsetsCompat.Type.systemBars())
                rootView.requestApplyInsets()
            }
        }
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
            startService()
            showWebView(url)
        }
    }

    private fun showWebView(url: String) {
        setupContainer.visibility = View.GONE
        webView.visibility = View.VISIBLE
        webView.loadUrl(url)
    }

    private fun startService() {
        startService(Intent(this, PlaybackService::class.java))
    }

    inner class WebBridge {
        @JavascriptInterface
        fun updateMetadata(title: String, artist: String, album: String, thumbUrl: String) {
            runOnUiThread {
                PlaybackService.instance?.getPlayer()?.onJsMetadata(title, artist, album)
            }
        }

        @JavascriptInterface
        fun updatePlaybackState(isPlaying: Boolean) {
            runOnUiThread {
                val player = PlaybackService.instance?.getPlayer() ?: return@runOnUiThread
                if (isPlaying) player.onJsPlaying() else player.onJsPaused()
            }
        }

        @JavascriptInterface
        fun clearMedia() {
            runOnUiThread {
                PlaybackService.instance?.getPlayer()?.onJsStopped()
            }
        }

        @JavascriptInterface
        fun resetServer() {
            getSharedPreferences("raikiri", MODE_PRIVATE).edit()
                .remove("server_url").apply()
            runOnUiThread { showSetup() }
        }
    }

    override fun onDestroy() {
        unregisterReceiver(jsCommandReceiver)
        super.onDestroy()
    }
}
