package index

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

type OpenAIProvider struct {
	baseURL   string
	apiKey    string
	model     string
	dimension int
	client    *http.Client
}

func NewOpenAIProvider(baseURL, apiKey, model string, dim int) *OpenAIProvider {
	return &OpenAIProvider{
		baseURL:   baseURL,
		apiKey:    apiKey,
		model:     model,
		dimension: dim,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *OpenAIProvider) Dimensions() int { return p.dimension }

func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	clean := strings.TrimSpace(text)
	if len(clean) < 2 {
		return nil, fmt.Errorf("texto curto demais")
	}

	cacheKey := "openai|" + p.model + "|" + clean
	if cached, ok := cacheGet(cacheKey); ok {
		return cached, nil
	}

	reqBody := map[string]interface{}{
		"model": p.model,
		"input": clean,
	}
	if p.dimension > 0 {
		reqBody["dimensions"] = p.dimension
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/embeddings", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embedding vazio")
	}

	vec := result.Data[0].Embedding
	NormalizeVector(vec)

	cacheSet(cacheKey, vec)
	return vec, nil
}
