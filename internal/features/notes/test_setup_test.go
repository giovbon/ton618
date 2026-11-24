package notes

import (
	"os"
	"path/filepath"
	"testing"

	"ton618/core/internal/core/config"
	"ton618/core/internal/core/db"
)

func setupTestStoreAndSvc(t *testing.T) (*db.Store, *NoteService, *config.AppConfig, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	docsDir := filepath.Join(dir, "docs")
	os.MkdirAll(docsDir, 0755)
	os.MkdirAll(filepath.Join(docsDir, "notes"), 0755)

	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("erro ao criar store: %v", err)
	}

	cfg := &config.AppConfig{
		DocsDir: docsDir,
	}

	svc := NewNoteService(store, store, store, store, store, store, docsDir)

	cleanup := func() {
		store.Close()
	}

	return store, svc, cfg, cleanup
}
