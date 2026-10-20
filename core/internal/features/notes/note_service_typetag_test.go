package notes

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ton618/core/internal/core/domain"
)

func TestEnsureTypeTags_PersistsContentDerivedType(t *testing.T) {
	store, svc, cfg, cleanup := setupTestStoreAndSvc(t)
	defer cleanup()

	now := time.Now().UTC().Format(time.RFC3339)

	// Nota legada: "type: markmap" no frontmatter, mas SEM a tag "markmap" persistida
	// e SEM "markmap" no nome do arquivo. É o caso que fazia o ícone mudar conforme
	// o conteúdo era (ou não) passado.
	legacy := "notes/legacy-nota.md"
	legacyContent := "---\ntype: markmap\n---\n# Meu Mapa\n- Tópico A\n- Tópico B"
	if err := store.SaveNote(legacy, legacyContent, now); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if err := store.SetFileMod(legacy, now); err != nil {
		t.Fatalf("SetFileMod: %v", err)
	}
	os.WriteFile(filepath.Join(cfg.DocsDir, legacy), []byte(legacyContent), 0644)

	// Nota normal — não deve ganhar tag de tipo.
	normal := "notes/normal.md"
	normalContent := "# Normal\nsem tipo"
	if err := store.SaveNote(normal, normalContent, now); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if err := store.SetFileMod(normal, now); err != nil {
		t.Fatalf("SetFileMod: %v", err)
	}
	os.WriteFile(filepath.Join(cfg.DocsDir, normal), []byte(normalContent), 0644)

	if err := svc.EnsureTypeTags(context.Background()); err != nil {
		t.Fatalf("EnsureTypeTags: %v", err)
	}

	// A nota legada agora deve ter a tag "markmap" persistida.
	tags, err := store.GetFileTags(legacy)
	if err != nil {
		t.Fatalf("GetFileTags: %v", err)
	}
	found := false
	for _, tg := range tags {
		if tg == "markmap" {
			found = true
		}
	}
	if !found {
		t.Errorf("esperava tag 'markmap' persistida em %s, got %v", legacy, tags)
	}

	// E a detecção SEM conteúdo agora é estável (mesmo tipo em qualquer lugar).
	if detected := domain.DetectNoteType(tags, legacy); detected != domain.NoteTypeMindmap {
		t.Errorf("DetectNoteType() após backfill = %v, want markmap", detected)
	}

	// A nota normal não deve ter ganho tag de tipo.
	normalTags, err := store.GetFileTags(normal)
	if err != nil {
		t.Fatalf("GetFileTags(normal): %v", err)
	}
	if len(normalTags) != 0 {
		t.Errorf("nota normal não deveria ganhar tag de tipo, got %v", normalTags)
	}
}
