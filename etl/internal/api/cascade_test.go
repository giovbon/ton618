package api

import (
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/search"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCascadeDelete(t *testing.T) {
	// 1. Setup ambiente temporário
	tmpDir, _ := os.MkdirTemp("", "cascade_test")
	defer os.RemoveAll(tmpDir)

	cfg := &config.AppConfig{
		DocsDir:       tmpDir,
		StateDir:      filepath.Join(tmpDir, "state"),
		BleveIndexDir: filepath.Join(tmpDir, "index"),
	}
	os.MkdirAll(cfg.StateDir, 0755)
	os.MkdirAll(filepath.Join(tmpDir, "notes"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "attachments"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "pdfs"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "assets"), 0755)

	state := ingest.NewAppState(cfg)
	defer state.Close()
	search.InitIndex(cfg.BleveIndexDir)
	defer search.CloseIndex()

	ctx := &HandlerContext{Cfg: cfg, State: state}

	tests := []struct {
		name       string
		noteFile   string
		linkedFile string
	}{
		{"Cascade Imagem", "notes/ocr_test.md", "attachments/image.png"},
		{"Cascade PDF", "notes/doc_test.md", "pdfs/document.pdf"},
		{"Cascade ZIP", "notes/bundle_test.md", "assets/archive.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Criar arquivo vinculado
			linkedPath := filepath.Join(tmpDir, tt.linkedFile)
			os.WriteFile(linkedPath, []byte("conteudo original"), 0644)

			// Criar nota
			notePath := filepath.Join(tmpDir, tt.noteFile)
			os.WriteFile(notePath, []byte("link: (/api/file?name="+tt.linkedFile+")"), 0644)

			// Simular que o syncer já extraiu o link
			state.SetFileLinks(tt.noteFile, []string{tt.linkedFile})

			// Executar DELETE via handler
			req := httptest.NewRequest("DELETE", "/api/file?name="+tt.noteFile, nil)
			w := httptest.NewRecorder()
			ctx.HandleFile(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Esperava status 200, obteve %d", w.Code)
			}

			// Verificar se a NOTA foi apagada
			if _, err := os.Stat(notePath); !os.IsNotExist(err) {
				t.Errorf("Nota %s não foi apagada", tt.noteFile)
			}

			// Verificar se o ARQUIVO VINCULADO foi apagado (CASCATA)
			if _, err := os.Stat(linkedPath); !os.IsNotExist(err) {
				t.Errorf("Arquivo vinculado %s não foi apagado em cascata", tt.linkedFile)
			}
		})
	}
}
