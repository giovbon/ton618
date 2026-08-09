package db

import (
	"testing"
)

func TestScratch_InspectNotes(t *testing.T) {
	dbPath := "/home/giobon/Área de trabalho/ton618/core/data/ton618.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	noteNames := []string{
		"notes/img_1786140118423_image.png.md",
		"notes/corsa-97-wind-plan-gastos.md",
		"notes/mapa-deliverys-etc.md",
		"notes/Calendário Acadêmico 2026-2.md",
		"notes/gastos apê.md",
		"notes/plan-pc-gamer-2026.md",
		"notes/Manual de Direito Defensivo.md",
	}

	for _, fname := range noteNames {
		content, err := store.GetNote(fname)
		tags, _ := store.GetFileTags(fname)
		embeddable := store.IsNoteEmbeddable(fname, tags)
		t.Logf("=== NOTE: %s ===", fname)
		t.Logf("  err=%v embeddable=%t tags=%v len=%d", err, embeddable, tags, len(content))
		if len(content) < 500 {
			t.Logf("  content=%q", content)
		} else {
			t.Logf("  content (first 200)=%q", content[:200])
		}
	}
}
