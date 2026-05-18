package search_test

import (
	"encoding/json"
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/models"
	"etl/internal/search"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// MockHandlerContext simula o HandlerContext da API para testes de integração
type MockHandlerContext struct {
	State *ingest.AppState
	Cfg   *config.AppConfig
}

func (ctx *MockHandlerContext) HandleSearch(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Query struct {
			Term string `json:"term"`
		} `json:"query"`
		Compact  bool `json:"compact"`
		Semantic bool `json:"semantic"`
		From     int  `json:"from"`
		Size     int  `json:"size"`
	}
	json.NewDecoder(r.Body).Decode(&payload)

	res, err := search.ExecuteSearch(r.Context(), payload.Query.Term, payload.Compact, payload.From, payload.Size)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(res)
}

func TestSearchLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Pulando teste de carga")
	}

	// 1. Setup Environment
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{
		BleveIndexDir:  tmpDir + "/bleve",
		StateDir:       tmpDir + "/state",
		SemanticEnable: true,
		OllamaModel:    "mock",
	}

	// Mock index
	search.InitIndex(cfg.BleveIndexDir)
	defer search.CloseIndex()

	state := ingest.NewAppState(cfg)
	ctx := &MockHandlerContext{State: state, Cfg: cfg}

	// 2. Injetar 100 documentos
	for i := 0; i < 100; i++ {
		doc := models.Document{
			ID:      fmt.Sprintf("doc_%d", i),
			Arquivo: fmt.Sprintf("file_%d.md", i),
			Texto:   fmt.Sprintf("Este é o conteúdo do documento de teste número %d.", i),
			Tags:    []string{"teste"},
		}
		search.BatchIndexDocuments(map[string]interface{}{doc.ID: doc})

	}

	// 3. Simular rajada de buscas (Keystroke simulation)
	server := httptest.NewServer(http.HandlerFunc(ctx.HandleSearch))
	defer server.Close()

	client := &http.Client{}
	start := time.Now()
	numRequests := 20

	for i := 0; i < numRequests; i++ {
		payload := fmt.Sprintf(`{"query":{"term":"teste %d"}, "semantic":true, "size":10}`, i)
		req, _ := http.NewRequest("POST", server.URL, strings.NewReader(payload))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Falha na requisição %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("Status %d na requisição %d", resp.StatusCode, i)
		}
	}

	duration := time.Since(start)
	t.Logf("Processadas %d buscas semânticas em %v (Média: %v/req)", numRequests, duration, duration/time.Duration(numRequests))
}
