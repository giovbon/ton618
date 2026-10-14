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
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.io.File
import java.io.FileOutputStream

class TonServerService : Service() {

    private val serviceScope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    private var serverProcess: Process? = null

    companion object {
        const val TAG = "TonServerService"
        const val CHANNEL_ID = "ton618_service_channel"
        const val NOTIFICATION_ID = 6180
        const val DEFAULT_PORT = 6180

        const val ACTION_START = "org.ton618.mobile.action.START"
        const val ACTION_STOP = "org.ton618.mobile.action.STOP"

        @Volatile
        var isRunning: Boolean = false
            private set
    }

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> {
                stopSelf()
                return START_NOT_STICKY
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
        isRunning = false
    }

    private fun startServerProcess() {
        if (serverProcess != null && isRunning) return

        serviceScope.launch {
            try {
                val binFile = prepareBinary()
                if (binFile == null || !binFile.exists()) {
                    Log.e(TAG, "Binário TON-618 não encontrado nos assets.")
                    updateNotification("Erro: Binário não encontrado.")
                    return@launch
                }

                // Diretórios de trabalho do TON-618 no armazenamento interno do app
                val dataDir = File(filesDir, "data").apply { mkdirs() }
                val docsDir = File(filesDir, "docs").apply { mkdirs() }
                val dbPath = File(dataDir, "ton618.db").absolutePath

                val processBuilder = ProcessBuilder(binFile.absolutePath)
                processBuilder.directory(filesDir)

                val env = processBuilder.environment()
                env["PORT"] = DEFAULT_PORT.toString()
                env["DATA_DIR"] = dataDir.absolutePath
                env["DOCS_DIR"] = docsDir.absolutePath
                env["DB_PATH"] = dbPath
                env["EMBEDDING_ALL"] = "false"

                Log.i(TAG, "Iniciando processo Go: ${binFile.absolutePath} na porta $DEFAULT_PORT")
                serverProcess = processBuilder.start()
                isRunning = true

                // Leitor assíncrono de logs do stdout/stderr
                launch {
                    serverProcess?.inputStream?.bufferedReader()?.useLines { lines ->
                        lines.forEach { Log.d(TAG, "[Go] $it") }
                    }
                }
                launch {
                    serverProcess?.errorStream?.bufferedReader()?.useLines { lines ->
                        lines.forEach { Log.e(TAG, "[Go Err] $it") }
                    }
                }

                // Aguarda a porta abrir
                var tries = 0
                while (isActive && tries < 20) {
                    if (NetworkUtils.isServerPortOpen(DEFAULT_PORT)) {
                        val ip = NetworkUtils.getLocalIpAddress(this@TonServerService)
                        val url = "http://$ip:$DEFAULT_PORT"
                        Log.i(TAG, "Servidor TON-618 pronto em: $url")
                        updateNotification("Ativo em $url")
                        break
                    }
                    delay(300)
                    tries++
                }

                // Monitora o término do processo caso ele caia
                val exitCode = serverProcess?.waitFor() ?: -1
                Log.w(TAG, "Processo TON-618 encerrou com código $exitCode")
                isRunning = false
                updateNotification("Servidor parado (código $exitCode)")
            } catch (e: Exception) {
                Log.e(TAG, "Falha ao executar TON-618: ${e.message}", e)
                isRunning = false
                updateNotification("Erro ao rodar servidor: ${e.localizedMessage}")
            }
        }
    }

    private fun stopServerProcess() {
        try {
            serverProcess?.destroy()
            serverProcess = null
            isRunning = false
            Log.i(TAG, "Processo do servidor encerrado.")
        } catch (e: Exception) {
            Log.e(TAG, "Erro ao parar processo: ${e.message}")
        }
    }

    /**
     * Extrai o executável Go dos assets do APK para filesDir/bin/ton618 e aplica chmod +x.
     */
    private fun prepareBinary(): File? {
        val abi = Build.SUPPORTED_ABIS.firstOrNull() ?: "arm64-v8a"
        val assetName = if (abi.contains("x86")) "ton618-x86_64" else "ton618-arm64"

        val binDir = File(filesDir, "bin").apply { mkdirs() }
        val targetFile = File(binDir, "ton618")

        // Se o arquivo já existe e o APK não foi atualizado, pode reutilizar
        if (targetFile.exists() && targetFile.length() > 0) {
            targetFile.setExecutable(true)
            return targetFile
        }

        try {
            assets.open(assetName).use { input ->
                FileOutputStream(targetFile).use { output ->
                    input.copyTo(output)
                }
            }
            targetFile.setExecutable(true, false)
            Log.i(TAG, "Binário extraído com sucesso: ${targetFile.absolutePath} (${targetFile.length()} bytes)")
            return targetFile
        } catch (e: Exception) {
            Log.w(TAG, "Asset $assetName não encontrado ou falhou ao extrair: ${e.message}")
            return null
        }
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
