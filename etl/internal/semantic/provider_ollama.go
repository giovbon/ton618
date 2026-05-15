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
	"strings"
	"sync/atomic"
	"time"
)

// ── OllamaProvider ────────────────────────────────────────────────────

type OllamaProvider struct {
	host      string
	model     string
	dimension int
	client    *http.Client
}

func NewOllamaProvider(host, model string, dim int) *OllamaProvider {
	return &OllamaProvider{
		host:      strings.TrimSuffix(host, "/"),
		model:     model,
		dimension: dim,
		client:    &http.Client{Timeout: 5 * time.Minute},
	}
}

func (p *OllamaProvider) Dimensions() int { return p.dimension }

func (p *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	cleanText := strings.TrimSpace(text)
	if len(cleanText) < 3 {
		return nil, fmt.Errorf("texto curto demais para embedding")
	}

	if !globalCB.allow() {
		return nil, fmt.Errorf("embedding service unavailable (circuit open)")
	}

	// Cache
	cacheKey := fmt.Sprintf("ollama|%s|%d|%s", p.model, p.dimension, cleanText)
	queryCacheMu.RLock()
	if cached, exists := queryCache[cacheKey]; exists {
		queryCacheMu.RUnlock()
		return cached, nil
	}
	queryCacheMu.RUnlock()

	// Chunking para textos longos
	var finalEmbedding []float32
	const maxChunkSize = 1000
	const maxChunks = 8

	if len(cleanText) > maxChunkSize {
		log.Printf("[Ollama] Texto longo detectado (%d chars). Fatiando em blocos de %d...\n", len(cleanText), maxChunkSize)
		chunks := chunkByChars(cleanText, maxChunkSize)
		if len(chunks) > maxChunks {
			log.Printf("[Ollama] AVISO: Texto de %d chars truncado para %d chunks.\n", len(cleanText), maxChunks)
			chunks = chunks[:maxChunks]
		}
		var sumVec []float32
		count := 0
		for _, chunk := range chunks {
			vec, err := p.embedSingle(ctx, chunk)
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
			return nil, fmt.Errorf("falha ao gerar embedding para todos os chunks")
		}
		for i := range sumVec {
			sumVec[i] /= float32(count)
		}
		finalEmbedding = sumVec
	} else {
		var err error
		finalEmbedding, err = p.embedSingle(ctx, cleanText)
		if err != nil {
			globalCB.failure()
			return nil, err
		}
	}
	globalCB.success()

	// Matryoshka
	if len(finalEmbedding) > p.dimension {
		finalEmbedding = finalEmbedding[:p.dimension]
	}
	normalizeVector(finalEmbedding)

	// Cache
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

func (p *OllamaProvider) EmbedBatch(ctx context.Context, texts []string) (map[string][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("nenhum texto para embedding")
	}

	url := fmt.Sprintf("%s/api/embed", p.host)
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":      p.model,
		"input":      texts,
		"keep_alive": "10m",
		"options":    map[string]interface{}{"num_ctx": 8192},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		globalCB.failure()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
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
		return nil, fmt.Errorf("numero de embeddings (%d) difere do enviado (%d)", len(res.Embeddings), len(texts))
	}

	dim := p.dimension
	if dim <= 0 {
		dim = 512
	}

	result := make(map[string][]float32, len(texts))
	for i, text := range texts {
		vec := res.Embeddings[i]
		if len(vec) > dim {
			vec = vec[:dim]
		}
		normalizeVector(vec)
		result[text] = vec
	}
	return result, nil
}

func (p *OllamaProvider) embedSingle(ctx context.Context, text string) ([]float32, error) {
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

	callCtx := ctx
	if globalCB.isHalfOpen() {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}

	var embedding []float32
	var err error
	for i := 0; i < 2; i++ {
		embedding, err = callOllamaEmbed(callCtx, p.client, p.host, p.model, text)
		if err == nil {
			return embedding, nil
		}
		if strings.Contains(err.Error(), "context length") {
			return nil, err
		}
		slog.Debug(fmt.Sprintf("[Ollama] Tentativa %d/2 falhou: %v", i+1, err))
		if i < 1 {
			time.Sleep(time.Second * time.Duration(i+1))
		}
	}
	return nil, err
}

func callOllamaEmbed(ctx context.Context, client *http.Client, host, model, text string) ([]float32, error) {
	url := fmt.Sprintf("%s/api/embed", host)
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":      model,
		"input":      text,
		"keep_alive": "10m",
		"options":    map[string]interface{}{"num_ctx": 8192},
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
