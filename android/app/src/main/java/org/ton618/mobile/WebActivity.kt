package org.ton618.mobile

import android.Manifest
import android.annotation.SuppressLint
import android.app.Activity
import android.app.DownloadManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.Environment
import android.view.View
import android.webkit.CookieManager
import android.webkit.HttpAuthHandler
import android.webkit.URLUtil
import android.webkit.ValueCallback
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Button
import android.widget.ProgressBar
import android.widget.TextView
import android.widget.Toast
import androidx.activity.OnBackPressedCallback
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat

/**
 * Interface web do TON-618 dentro do app.
 *
 * O core roda em loopback (127.0.0.1:6180) e protege as rotas com autenticação
 * por senha. A sessão já começa autenticada porque o cookie `ton_auth` é semeado
 * com a senha configurada antes da primeira navegação (ver [loadAuthenticated]).
 */
class WebActivity : AppCompatActivity() {

    companion object {
        const val EXTRA_URL = "extra_url"

        private const val REQ_WRITE_STORAGE = 2001

        /** Loopback: mesma máquina, sem depender de Wi-Fi. */
        val LOCAL_URL = "http://127.0.0.1:${TonServerService.DEFAULT_PORT}"
    }

    private lateinit var webView: WebView
    private lateinit var progressBar: ProgressBar
    private lateinit var tvTitle: TextView

    /** Callback pendente do `<input type="file">` da página (import de backup). */
    private var pendingFileCallback: ValueCallback<Array<Uri>>? = null

    private val fileChooserLauncher =
        registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
            val callback = pendingFileCallback ?: return@registerForActivityResult
            pendingFileCallback = null
            if (result.resultCode != Activity.RESULT_OK) {
                callback.onReceiveValue(null)
                return@registerForActivityResult
            }
            callback.onReceiveValue(
                WebChromeClient.FileChooserParams.parseResult(result.resultCode, result.data)
            )
        }

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_web)

        webView = findViewById(R.id.webView)
        progressBar = findViewById(R.id.webProgress)
        tvTitle = findViewById(R.id.tvWebTitle)

        findViewById<Button>(R.id.btnWebBack).setOnClickListener { goBackOrFinish() }
        findViewById<Button>(R.id.btnWebReload).setOnClickListener { webView.reload() }

        webView.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            databaseEnabled = true
            loadWithOverviewMode = true
            useWideViewPort = true
            setSupportZoom(true)
            builtInZoomControls = true
            displayZoomControls = false
            mediaPlaybackRequiresUserGesture = false
            allowFileAccess = false
            allowContentAccess = false
            cacheMode = WebSettings.LOAD_DEFAULT
        }

        webView.webViewClient = object : WebViewClient() {
            // Só é chamado se o servidor enviar `WWW-Authenticate` (rotas /api/*).
            // O caminho normal é o cookie semeado em loadAuthenticated().
            override fun onReceivedHttpAuthRequest(
                view: WebView?,
                handler: HttpAuthHandler?,
                host: String?,
                realm: String?
            ) {
                handler?.proceed(TonSettings.AUTH_USER, TonSettings.password(this@WebActivity))
            }

            override fun shouldOverrideUrlLoading(
                view: WebView?,
                request: WebResourceRequest?
            ): Boolean {
                val url = request?.url ?: return false
                val host = url.host ?: return false
                // Navegação do próprio app segue no WebView; links externos vão
                // para o navegador do sistema.
                if (host == "127.0.0.1" || host == "localhost") return false
                return try {
                    startActivity(Intent(Intent.ACTION_VIEW, url))
                    true
                } catch (_: Exception) {
                    true
                }
            }
        }

        webView.webChromeClient = object : WebChromeClient() {
            override fun onProgressChanged(view: WebView?, newProgress: Int) {
                progressBar.progress = newProgress
                progressBar.visibility = if (newProgress in 1..99) View.VISIBLE else View.GONE
            }

            override fun onReceivedTitle(view: WebView?, title: String?) {
                if (!title.isNullOrBlank()) tvTitle.text = title
            }

            // Sem isto o `<input type="file">` da página (restaurar backup) não abre nada.
            override fun onShowFileChooser(
                webView: WebView?,
                filePathCallback: ValueCallback<Array<Uri>>?,
                fileChooserParams: FileChooserParams?
            ): Boolean {
                pendingFileCallback?.onReceiveValue(null) // descarta um callback pendente
                pendingFileCallback = filePathCallback
                val intent = fileChooserParams?.createIntent()
                if (intent == null) {
                    pendingFileCallback = null
                    return false
                }
                return try {
                    fileChooserLauncher.launch(intent)
                    true
                } catch (e: Exception) {
                    pendingFileCallback = null
                    false
                }
            }
        }

        // Downloads da página (ZIP de backup) não acontecem sem este listener:
        // o WebView abre um "download" que ninguém atende.
        webView.setDownloadListener { url, userAgent, contentDisposition, mimeType, _ ->
            startDownload(url, userAgent, contentDisposition, mimeType)
        }

        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                goBackOrFinish()
            }
        })

        loadAuthenticated(intent.getStringExtra(EXTRA_URL) ?: LOCAL_URL)
    }

    /**
     * Semeia o cookie `ton_auth` com a senha configurada e só então navega.
     *
     * O core NÃO responde 401 à navegação de página: ele redireciona para
     * `/login`. Sem o cookie, o usuário cairia na tela de login dentro do app.
     * Com o cookie (o core ignora o username, valida só a senha), a sessão já
     * começa autenticada — e o `app.js` promove o cookie para `localStorage`,
     * o que injeta o header `Authorization` nas requisições HTMX/fetch.
     */
    private fun loadAuthenticated(url: String) {
        val cookies = CookieManager.getInstance()
        cookies.setAcceptCookie(true)
        cookies.setCookie(url, "ton_auth=${TonSettings.basicToken(this)}; path=/", null)
        cookies.flush()
        webView.loadUrl(url)
    }

    private fun goBackOrFinish() {
        if (webView.canGoBack()) webView.goBack() else finish()
    }

    /**
     * Entrega o download da página ao DownloadManager.
     *
     * O DownloadManager não compartilha a sessão do WebView, então o cookie de
     * autenticação (`ton_auth`) vai no header — sem ele o servidor responde 302
     * para /login e o "backup" baixado seria a página de login.
     */
    private fun startDownload(
        url: String,
        userAgent: String?,
        contentDisposition: String?,
        mimeType: String?
    ) {
        if (!url.startsWith("http")) {
            // blob:/data: não passam pelo DownloadManager.
            Toast.makeText(this, "Download não suportado neste formato.", Toast.LENGTH_LONG).show()
            return
        }
        // Em Android <= 28 gravar em Downloads exige WRITE_EXTERNAL_STORAGE.
        if (Build.VERSION.SDK_INT <= Build.VERSION_CODES.P &&
            ContextCompat.checkSelfPermission(this, Manifest.permission.WRITE_EXTERNAL_STORAGE) !=
            PackageManager.PERMISSION_GRANTED
        ) {
            requestPermissions(arrayOf(Manifest.permission.WRITE_EXTERNAL_STORAGE), REQ_WRITE_STORAGE)
            Toast.makeText(this, "Conceda a permissão e toque em baixar de novo.", Toast.LENGTH_LONG).show()
            return
        }

        try {
            val fileName = URLUtil.guessFileName(url, contentDisposition, mimeType)
            val request = DownloadManager.Request(Uri.parse(url))
                .setTitle(fileName)
                .setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
                .setDestinationInExternalPublicDir(Environment.DIRECTORY_DOWNLOADS, fileName)

            userAgent?.let { request.addRequestHeader("User-Agent", it) }
            CookieManager.getInstance().getCookie(url)?.let { request.addRequestHeader("Cookie", it) }
            mimeType?.let { request.setMimeType(it) }

            val manager = getSystemService(Context.DOWNLOAD_SERVICE) as DownloadManager
            manager.enqueue(request)
            Toast.makeText(this, "Baixando $fileName (veja em Downloads)...", Toast.LENGTH_LONG).show()
        } catch (e: Exception) {
            Toast.makeText(this, "Falha ao baixar: ${e.message}", Toast.LENGTH_LONG).show()
        }
    }

    override fun onDestroy() {
        webView.stopLoading()
        webView.destroy()
        super.onDestroy()
    }
}
