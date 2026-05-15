package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ── OpenAIProvider ────────────────────────────────────────────────────
// Compatível com qualquer API que siga o formato OpenAI:
// POST /v1/embeddings { model, input }
// Funciona com: OpenAI, Cohere (via LiteLLM), vLLM, Ollama (modo OpenAI-compat), etc.

type OpenAIProvider struct {
	baseURL   string
	model     string
	apiKey    string
	dimension int
	client    *http.Client
}

func NewOpenAIProvider(baseURL, model, apiKey string, dim int) *OpenAIProvider {
	return &OpenAIProvider{
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		model:     model,
		apiKey:    apiKey,
		dimension: dim,
		client:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *OpenAIProvider) Dimensions() int { return p.dimension }

func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	clean := strings.TrimSpace(text)
	if len(clean) < 2 {
		return nil, fmt.Errorf("texto curto demais para embedding")
	}

	// Cache
	cacheKey := fmt.Sprintf("openai|%s|%d|%s", p.model, p.dimension, clean)
	if cached := getFromCache(cacheKey); cached != nil {
		return cached, nil
	}

	// Trunca textos muito longos (OpenAI tem limite de ~8191 tokens)
	const maxLen = 8000
	if len(clean) > maxLen {
		clean = clean[:maxLen]
	}

	vec, err := p.callOpenAI(ctx, []string{clean})
	if err != nil {
		return nil, err
	}

	if len(vec) > p.dimension {
		vec = vec[:p.dimension]
	}
	normalizeVector(vec)

	storeInCache(cacheKey, vec)
	return vec, nil
}

func (p *OpenAIProvider) EmbedBatch(ctx context.Context, texts []string) (map[string][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("nenhum texto para embedding")
	}

	// OpenAI aceita múltiplos inputs por chamada
	vecs, err := p.callOpenAI(ctx, texts)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]float32, len(texts))
	for i, text := range texts {
		if i >= len(vecs) {
			break
		}
		if len(vecs[i]) > p.dimension {
			vecs[i] = vecs[i][:p.dimension]
		}
		normalizeVector(vecs[i])
		result[text] = vecs[i]
	}
	return result, nil
}

func (p *OpenAIProvider) callOpenAI(ctx context.Context, texts []string) ([][]float32, error) {
	url := fmt.Sprintf("%s/embeddings", p.baseURL)

	body, _ := json.Marshal(map[string]interface{}{
		"model": p.model,
		"input": texts,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openai decode error: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("openai retornou embedding vazio")
	}

	vecs := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		vecs[i] = d.Embedding
	}
	return vecs, nil
}

// ── Cache helpers (compartilhados entre providers) ────────────────────

func getFromCache(key string) []float32 {
	queryCacheMu.RLock()
	defer queryCacheMu.RUnlock()
	return queryCache[key]
}

func storeInCache(key string, vec []float32) {
	queryCacheMu.Lock()
	defer queryCacheMu.Unlock()
	if len(queryCache) >= queryCacheLimit {
		if len(queryCacheKeys) > 0 {
			oldestKey := queryCacheKeys[0]
			queryCacheKeys = queryCacheKeys[1:]
			delete(queryCache, oldestKey)
		}
	}
	queryCache[key] = vec
	queryCacheKeys = append(queryCacheKeys, key)
}
