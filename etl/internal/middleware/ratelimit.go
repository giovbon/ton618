package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter gerencia token buckets por IP com limpeza periódica.
type RateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rateLimiterEntry
	rate     rate.Limit
	burst    int
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter cria um rate limiter com taxa sustentada e burst.
// rps: requisições por segundo sustentadas
// burst: rajada máxima permitida
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		rate:     rate.Limit(rps),
		burst:    burst,
	}
	go rl.cleanupLoop()
	return rl
}

// Allow verifica se o IP pode fazer a requisição.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	entry, exists := rl.limiters[ip]
	if !exists {
		entry = &rateLimiterEntry{
			limiter:  rate.NewLimiter(rl.rate, rl.burst),
			lastSeen: time.Now(),
		}
		rl.limiters[ip] = entry
	}
	entry.lastSeen = time.Now()
	rl.mu.Unlock()

	return entry.limiter.Allow()
}

// cleanupLoop remove IPs que não são vistos há mais de 15 minutos.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-15 * time.Minute)
		for ip, entry := range rl.limiters {
			if entry.lastSeen.Before(cutoff) {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitMiddleware retorna um middleware HTTP que aplica rate limiting por IP.
func RateLimitMiddleware(rps float64, burst int) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(rps, burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r)
			if !limiter.Allow(ip) {
				w.Header().Set("X-RateLimit-Limit", "30")
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", "1")
				w.Header().Set("Retry-After", "1")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded","retry_after_seconds":1}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractClientIP extrai o IP real do cliente, considerando proxies.
func extractClientIP(r *http.Request) string {
	// Verificar X-Forwarded-For (proxies/reverse proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	// Verificar X-Real-IP (nginx/Caddy)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fallback: RemoteAddr — remover porta
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
