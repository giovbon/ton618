package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	defer rl.Stop()

	// 3 requests devem ser permitidos
	for i := 0; i < 3; i++ {
		if !rl.Allow("192.168.1.1") {
			t.Fatalf("request %d deveria ser permitido", i+1)
		}
	}

	// O 4o deve ser negado
	if rl.Allow("192.168.1.1") {
		t.Error("4o request deveria ser negado")
	}
}

func TestRateLimiter_IPsDiferentes(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	defer rl.Stop()

	if !rl.Allow("ip1") {
		t.Error("ip1 request 1 deveria ser permitido")
	}
	if !rl.Allow("ip2") {
		t.Error("ip2 request 1 deveria ser permitido")
	}
	if !rl.Allow("ip1") {
		t.Error("ip1 request 2 deveria ser permitido")
	}
	if rl.Allow("ip1") {
		t.Error("ip1 request 3 deveria ser negado")
	}
	if !rl.Allow("ip2") {
		t.Error("ip2 request 2 deveria ser permitido")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(1, 50*time.Millisecond)
	defer rl.Stop()

	if !rl.Allow("ip") {
		t.Error("request 1 deveria ser permitido")
	}
	if rl.Allow("ip") {
		t.Error("request 2 deveria ser negado (janela ativa)")
	}

	// Aguarda reset
	time.Sleep(60 * time.Millisecond)

	if !rl.Allow("ip") {
		t.Error("request 3 deveria ser permitido (janela resetada)")
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	defer rl.Stop()

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Primeiro request — deve passar
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "10.0.0.1:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("esperado 200, got %d", rec1.Code)
	}

	// Segundo request — deve ser 429
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.1:12345"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("esperado 429, got %d", rec2.Code)
	}
}

func TestExtractIP_Fallback(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:54321"
	ip := extractIP(r)
	if ip != "10.0.0.1" {
		t.Errorf("esperado 10.0.0.1, got %q", ip)
	}
}

func TestExtractIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.1")
	r.RemoteAddr = "10.0.0.1:54321"
	ip := extractIP(r)
	if ip != "203.0.113.1" {
		t.Errorf("esperado 203.0.113.1, got %q", ip)
	}
}
