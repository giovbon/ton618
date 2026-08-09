package db

import (
	"encoding/json"
	"testing"
	"unicode/utf8"
)

func TestScratch_TestAllNotesEmbedding(t *testing.T) {
	dbPath := "/home/giobon/Área de trabalho/ton618/core/data/ton618.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	allNotes, err := store.GetAllNotes()
	if err != nil {
		t.Fatalf("GetAllNotes: %v", err)
	}

	t.Logf("Checking %d total notes in DB...", len(allNotes))

	for fname := range allNotes {
		content, err := store.GetNote(fname)
		if err != nil {
			t.Errorf("Error reading note %s: %v", fname, err)
			continue
		}

		// Check UTF-8 validity of original content
		if !utf8.ValidString(content) {
			t.Errorf("INVALID UTF-8 in note: %s", fname)
		}

		// Check truncating logic (current bug: slicing bytes instead of runes)
		c := content
		if len(c) > 10000 {
			sliced := c[:10000]
			if !utf8.ValidString(sliced) {
				t.Errorf("CRITICAL BUG: Slicing bytes c[:10000] produced INVALID UTF-8 for note: %s (last byte: 0x%x)", fname, sliced[len(sliced)-1])
			}

			// Try JSON marshaling the sliced content
			type item struct {
				Filename string `json:"filename"`
				Content  string `json:"content"`
			}
			_, jsonErr := json.Marshal(item{Filename: fname, Content: sliced})
			if jsonErr != nil {
				t.Errorf("JSON MARSHAL ERROR on sliced note %s: %v", fname, jsonErr)
			}
		}

		// Check for NULL bytes or weird control characters
		for i, b := range []byte(content) {
			if b == 0 {
				t.Errorf("NULL byte in note %s at pos %d", fname, i)
			}
		}
	}
}
