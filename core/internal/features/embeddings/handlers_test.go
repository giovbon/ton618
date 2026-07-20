package embeddings

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ton618/core/internal/core/config"
	"ton618/core/internal/core/db"
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
	if status.EmbeddingDim != db.EmbeddingDim {
		t.Fatalf("EmbeddingDim esperado %d, got %d", db.EmbeddingDim, status.EmbeddingDim)
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

// ── HandleEmbeddingSave – rejeição de notas não-embeddáveis ────────

// TestHandleEmbeddingSave_NotaNaoEmbeddavel verifica que o handler rejeita (400)
// notas cujo tipo não suporta indexação semântica (drawing, pdf, spreadsheet,
// mermaid, archives), e aceita (200) as que são embeddáveis.
func TestHandleEmbeddingSave_NotaNaoEmbeddavel(t *testing.T) {
	embJSON := embeddingJSON(db.EmbeddingDim)

	cases := []struct {
		name         string
		filename     string
		tags         []string
		wantCode     int
		wantEmbedded bool
	}{
		// ── Não embeddáveis ──────────────────────────────────────────
		{
			name:     "drawing rejeitado",
			filename: "notes/meu-desenho.md",
			tags:     []string{"drawing"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "spreadsheet rejeitado",
			filename: "notes/minha-planilha.md",
			tags:     []string{"spreadsheet"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "mermaid rejeitado",
			filename: "notes/fluxo.md",
			tags:     []string{"mermaid"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "mapa rejeitado por tag",
			filename: "notes/minha-rota.md",
			tags:     []string{"map"},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "pdf rejeitado por caminho",
			filename: "pdfs/livro.pdf",
			tags:     []string{},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "attachment rejeitado por caminho",
			filename: "attachments/foto.png",
			tags:     []string{},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "archive rejeitado por caminho",
			filename: "archives/velho.md",
			tags:     []string{},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "mapa rejeitado por nome de arquivo",
			filename: "notes/mapa-da-cidade.md",
			tags:     []string{},
			wantCode: http.StatusBadRequest,
		},
		// ── Embeddáveis ──────────────────────────────────────────────
		{
			name:         "nota markdown aceita",
			filename:     "notes/nota-normal.md",
			tags:         []string{},
			wantCode:     http.StatusOK,
			wantEmbedded: true,
		},
		{
			name:         "typst aceito",
			filename:     "notes/documento.md",
			tags:         []string{"typst"},
			wantCode:     http.StatusOK,
			wantEmbedded: true,
		},
		{
			name:         "mindmap aceito",
			filename:     "notes/mindmap-nota.md",
			tags:         []string{"mindmap"},
			wantCode:     http.StatusOK,
			wantEmbedded: true,
		},
		{
			name:         "artigo aceito",
			filename:     "notes/artigo-sobre-go.md",
			tags:         []string{"artigo"},
			wantCode:     http.StatusOK,
			wantEmbedded: true,
		},
		{
			name:         "youtube aceito",
			filename:     "notes/video-tutorial.md",
			tags:         []string{"youtube"},
			wantCode:     http.StatusOK,
			wantEmbedded: true,
		},
		{
			name:         "captura aceita",
			filename:     "notes/captura-web.md",
			tags:         []string{"capture"},
			wantCode:     http.StatusOK,
			wantEmbedded: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newTestHandlerContext(t)

			// Cria a nota e suas tags no banco
			ctx.Store.SaveNote(tc.filename, "# Conteúdo de teste para "+tc.filename, "2024-01-01T00:00:00Z")
			ctx.Store.SetFileTags(tc.filename, tc.tags)

			payload := `{"filename":"` + tc.filename + `","chunks":[{"chunk_id":"` + tc.filename + `#0","filename":"` + tc.filename + `","index":0,"content":"Conteudo","embedding":[` + embJSON + `]}]}`
			req := httptest.NewRequest("POST", "/api/embeddings/save", bytes.NewReader([]byte(payload)))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			ctx.HandleEmbeddingSave(rr, req)

			if rr.Code != tc.wantCode {
				t.Fatalf("esperado HTTP %d, got %d — body: %s", tc.wantCode, rr.Code, rr.Body.String())
			}

			if tc.wantEmbedded && !ctx.Store.HasEmbedding(tc.filename) {
				t.Fatal("esperado que o embedding fosse persistido, mas HasEmbedding retornou false")
			}
			if !tc.wantEmbedded && ctx.Store.HasEmbedding(tc.filename) {
				t.Fatal("esperado que o embedding NÃO fosse persistido, mas HasEmbedding retornou true")
			}
		})
	}
}

// TestHandleEmbeddingSave_ChunksVazio verifica que o handler rejeita payload
// com lista de chunks vazia.
func TestHandleEmbeddingSave_ChunksVazio(t *testing.T) {
	ctx := newTestHandlerContext(t)

	ctx.Store.SaveNote("notes/teste.md", "# Nota", "2024-01-01T00:00:00Z")
	ctx.Store.SetFileTags("notes/teste.md", []string{})

	payload := `{"filename":"notes/teste.md","chunks":[]}`
	req := httptest.NewRequest("POST", "/api/embeddings/save", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingSave(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para chunks vazio, got %d", rr.Code)
	}
}

// ── HandleEmbeddingReset ──────────────────────────────────────────

func TestHandleEmbeddingReset_MethodNotAllowed(t *testing.T) {
	ctx := newTestHandlerContext(t)

	req := httptest.NewRequest("GET", "/api/embeddings/reset", nil)
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingReset(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("esperado 405, got %d", rr.Code)
	}
}

func TestHandleEmbeddingReset_LimpaChunksEEmbeddings(t *testing.T) {
	ctx := newTestHandlerContext(t)
	embJSON := embeddingJSON(db.EmbeddingDim)

	// Cria nota e indexa chunks via SaveEmbeddingSave
	notas := []string{"notes/a.md", "notes/b.md", "notes/c.md"}
	for _, nome := range notas {
		ctx.Store.SaveNote(nome, "# Nota "+nome, "2024-01-01T00:00:00Z")
		ctx.Store.SetFileTags(nome, []string{})

		payload := `{"filename":"` + nome + `","chunks":[{"chunk_id":"` + nome + `#0","filename":"` + nome + `","index":0,"content":"Conteudo","embedding":[` + embJSON + `]}]}`
		req := httptest.NewRequest("POST", "/api/embeddings/save", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		ctx.HandleEmbeddingSave(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("falha ao indexar %s: %d – %s", nome, rr.Code, rr.Body.String())
		}
	}

	// Verifica que estão indexadas
	for _, nome := range notas {
		if !ctx.Store.HasEmbedding(nome) {
			t.Fatalf("nota %s deveria estar indexada antes do reset", nome)
		}
	}

	// Reseta
	req := httptest.NewRequest("POST", "/api/embeddings/reset", nil)
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingReset(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("HandleEmbeddingReset esperado 200, got %d – %s", rr.Code, rr.Body.String())
	}

	// Nenhuma nota deve ter embedding após o reset
	for _, nome := range notas {
		if ctx.Store.HasEmbedding(nome) {
			t.Fatalf("nota %s ainda tem embedding após o reset", nome)
		}
	}

	// Status deve refletir o reset: todas as notas pendentes
	status, err := ctx.Store.GetEmbeddingStatus()
	if err != nil {
		t.Fatalf("GetEmbeddingStatus falhou: %v", err)
	}
	if status.IndexedNotes != 0 {
		t.Fatalf("IndexedNotes esperado 0 após reset, got %d", status.IndexedNotes)
	}
	if status.PendingNotes != status.TotalNotes {
		t.Fatalf("PendingNotes (%d) deveria ser igual a TotalNotes (%d) após reset", status.PendingNotes, status.TotalNotes)
	}
}

// ── HandleEmbeddingPending – com dados reais ──────────────────────

// TestHandleEmbeddingPending_FiltraNaoEmbedaveis garante que notas não-embeddáveis
// (drawing, pdf, etc.) não aparecem na lista de pendentes retornada pelo handler.
func TestHandleEmbeddingPending_FiltraNaoEmbedaveis(t *testing.T) {
	ctx := newTestHandlerContext(t)

	// Notas embeddáveis
	ctx.Store.SaveNote("notes/normal.md", "# Nota embeddável", "2024-01-01T00:00:00Z")
	ctx.Store.SetFileTags("notes/normal.md", []string{})

	ctx.Store.SaveNote("notes/artigo.md", "# Artigo embeddável", "2024-01-01T00:00:00Z")
	ctx.Store.SetFileTags("notes/artigo.md", []string{"artigo"})

	// Notas NÃO embeddáveis
	ctx.Store.SaveNote("notes/desenho.md", "# Desenho", "2024-01-01T00:00:00Z")
	ctx.Store.SetFileTags("notes/desenho.md", []string{"drawing"})

	ctx.Store.SaveNote("notes/planilha.md", "# Planilha", "2024-01-01T00:00:00Z")
	ctx.Store.SetFileTags("notes/planilha.md", []string{"spreadsheet"})

	req := httptest.NewRequest("GET", "/api/embeddings/pending?limit=50", nil)
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingPending(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rr.Code)
	}

	type pendingItem struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	var items []pendingItem
	if err := json.NewDecoder(rr.Body).Decode(&items); err != nil {
		t.Fatalf("decode falhou: %v", err)
	}

	// Deve retornar exatamente 2 (as embeddáveis)
	if len(items) != 2 {
		t.Fatalf("esperado 2 pendentes embeddáveis, got %d", len(items))
	}

	pendingSet := make(map[string]bool)
	for _, item := range items {
		pendingSet[item.Filename] = true
	}

	if !pendingSet["notes/normal.md"] {
		t.Error("notes/normal.md deveria estar nos pendentes")
	}
	if !pendingSet["notes/artigo.md"] {
		t.Error("notes/artigo.md deveria estar nos pendentes")
	}
	if pendingSet["notes/desenho.md"] {
		t.Error("notes/desenho.md (drawing) NÃO deveria estar nos pendentes")
	}
	if pendingSet["notes/planilha.md"] {
		t.Error("notes/planilha.md (spreadsheet) NÃO deveria estar nos pendentes")
	}
}

// TestHandleEmbeddingPending_NotaJaIndexadaNaoAparece garante que uma nota que
// já foi indexada (possui chunks) não volta a aparecer como pendente.
func TestHandleEmbeddingPending_NotaJaIndexadaNaoAparece(t *testing.T) {
	ctx := newTestHandlerContext(t)
	embJSON := embeddingJSON(db.EmbeddingDim)

	ctx.Store.SaveNote("notes/ja-indexada.md", "# Nota já indexada", "2024-01-01T00:00:00Z")
	ctx.Store.SetFileTags("notes/ja-indexada.md", []string{})

	ctx.Store.SaveNote("notes/pendente.md", "# Nota pendente", "2024-01-01T00:00:00Z")
	ctx.Store.SetFileTags("notes/pendente.md", []string{})

	// Indexa apenas a primeira
	payload := `{"filename":"notes/ja-indexada.md","chunks":[{"chunk_id":"notes/ja-indexada.md#0","filename":"notes/ja-indexada.md","index":0,"content":"Conteudo","embedding":[` + embJSON + `]}]}`
	req := httptest.NewRequest("POST", "/api/embeddings/save", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ctx.HandleEmbeddingSave(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("save falhou: %d", rr.Code)
	}

	// Consulta pendentes
	req2 := httptest.NewRequest("GET", "/api/embeddings/pending?limit=50", nil)
	rr2 := httptest.NewRecorder()
	ctx.HandleEmbeddingPending(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("pending falhou: %d", rr2.Code)
	}

	type pendingItem struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	var items []pendingItem
	if err := json.NewDecoder(rr2.Body).Decode(&items); err != nil {
		t.Fatalf("decode falhou: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("esperado 1 pendente, got %d", len(items))
	}
	if items[0].Filename != "notes/pendente.md" {
		t.Fatalf("esperado notes/pendente.md, got %s", items[0].Filename)
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
