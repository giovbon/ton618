package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
