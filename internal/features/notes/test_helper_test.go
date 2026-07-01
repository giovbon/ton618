package notes

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ton618/internal/core/config"
	"ton618/internal/core/db"
	"ton618/internal/core/services"
	"ton618/internal/watcher"
)

// newTestContext cria um HandlerContext isolado para testes.
func newTestContext(t *testing.T) *HandlerContext {
	t.Helper()
	docsDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test.db")

	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := &config.AppConfig{
		DocsDir: docsDir,
	}

	w := watcher.NewWatcher(cfg, store)
	notesSvc := NewNoteService(store, store, store, store, store, store, docsDir)
	backupSvc := services.NewBackupService(store, store, docsDir)

	ctx := NewHandlerContext(cfg, store, w, notesSvc, backupSvc)

	return ctx
}

// saveTestNote cria uma nota de teste no disco, no banco (notes table) e registra metadados.
func saveTestNote(t *testing.T, ctx *HandlerContext, filename, content, tags string) {
	t.Helper()
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
	ctx.Store.SaveNote(filename, content, time.Now().Format(time.RFC3339))
	if tags != "" {
		tagList := strings.Split(tags, ",")
		ctx.Store.SetFileTags(filename, tagList)
	}
	ctx.Store.SetFileMod(filename, time.Now().Format(time.RFC3339))
}

// createMinimalPDF escreve um PDF valido com o texto informado.
func createMinimalPDF(t *testing.T, path, text string) {
	t.Helper()
	paddedText := text
	if len(paddedText) < 11 {
		paddedText = paddedText + strings.Repeat(" ", 11-len(paddedText))
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	content := fmt.Sprintf("%%PDF-1.4\n"+
		"1 0 obj\n"+
		"<< /Type /Catalog /Pages 2 0 R >>\n"+
		"endobj\n"+
		"2 0 obj\n"+
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n"+
		"endobj\n"+
		"3 0 obj\n"+
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]\n"+
		"   /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\n"+
		"endobj\n"+
		"4 0 obj\n"+
		"<< /Length 44 >>\n"+
		"stream\n"+
		"BT /F1 12 Tf 100 700 Td (%s) Tj ET\n"+
		"endstream\n"+
		"endobj\n"+
		"5 0 obj\n"+
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\n"+
		"endobj\n"+
		"xref\n"+
		"0 6\n"+
		"0000000000 65535 f \n"+
		"0000000009 00000 n \n"+
		"0000000058 00000 n \n"+
		"0000000115 00000 n \n"+
		"0000000266 00000 n \n"+
		"0000000363 00000 n \n"+
		"trailer\n"+
		"<< /Size 6 /Root 1 0 R >>\n"+
		"startxref\n"+
		"442\n"+
		"%%EOF", paddedText)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("escrever PDF: %v", err)
	}
}

// Wrapper local para manter compatibilidade com testes legados
func isNoteOrPdf(path string) bool {
	return IsNoteOrPdf(path)
}

// Função local para manter compatibilidade com testes legados
func countFilesInZip(zipPath string) int {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0
	}
	defer r.Close()
	return len(r.File)
}
