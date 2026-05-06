package api

import (
	"encoding/json"
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/models"
	"etl/internal/search"
	"net/http"
	"net/http/httptest"

	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleSearch(t *testing.T) {
	// Preparar índice temporário
	indexDir := filepath.Join(t.TempDir(), "test.bleve")
	err := search.InitIndex(indexDir)
	if err != nil {
		t.Fatalf("Falha ao inicializar índice: %v", err)
	}
	defer search.CloseIndex()

	// Indexar um documento de teste
	doc := models.Document{
		ID:        "test-search-id",
		Texto:     "conteúdo de busca unitário",
		Arquivo:   "unit-test.md",
		Timestamp: time.Now().Format(time.RFC3339),
	}
	search.IndexDocument(doc.ID, doc)

	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{},
		State: appState,
	}

	t.Run("Success", func(t *testing.T) {
		payload := `{"query": {"term": "unitário"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/search", strings.NewReader(payload))
		w := httptest.NewRecorder()

		ctx.HandleSearch(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Esperado 200 OK, obteve %d", w.Code)
		}

		var resp models.SearchResults
		json.Unmarshal(w.Body.Bytes(), &resp)

		if resp.Hits.Total.Value == 0 {
			t.Error("Deveria ter encontrado pelo menos 1 hit")
		}
	})

	t.Run("InvalidPayload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/search", strings.NewReader(`{invalid json`))
		w := httptest.NewRecorder()

		ctx.HandleSearch(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Esperado 400 Bad Request, obteve %d", w.Code)
		}
	})

	t.Run("CacheHIT", func(t *testing.T) {
		payload := `{"query": {"term": "unitário"}}`
		// Primeira vez (MISS)
		req1 := httptest.NewRequest(http.MethodPost, "/api/search", strings.NewReader(payload))
		w1 := httptest.NewRecorder()
		ctx.HandleSearch(w1, req1)

		// Segunda vez (HIT)
		req2 := httptest.NewRequest(http.MethodPost, "/api/search", strings.NewReader(payload))
		w2 := httptest.NewRecorder()
		ctx.HandleSearch(w2, req2)

		if w2.Header().Get("X-Cache") != "HIT" {
			t.Error("Segunda busca deveria ser um Cache HIT")
		}
	})

	t.Run("WrongMethod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/search", nil)
		w := httptest.NewRecorder()

		ctx.HandleSearch(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Esperado 405 Method Not Allowed, obteve %d", w.Code)
		}
	})
}

func TestHandleTrack(t *testing.T) {
	tmpDir := t.TempDir()
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	ctx := &HandlerContext{
		Cfg: &config.AppConfig{
			StateDir:  tmpDir,
			StateFile: filepath.Join(tmpDir, "state.json"),
			DocsDir:   tmpDir,
		},
		State: appState,
	}

	filename := "popularity-test.md"

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/track?name="+filename, nil)
		w := httptest.NewRecorder()

		ctx.HandleTrack(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("Esperado 204 No Content, obteve %d", w.Code)
		}

		// Polling para aguardar a goroutine de incremento
		var count int
		for i := 0; i < 10; i++ {
			count = ctx.State.GetPopularity(filename)
			if count > 0 {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}

		if count != 1 {
			t.Errorf("Esperado popularidade 1, obteve %d", count)
		}
	})

	t.Run("MissingName", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/track", nil)
		w := httptest.NewRecorder()

		ctx.HandleTrack(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Esperado 400 Bad Request, obteve %d", w.Code)
		}
	})
}
