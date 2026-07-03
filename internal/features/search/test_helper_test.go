package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ton618/internal/core/config"
	"ton618/internal/core/db"
)

// newTestContext cria um HandlerContext isolado para testes de busca.
func newTestContext(t *testing.T) *HandlerContext {
	t.Helper()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test.db")

	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := &config.AppConfig{
		DocsDir: t.TempDir(),
	}

	return NewHandlerContext(cfg, store)
}

// saveTestNote cria uma nota de teste no banco e no disco.
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
