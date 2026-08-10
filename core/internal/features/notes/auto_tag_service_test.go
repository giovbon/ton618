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
	modified, err := ApplyDecayTags(store, svc)
	if err != nil {
		t.Fatalf("ApplyDecayTags falhou: %v", err)
	}
	if modified != 2 {
		t.Errorf("Esperava 2 notas modificadas, got %d", modified)
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

func TestApplyDecayTagsUsesLastInteracted(t *testing.T) {
	store, svc, cfg, cleanup := setupTestStoreAndSvc(t)
	defer cleanup()

	now := time.Now()
	oldMtime := now.Add(-45 * 24 * time.Hour)    // editada há 45 dias
	recentOpen := now.Add(-2 * 24 * time.Hour)   // aberta há 2 dias
	longAgoOpen := now.Add(-60 * 24 * time.Hour) // aberta há 60 dias

	createNote := func(path, content string, mtime time.Time) {
		store.SaveNote(path, content, mtime.Format(time.RFC3339))
		store.SetFileMod(path, mtime.Format(time.RFC3339))
		os.WriteFile(filepath.Join(cfg.DocsDir, path), []byte(content), 0644)
		os.Chtimes(filepath.Join(cfg.DocsDir, path), mtime, mtime)
	}
	setLastInteracted := func(path string, ts time.Time) {
		_, err := store.DB.Exec(`INSERT OR REPLACE INTO popularity (arquivo, count, weight, last_interacted_at) VALUES (?, 1, 1.0, ?)`, path, ts.Format(time.RFC3339))
		if err != nil {
			t.Fatalf("erro ao inserir popularity: %v", err)
		}
	}

	// Aberta recentemente (2d) → NÃO deve receber a tag, mesmo com edição antiga (45d)
	recentPath := "notes/recent.md"
	createNote(recentPath, "---\ntitle: Recent\n---\nConteudo", oldMtime)
	setLastInteracted(recentPath, recentOpen)

	// Nunca aberta (sem registro em popularity) → fallback para mtime (45d) → deve receber a tag
	neverPath := "notes/never.md"
	createNote(neverPath, "---\ntitle: Never\n---\nConteudo", oldMtime)

	// Aberta há muito tempo (60d) → deve receber a tag
	longAgoPath := "notes/longago.md"
	createNote(longAgoPath, "---\ntitle: LongAgo\n---\nConteudo", oldMtime)
	setLastInteracted(longAgoPath, longAgoOpen)

	rules := []domain.AutoTagRule{{Days: 30, Tag: "stale"}}
	rulesJSON, _ := json.Marshal(rules)
	store.SetSetting("auto_tag_decay_config", string(rulesJSON))

	modified, err := ApplyDecayTags(store, svc)
	if err != nil {
		t.Fatalf("ApplyDecayTags falhou: %v", err)
	}
	if modified != 2 {
		t.Errorf("Esperava 2 notas modificadas, got %d", modified)
	}

	// recent.md: NÃO deve ter a tag stale (aberta há 2 dias)
	tagsRecent, _ := store.GetFileTags(recentPath)
	if len(tagsRecent) != 0 {
		t.Errorf("recent.md não deveria ter tags (aberta recentemente), got %v", tagsRecent)
	}

	// never.md: deve ter a tag (fallback para mtime)
	tagsNever, _ := store.GetFileTags(neverPath)
	if len(tagsNever) != 1 || tagsNever[0] != "stale" {
		t.Errorf("never.md deveria ter a tag stale (fallback mtime), got %v", tagsNever)
	}

	// longago.md: deve ter a tag (não aberta há 60 dias)
	tagsLongAgo, _ := store.GetFileTags(longAgoPath)
	if len(tagsLongAgo) != 1 || tagsLongAgo[0] != "stale" {
		t.Errorf("longago.md deveria ter a tag stale, got %v", tagsLongAgo)
	}
}
