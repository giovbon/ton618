package middleware

import (
	"log/slog"

	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
)

// checkCredentials valida user:pass contra as credenciais configuradas.
func checkCredentials(r *http.Request, user, pass string) bool {
	// 1. Tenta Basic Auth header nativo (browser, fetch)
	u, p, ok := r.BasicAuth()
	if ok && u == user && p == pass {
		return true
	}

	// 2. Tenta Authorization header manual (sessionStorage JS)
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Basic ") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
		if err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 && parts[0] == user && parts[1] == pass {
				return true
			}
		}
	}

	// 3. Tenta cookie (para navegações nativas após login via JS)
	// O cookie pode conter base64 bruto, URL-encoded ou o formato legado com prefixo "Basic ".
	cookie, err := r.Cookie("ton_auth")
	if err == nil {
		val := cookie.Value
		if strings.Contains(val, "%") {
			if unescaped, unErr := url.QueryUnescape(val); unErr == nil {
				val = unescaped
			}
		}
		if strings.HasPrefix(val, "Basic ") {
			val = strings.TrimPrefix(val, "Basic ")
		}
		if decoded, decErr := base64.StdEncoding.DecodeString(val); decErr == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 && parts[0] == user && parts[1] == pass {
				return true
			}
		}
	}

	return false
}

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

		if checkCredentials(r, user, pass) {
			next.ServeHTTP(w, r)
			return
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

// LoggingMiddleware loga as requisicoes HTTP
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// SecurityHeadersMiddleware adiciona cabeçalhos de segurança HTTP globais
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}
