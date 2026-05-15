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

// ── GeminiProvider ────────────────────────────────────────────────────
// API gratuita: https://ai.google.dev/gemini-api/docs/embeddings
// Modelo padrão: text-embedding-004 (768 dimensões)

type GeminiProvider struct {
	apiKey    string
	model     string
	dimension int
	client    *http.Client
}

func NewGeminiProvider(apiKey, model string, dim int) *GeminiProvider {
	return &GeminiProvider{
		apiKey:    apiKey,
		model:     model,
		dimension: dim,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *GeminiProvider) Dimensions() int { return p.dimension }

func (p *GeminiProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	clean := strings.TrimSpace(text)
	if len(clean) < 2 {
		return nil, fmt.Errorf("texto curto demais para embedding")
	}

	// Cache
	cacheKey := fmt.Sprintf("gemini|%s|%d|%s", p.model, p.dimension, clean)
	if cached := getFromCache(cacheKey); cached != nil {
		return cached, nil
	}

	// Gemini não suporta batch nativo via REST (apenas chamadas individuais)
	// Para textos longos, trunca nos primeiros 2048 caracteres
	const maxLen = 2048
	if len(clean) > maxLen {
		clean = clean[:maxLen]
	}

	vec, err := p.callGemini(ctx, clean)
	if err != nil {
		return nil, err
	}

	// Matryoshka
	if len(vec) > p.dimension {
		vec = vec[:p.dimension]
	}
	normalizeVector(vec)

	storeInCache(cacheKey, vec)
	return vec, nil
}

func (p *GeminiProvider) EmbedBatch(ctx context.Context, texts []string) (map[string][]float32, error) {
	// Gemini não tem endpoint batch real — processa sequencialmente
	result := make(map[string][]float32, len(texts))
	for _, text := range texts {
		vec, err := p.Embed(ctx, text)
		if err != nil {
			continue
		}
		result[text] = vec
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("todos os embeddings falharam")
	}
	return result, nil
}

func (p *GeminiProvider) callGemini(ctx context.Context, text string) ([]float32, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1/models/%s:embedContent?key=%s",
		p.model, p.apiKey,
	)

	type embedRequest struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	}

	reqBody := embedRequest{}
	reqBody.Content.Parts = []struct {
		Text string `json:"text"`
	}{{Text: text}}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("gemini decode error: %w", err)
	}

	if len(result.Embedding.Values) == 0 {
		return nil, fmt.Errorf("gemini retornou embedding vazio")
	}

	return result.Embedding.Values, nil
}
