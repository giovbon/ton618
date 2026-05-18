package api

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

// BasicAuthMiddleware retorna um middleware HTTP Basic Auth.
// Se user e pass forem vazios, permite acesso sem autenticação.
func BasicAuthMiddleware(next http.Handler, user, pass string) http.Handler {
	if user == "" && pass == "" {
		return next
	}

	expectedAuth := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check and static assets
		if r.URL.Path == "/api/health" || strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
			w.Header().Set("WWW-Authenticate", `Basic realm="TON-618", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		providedAuth := strings.TrimPrefix(authHeader, "Basic ")
		if subtle.ConstantTimeCompare([]byte(providedAuth), []byte(expectedAuth)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="TON-618", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Recovery middleware captura panics e retorna 500 em vez de crashar o servidor.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
