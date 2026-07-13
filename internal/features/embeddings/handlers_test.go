package embeddings

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ton618/internal/core/config"
	"ton618/internal/core/db"
)

// ── Test helpers ──

func newTestHandlerContext(t *testing.T) *HandlerContext {
	t.Helper()
	cfg := &config.AppConfig{
		DBPath:  t.TempDir() + "/test.db",
		DocsDir: t.TempDir() + "/docs",
		WebDir:  t.TempDir() + "/web",
		Port:    "0",
	}
	cfg.EnsureDirs()

	store, err := db.NewStore(cfg.DBPath)
	if err != nil {
		t.Fatalf("NewStore falhou: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	return NewHandlerContext(cfg, store)
}

// ── HandleEmbeddingSave ─────────────────────────────────────────

func TestHandleEmbeddingSave_MethodNotAllowed(t *testing.T) {
	ctx := newTestHandlerContext(t)

	req := httptest.NewRequest("GET", "/api/embeddings/save", nil)
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingSave(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("esperado 405, got %d", rr.Code)
	}
}

func TestHandleEmbeddingSave_JSONInvalido(t *testing.T) {
	ctx := newTestHandlerContext(t)

	body := bytes.NewReader([]byte("nao e json"))
	req := httptest.NewRequest("POST", "/api/embeddings/save", body)
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingSave(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400, got %d", rr.Code)
	}
}

func TestHandleEmbeddingSave_FilenameVazio(t *testing.T) {
	ctx := newTestHandlerContext(t)

	payload := `{"filename":"","embedding":[` + embeddingJSON(384) + `]}`
	req := httptest.NewRequest("POST", "/api/embeddings/save", bytes.NewReader([]byte(payload)))
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingSave(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para filename vazio, got %d", rr.Code)
	}
}

func TestHandleEmbeddingSave_DimensaoErrada(t *testing.T) {
	ctx := newTestHandlerContext(t)

	// Embedding com apenas 10 dimensões
	payload := `{"filename":"notes/test.md","embedding":[` + embeddingJSON(10) + `]}`
	req := httptest.NewRequest("POST", "/api/embeddings/save", bytes.NewReader([]byte(payload)))
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingSave(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para dimensao errada, got %d", rr.Code)
	}
}

func TestHandleEmbeddingSave_Sucesso(t *testing.T) {
	ctx := newTestHandlerContext(t)

	// Precisa criar a nota no banco primeiro
	ctx.Store.SaveNote("notes/test.md", "# Teste\n\nParágrafo 1\n\nParágrafo 2", "2024-01-01T00:00:00Z")
	ctx.Store.SetFileTags("notes/test.md", []string{})

	embJSON := embeddingJSON(db.EmbeddingDim)
	payload := `{"filename":"notes/test.md","chunks":[{"chunk_id":"notes/test.md#0","filename":"notes/test.md","index":0,"content":"Parágrafo 1","embedding":[` + embJSON + `]}]}`
	req := httptest.NewRequest("POST", "/api/embeddings/save", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingSave(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verifica que foi persistido
	if !ctx.Store.HasEmbedding("notes/test.md") {
		t.Fatal("embedding nao foi persistido")
	}
}

// ── HandleEmbeddingSearch ───────────────────────────────────────

func TestHandleEmbeddingSearch_MethodNotAllowed(t *testing.T) {
	ctx := newTestHandlerContext(t)

	req := httptest.NewRequest("GET", "/api/embeddings/search", nil)
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingSearch(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("esperado 405, got %d", rr.Code)
	}
}

func TestHandleEmbeddingSearch_DimensaoErrada(t *testing.T) {
	ctx := newTestHandlerContext(t)

	payload := `{"embedding":[` + embeddingJSON(5) + `],"limit":10}`
	req := httptest.NewRequest("POST", "/api/embeddings/search", bytes.NewReader([]byte(payload)))
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingSearch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para dimensao errada, got %d", rr.Code)
	}
}

func TestHandleEmbeddingSearch_SemResultados(t *testing.T) {
	ctx := newTestHandlerContext(t)

	payload := `{"embedding":[` + embeddingJSON(db.EmbeddingDim) + `],"limit":10}`
	req := httptest.NewRequest("POST", "/api/embeddings/search", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp searchEmbeddingResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode falhou: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("esperado 0 resultados, got %d", len(resp.Results))
	}
}

// ── HandleEmbeddingStatus ───────────────────────────────────────

func TestHandleEmbeddingStatus_Sucesso(t *testing.T) {
	ctx := newTestHandlerContext(t)

	req := httptest.NewRequest("GET", "/api/embeddings/status", nil)
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rr.Code)
	}

	var status db.EmbeddingStatus
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("decode falhou: %v", err)
	}
	if status.TotalNotes != 0 {
		t.Fatalf("TotalNotes esperado 0, got %d", status.TotalNotes)
	}

	// Verifica Cache-Control
	cacheControl := rr.Header().Get("Cache-Control")
	if cacheControl == "" {
		t.Fatal("Cache-Control header nao definido")
	}
}

// ── HandleEmbeddingPending ──────────────────────────────────────

func TestHandleEmbeddingPending_Sucesso(t *testing.T) {
	ctx := newTestHandlerContext(t)

	req := httptest.NewRequest("GET", "/api/embeddings/pending", nil)
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingPending(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rr.Code)
	}

	var items []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&items); err != nil {
		t.Fatalf("decode falhou: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("esperado 0 pendentes, got %d", len(items))
	}
}

func TestHandleEmbeddingPending_RespeitaLimite(t *testing.T) {
	ctx := newTestHandlerContext(t)

	req := httptest.NewRequest("GET", "/api/embeddings/pending?limit=5", nil)
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingPending(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rr.Code)
	}
}

func TestHandleEmbeddingPending_LimiteInvalido(t *testing.T) {
	ctx := newTestHandlerContext(t)

	// Limite negativo deve usar default 20
	req := httptest.NewRequest("GET", "/api/embeddings/pending?limit=-1", nil)
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingPending(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rr.Code)
	}
}

// ── Helpers ──

// embeddingJSON gera uma string JSON com `n` valores float (ex: "0.1,0.2,...")
func embeddingJSON(n int) string {
	if n <= 0 {
		return ""
	}
	var buf bytes.Buffer
	for i := 0; i < n; i++ {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString("0.5")
	}
	return buf.String()
}
