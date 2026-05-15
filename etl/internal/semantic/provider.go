package semantic

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"etl/internal/models"
)

// ── EmbeddingProvider interface ──────────────────────────────────────

// EmbeddingProvider abstrai o backend de embeddings (Ollama, OpenAI, Gemini, etc.)
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) (map[string][]float32, error)
	Dimensions() int
}

// ── Factory ───────────────────────────────────────────────────────────

// NewEmbeddingProvider cria o provider baseado na configuracao.
// Prioridade: AppSettings.EmbeddingProvider; fallback para Ollama.
func NewEmbeddingProvider(cfg *models.AppSettings, ollamaHost, ollamaModel string) EmbeddingProvider {
	provider := strings.ToLower(cfg.EmbeddingProvider)
	key := cfg.EmbeddingAPIKey
	model := cfg.EmbeddingModel
	baseURL := cfg.EmbeddingBaseURL
	dim := cfg.EmbeddingDimension
	if dim <= 0 {
		dim = 512
	}

	switch provider {
	case "gemini":
		if key == "" {
			slog.Warn("[Embedding] Gemini configurado mas sem API key, fallback para Ollama")
			return NewOllamaProvider(ollamaHost, ollamaModel, dim)
		}
		if model == "" {
			model = "text-embedding-004"
		}
		slog.Info("[Embedding] Usando Gemini", "model", model)
		return NewGeminiProvider(key, model, dim)

	case "openai":
		if key == "" {
			slog.Warn("[Embedding] OpenAI configurado mas sem API key, fallback para Ollama")
			return NewOllamaProvider(ollamaHost, ollamaModel, dim)
		}
		if model == "" {
			model = "text-embedding-3-small"
		}
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		slog.Info("[Embedding] Usando OpenAI", "model", model, "baseURL", baseURL)
		return NewOpenAIProvider(baseURL, model, key, dim)

	case "custom":
		if baseURL == "" {
			slog.Warn("[Embedding] Custom provider sem baseURL, fallback para Ollama")
			return NewOllamaProvider(ollamaHost, ollamaModel, dim)
		}
		if model == "" {
			model = "text-embedding-3-small"
		}
		slog.Info("[Embedding] Usando Custom provider", "model", model, "baseURL", baseURL)
		return NewOpenAIProvider(baseURL, model, key, dim)

	case "ollama", "":
		fallthrough
	default:
		if ollamaHost == "" {
			ollamaHost = "http://localhost:11434"
		}
		if ollamaModel == "" {
			ollamaModel = "nomic-embed-text"
		}
		slog.Info("[Embedding] Usando Ollama", "model", ollamaModel, "host", ollamaHost)
		return NewOllamaProvider(ollamaHost, ollamaModel, dim)
	}
}

// ── Backward-compat wrappers ──────────────────────────────────────────

// NewOllamaEmbedding é wrapper backward-compat. Deprecated: use NewEmbeddingProvider.
func NewOllamaEmbedding(model, host string, dimension int) func(ctx context.Context, text string) ([]float32, error) {
	p := NewOllamaProvider(host, model, dimension)
	return p.Embed
}

// EmbedBatch é wrapper backward-compat. Deprecated: use EmbeddingProvider.EmbedBatch.
func EmbedBatch(ctx context.Context, host, model string, texts []string, dimension int) (map[string][]float32, error) {
	p := NewOllamaProvider(host, model, dimension)
	return p.EmbedBatch(ctx, texts)
}

// ── Circuit Breaker ───────────────────────────────────────────────────

type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

type circuitBreaker struct {
	mu          sync.Mutex
	state       circuitState
	failures    int
	lastFailure time.Time
	threshold   int
	openTimeout time.Duration
}

func newCircuitBreaker() *circuitBreaker {
	return &circuitBreaker{
		state:       circuitClosed,
		threshold:   3,
		openTimeout: 30 * time.Second,
	}
}

func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitClosed:
		return true
	case circuitOpen:
		if time.Since(cb.lastFailure) > cb.openTimeout {
			cb.state = circuitHalfOpen
			slog.Info("[Semantic] Circuit breaker half-open, testando...")
			return true
		}
		return false
	case circuitHalfOpen:
		return true
	default:
		return true
	}
}

func (cb *circuitBreaker) success() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == circuitHalfOpen {
		slog.Info("[Semantic] Circuit breaker fechado, servico recuperado")
	}
	cb.state = circuitClosed
	cb.failures = 0
}

func (cb *circuitBreaker) failure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.threshold {
		cb.state = circuitOpen
		slog.Warn(fmt.Sprintf("[Semantic] Circuit breaker aberto! %d falhas consecutivas. Tentando em %v",
			cb.failures, cb.openTimeout))
	}
}

func (cb *circuitBreaker) isHalfOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state == circuitHalfOpen
}

// ── Global State ──────────────────────────────────────────────────────

var (
	queryCache      = make(map[string][]float32)
	queryCacheKeys  []string
	queryCacheMu    sync.RWMutex
	queryCacheLimit = 200
	cacheFilePath   string

	ollamaSem = make(chan struct{}, 2)

	globalCB = newCircuitBreaker()

	ollamaActive  int32
	ollamaWaiting int32
)

type Metrics struct {
	Active  int32 `json:"active"`
	Waiting int32 `json:"waiting"`
	Limit   int   `json:"limit"`
}

func GetMetrics() Metrics {
	return Metrics{
		Active:  atomic.LoadInt32(&ollamaActive),
		Waiting: atomic.LoadInt32(&ollamaWaiting),
		Limit:   cap(ollamaSem),
	}
}

func Ping(host string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/", strings.TrimSuffix(host, "/"))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("returned status %d", resp.StatusCode)
	}
	return nil
}

func InitCache(path string) {
	cacheFilePath = path
	loadQueryCache()
}

func loadQueryCache() {
	if cacheFilePath == "" {
		return
	}
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return
	}
	queryCacheMu.Lock()
	defer queryCacheMu.Unlock()
	var loaded struct {
		Cache map[string][]float32 `json:"cache"`
		Keys  []string             `json:"keys"`
	}
	if err := json.Unmarshal(data, &loaded); err == nil {
		queryCache = loaded.Cache
		queryCacheKeys = loaded.Keys
		log.Printf("[Semantic] Cache persistente carregado: %d entradas\n", len(queryCache))
	}
}

func SaveCache() {
	queryCacheMu.RLock()
	defer queryCacheMu.RUnlock()
	if len(queryCache) == 0 {
		return
	}
	data, _ := json.Marshal(map[string]interface{}{
		"cache": queryCache,
		"keys":  queryCacheKeys,
	})
	os.WriteFile(cacheFilePath, data, 0644)
}

// ── Shared Utilities ──────────────────────────────────────────────────

func chunkByChars(s string, size int) []string {
	var chunks []string
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}

func normalizeVector(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	norm := float32(math.Sqrt(sum))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
}
