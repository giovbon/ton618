package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Circuit breaker states
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
			slog.Info("[Semantic] Circuit breaker half-open, testando Ollama...")
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
		slog.Info("[Semantic] Circuit breaker fechado, Ollama recuperado")
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
		slog.Warn(fmt.Sprintf("[Semantic] Circuit breaker aberto! Ollama falhou %d vezes consecutivas. Tentando novamente em %v",
			cb.failures, cb.openTimeout))
	}
}

func (cb *circuitBreaker) isHalfOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state == circuitHalfOpen
}

var (
	// Cache de embeddings: mapa + fila de chaves para evicção ordenada (FIFO)
	queryCache      = make(map[string][]float32)
	queryCacheKeys  []string
	queryCacheMu    sync.RWMutex
	queryCacheLimit = 200
	cacheFilePath   string

	// Semáforo para controlar concorrência no Ollama
	ollamaSem = make(chan struct{}, 2)

	// Métricas do semáforo
	ollamaActive  int32
	ollamaWaiting int32
)

// Metrics contém estatísticas de uso do motor semântico
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/", host)
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
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
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

func NewOllamaEmbedding(model, host string) func(ctx context.Context, text string) ([]float32, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	cb := newCircuitBreaker()

	return func(ctx context.Context, text string) ([]float32, error) {
		cleanText := strings.TrimSpace(text)
		if len(cleanText) < 3 {
			return nil, fmt.Errorf("texto curto demais para embedding")
		}

		// Circuit breaker: verificar se o serviço está disponível
		if !cb.allow() {
			return nil, fmt.Errorf("ollama service unavailable (circuit open)")
		}

		// 1. Verificar cache primeiro (usando o texto original como chave para evitar colisões de truncamento)
		queryCacheMu.RLock()
		if cached, exists := queryCache[cleanText]; exists {
			queryCacheMu.RUnlock()
			return cached, nil
		}
		queryCacheMu.RUnlock()

		// 2. Estratégia de Chunking para Textos Longos
		// Reduzido para 1000 chars para garantir compatibilidade TOTAL mesmo com modelos pequenos
		var finalEmbedding []float32
		const maxChunkSize = 1000

		if len(cleanText) > maxChunkSize {
			log.Printf("[Semantic] Texto longo detectado (%d chars). Fatiando em blocos de %d...\n", len(cleanText), maxChunkSize)
			chunks := chunkByChars(cleanText, maxChunkSize)

			// Processar apenas os primeiros 5 chunks (40k chars) para evitar loop infinito/demora excessiva no mapa
			if len(chunks) > 5 {
				chunks = chunks[:5]
			}

			var sumVec []float32
			count := 0
			for _, chunk := range chunks {
				vec, err := getSingleEmbedding(ctx, client, host, model, chunk, cb)
				if err != nil {
					continue
				}
				if sumVec == nil {
					sumVec = make([]float32, len(vec))
				}
				for i := range vec {
					sumVec[i] += vec[i]
				}
				count++
			}

			if count == 0 {
				cb.failure()
				return nil, fmt.Errorf("falha ao gerar embedding para todos os fatias")
			}

			// Calcular média
			for i := range sumVec {
				sumVec[i] /= float32(count)
			}
			finalEmbedding = sumVec
		} else {
			// Texto pequeno: processamento normal
			var err error
			finalEmbedding, err = getSingleEmbedding(ctx, client, host, model, cleanText, cb)
			if err != nil {
				cb.failure()
				return nil, err
			}
			cb.success()
		}

		// 3. Matryoshka Optimization (se necessário)
		if len(finalEmbedding) > 512 {
			finalEmbedding = finalEmbedding[:512]
		}

		// 4. Atualizar Cache
		queryCacheMu.Lock()
		if len(queryCache) >= queryCacheLimit {
			if len(queryCacheKeys) > 0 {
				oldestKey := queryCacheKeys[0]
				queryCacheKeys = queryCacheKeys[1:]
				delete(queryCache, oldestKey)
			}
		}
		queryCache[cleanText] = finalEmbedding
		queryCacheKeys = append(queryCacheKeys, cleanText)
		queryCacheMu.Unlock()

		return finalEmbedding, nil
	}
}

// getSingleEmbedding gerencia o semáforo e retries para uma única chamada ao Ollama
func getSingleEmbedding(ctx context.Context, client *http.Client, host, model, text string, cb *circuitBreaker) ([]float32, error) {
	atomic.AddInt32(&ollamaWaiting, 1)
	select {
	case ollamaSem <- struct{}{}:
		atomic.AddInt32(&ollamaWaiting, -1)
		atomic.AddInt32(&ollamaActive, 1)
		defer func() {
			atomic.AddInt32(&ollamaActive, -1)
			<-ollamaSem
		}()
	case <-ctx.Done():
		atomic.AddInt32(&ollamaWaiting, -1)
		return nil, ctx.Err()
	}

	// Se o circuito estiver half-open (teste), reduzir timeout para 10s
	if cb.isHalfOpen() {
		client.Timeout = 10 * time.Second
	}

	var embedding []float32
	var err error
	maxRetries := 2 // Reduzido retries para falhar mais rápido em caso de erro real

	for i := 0; i < maxRetries; i++ {
		embedding, err = callOllamaEmbed(ctx, client, host, model, text)
		if err == nil {
			return embedding, nil
		}
		// Se for erro de contexto, não adianta tentar de novo
		if strings.Contains(err.Error(), "context length") {
			return nil, err
		}

		slog.Debug(fmt.Sprintf("[Semantic] Tentativa %d/%d falhou para embedding: %v", i+1, maxRetries, err))

		if i < maxRetries-1 {
			time.Sleep(time.Second * time.Duration(i+1))
		}
	}
	return nil, err
}

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

func callOllamaEmbed(ctx context.Context, client *http.Client, host, model, text string) ([]float32, error) {
	url := fmt.Sprintf("%s/api/embed", host)
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":      model,
		"input":      text,
		"keep_alive": "10m",
		"options": map[string]interface{}{
			"num_ctx": 8192,
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var res struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if len(res.Embeddings) == 0 {
		return nil, fmt.Errorf("nenhum embedding retornado")
	}

	return res.Embeddings[0], nil
}
