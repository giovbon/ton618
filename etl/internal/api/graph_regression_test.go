package api

import (
	"encoding/json"
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/search"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleKnowledgeMap_TagFallback(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	indexDir := filepath.Join(tmpDir, "index.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: docsDir},
		State: appState,
		Index: search.GetIndex(),
	}

	// 1. Criar um arquivo com #embed no disco E indexar no Bleve
	filename := "test.md"
	content := "Conteudo com #embed"
	os.WriteFile(filepath.Join(docsDir, filename), []byte(content), 0644)

	search.IndexDocument(filename, map[string]interface{}{
		"arquivo": filename,
		"texto":   content,
	})

	// 2. Adicionar vetor manualmente ao estado
	appState.SetNoteVector(filename, []float32{1.0, 0.5, 0.2}, "Conteudo com embed")

	// 3. Chamar HandleKnowledgeMap
	req := httptest.NewRequest(http.MethodGet, "/api/graph/map", nil)
	w := httptest.NewRecorder()
	ctx.HandleKnowledgeMap(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Esperado 200, obteve %d", w.Code)
	}

	var resp KnowledgeMapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, n := range resp.Notes {
		if n.ID == filename {
			found = true
			break
		}
	}

	if !found {
		t.Error("Nota com #embed (apenas no Bleve, sem tag em cache) nao foi encontrada no mapa.")
	}
}

func TestHandleReindexVectors_TagFallback(t *testing.T) {
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	indexDir := filepath.Join(tmpDir, "index.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: docsDir, OllamaModel: "nomic-embed-text"},
		State: appState,
		Index: search.GetIndex(),
	}

	filename := "reindex-test.md"
	content := "Nota para reindex #embed"
	os.WriteFile(filepath.Join(docsDir, filename), []byte(content), 0644)

	search.IndexDocument("doc1", map[string]interface{}{
		"arquivo": filename,
		"texto":   content,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/graph/reindex", nil)
	w := httptest.NewRecorder()
	ctx.HandleReindexVectors(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Esperado 202 Accepted, obteve %d", w.Code)
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		content  string
		filename string
		expected string
	}{
		{"# Meu Titulo\nConteudo aqui", "notes/test.md", "Meu Titulo"},
		{"## Subtitulo\nTexto", "notes/doc.md", "Subtitulo"},
		{"Sem heading\nApenas texto", "notes/plain.md", "Sem heading"},
		{"\n\n   # Titulo com espacos   \ncorpo", "notes/space.md", "Titulo com espacos"},
		{"", "notes/empty.md", "empty"},
		{"# \nSo heading vazio\nTexto real", "notes/vazio.md", "So heading vazio"},
		{"#\nLinha real", "notes/hash.md", "Linha real"},
	}

	for _, tt := range tests {
		result := extractTitle(tt.content, tt.filename)
		if result != tt.expected {
			t.Errorf("extractTitle(%q, %q) = %q, want %q", tt.content, tt.filename, result, tt.expected)
		}
	}
}

func TestHandleKnowledgeMap_FastPath(t *testing.T) {
	// Verifica o caminho otimizado: tag #embed no cache + titulo armazenado no NoteVector
	// Neste caminho, NAO deve haver queries ao Bleve para deteccao de #embed nem para titulos.
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	indexDir := filepath.Join(tmpDir, "index.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: docsDir},
		State: appState,
		Index: search.GetIndex(),
	}

	// Criar arquivo no disco
	filename := "fastpath.md"
	content := "# Meu Titulo Cacheado\nConteudo com #embed"
	os.WriteFile(filepath.Join(docsDir, filename), []byte(content), 0644)

	// Adicionar tag #embed ao cache
	appState.SetFileTags(filename, []string{"embed"})

	// Adicionar vetor COM titulo armazenado (P3.3)
	appState.SetNoteVector(filename, []float32{0.5, 0.8, 0.3}, "Meu Titulo Cacheado")

	// Executar
	req := httptest.NewRequest(http.MethodGet, "/api/graph/map", nil)
	w := httptest.NewRecorder()
	ctx.HandleKnowledgeMap(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperado 200, obteve %d", w.Code)
	}

	var resp KnowledgeMapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.Notes) != 1 {
		t.Fatalf("Esperava 1 nota, obteve %d", len(resp.Notes))
	}

	// Verificar que o titulo veio do NoteVector (nao do Bleve)
	if resp.Notes[0].Title != "Meu Titulo Cacheado" {
		t.Errorf("Titulo esperado 'Meu Titulo Cacheado', obteve '%s'", resp.Notes[0].Title)
	}
	if resp.Notes[0].ID != filename {
		t.Errorf("ID esperado '%s', obteve '%s'", filename, resp.Notes[0].ID)
	}
}

func TestHandleGraphQueryPoint_WithNoteVectors(t *testing.T) {
	// Verifica que o endpoint de query point funciona com o novo formato NoteVector (P3.3)
	// sem depender do Ollama real (usa vetores pre-armazenados)
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	// Armazenar vetores e projecoes no estado
	appState.SetNoteVector("notes/a.md", []float32{0.9, 0.1, 0.0}, "Nota A")
	appState.SetNoteVector("notes/b.md", []float32{0.1, 0.9, 0.0}, "Nota B")
	appState.SetNoteVector("notes/c.md", []float32{0.0, 0.1, 0.9}, "Nota C")

	appState.SetNoteProjections(map[string][]float64{
		"notes/a.md": {10, 10},
		"notes/b.md": {90, 10},
		"notes/c.md": {50, 90},
	})

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: docsDir, OllamaHost: "http://localhost:11434", OllamaModel: "nomic-embed-text"},
		State: appState,
	}

	// Caso 1: Sem Ollama (erro esperado ao gerar embedding da query)
	body := strings.NewReader(`{"query":"teste"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/graph/query-point", body)
	w := httptest.NewRecorder()
	ctx.HandleGraphQueryPoint(w, req)

	// Deve falhar porque nao tem Ollama, mas nao deve panicar
	if w.Code == 0 {
		t.Error("HandleGraphQueryPoint deveria ter respondido (mesmo com erro)")
	}
}

func TestHandleGraphQueryPoint_MethodValidation(t *testing.T) {
	tmpDir := t.TempDir()
	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{},
		State: appState,
	}

	// GET nao deve ser aceito
	req := httptest.NewRequest(http.MethodGet, "/api/graph/query-point", nil)
	w := httptest.NewRecorder()
	ctx.HandleGraphQueryPoint(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Esperado 405, obteve %d", w.Code)
	}
}

func TestHandleGraphQueryPoint_EmptyQuery(t *testing.T) {
	tmpDir := t.TempDir()
	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{},
		State: appState,
	}

	body := strings.NewReader(`{"query":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/graph/query-point", body)
	w := httptest.NewRecorder()
	ctx.HandleGraphQueryPoint(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Esperado 400 para query vazia, obteve %d", w.Code)
	}
}

func TestHandleGraphQueryPoint_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   nil,
		State: appState,
	}

	body := strings.NewReader(`{"query":"teste"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/graph/query-point", body)
	w := httptest.NewRecorder()
	ctx.HandleGraphQueryPoint(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Esperado 503 sem config, obteve %d", w.Code)
	}
}

func TestHandleKnowledgeMap_MultipleNotes(t *testing.T) {
	// Verifica cluster labeling com multiplas notas (exercita batch DisjunctionQuery P4.1)
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	indexDir := filepath.Join(tmpDir, "index.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := ingest.NewAppState(&config.AppConfig{StateDir: tmpDir})
	defer appState.Close()

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: docsDir},
		State: appState,
		Index: search.GetIndex(),
	}

	// Criar 3 notas com #embed, tags cacheadas, titulos armazenados
	notes := []struct {
		filename string
		content  string
		title    string
		vector   []float32
	}{
		{"notes/a.md", "# Alpha\nConteudo sobre alpha #embed", "Alpha", []float32{1.0, 0.0, 0.0}},
		{"notes/b.md", "# Beta\nConteudo sobre beta #embed", "Beta", []float32{0.0, 1.0, 0.0}},
		{"notes/c.md", "# Gamma\nConteudo sobre gamma #embed", "Gamma", []float32{0.0, 0.0, 1.0}},
	}

	for _, n := range notes {
		os.WriteFile(filepath.Join(docsDir, n.filename), []byte(n.content), 0644)
		search.IndexDocument(n.filename, map[string]interface{}{
			"arquivo": n.filename,
			"texto":   n.content,
		})
		appState.SetFileTags(n.filename, []string{"embed"})
		appState.SetNoteVector(n.filename, n.vector, n.title)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/graph/map", nil)
	w := httptest.NewRecorder()
	ctx.HandleKnowledgeMap(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperado 200, obteve %d", w.Code)
	}

	var resp KnowledgeMapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.Notes) != 3 {
		t.Fatalf("Esperava 3 notas, obteve %d", len(resp.Notes))
	}

	// Verificar que cada nota tem cluster atribuido
	for _, n := range resp.Notes {
		if n.ClusterID < 0 {
			t.Errorf("Nota %s sem cluster atribuido", n.ID)
		}
	}

	// Verificar que clusters foram gerados
	if len(resp.Clusters) == 0 {
		t.Error("Nenhum cluster gerado para 3 notas")
	}
}
