package api

import (
	"encoding/base64"
	"net/http"
	"strings"
	"log/slog"
)

// BasicAuthMiddleware protege rotas de API e Documentos com autenticação básica.
func BasicAuthMiddleware(next http.Handler, username, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Permitir acesso público ao Frontend, avatares e pastas de mídia (para renderização em notas)
		nameParam := r.URL.Query().Get("name")
		isPublicMedia := strings.HasPrefix(nameParam, "assets/") ||
			strings.HasPrefix(nameParam, "attachments/") ||
			strings.HasPrefix(nameParam, "pdfs/")

		isPublic := (!strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/docs/")) ||
			r.URL.Path == "/api/tags/avatar" ||
			(r.URL.Path == "/api/file" && isPublicMedia)

		if isPublic {
			next.ServeHTTP(w, r)
			return
		}

		// 1. Tentar via Header padrão
		user, pass, ok := r.BasicAuth()
		slog.Debug("auth check", "path", r.URL.Path, "user", user, "ok", ok, "expected_user", username)
		if ok && user == username && pass == password {
			next.ServeHTTP(w, r)
			return
		}

		// 2. Tentar via query parameter "token" (para window.open de PDFs/Imagens)
		token := r.URL.Query().Get("token")
		if token != "" {
			if strings.HasPrefix(token, "Basic ") {
				token = strings.TrimPrefix(token, "Basic ")
			}
			expected := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
			if token == expected {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="TON-618 Knowledge Singularity"`)
		http.Error(w, "Acesso não autorizado. Por favor, faça login.", http.StatusUnauthorized)
	})
}
