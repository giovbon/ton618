package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math"
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
	// Cache de embeddings: mapa + fila de chaves para eviccao ordenada (FIFO)
	queryCache      = make(map[string][]float32)
	queryCacheKeys  []string
	queryCacheMu    sync.RWMutex
	queryCacheLimit = 200
	cacheFilePath   string

	// Semaforo para controlar concorrencia no Ollama
	ollamaSem = make(chan struct{}, 2)

	// Circuit breaker GLOBAL compartilhado entre todas as chamadas Ollama.
	// Anteriormente cada NewOllamaEmbedding criava seu proprio circuit breaker,
	// anulando a protecao (nunca acumulava falhas suficientes para abrir).
	globalCB = newCircuitBreaker()

	// Metricas do semaforo
	ollamaActive  int32
	ollamaWaiting int32
)

// Metrics contem estatisticas de uso do motor semantico
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

// NewOllamaEmbedding cria uma funcao de embedding que usa o circuit breaker
// GLOBAL compartilhado entre todas as chamadas, garantindo protecao real
// contra falhas consecutivas no Ollama.
func NewOllamaEmbedding(model, host string, dimension int) func(ctx context.Context, text string) ([]float32, error) {
	client := &http.Client{Timeout: 5 * time.Minute}

	return func(ctx context.Context, text string) ([]float32, error) {
		cleanText := strings.TrimSpace(text)
		if len(cleanText) < 3 {
			return nil, fmt.Errorf("texto curto demais para embedding")
		}

		// Circuit breaker global: verificar se o servico esta disponivel
		if !globalCB.allow() {
			return nil, fmt.Errorf("ollama service unavailable (circuit open)")
		}

		// 1. Verificar cache primeiro (chave composta: modelo|dimensao|texto)
		cacheKey := fmt.Sprintf("%s|%d|%s", model, dimension, cleanText)
		queryCacheMu.RLock()
		if cached, exists := queryCache[cacheKey]; exists {
			queryCacheMu.RUnlock()
			return cached, nil
		}
		queryCacheMu.RUnlock()

		// 2. Estrategia de Chunking para Textos Longos
		// Reduzido para 1000 chars para garantir compatibilidade TOTAL mesmo com modelos pequenos
		var finalEmbedding []float32
		const maxChunkSize = 1000
		const maxChunks = 8 // Limite aumentado de 5 para 8 (8000 chars) com warning

		if len(cleanText) > maxChunkSize {
			log.Printf("[Semantic] Texto longo detectado (%d chars). Fatiando em blocos de %d...\n", len(cleanText), maxChunkSize)
			chunks := chunkByChars(cleanText, maxChunkSize)

			// Limitar numero de chunks com warning explicito
			if len(chunks) > maxChunks {
				log.Printf("[Semantic] AVISO: Texto de %d chars truncado para %d chunks (%d chars). "+
					"Considere reduzir o tamanho da nota para embeddings mais precisos.\n",
					len(cleanText), maxChunks, maxChunks*maxChunkSize)
				chunks = chunks[:maxChunks]
			}

			var sumVec []float32
			count := 0
			for _, chunk := range chunks {
				vec, err := getSingleEmbedding(ctx, client, host, model, chunk, globalCB)
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
				globalCB.failure()
				return nil, fmt.Errorf("falha ao gerar embedding para todos os fatias")
			}

			// Calcular media
			for i := range sumVec {
				sumVec[i] /= float32(count)
			}
			finalEmbedding = sumVec
			globalCB.success()
		} else {
			// Texto pequeno: processamento normal
			var err error
			finalEmbedding, err = getSingleEmbedding(ctx, client, host, model, cleanText, globalCB)
			if err != nil {
				globalCB.failure()
				return nil, err
			}
			globalCB.success()
		}

		// 3. Matryoshka Optimization (se necessario)
		if dimension <= 0 {
			dimension = 512
		}
		if len(finalEmbedding) > dimension {
			finalEmbedding = finalEmbedding[:dimension]
		}

		// Normalizar vetor antes de cache (P5.1)
		normalizeVector(finalEmbedding)

		// 4. Atualizar Cache
		queryCacheMu.Lock()
		if len(queryCache) >= queryCacheLimit {
			if len(queryCacheKeys) > 0 {
				oldestKey := queryCacheKeys[0]
				queryCacheKeys = queryCacheKeys[1:]
				delete(queryCache, oldestKey)
			}
		}
		queryCache[cacheKey] = finalEmbedding
		queryCacheKeys = append(queryCacheKeys, cacheKey)
		queryCacheMu.Unlock()

		return finalEmbedding, nil
	}
}

// getSingleEmbedding gerencia o semaforo e retries para uma unica chamada ao Ollama
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

	// Se o circuito estiver half-open (teste), reduzir timeout para 10s via contexto
	callCtx := ctx
	if cb.isHalfOpen() {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}

	var embedding []float32
	var err error
	maxRetries := 2 // Reduzido retries para falhar mais rapido em caso de erro real

	for i := 0; i < maxRetries; i++ {
		embedding, err = callOllamaEmbed(callCtx, client, host, model, text)
		if err == nil {
			return embedding, nil
		}
		// Se for erro de contexto, nao adianta tentar de novo
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
	url := fmt.Sprintf("%s/api/embed", strings.TrimSuffix(host, "/"))
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

// EmbedBatch envia multiplos textos em uma unica chamada HTTP ao Ollama.
// Muito mais eficiente que chamadas individuais (P4.2).
func EmbedBatch(ctx context.Context, host, model string, texts []string, dimension int) (map[string][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("nenhum texto para embedding")
	}

	client := &http.Client{Timeout: 5 * time.Minute}

	url := fmt.Sprintf("%s/api/embed", strings.TrimSuffix(host, "/"))
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":      model,
		"input":      texts,
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
		globalCB.failure()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		globalCB.failure()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var res struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		globalCB.failure()
		return nil, err
	}

	globalCB.success()

	if len(res.Embeddings) != len(texts) {
		return nil, fmt.Errorf("numero de embeddings retornados (%d) difere do enviado (%d)", len(res.Embeddings), len(texts))
	}

	if dimension <= 0 {
		dimension = 512
	}

	result := make(map[string][]float32, len(texts))
	for i, text := range texts {
		vec := res.Embeddings[i]
		if len(vec) > dimension {
			vec = vec[:dimension]
		}
		normalizeVector(vec)
		result[text] = vec
	}

	return result, nil
}

// normalizeVector normaliza um vetor para comprimento unitario (L2 norm).
// Evita distorcoes no cosine similarity quando o modelo nao entrega vetores normalizados.
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
