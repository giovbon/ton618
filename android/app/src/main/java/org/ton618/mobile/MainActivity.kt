package org.ton618.mobile

import android.Manifest
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.view.View
import android.webkit.CookieManager
import android.webkit.WebStorage
import android.widget.Button
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import androidx.lifecycle.lifecycleScope
import com.google.android.material.textfield.TextInputEditText
import com.google.android.material.textfield.TextInputLayout
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class MainActivity : AppCompatActivity() {

    private lateinit var tvStatusText: TextView
    private lateinit var statusDot: View
    private lateinit var tvServerUrl: TextView
    private lateinit var btnCopyUrl: Button
    private lateinit var btnOpenWeb: Button
    private lateinit var btnStart: Button
    private lateinit var btnStop: Button
    private lateinit var btnRestart: Button
    private lateinit var tvLogs: TextView
    private lateinit var btnClearLogs: TextView
    private lateinit var logScrollView: ScrollView
    private lateinit var tilAuthPass: TextInputLayout
    private lateinit var etAuthPass: TextInputEditText
    private lateinit var tvAuthHint: TextView
    private lateinit var btnSaveAuth: Button

    private var currentUrl: String = "http://127.0.0.1:6180"

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        initViews()
        setupListeners()
        requestNotificationPermission()

        observeServerState()
        observeLogs()

        // Inicia o serviço do servidor no boot inicial da Activity
        sendServiceAction(TonServerService.ACTION_START)
    }

    private fun initViews() {
        tvStatusText = findViewById(R.id.tvStatusText)
        statusDot = findViewById(R.id.statusDot)
        tvServerUrl = findViewById(R.id.tvServerUrl)
        btnCopyUrl = findViewById(R.id.btnCopyUrl)
        btnOpenWeb = findViewById(R.id.btnOpenWeb)
        btnStart = findViewById(R.id.btnStart)
        btnStop = findViewById(R.id.btnStop)
        btnRestart = findViewById(R.id.btnRestart)
        tvLogs = findViewById(R.id.tvLogs)
        btnClearLogs = findViewById(R.id.btnClearLogs)
        logScrollView = findViewById(R.id.logScrollView)

        tilAuthPass = findViewById(R.id.tilAuthPass)
        etAuthPass = findViewById(R.id.etAuthPass)
        tvAuthHint = findViewById(R.id.tvAuthHint)
        btnSaveAuth = findViewById(R.id.btnSaveAuth)
        etAuthPass.setText(TonSettings.password(this))
        updateAuthHint()

        // Preenche com os logs recentes já existentes
        synchronized(TonServerService.recentLogs) {
            if (TonServerService.recentLogs.isNotEmpty()) {
                tvLogs.text = TonServerService.recentLogs.joinToString("\n")
            }
        }
    }

    private fun setupListeners() {
        btnCopyUrl.setOnClickListener {
            val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
            val clip = ClipData.newPlainText("TON-618 URL", currentUrl)
            clipboard.setPrimaryClip(clip)
            Toast.makeText(this, "URL copiada: $currentUrl", Toast.LENGTH_SHORT).show()
        }

        btnOpenWeb.setOnClickListener {
            // Loopback: a interface abre dentro do app (WebActivity), sem passar
            // pelo navegador externo nem pedir credenciais de novo.
            val intent = Intent(this, WebActivity::class.java).apply {
                putExtra(WebActivity.EXTRA_URL, WebActivity.LOCAL_URL)
            }
            startActivity(intent)
        }

        btnStart.setOnClickListener {
            sendServiceAction(TonServerService.ACTION_START)
        }

        btnStop.setOnClickListener {
            sendServiceAction(TonServerService.ACTION_STOP)
        }

        btnRestart.setOnClickListener {
            sendServiceAction(TonServerService.ACTION_RESTART)
        }

        btnClearLogs.setOnClickListener {
            synchronized(TonServerService.recentLogs) {
                TonServerService.recentLogs.clear()
            }
            tvLogs.text = "Logs limpos.\n"
        }

        btnSaveAuth.setOnClickListener { savePassword() }
    }

    /**
     * Persiste a senha, descarta as credenciais em cache do WebView e reinicia o
     * servidor, para o processo Go subir com o novo `AUTH_PASS`.
     */
    private fun savePassword() {
        val raw = etAuthPass.text?.toString()
        val error = TonSettings.validate(raw)
        if (error != null) {
            tilAuthPass.error = error
            return
        }

        val newPass = raw!!.trim()
        tilAuthPass.error = null
        val changed = newPass != TonSettings.password(this)

        TonSettings.setPassword(this, newPass)
        etAuthPass.setText(newPass)
        updateAuthHint()

        if (!changed) {
            Toast.makeText(this, "Senha mantida (sem alterações).", Toast.LENGTH_SHORT).show()
            return
        }

        // O cookie `ton_auth` (e o token que o app.js copia para o localStorage)
        // guardam a credencial antiga; sem essa limpeza a interface continuaria
        // enviando a senha vencida. O localStorage do TON-618 só guarda estado de
        // UI (linhas expandidas, colunas visíveis, última consulta), então
        // descartá-lo aqui não perde conteúdo de nota.
        CookieManager.getInstance().removeAllCookies(null)
        CookieManager.getInstance().flush()
        WebStorage.getInstance().deleteAllData()

        sendServiceAction(TonServerService.ACTION_RESTART)
        Toast.makeText(this, "Senha salva. Reiniciando o servidor...", Toast.LENGTH_SHORT).show()
    }

    private fun updateAuthHint() {
        if (TonSettings.isDefaultPassword(this)) {
            tvAuthHint.text =
                "⚠️ Senha padrão (ton618) em uso — qualquer aparelho na mesma rede Wi-Fi consegue acessar. Troque por uma sua."
            tvAuthHint.setTextColor(ContextCompat.getColor(this, R.color.accent_amber))
        } else {
            tvAuthHint.text =
                "Usada no app e no acesso pelo PC (http://IP:6180). Mínimo de ${TonSettings.MIN_PASS_LENGTH} caracteres."
            tvAuthHint.setTextColor(ContextCompat.getColor(this, R.color.text_secondary))
        }
    }

    private fun sendServiceAction(action: String) {
        val intent = Intent(this, TonServerService::class.java).apply {
            this.action = action
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }

    private fun observeServerState() {
        lifecycleScope.launch {
            TonServerService.serverState.collectLatest { state ->
                withContext(Dispatchers.Main) {
                    when (state) {
                        ServerState.RUNNING -> {
                            val ip = NetworkUtils.getLocalIpAddress(this@MainActivity)
                            currentUrl = "http://$ip:${TonServerService.DEFAULT_PORT}"
                            tvServerUrl.text = currentUrl
                            tvStatusText.text = "Servidor Ativo"
                            tvStatusText.setTextColor(ContextCompat.getColor(this@MainActivity, R.color.status_green))
                            statusDot.setBackgroundResource(R.drawable.bg_dot_green)

                            btnStart.isEnabled = false
                            btnStop.isEnabled = true
                            btnRestart.isEnabled = true
                            btnOpenWeb.isEnabled = true
                        }
                        ServerState.STARTING -> {
                            tvStatusText.text = "Iniciando servidor..."
                            tvStatusText.setTextColor(ContextCompat.getColor(this@MainActivity, R.color.accent_amber))
                            statusDot.setBackgroundResource(R.drawable.bg_dot_amber)

                            btnStart.isEnabled = false
                            btnStop.isEnabled = true
                            btnRestart.isEnabled = false
                            btnOpenWeb.isEnabled = false
                        }
                        ServerState.STOPPED -> {
                            tvStatusText.text = "Servidor Parado"
                            tvStatusText.setTextColor(ContextCompat.getColor(this@MainActivity, R.color.text_secondary))
                            statusDot.setBackgroundResource(R.drawable.bg_dot_amber)

                            btnStart.isEnabled = true
                            btnStop.isEnabled = false
                            btnRestart.isEnabled = false
                            btnOpenWeb.isEnabled = false
                        }
                        ServerState.ERROR -> {
                            tvStatusText.text = "Erro ao iniciar"
                            tvStatusText.setTextColor(ContextCompat.getColor(this@MainActivity, R.color.status_red))
                            statusDot.setBackgroundResource(R.drawable.bg_dot_red)

                            btnStart.isEnabled = true
                            btnStop.isEnabled = false
                            btnRestart.isEnabled = true
                            btnOpenWeb.isEnabled = false
                        }
                    }
                }
            }
        }
    }

    private fun observeLogs() {
        lifecycleScope.launch {
            TonServerService.logFlow.collect { line ->
                withContext(Dispatchers.Main) {
                    if (tvLogs.text.toString() == "Aguardando inicialização...") {
                        tvLogs.text = line + "\n"
                    } else {
                        tvLogs.append(line + "\n")
                    }
                    logScrollView.post {
                        logScrollView.fullScroll(View.FOCUS_DOWN)
                    }
                }
            }
        }
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
