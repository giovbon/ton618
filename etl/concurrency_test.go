package main

import (
	"bytes"
	"context"
	"encoding/json"
	"etl/internal/api"
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/search"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestConcurrencySaves(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "concurrency.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	testCfg := &config.AppConfig{
		DocsDir:  tmpDir,
		StateDir: tmpDir,
	}
	apiCtx := &api.HandlerContext{Cfg: testCfg, State: appState}

	numNotes := 20
	var wg sync.WaitGroup
	wg.Add(numNotes)

	errChan := make(chan error, numNotes)

	// 1. Salvar múltiplas notas em paralelo
	for i := 0; i < numNotes; i++ {
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("notes/note-%d.md", idx)
			payload := fmt.Sprintf(`{"content": "# Nota %d\nConteúdo concorrente %d"}`, idx, idx)

			req := httptest.NewRequest(http.MethodPost, "/api/file?name="+name, bytes.NewBuffer([]byte(payload)))
			w := httptest.NewRecorder()
			apiCtx.HandleFile(w, req)

			if w.Code != http.StatusOK {
				errChan <- fmt.Errorf("Erro ao salvar nota %d: status %d", idx, w.Code)
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Error(err)
	}

	// 2. Polling para garantir que todas foram indexadas (indexação é async nos handlers)
	success := false
	for attempt := 0; attempt < 20; attempt++ {
		res, _ := search.ExecuteSearch(context.Background(), "concorrente", false, 0, 100)
		if res.Hits.Total.Value == numNotes {
			success = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !success {
		res, _ := search.ExecuteSearch(context.Background(), "concorrente", false, 0, 100)
		t.Errorf("Esperado %d notas indexadas, obteve %d após timeout", numNotes, res.Hits.Total.Value)
	}

	// 3. Deletar todas em paralelo
	wg.Add(numNotes)
	for i := 0; i < numNotes; i++ {
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("notes/note-%d.md", idx)
			req := httptest.NewRequest(http.MethodDelete, "/api/file?name="+name, nil)
			w := httptest.NewRecorder()
			apiCtx.HandleFile(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Erro ao deletar nota %d: status %d", idx, w.Code)
			}
		}(i)
	}
	wg.Wait()

	// 4. Polling para garantir que todas sumiram
	gone := false
	for attempt := 0; attempt < 20; attempt++ {
		res, _ := search.ExecuteSearch(context.Background(), "concorrente", false, 0, 100)
		if res.Hits.Total.Value == 0 {
			gone = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !gone {
		res, _ := search.ExecuteSearch(context.Background(), "concorrente", false, 0, 100)
		t.Errorf("Notas ainda presentes no índice após deleção: %d", res.Hits.Total.Value)
	}
}

func TestEmbeddingsDisabled(t *testing.T) {
	// Setup com embedding desabilitado
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "simple.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	testCfg := &config.AppConfig{
		DocsDir:        tmpDir,
		SemanticEnable: false, // DESABILITADO
	}
	apiCtx := &api.HandlerContext{Cfg: testCfg, State: appState}

	// Salvar uma nota
	name := "notes/simple.md"
	payload := `{"content": "# Simples\nBusca puramente Bleve."}`
	req := httptest.NewRequest(http.MethodPost, "/api/file?name="+name, bytes.NewBuffer([]byte(payload)))
	w := httptest.NewRecorder()
	apiCtx.HandleFile(w, req)

	// Esperar indexação
	time.Sleep(300 * time.Millisecond)

	// Buscar - deve funcionar mesmo sem embeddings
	res, err := search.ExecuteSearch(context.Background(), "puramente", false, 0, 10)
	if err != nil {
		t.Fatalf("Erro na busca Bleve pura: %v", err)
	}
	if res.Hits.Total.Value == 0 {
		t.Error("Deveria ter encontrado a nota via Bleve mesmo com Embeddings desabilitados")
	}
}

func TestBoundaryNotes(t *testing.T) {
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "boundary.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	testCfg := &config.AppConfig{DocsDir: tmpDir}
	apiCtx := &api.HandlerContext{Cfg: testCfg, State: appState}

	t.Run("EmptyNote", func(t *testing.T) {
		name := "notes/empty.md"
		payload := `{"content": ""}`
		req := httptest.NewRequest(http.MethodPost, "/api/file?name="+name, bytes.NewBuffer([]byte(payload)))
		w := httptest.NewRecorder()
		apiCtx.HandleFile(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Falha ao salvar nota vazia: %d", w.Code)
		}
	})

	t.Run("VeryLargeNote", func(t *testing.T) {
		name := "notes/large.md"
		largeContent := "# Large\n"
		for i := 0; i < 5000; i++ {
			largeContent += "Repetindo conteúdo para criar uma nota grande de muitos fragmentos... "
		}

		payload := map[string]string{"content": largeContent}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/file?name="+name, bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		apiCtx.HandleFile(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Falha ao salvar nota grande: %d", w.Code)
		}

		// Esperar indexação pesada
		time.Sleep(1 * time.Second)

		res, _ := search.ExecuteSearch(context.Background(), "fragmentos", false, 0, 10)
		if res.Hits.Total.Value == 0 {
			t.Error("Nota grande não foi indexada corretamente")
		}
	})
}
