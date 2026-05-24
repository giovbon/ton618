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

type OllamaProvider struct {
	host      string
	model     string
	dimension int
	client    *http.Client
}

func NewOllamaProvider(host, model string, dim int) *OllamaProvider {
	return &OllamaProvider{
		host:      host,
		model:     model,
		dimension: dim,
		client:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *OllamaProvider) Dimensions() int { return p.dimension }

func (p *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	clean := strings.TrimSpace(text)
	if len(clean) < 2 {
		return nil, fmt.Errorf("texto curto demais")
	}

	cacheKey := "ollama|" + p.model + "|" + clean
	if cached, ok := cacheGet(cacheKey); ok {
		return cached, nil
	}

	reqBody := map[string]interface{}{
		"model": p.model,
		"input": clean,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", p.host+"/api/embed", bytes.NewBuffer(body))
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
		return nil, fmt.Errorf("ollama %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Embeddings) == 0 || len(result.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("embedding vazio")
	}

	vec := result.Embeddings[0]
	if len(vec) > p.dimension {
		vec = vec[:p.dimension]
	}
	NormalizeVector(vec)

	cacheSet(cacheKey, vec)
	return vec, nil
}
