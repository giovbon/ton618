package notes

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ton618/internal/core/db"
	"ton618/internal/search"
)

func newStoreAndSvc(t *testing.T) (*db.Store, *NoteService, string) {
	t.Helper()
	docsDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test.db")

	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	svc := NewNoteService(store, store, store, store, store, store, docsDir)
	return store, svc, docsDir
}

func saveNote(t *testing.T, svc *NoteService, filename, content string) {
	t.Helper()
	if err := svc.Save(filename, content, nil); err != nil {
		t.Fatalf("Save %s: %v", filename, err)
	}
}

func searchNote(t *testing.T, store *db.Store, query string) *search.SearchResults {
	t.Helper()
	getBL := func(string) int { return 0 }
	getSW := func(string) float64 { return 1.0 }
	results, err := search.Search(context.Background(), store, query, 0, 10, getBL, getSW)
	if err != nil {
		t.Fatalf("Search(%q): %v", query, err)
	}
	return results
}

func noteFound(results *search.SearchResults, arquivo string) bool {
	for _, hit := range results.Hits {
		if hit.Doc.Arquivo == arquivo {
			return true
		}
	}
	return false
}

// ── Testes ─────────────────────────────────────────────────────────

func TestSaveReindexSearch_SaveThenSearch(t *testing.T) {
	store, svc, _ := newStoreAndSvc(t)

	content := "---\ntags: [golang, teste]\n---\n\n# Introducao ao Go\n\nGo eh uma linguagem compilada e concorrente.\n\n## Variaveis\n\nEm Go declaramos variaveis com var ou :=.\n\n## Funcoes\n\nFuncoes em Go retornam multiplos valores.\n\n[[outra-nota]]\n"
	saveNote(t, svc, "nota-teste.md", content)

	// Documento foi indexado
	docs, err := store.GetDocumentsByFile("notes/nota-teste.md")
	if err != nil {
		t.Fatalf("GetDocumentsByFile: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("nenhum documento indexado")
	}

	// Tags foram indexadas
	tags, err := store.GetFileTags("notes/nota-teste.md")
	if err != nil {
		t.Fatalf("GetFileTags: %v", err)
	}
	foundGolang := false
	for _, tag := range tags {
		if tag == "golang" {
			foundGolang = true
			break
		}
	}
	if !foundGolang {
		t.Fatalf("tag 'golang' nao encontrada entre %v", tags)
	}

	// Link foi indexado
	links, err := store.GetLinks("notes/nota-teste.md")
	if err != nil {
		t.Fatalf("GetLinks: %v", err)
	}
	if len(links) == 0 || links[0] != "notes/outra-nota.md" {
		t.Fatalf("link esperado 'notes/outra-nota.md', got %v", links)
	}

	// Busca por "concorrente" encontra a nota
	results := searchNote(t, store, "concorrente")
	if results.Total == 0 {
		t.Fatal("busca por 'concorrente' nao retornou resultados")
	}
	if !noteFound(results, "notes/nota-teste.md") {
		t.Fatal("nota nao encontrada nos resultados")
	}

	t.Logf("OK: %d resultados, nota com score %.2f", results.Total, results.Hits[0].FinalScore)
}

func TestSaveReindexSearch_MultipleNotes(t *testing.T) {
	store, svc, docsDir := newStoreAndSvc(t)

	os.MkdirAll(filepath.Join(docsDir, "notes"), 0755)
	saveNote(t, svc, "golang.md", "---\ntags: [programacao, go]\n---\n# Golang\n\nGo eh uma linguagem do Google com goroutines.")
	saveNote(t, svc, "python.md", "---\ntags: [programacao, python]\n---\n# Python\n\nPython eh uma linguagem interpretada e versatil.")
	saveNote(t, svc, "receita.md", "---\ntags: [culinaria]\n---\n# Bolo de Cenoura\n\nIngredientes: cenoura, farinha, acucar, chocolate.")

	// Busca "linguagem" encontra Go e Python
	r := searchNote(t, store, "linguagem")
	if r.Total < 2 {
		t.Fatalf("esperado >=2 para 'linguagem', got %d", r.Total)
	}

	// Busca "cenoura" encontra apenas receita
	r = searchNote(t, store, "cenoura")
	if r.Total != 1 {
		t.Fatalf("esperado 1 para 'cenoura', got %d", r.Total)
	}

	// Busca por tag encontra as 2 de programacao
	r = searchNote(t, store, "tags:programacao")
	if r.Total < 2 {
		t.Fatalf("esperado >=2 para 'tags:programacao', got %d", r.Total)
	}

	t.Logf("OK: linguagem=%d cenoura=%d tag_programacao=%d", r.Total, r.Total, r.Total)
}

func TestSaveReindexSearch_UpdateAndReSearch(t *testing.T) {
	store, svc, _ := newStoreAndSvc(t)

	saveNote(t, svc, "nota-update.md", "# Nota Original\nConteudo sobre gatos.")

	// Termo original existe
	r := searchNote(t, store, "gatos")
	if r.Total == 0 {
		t.Fatal("'gatos' deveria existir apos save")
	}

	// Atualiza com conteudo diferente
	time.Sleep(10 * time.Millisecond)
	saveNote(t, svc, "nota-update.md", "# Nota Atualizada\nConteudo sobre caes e cachorros.")

	// Termo antigo NAO deve mais aparecer
	r = searchNote(t, store, "gatos")
	if r.Total > 0 {
		t.Error("'gatos' nao deveria aparecer apos atualizacao")
	}

	// Termo novo deve aparecer
	r = searchNote(t, store, "caes")
	if r.Total == 0 {
		t.Fatal("'caes' deveria aparecer apos atualizacao")
	}

	t.Logf("OK: gatos=%d caes=%d", 0, r.Total)
}

func TestSaveReindexSearch_DeleteRemovesFromIndex(t *testing.T) {
	store, svc, docsDir := newStoreAndSvc(t)

	os.MkdirAll(filepath.Join(docsDir, "notes"), 0755)
	saveNote(t, svc, "nota-delete.md", "# Nota Para Deletar\nConteudo temporario.")

	r := searchNote(t, store, "temporario")
	if r.Total == 0 {
		t.Fatal("'temporario' deveria existir antes do delete")
	}

	if err := svc.Delete("nota-delete.md"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	r = searchNote(t, store, "temporario")
	if r.Total > 0 {
		t.Error("'temporario' nao deveria aparecer apos delete")
	}

	t.Log("OK: nota removida do indice apos delecao")
}
