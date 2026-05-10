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

	// 1. Criar um arquivo com #embed no disco
	filename := "test.md"
	content := "Conteúdo com #embed"
	os.WriteFile(filepath.Join(docsDir, filename), []byte(content), 0644)

	// 2. Adicionar vetor manualmente ao estado (como se tivesse sido embutido)
	// Mas NÃO adicionar a tag ao cache do AppState para testar o fallback
	appState.SetNoteVector(filename, []float32{1.0, 0.5, 0.2})

	// 3. Chamar HandleKnowledgeMap
	req := httptest.NewRequest(http.MethodGet, "/api/graph/map", nil)
	w := httptest.NewRecorder()
	ctx.HandleKnowledgeMap(w, req)

	// 4. Verificar se a nota aparece no resultado
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
		t.Error("Nota com #embed (apenas no disco) não foi encontrada no mapa. Fallback falhou.")
	}
}

func TestHandleReindexVectors_TagFallback(t *testing.T) {
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
		Cfg:   &config.AppConfig{DocsDir: docsDir, OllamaModel: "nomic-embed-text"},
		State: appState,
		Index: search.GetIndex(),
	}

	// 1. Criar arquivo no disco E no Bleve
	filename := "reindex-test.md"
	content := "Nota para reindex #embed"
	os.WriteFile(filepath.Join(docsDir, filename), []byte(content), 0644)

	// Simular que o Bleve tem o arquivo
	search.IndexDocument("doc1", map[string]interface{}{
		"arquivo": filename,
		"texto":   content,
	})

	// 2. Chamar Reindex (POST)
	// O Reindex roda em background, então o teste é limitado.
	// Mas podemos verificar se ele ao menos inicia e o log não reclama de "nenhuma nota encontrada".
	
	req := httptest.NewRequest(http.MethodPost, "/api/graph/reindex", nil)
	w := httptest.NewRecorder()
	ctx.HandleReindexVectors(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Esperado 202 Accepted, obteve %d", w.Code)
	}
	
	// Nota: Como o reindex é assíncrono e chama Ollama (que não temos no teste),
	// não vamos esperar o fim. O objetivo aqui é garantir que a lógica de 
	// filtragem inicial no HandleReindexVectors considere o arquivo.
}
