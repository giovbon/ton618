package db

import (
	"fmt"
	"testing"
)

func TestScratch_FindProblematicNote(t *testing.T) {
	dbPath := "/home/giobon/Área de trabalho/ton618/core/data/ton618.db"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// 1. Ver o schema das tabelas de embeddings
	rows, err := store.DB.Query(`
		SELECT name FROM sqlite_master
		WHERE type='table'
		ORDER BY name
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	t.Log("=== TABLES ===")
	for rows.Next() {
		var name string
		rows.Scan(&name)
		t.Log(" -", name)
	}
	rows.Close()

	// 2. Conteúdo de todas as colunas da tabela de chunks
	rows2, err := store.DB.Query(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name LIKE '%chunk%' OR name LIKE '%embed%'
		ORDER BY name
	`)
	if err == nil {
		t.Log("=== EMBED/CHUNK TABLES ===")
		for rows2.Next() {
			var name string
			rows2.Scan(&name)
			t.Log(" -", name)
		}
		rows2.Close()
	}

	// 3. Notas pendentes de embedding (sem chunks)
	t.Log("=== PENDING EMBEDDING NOTES ===")
	pending, err := store.GetPendingEmbeddingNotes(100)
	if err != nil {
		t.Errorf("GetPendingEmbeddingNotes: %v", err)
	} else {
		for _, p := range pending {
			t.Logf("  filename=%s content_len=%d", p.Filename, len(p.Content))
		}
		t.Logf("Total pending: %d", len(pending))
	}

	// 4. Status geral
	t.Log("=== EMBEDDING STATUS ===")
	status, err := store.GetEmbeddingStatus()
	if err != nil {
		t.Errorf("GetEmbeddingStatus: %v", err)
	} else {
		t.Logf("  TotalNotes=%d IndexedNotes=%d PendingNotes=%d StaleNotes=%d",
			status.TotalNotes, status.IndexedNotes, status.PendingNotes, status.StaleNotes)
	}

	// 5. Notas que têm chunks (indexadas) — verificar consistência
	t.Log("=== EMBEDDED FILES ===")
	embedded, err := store.GetEmbeddedFiles()
	if err != nil {
		t.Errorf("GetEmbeddedFiles: %v", err)
	} else {
		for fname := range embedded {
			t.Logf("  embedded: %s", fname)
		}
		t.Logf("Total embedded: %d", len(embedded))
	}

	// 6. Verificar notas com conteúdo muito grande (>10000 chars, truncadas pelo handler)
	t.Log("=== NOTES WITH LARGE CONTENT (>10000 chars) ===")
	allNotes, _ := store.GetAllNotes()
	for fname := range allNotes {
		content, err := store.GetNote(fname)
		if err != nil {
			continue
		}
		if len(content) > 10000 {
			t.Logf("  LARGE: %s (content_len=%d)", fname, len(content))
		}
	}

	// 7. Verificar notas com conteúdo vazio ou suspeito
	t.Log("=== NOTES WITH EMPTY OR VERY SHORT CONTENT ===")
	for fname := range allNotes {
		content, err := store.GetNote(fname)
		if err != nil || len(content) < 5 {
			t.Logf("  SUSPECT: %s (content=%q err=%v)", fname, content, err)
		}
	}

	// 8. Verificar se há notas com encoding inválido no conteúdo
	t.Log("=== CHECKING FOR NOTES WITH BINARY/INVALID CONTENT ===")
	for fname := range allNotes {
		content, err := store.GetNote(fname)
		if err != nil {
			continue
		}
		hasBinary := false
		for _, b := range []byte(content[:min(len(content), 500)]) {
			if b < 0x09 || (b >= 0x0E && b <= 0x1F && b != 0x1B) {
				hasBinary = true
				break
			}
		}
		if hasBinary {
			t.Logf("  BINARY: %s (first 100 bytes: %q)", fname, content[:min(len(content), 100)])
		}
	}

	// 9. Procurar por notas que têm nome tipo "img_" (PNG salvo como nota)
	t.Log("=== NOTES NAMED LIKE IMAGES (img_*.md) ===")
	for fname := range allNotes {
		if len(fname) > 4 && fname[0:4] == "note" {
			content, _ := store.GetNote(fname)
			t.Logf("  img-like: %s tags check, content_len=%d", fname, len(content))
		}
	}

	// 10. Verificar tags de cada nota para ver IsNoteEmbeddable
	t.Log("=== EMBEDDABILITY CHECK PER NOTE ===")
	for fname := range allNotes {
		tags, _ := store.GetFileTags(fname)
		embeddable := store.IsNoteEmbeddable(fname, tags)
		if !embeddable {
			t.Logf("  NOT-EMBEDDABLE: %s tags=%v", fname, tags)
		}
	}

	fmt.Println("done") // satisfaz compilador
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
