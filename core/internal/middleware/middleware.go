package middleware

import (
	"log/slog"

	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
)

// checkCredentials valida apenas a senha contra as credenciais configuradas.
// O usuário no token Basic Auth é ignorado — o login é feito exclusivamente por senha.
func checkCredentials(r *http.Request, pass string) bool {
	if pass == "" {
		return false
	}

	// extrai a senha de uma string "user:pass" decodificada de base64
	extractPass := func(decoded string) string {
		parts := strings.SplitN(decoded, ":", 2)
		if len(parts) == 2 {
			return parts[1]
		}
		return ""
	}

	// 1. Tenta Basic Auth header nativo (browser, fetch)
	_, p, ok := r.BasicAuth()
	if ok && p == pass {
		return true
	}

	// 2. Tenta Authorization header manual (localStorage JS)
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Basic ") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
		if err == nil && extractPass(string(decoded)) == pass {
			return true
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
			if extractPass(string(decoded)) == pass {
				return true
			}
		}
	}

	return false
}

// BasicAuthMiddleware retorna um middleware de autenticação por senha.
// Se pass for vazio (e user vazio), permite acesso sem autenticação.
// O username no token Basic Auth é ignorado — apenas a senha é validada.
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

		if checkCredentials(r, pass) {
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
				// Adicionado https://static.cloudflareinsights.com para liberar o script do Cloudflare Beacon
				// Adicionado blob: para suportar Web Workers criados via blob URL (Transformers.js/ONNX)
				"script-src 'self' 'unsafe-inline' 'unsafe-eval' blob: https://cdn.jsdelivr.net https://static.cloudflareinsights.com; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdn.jsdelivr.net; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data: blob: http: https: https://*.tile.openstreetmap.org https://server.arcgisonline.com; "+
				// worker-src necessário para Web Workers (Transformers.js/ONNX) no HTTPS
				"worker-src 'self' blob:; "+
				// Permitir HuggingFace API, CDNs de modelos (AWS, LFS, XetHub) e serviços de mapa
				"connect-src 'self' https://nominatim.openstreetmap.org https://router.project-osrm.org https://huggingface.co https://*.huggingface.co https://*.xethub.hf.co https://*.hf.co https://*.cdn.hf.co https://cdn-lfs.huggingface.co")
		next.ServeHTTP(w, r)
	})
}
