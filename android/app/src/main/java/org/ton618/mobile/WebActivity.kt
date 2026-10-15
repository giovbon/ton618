package org.ton618.mobile

import android.annotation.SuppressLint
import android.content.Intent
import android.os.Bundle
import android.view.View
import android.webkit.CookieManager
import android.webkit.HttpAuthHandler
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Button
import android.widget.ProgressBar
import android.widget.TextView
import androidx.activity.OnBackPressedCallback
import androidx.appcompat.app.AppCompatActivity

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

        /** Loopback: mesma máquina, sem depender de Wi-Fi. */
        val LOCAL_URL = "http://127.0.0.1:${TonServerService.DEFAULT_PORT}"
    }

    private lateinit var webView: WebView
    private lateinit var progressBar: ProgressBar
    private lateinit var tvTitle: TextView

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

    override fun onDestroy() {
        webView.stopLoading()
        webView.destroy()
        super.onDestroy()
    }
}
