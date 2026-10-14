package org.ton618.mobile

import android.Manifest
import android.annotation.SuppressLint
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Color
import android.graphics.drawable.GradientDrawable
import android.os.Build
import android.os.Bundle
import android.view.View
import android.webkit.WebChromeClient
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Button
import android.widget.ProgressBar
import android.widget.TextView
import android.widget.Toast
import androidx.activity.OnBackPressedCallback
import androidx.appcompat.app.AppCompatActivity
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class MainActivity : AppCompatActivity() {

    private lateinit var webView: WebView
    private lateinit var loadingSpinner: ProgressBar
    private lateinit var tvStatusTitle: TextView
    private lateinit var tvServerUrl: TextView
    private lateinit var statusIndicator: View
    private lateinit var btnCopyUrl: Button
    private lateinit var btnReload: Button

    private var currentUrlToShare: String = "http://127.0.0.1:6180"

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        initViews()
        setupWebView()
        setupBackNavigation()
        requestNotificationPermission()

        // Inicia o ForegroundService do servidor TON-618
        startTonServerService()

        // Inicia o monitoramento de inicialização do servidor
        monitorServerReady()
    }

    private fun initViews() {
        webView = findViewById(R.id.webView)
        loadingSpinner = findViewById(R.id.loadingSpinner)
        tvStatusTitle = findViewById(R.id.tvStatusTitle)
        tvServerUrl = findViewById(R.id.tvServerUrl)
        statusIndicator = findViewById(R.id.statusIndicator)
        btnCopyUrl = findViewById(R.id.btnCopyUrl)
        btnReload = findViewById(R.id.btnReload)

        updateIndicatorColor("#f59e0b") // Âmbar (iniciando)

        btnCopyUrl.setOnClickListener {
            val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
            val clip = ClipData.newPlainText("TON-618 URL", currentUrlToShare)
            clipboard.setPrimaryClip(clip)
            Toast.makeText(this, R.string.url_copied, Toast.LENGTH_SHORT).show()
        }

        btnReload.setOnClickListener {
            webView.reload()
        }
    }

    @SuppressLint("SetJavaScriptEnabled")
    private fun setupWebView() {
        webView.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            databaseEnabled = true
            useWideViewPort = true
            loadWithOverviewMode = true
            cacheMode = WebSettings.LOAD_DEFAULT
            allowFileAccess = false
            allowContentAccess = false
        }

        webView.webViewClient = object : WebViewClient() {
            override fun onPageFinished(view: WebView?, url: String?) {
                super.onPageFinished(view, url)
                loadingSpinner.visibility = View.GONE
            }
        }

        webView.webChromeClient = WebChromeClient()
    }

    private fun setupBackNavigation() {
        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                if (webView.canGoBack()) {
                    webView.goBack()
                } else {
                    // Minimiza o app em vez de fechar, mantendo o servidor rodando
                    moveTaskToBack(true)
                }
            }
        })
    }

    private fun startTonServerService() {
        val intent = Intent(this, TonServerService::class.java).apply {
            action = TonServerService.ACTION_START
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }

    private fun monitorServerReady() {
        lifecycleScope.launch(Dispatchers.IO) {
            var ready = false
            var tries = 0
            while (isActive && tries < 30) {
                if (NetworkUtils.isServerPortOpen(TonServerService.DEFAULT_PORT)) {
                    ready = true
                    break
                }
                delay(400)
                tries++
            }

            withContext(Dispatchers.Main) {
                if (ready) {
                    val localIp = NetworkUtils.getLocalIpAddress(this@MainActivity)
                    currentUrlToShare = "http://$localIp:${TonServerService.DEFAULT_PORT}"

                    tvStatusTitle.text = getString(R.string.server_running)
                    tvServerUrl.text = currentUrlToShare
                    updateIndicatorColor("#10b981") // Verde

                    // Carrega localmente via 127.0.0.1 com latência zero
                    webView.loadUrl("http://127.0.0.1:${TonServerService.DEFAULT_PORT}")
                } else {
                    tvStatusTitle.text = getString(R.string.server_stopped)
                    tvServerUrl.text = "Falha ao iniciar na porta ${TonServerService.DEFAULT_PORT}"
                    updateIndicatorColor("#ef4444") // Vermelho
                    loadingSpinner.visibility = View.GONE
                }
            }
        }
    }

    private fun updateIndicatorColor(hexColor: String) {
        val shape = GradientDrawable().apply {
            shape = GradientDrawable.OVAL
            setColor(Color.parseColor(hexColor))
        }
        statusIndicator.background = shape
    }

    private fun requestNotificationPermission() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS)
                != PackageManager.PERMISSION_GRANTED
            ) {
                ActivityCompat.requestPermissions(
                    this,
                    arrayOf(Manifest.permission.POST_NOTIFICATIONS),
                    1001
                )
            }
        }
    }
}
