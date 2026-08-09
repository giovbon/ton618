package db

import (
	"testing"
)

func TestScratch_PendingNotesDetails(t *testing.T) {
	dbPath := "/home/giobon/Área de trabalho/ton618/core/data/ton618.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	pending, err := store.GetPendingEmbeddingNotes(100)
	if err != nil {
		t.Fatalf("GetPendingEmbeddingNotes: %v", err)
	}

	t.Logf("=== PENDING NOTES COUNT: %d ===", len(pending))
	for i, p := range pending {
		tags, _ := store.GetFileTags(p.Filename)
		t.Logf("[%d] %s | len=%d | tags=%v", i+1, p.Filename, len(p.Content), tags)
	}
}
