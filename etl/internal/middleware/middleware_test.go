package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogger(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logged := Logger(handler)
	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	logged.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Logger middleware should pass request, got status %d", rec.Code)
	}
}

func TestRecovery(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	recovered := Recovery(handler)
	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	recovered.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Recovery should return 500 on panic, got %d", rec.Code)
	}
}

func TestRecoveryNoPanic(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	recovered := Recovery(handler)
	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	recovered.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Recovery should pass request without panic, got status %d", rec.Code)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// Rate: 10 req/sec, burst: 5
	rl := RateLimitMiddleware(10, 5)(handler)

	// Should allow burst requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		rec := httptest.NewRecorder()
		rl.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Request %d should be allowed, got status %d", i+1, rec.Code)
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	rl.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Request after burst should be rate limited, got status %d", rec.Code)
	}
}

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remote   string
		expected string
	}{
		{"X-Forwarded-For", map[string]string{"X-Forwarded-For": "10.0.0.1, 10.0.0.2"}, "127.0.0.1:1234", "10.0.0.1"},
		{"X-Real-IP", map[string]string{"X-Real-IP": "10.0.0.3"}, "127.0.0.1:1234", "10.0.0.3"},
		{"RemoteAddr", map[string]string{}, "192.168.1.1:8080", "192.168.1.1"},
		{"IPv6 RemoteAddr", map[string]string{}, "[::1]:8080", "[::1]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tt.remote
			got := extractClientIP(req)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

	rw.WriteHeader(http.StatusTeapot)
	if rw.status != http.StatusTeapot {
		t.Errorf("expected status 418, got %d", rw.status)
	}
}
