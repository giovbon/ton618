package search

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ton618/core/internal/core/db"
	"ton618/core/internal/features/notes"
)

// makeEmbedding cria um vetor 384-d determinístico; os primeiros valores podem
// ser passados para controlar a proximidade semântica nos testes.
func makeEmbedding(nonZero ...float32) []float32 {
	emb := make([]float32, db.EmbeddingDim)
	for i, v := range nonZero {
		if i >= len(emb) {
			break
		}
		emb[i] = v
	}
	return emb
}

// indexNoteFTS salva a nota pelo NoteService (indexa FTS5/documentos).
func indexNoteFTS(t *testing.T, ctx *HandlerContext, filename, content string) {
	t.Helper()
	svc := notes.NewNoteService(ctx.Store, ctx.Store, ctx.Store, ctx.Store, ctx.Store, ctx.Store, ctx.Cfg.DocsDir)
	if err := svc.Save(filename, content, nil); err != nil {
		t.Fatalf("notes save %s: %v", filename, err)
	}
}

// indexNoteSemantic grava um chunk com embedding (indexa a busca semântica).
func indexNoteSemantic(t *testing.T, ctx *HandlerContext, filename string, emb []float32) {
	t.Helper()
	chunk := db.ChunkInfo{
		ChunkID:    filename + "#0",
		Filename:   filename,
		ChunkIndex: 0,
		Content:    "conteudo",
		Embedding:  emb,
	}
	if err := ctx.Store.SaveNoteChunks(filename, []db.ChunkInfo{chunk}); err != nil {
		t.Fatalf("SaveNoteChunks %s: %v", filename, err)
	}
}

func TestHandleHybridSearch_FusesBothEngines(t *testing.T) {
	ctx := newTestContext(t)

	// python.md: casa no FTS ("python") E na semântica (embedding == query).
	indexNoteFTS(t, ctx, "notes/python.md", "# Python\nPython é ótimo para análise de dados.")
	indexNoteSemantic(t, ctx, "notes/python.md", makeEmbedding(1.0))

	// semantica.md: NÃO casa no FTS (não contém "python"), mas casa na semântica.
	indexNoteFTS(t, ctx, "notes/semantica.md", "# Música\nSobre música clássica e orquestras.")
	indexNoteSemantic(t, ctx, "notes/semantica.md", makeEmbedding(0.9, 0.1))

	body, _ := json.Marshal(map[string]interface{}{
		"query":     "python",
		"embedding": makeEmbedding(1.0),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Results) < 2 {
		t.Fatalf("esperava >= 2 resultados (um de cada motor), got %d", len(resp.Results))
	}

	// python.md deve vir primeiro (aparece nos DOIS motores → soma RRF).
	if resp.Results[0].Filename != "notes/python.md" {
		t.Errorf("esperava notes/python.md primeiro, got %q", resp.Results[0].Filename)
	}

	byName := map[string]hybridSearchResult{}
	for _, r := range resp.Results {
		byName[r.Filename] = r
	}

	python := byName["notes/python.md"]
	if python.RankFTS == nil || *python.RankFTS != 1 {
		t.Errorf("python.md: esperava rank_fts=1, got %v", python.RankFTS)
	}
	if python.RankSem == nil || *python.RankSem != 1 {
		t.Errorf("python.md: esperava rank_sem=1, got %v", python.RankSem)
	}
	if !python.HasHighlight {
		t.Errorf("python.md: esperava snippet com highlight (casou no FTS)")
	}

	sem := byName["notes/semantica.md"]
	if sem.RankFTS != nil {
		t.Errorf("semantica.md: não deveria ter rank_fts")
	}
	if sem.RankSem == nil || *sem.RankSem != 2 {
		t.Errorf("semantica.md: esperava rank_sem=2, got %v", sem.RankSem)
	}
	if sem.HasHighlight {
		t.Errorf("semantica.md: não deveria ter highlight (veio só da semântica)")
	}
	if sem.Snippet == "" {
		t.Errorf("semantica.md: snippet (prévia genérica) não deveria estar vazio")
	}
	if sem.SemSimilarity == nil || *sem.SemSimilarity < 50 {
		t.Errorf("semantica.md: esperava sem_similarity alto, got %v", sem.SemSimilarity)
	}
}

func TestHandleHybridSearch_DegradesToFTSWithoutEmbedding(t *testing.T) {
	ctx := newTestContext(t)

	indexNoteFTS(t, ctx, "notes/python.md", "# Python\nPython é ótimo.")
	indexNoteFTS(t, ctx, "notes/semantica.md", "# Música\nMúsica clássica e orquestras.")
	indexNoteSemantic(t, ctx, "notes/semantica.md", makeEmbedding(0.9, 0.1))

	// Embedding zerado → degrada para FTS5 puro (semantica.md não deve aparecer).
	body, _ := json.Marshal(map[string]interface{}{
		"query":     "python",
		"embedding": makeEmbedding(),
		"limit":     10,
	})
	req := httptest.NewRequest("POST", "/api/search/hybrid", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleHybridSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []hybridSearchResult `json:"results"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)

	for _, r := range resp.Results {
		if r.Filename == "notes/semantica.md" {
			t.Errorf("semantica.md não deveria aparecer sem embedding (FTS puro)")
			break
		}
	}
}
