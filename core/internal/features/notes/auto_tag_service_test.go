package notes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/config"
	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
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

func TestApplyDecayTags(t *testing.T) {
	store, svc, cfg, cleanup := setupTestStoreAndSvc(t)
	defer cleanup()

	now := time.Now()
	
	// Create a note that is 40 days old
	oldMtime := now.Add(-45 * 24 * time.Hour)
	oldNotePath := "notes/old.md"
	oldNoteContent := "---\ntitle: Old Note\n---\nConteudo"
	store.SaveNote(oldNotePath, oldNoteContent, oldMtime.Format(time.RFC3339))
	store.SetFileMod(oldNotePath, oldMtime.Format(time.RFC3339))
	os.WriteFile(filepath.Join(cfg.DocsDir, oldNotePath), []byte(oldNoteContent), 0644)
	os.Chtimes(filepath.Join(cfg.DocsDir, oldNotePath), oldMtime, oldMtime)

	// Create a note that is 10 days old (with #stale tag, to test removal)
	youngMtime := now.Add(-10 * 24 * time.Hour)
	youngNotePath := "notes/young.md"
	youngNoteContent := "---\ntitle: Young Note\ntags: [stale]\n---\nConteudo"
	store.SaveNote(youngNotePath, youngNoteContent, youngMtime.Format(time.RFC3339))
	store.SetFileMod(youngNotePath, youngMtime.Format(time.RFC3339))
	store.SetFileTags(youngNotePath, []string{"stale"})
	os.WriteFile(filepath.Join(cfg.DocsDir, youngNotePath), []byte(youngNoteContent), 0644)
	os.Chtimes(filepath.Join(cfg.DocsDir, youngNotePath), youngMtime, youngMtime)

	// Configure rules: 30 dias -> stale
	rules := []domain.AutoTagRule{
		{Days: 30, Tag: "stale"},
	}
	rulesJSON, _ := json.Marshal(rules)
	store.SetSetting("auto_tag_decay_config", string(rulesJSON))

	// Execute service
	err := ApplyDecayTags(store, svc)
	if err != nil {
		t.Fatalf("ApplyDecayTags falhou: %v", err)
	}

	// Verify old note received tag
	tagsOld, _ := store.GetFileTags(oldNotePath)
	if len(tagsOld) != 1 || tagsOld[0] != "stale" {
		t.Errorf("Esperava a tag 'stale' na nota velha, got %v", tagsOld)
	}
	
	// Verify frontmatter was updated for old note
	contentOld, _ := store.GetNote(oldNotePath)
	if !strings.Contains(contentOld, "stale") {
		t.Errorf("Frontmatter da nota velha não contem a tag stale")
	}

	// Verify young note had tag removed
	tagsYoung, _ := store.GetFileTags(youngNotePath)
	if len(tagsYoung) != 0 {
		t.Errorf("Esperava que a tag 'stale' fosse removida da nota jovem, got %v", tagsYoung)
	}

	// Verify frontmatter was updated for young note
	contentYoung, _ := store.GetNote(youngNotePath)
	if strings.Contains(contentYoung, "stale") {
		t.Errorf("Frontmatter da nota jovem ainda contem a tag stale: %s", contentYoung)
	}
}
