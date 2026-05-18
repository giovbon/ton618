package semantic

import (
	"context"
	"strings"
	"sync"
)

// EmbeddingProvider abstrai backends de embedding (Gemini, Ollama, OpenAI).
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimensions() int
}

// NewProvider cria o provider baseado na configuração.
func NewProvider(cfgProvider, apiKey, model, baseURL, ollamaHost, ollamaModel string, dim int) EmbeddingProvider {
	provider := strings.ToLower(cfgProvider)
	switch provider {
	case "gemini":
		if apiKey == "" {
			return NewOllamaProvider(ollamaHost, ollamaModel, dim)
		}
		if model == "" {
			model = "text-embedding-004"
		}
		if dim <= 0 {
			dim = 768
		}
		return NewGeminiProvider(apiKey, model, dim)

	case "openai":
		if apiKey == "" {
			return NewOllamaProvider(ollamaHost, ollamaModel, dim)
		}
		if model == "" {
			model = "text-embedding-3-small"
		}
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		if dim <= 0 {
			dim = 1536
		}
		return NewOpenAIProvider(baseURL, apiKey, model, dim)

	default:
		// ollama or empty
		if ollamaHost == "" {
			ollamaHost = "http://localhost:11434"
		}
		if ollamaModel == "" {
			ollamaModel = "nomic-embed-text"
		}
		if dim <= 0 {
			dim = 768
		}
		return NewOllamaProvider(ollamaHost, ollamaModel, dim)
	}
}

// ── Cache ──

var (
	embedCache   = make(map[string][]float32)
	embedCacheMu sync.RWMutex
)

func cacheGet(key string) ([]float32, bool) {
	embedCacheMu.RLock()
	defer embedCacheMu.RUnlock()
	v, ok := embedCache[key]
	return v, ok
}

func cacheSet(key string, vec []float32) {
	embedCacheMu.Lock()
	defer embedCacheMu.Unlock()
	embedCache[key] = vec
}

// NormalizeVector normaliza um vetor para norma L2 unitária.
func NormalizeVector(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	norm := float32(sum)
	if norm > 0 && norm != 1.0 {
		norm = float32(1.0 / float64(norm))
		for i := range vec {
			vec[i] *= norm
		}
	}
}
