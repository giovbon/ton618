package api

import (
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public paths: no auth required
		if r.URL.Path == "/api/health" || r.URL.Path == "/login" || strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		// Check Basic Auth
		u, p, ok := r.BasicAuth()
		if ok && u == user && p == pass {
			next.ServeHTTP(w, r)
			return
		}

		// Check for auth header from JS (sessionStorage)
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Basic ") {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
			if err == nil {
				parts := strings.SplitN(string(decoded), ":", 2)
				if len(parts) == 2 && parts[0] == user && parts[1] == pass {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		// Not authenticated for HTML page → redirect to login
		// Not authenticated for API → return 401
		if strings.HasPrefix(r.URL.Path, "/api/") || r.Method != "GET" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// For page requests, redirect to login
		http.Redirect(w, r, "/login", http.StatusFound)
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
