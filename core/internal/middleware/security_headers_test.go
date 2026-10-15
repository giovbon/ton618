package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// cspDirective extrai o valor de uma diretiva do header Content-Security-Policy.
// Ex: cspDirective(csp, "connect-src") → "'self' https://cdn.jsdelivr.net …"
func cspDirective(csp, name string) string {
	for _, part := range strings.Split(csp, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, name+" ") {
			return strings.TrimSpace(strings.TrimPrefix(part, name+" "))
		}
	}
	return ""
}

func aplicarSecurityHeaders() *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec, req)
	return rec
}

func TestSecurityHeadersMiddleware_HeadersBasicos(t *testing.T) {
	rec := aplicarSecurityHeaders()

	esperados := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for header, valor := range esperados {
		if got := rec.Header().Get(header); got != valor {
			t.Errorf("%s = %q, esperado %q", header, got, valor)
		}
	}

	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("Content-Security-Policy ausente")
	}
}

// Guarda de regressão (bug real, 15/10/2026): o APK padrão do Android NÃO embarca
// `web/static/models/ort/` (22 MB omitidos por padrão). Sem esses arquivos, o Web
// Worker de embeddings carrega o runtime do ONNX Runtime do CDN do jsdelivr — o
// `.mjs` por import dinâmico (script-src) e o `.wasm` por fetch (connect-src).
// Faltando qualquer uma das duas origens, o fallback é bloqueado pelo browser e a
// busca semântica falha com "no available backend found".
func TestSecurityHeadersMiddleware_CSPLiberaCDNdoORT(t *testing.T) {
	rec := aplicarSecurityHeaders()
	csp := rec.Header().Get("Content-Security-Policy")

	const jsdelivr = "https://cdn.jsdelivr.net"

	if script := cspDirective(csp, "script-src"); !strings.Contains(script, jsdelivr) {
		t.Errorf("script-src deve liberar %s (import dinâmico do ort-wasm-simd-threaded.asyncify.mjs)\ngot: %q", jsdelivr, script)
	}

	if connect := cspDirective(csp, "connect-src"); !strings.Contains(connect, jsdelivr) {
		t.Errorf("connect-src deve liberar %s (fetch do ort-wasm-simd-threaded.asyncify.wasm)\ngot: %q", jsdelivr, connect)
	}
}

// O modelo de embeddings continua vindo do HuggingFace (CDN) quando não há cópia
// local — o mesmo caminho usado no Android (MODEL_DOWNLOAD=false).
func TestSecurityHeadersMiddleware_CSPLiberaHuggingFace(t *testing.T) {
	rec := aplicarSecurityHeaders()
	connect := cspDirective(rec.Header().Get("Content-Security-Policy"), "connect-src")

	for _, origem := range []string{"https://huggingface.co", "https://*.huggingface.co"} {
		if !strings.Contains(connect, origem) {
			t.Errorf("connect-src deve liberar %s, got: %q", origem, connect)
		}
	}
}
