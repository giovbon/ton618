package api

import (
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// rateEntry armazena o estado de rate limiting para um IP.
type rateEntry struct {
	count    int
	resetAt  time.Time
}

// RateLimiter implementa um token bucket simples baseado em IP.
// Sem dependências externas — usa apenas sync.Mutex e time.Ticker.
type RateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*rateEntry
	limit    int           // requests máximos por janela
	window   time.Duration // janela de tempo (ex: 1 minuto)
	stopCh   chan struct{}
}

// NewRateLimiter cria um novo rate limiter com a capacidade e janela especificadas.
// Exemplo: NewRateLimiter(60, time.Minute) = 60 requests/minuto por IP.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		entries: make(map[string]*rateEntry),
		limit:   limit,
		window:  window,
		stopCh:  make(chan struct{}),
	}
	// Limpeza periódica de entradas expiradas (a cada 2 janelas)
	go rl.cleanupLoop()
	return rl
}

// Stop interrompe a goroutine de limpeza.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// Allow verifica se uma requisição do IP é permitida.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, ok := rl.entries[ip]

	if !ok || now.After(entry.resetAt) {
		// Nova janela
		rl.entries[ip] = &rateEntry{count: 1, resetAt: now.Add(rl.window)}
		return true
	}

	if entry.count >= rl.limit {
		return false
	}

	entry.count++
	return true
}

// Middleware retorna um HTTP middleware que aplica rate limiting por IP.
// Se o limite for excedido, retorna 429 Too Many Requests.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !rl.Allow(ip) {
			slog.Warn("rate limit excedido", "ip", ip, "path", r.URL.Path)
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extractIP extrai o IP real considerando proxies reversos.
func extractIP(r *http.Request) string {
	// Confia em X-Forwarded-For e X-Real-IP se presentes (ex: reverse proxy)
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if ip := net.ParseIP(fwd); ip != nil {
			return ip.String()
		}
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		if ip := net.ParseIP(realIP); ip != nil {
			return ip.String()
		}
	}
	// Fallback: RemoteAddr (inclui porta, ex: "127.0.0.1:12345")
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// cleanupLoop remove entradas expiradas a cada 2 janelas de tempo.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.window * 2)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, entry := range rl.entries {
				if now.After(entry.resetAt) {
					delete(rl.entries, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}
