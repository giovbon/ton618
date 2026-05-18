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
		return nil, fmt.Errorf("texto curto demais")
	}

	cacheKey := "gemini|" + p.model + "|" + clean
	if cached, ok := cacheGet(cacheKey); ok {
		return cached, nil
	}

	const maxLen = 2048
	if len(clean) > maxLen {
		clean = clean[:maxLen]
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent?key=%s",
		p.model, p.apiKey,
	)

	reqBody := map[string]interface{}{
		"model": "models/" + p.model,
		"content": map[string]interface{}{
			"parts": []map[string]string{{"text": clean}},
		},
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	vec := result.Embedding.Values
	if len(vec) == 0 {
		return nil, fmt.Errorf("embedding vazio")
	}

	if len(vec) > p.dimension {
		vec = vec[:p.dimension]
	}

	cacheSet(cacheKey, vec)
	return vec, nil
}
