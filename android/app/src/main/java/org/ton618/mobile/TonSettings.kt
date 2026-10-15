package org.ton618.mobile

import android.content.Context
import android.util.Base64
import androidx.core.content.edit

/**
 * Preferências locais do app (hoje: apenas a senha de acesso ao core).
 *
 * ⚠️ O core valida **somente a senha** — o username do token Basic Auth é
 * ignorado (`middleware.checkCredentials`, `core/internal/middleware/middleware.go`).
 * Por isso não existe campo de usuário na interface.
 *
 * A senha é usada em dois lugares:
 *  1. variável de ambiente `AUTH_PASS` do processo Go (protege também o acesso
 *     pelo PC via `http://<ip>:6180`);
 *  2. cookie `ton_auth` semeado no WebView, para entrar já autenticado.
 *
 * Fica em `SharedPreferences` privado do app (`/data/data/org.ton618.mobile/`),
 * que é o mesmo nível de proteção dos dados das notas.
 */
object TonSettings {

    private const val PREFS = "ton618_settings"
    private const val KEY_PASS = "auth_pass"

    /** Mesmo default de `config.Load()` no core. */
    const val DEFAULT_PASS = "ton618"

    /** Ignorado pelo core, mas o token Basic exige o formato "user:pass". */
    const val AUTH_USER = "admin"

    const val MIN_PASS_LENGTH = 4

    private fun prefs(context: Context) =
        context.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    fun password(context: Context): String =
        prefs(context).getString(KEY_PASS, DEFAULT_PASS) ?: DEFAULT_PASS

    fun setPassword(context: Context, value: String) {
        prefs(context).edit { putString(KEY_PASS, value) }
    }

    fun isDefaultPassword(context: Context): Boolean = password(context) == DEFAULT_PASS

    /**
     * Token do cookie `ton_auth`: base64("admin:senha").
     * `NO_WRAP` é obrigatório — quebra de linha quebraria o base64 esperado pelo core.
     */
    fun basicToken(context: Context): String = basicToken(password(context))

    fun basicToken(pass: String): String =
        Base64.encodeToString("$AUTH_USER:$pass".toByteArray(Charsets.UTF_8), Base64.NO_WRAP)

    /**
     * Valida a senha digitada. Retorna a mensagem de erro ou `null` se estiver ok.
     *
     * Não pode ser vazia: `config.Load()` cai no default "ton618" quando
     * `AUTH_PASS` é vazio — o app acharia que trocou a senha sem ter trocado.
     */
    fun validate(raw: String?): String? {
        val value = raw?.trim().orEmpty()
        return when {
            value.isEmpty() -> "Informe uma senha."
            value.length < MIN_PASS_LENGTH -> "Use ao menos $MIN_PASS_LENGTH caracteres."
            else -> null
        }
    }
}
