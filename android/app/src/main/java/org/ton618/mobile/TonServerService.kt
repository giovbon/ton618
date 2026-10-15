package org.ton618.mobile

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.io.File
import java.io.FileOutputStream

enum class ServerState {
    STOPPED,
    STARTING,
    RUNNING,
    ERROR
}

class TonServerService : Service() {

    private val serviceScope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    private var serverProcess: Process? = null

    /** true quando o encerramento partiu do app (Parar/Reiniciar), não do processo. */
    private var stopRequested = false

    /** Tentativas seguidas de subir o servidor de novo depois de uma queda. */
    private var restartAttempts = 0

    companion object {
        const val TAG = "TonServerService"
        const val CHANNEL_ID = "ton618_service_channel"
        const val NOTIFICATION_ID = 6180
        const val DEFAULT_PORT = 6180

        const val ACTION_START = "org.ton618.mobile.action.START"
        const val ACTION_STOP = "org.ton618.mobile.action.STOP"
        const val ACTION_RESTART = "org.ton618.mobile.action.RESTART"

        /** Quantas vezes seguidas o app tenta subir o servidor de novo após uma queda. */
        private const val MAX_AUTO_RESTARTS = 3

        // Credenciais do core. A senha vem de TonSettings (editável na
        // MainActivity); o username é ignorado pelo core (ver basicToken).

        /**
         * Assets sem os quais a interface abre mas não funciona (nenhum recurso
         * responde). Verificados após a extração para falhar de forma visível.
         */
        val REQUIRED_WEB_ASSETS = listOf(
            "app.css",
            "js/app.js",
            "js/vendor/htmx.min.js",
            "js/vendor/alpine.min.js",
            "fonts/fonts.css",
        )

        private val _serverState = MutableStateFlow(ServerState.STOPPED)
        val serverState = _serverState.asStateFlow()

        private val _logFlow = MutableSharedFlow<String>(replay = 50)
        val logFlow = _logFlow.asSharedFlow()

        val recentLogs = mutableListOf<String>()

        fun appendLog(msg: String) {
            synchronized(recentLogs) {
                if (recentLogs.size > 150) recentLogs.removeAt(0)
                recentLogs.add(msg)
            }
            _logFlow.tryEmit(msg)
        }
    }

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> {
                appendLog("[App] Parando servidor...")
                stopServerProcess()
                stopForeground(STOP_FOREGROUND_REMOVE)
                stopSelf()
                return START_NOT_STICKY
            }
            ACTION_RESTART -> {
                appendLog("[App] Reiniciando servidor...")
                // startForegroundService() exige um startForeground() em poucos
                // segundos — inclusive quando o serviço já está rodando (ex.:
                // troca de senha na tela inicial).
                startForeground(NOTIFICATION_ID, buildNotification("Reiniciando servidor TON-618..."))
                serviceScope.launch {
                    stopServerProcess()
                    delay(800)
                    startServerProcess()
                }
            }
            else -> {
                val notification = buildNotification("Iniciando servidor TON-618...")
                startForeground(NOTIFICATION_ID, notification)
                startServerProcess()
            }
        }
        return START_STICKY
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        super.onDestroy()
        stopServerProcess()
        serviceScope.cancel()
    }

    private fun startServerProcess() {
        if (serverProcess?.isAlive == true && _serverState.value == ServerState.RUNNING) return

        stopRequested = false
        _serverState.value = ServerState.STARTING
        appendLog("[App] Procurando executável nativo...")

        serviceScope.launch {
            try {
                val binFile = resolveBinary()
                if (binFile == null || !binFile.exists()) {
                    val err = "Erro: Binário não encontrado em nativeLibraryDir ou assets."
                    appendLog("❌ $err")
                    _serverState.value = ServerState.ERROR
                    updateNotification(err)
                    return@launch
                }

                appendLog("[App] Executável localizado: ${binFile.absolutePath}")

                // Garante estrutura de pastas em filesDir
                val dataDir = File(filesDir, "data").apply { mkdirs() }
                val docsDir = File(filesDir, "docs").apply { mkdirs() }
                val webDir = File(filesDir, "web").apply { mkdirs() }
                val staticDir = File(webDir, "static").apply { mkdirs() }
                val dbPath = File(dataDir, "ton618.db").absolutePath

                // Extrai assets web do APK (recursivo: js/, fonts/, ...)
                extractWebAssets(staticDir)

                val processBuilder = ProcessBuilder(binFile.absolutePath)
                processBuilder.directory(filesDir)

                val env = processBuilder.environment()
                env["PORT"] = DEFAULT_PORT.toString()
                env["DATA_DIR"] = dataDir.absolutePath
                env["STATE_DIR"] = dataDir.absolutePath
                env["DOCS_DIR"] = docsDir.absolutePath
                env["DB_PATH"] = dbPath
                env["WEB_DIR"] = webDir.absolutePath
                env["AUTH_USER"] = TonSettings.AUTH_USER
                env["AUTH_PASS"] = TonSettings.password(this@TonServerService)
                // O modelo de embeddings (~120MB) não é baixado no aparelho: o
                // WebView busca do CDN do HuggingFace, como o desktop já faz
                // enquanto o download não termina (allowRemoteModels).
                env["MODEL_DOWNLOAD"] = "false"
                appendLog("[App] Senha de acesso: ${authLabel()}")

                appendLog("[App] Iniciando processo Go na porta $DEFAULT_PORT...")
                serverProcess = processBuilder.start()

                // Leitor do stdout
                launch {
                    try {
                        serverProcess?.inputStream?.bufferedReader()?.useLines { lines ->
                            lines.forEach { line ->
                                Log.d(TAG, "[Go] $line")
                                appendLog("[Go] $line")
                            }
                        }
                    } catch (e: Exception) {
                        appendLog("[Go err read] ${e.message}")
                    }
                }

                // Leitor do stderr
                launch {
                    try {
                        serverProcess?.errorStream?.bufferedReader()?.useLines { lines ->
                            lines.forEach { line ->
                                Log.e(TAG, "[Go Err] $line")
                                appendLog("⚠️ [Go Err] $line")
                            }
                        }
                    } catch (e: Exception) {
                        appendLog("[Go err read] ${e.message}")
                    }
                }

                // Aguarda a porta abrir
                var tries = 0
                var portOpen = false
                while (isActive && tries < 25) {
                    if (NetworkUtils.isServerPortOpen(DEFAULT_PORT)) {
                        portOpen = true
                        break
                    }
                    delay(350)
                    tries++
                }

                if (portOpen) {
                    _serverState.value = ServerState.RUNNING
                    restartAttempts = 0 // servidor saudável: zera o contador de quedas
                    val ip = NetworkUtils.getLocalIpAddress(this@TonServerService)
                    val url = "http://$ip:$DEFAULT_PORT"
                    appendLog("✅ Servidor pronto e escutando em $url")
                    updateNotification("Ativo em $url")
                } else {
                    _serverState.value = ServerState.ERROR
                    appendLog("❌ Timeout: a porta $DEFAULT_PORT não respondeu.")
                    updateNotification("Falha ao abrir porta $DEFAULT_PORT")
                }

                // Monitora o encerramento do processo. Se ele cair sozinho, o app sobe
                // de novo: o caso comum é a restauração de backup, em que o core encerra
                // de propósito para reabrir o banco restaurado.
                val exitCode = serverProcess?.waitFor() ?: -1
                serverProcess = null
                appendLog("[App] Processo do servidor encerrou (código $exitCode)")

                if (stopRequested || !isActive) {
                    if (_serverState.value == ServerState.RUNNING) {
                        _serverState.value = ServerState.STOPPED
                    }
                    updateNotification("Servidor parado (código $exitCode)")
                    return@launch
                }

                if (restartAttempts >= MAX_AUTO_RESTARTS) {
                    _serverState.value = ServerState.STOPPED
                    appendLog("❌ O servidor encerrou $MAX_AUTO_RESTARTS vezes seguidas — use ▶ Iniciar.")
                    updateNotification("Servidor parado (código $exitCode)")
                    return@launch
                }

                restartAttempts++
                appendLog("[App] Subindo o servidor de novo ($restartAttempts/$MAX_AUTO_RESTARTS)...")
                updateNotification("Reiniciando servidor TON-618...")
                delay(1500)
                startServerProcess()
            } catch (e: Exception) {
                val err = "Falha ao iniciar processo: ${e.message}"
                Log.e(TAG, err, e)
                appendLog("❌ $err")
                _serverState.value = ServerState.ERROR
                updateNotification(err)
            }
        }
    }

    /**
     * Descrição da senha em uso para o painel de logs — nunca a senha em si.
     */
    private fun authLabel(): String =
        if (TonSettings.isDefaultPassword(this)) "padrão (ton618) — altere na tela inicial"
        else "personalizada"

    private fun stopServerProcess() {
        try {
            stopRequested = true
            serverProcess?.destroy()
            serverProcess = null
            _serverState.value = ServerState.STOPPED
            appendLog("[App] Servidor finalizado com sucesso.")
        } catch (e: Exception) {
            appendLog("⚠️ Erro ao encerrar processo: ${e.message}")
        }
    }

    /**
     * Resolve o binário nativo com prioridade para a pasta oficial jniLibs (nativeLibraryDir),
     * que tem permissão de execução nativa do SELinux no Android 10+.
     */
    private fun resolveBinary(): File? {
        // 1. Tenta a biblioteca nativa instalada pelo Android Package Manager
        val nativeDir = applicationInfo.nativeLibraryDir
        val soFiles = listOf("libton618.so", "ton618", "libserver.so")
        for (name in soFiles) {
            val f = File(nativeDir, name)
            if (f.exists() && f.canExecute()) {
                appendLog("[App] Usando binário nativo do sistema: ${f.name}")
                return f
            }
        }

        // 2. Fallback: extração a partir de assets
        val binDir = File(filesDir, "bin").apply { mkdirs() }
        val target = File(binDir, "ton618")

        val assetCandidates = listOf("libton618.so", "ton618-arm64", "ton618")
        for (candidate in assetCandidates) {
            try {
                assets.open(candidate).use { input ->
                    FileOutputStream(target).use { output ->
                        input.copyTo(output)
                    }
                }
                target.setExecutable(true, false)
                appendLog("[App] Extraído de assets/$candidate para ${target.absolutePath}")
                return target
            } catch (_: Exception) {}
        }

        return if (target.exists()) target else null
    }

    /**
     * Extrai os assets web do APK para o armazenamento interno, mantendo a
     * estrutura de pastas de `assets/web/static` (inclui js, fonts, models).
     *
     * ⚠️ A extração DEVE ser recursiva. O bundle web depende de subpastas
     * (`js/`, `js/vendor/`, `fonts/`, `models/`). Uma versão anterior copiava
     * apenas o primeiro nível de `assets/web/static`, então `htmx.min.js`,
     * `alpine.min.js` e `js/app.js` respondiam 404: a página abria (o HTML é
     * renderizado pelo servidor), mas nenhum recurso funcionava.
     *
     * A extração só se repete quando o APK muda (stamp = versionCode +
     * lastUpdateTime); nos boots seguintes é um no-op.
     */
    private fun extractWebAssets(staticDir: File) {
        val stampFile = File(filesDir, "web/.assets-stamp")
        val stamp = currentAppStamp()
        val complete = REQUIRED_WEB_ASSETS.all { File(staticDir, it).isFile }

        if (!complete || stampFile.readTextOrNull()?.trim() != stamp) {
            appendLog("[App] Extraindo assets web do APK (versão $stamp)...")
            try {
                staticDir.deleteRecursively()
                staticDir.mkdirs()
                val count = copyAssetTree("web/static", staticDir)
                stampFile.parentFile?.mkdirs()
                stampFile.writeText(stamp)
                appendLog("[App] Assets web extraídos: $count arquivo(s).")
            } catch (e: Exception) {
                appendLog("❌ Falha ao extrair assets web: ${e.message}")
            }
        } else {
            appendLog("[App] Assets web já extraídos (versão $stamp).")
        }

        val missing = REQUIRED_WEB_ASSETS.filterNot { File(staticDir, it).isFile }
        if (missing.isNotEmpty()) {
            appendLog("❌ Assets essenciais ausentes no APK: ${missing.joinToString(", ")}")
            appendLog("   → rode android/build-core-arm64.sh e gere o APK de novo.")
        }
    }

    /**
     * Copia recursivamente um diretório de assets — ou um único arquivo — para [dest].
     *
     * `assets.list()` devolve os filhos de um diretório e um array vazio para um
     * arquivo (ou pasta vazia), que é o caso tratado abaixo.
     */
    private fun copyAssetTree(assetPath: String, dest: File): Int {
        val children = try {
            assets.list(assetPath)
        } catch (_: Exception) {
            null
        }

        if (children.isNullOrEmpty()) {
            return try {
                dest.parentFile?.mkdirs()
                assets.open(assetPath).use { input ->
                    FileOutputStream(dest).use { output -> input.copyTo(output) }
                }
                1
            } catch (_: java.io.IOException) {
                0 // pasta vazia ou asset ilegível — não é fatal
            }
        }

        dest.mkdirs()
        var count = 0
        for (child in children) {
            count += copyAssetTree("$assetPath/$child", File(dest, child))
        }
        return count
    }

    private fun currentAppStamp(): String = try {
        val info = packageManager.getPackageInfo(packageName, 0)
        "${info.versionCode}-${info.lastUpdateTime}"
    } catch (_: Exception) {
        "0"
    }

    private fun File.readTextOrNull(): String? = try {
        if (isFile) readText() else null
    } catch (_: Exception) {
        null
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "Servidor TON-618",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "Mantém o servidor local do TON-618 ativo em segundo plano"
                setShowBadge(false)
            }
            val manager = getSystemService(NotificationManager::class.java)
            manager?.createNotificationChannel(channel)
        }
    }

    private fun buildNotification(statusText: String): Notification {
        val intent = Intent(this, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP
        }
        val pendingIntent = PendingIntent.getActivity(
            this,
            0,
            intent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("TON-618 PKM")
            .setContentText(statusText)
            .setSmallIcon(R.drawable.ic_launcher_ton618)
            .setOngoing(true)
            .setContentIntent(pendingIntent)
            .build()
    }

    private fun updateNotification(statusText: String) {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager
        manager?.notify(NOTIFICATION_ID, buildNotification(statusText))
    }
}
