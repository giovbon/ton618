package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore falhou: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// ── NewStore ───────────────────────────────────────────────────

func TestNewStore_CriaBanco(t *testing.T) {
	path := filepath.Join(t.TempDir(), "novo.db")
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore falhou: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("arquivo do banco nao foi criado")
	}
}

func TestNewStore_CaminhoInvalido(t *testing.T) {
	_, err := NewStore("/proc/nao_posso_escrever/nova.db")
	if err == nil {
		t.Fatal("esperado erro para caminho invalido")
	}
}

func TestClose_MultiplasVezes(t *testing.T) {
	s := newTestStore(t)
	s.Close()
	// Fechar de novo nao deve panic
	s.Close()
}

// ── TagsToSlice / SliceToTags ──────────────────────────────────

func TestTagsToSlice_Normal(t *testing.T) {
	result := TagsToSlice("go,web,test")
	expected := []string{"go", "web", "test"}
	if len(result) != len(expected) {
		t.Fatalf("esperado %d tags, got %d", len(expected), len(result))
	}
	for i := range expected {
		if result[i] != expected[i] {
			t.Fatalf("tag[%d]: esperado %q, got %q", i, expected[i], result[i])
		}
	}
}

func TestTagsToSlice_Vazio(t *testing.T) {
	if r := TagsToSlice(""); r != nil {
		t.Fatalf("esperado nil para string vazia, got %v", r)
	}
}

func TestSliceToTags_Normal(t *testing.T) {
	r := SliceToTags([]string{"go", "web"})
	if r != "go,web" {
		t.Fatalf("esperado 'go,web', got %q", r)
	}
}

func TestSliceToTags_Vazio(t *testing.T) {
	r := SliceToTags([]string{})
	if r != "" {
		t.Fatalf("esperado '' para slice vazio, got %q", r)
	}
}

// ── InsertDocument / GetDocument / DeleteDocument ──────────────

func TestInsertDocument_EObtem(t *testing.T) {
	s := newTestStore(t)

	doc := Document{
		ID: "test-1", Tipo: "md", Arquivo: "notes/test.md",
		Secao: "Geral", Texto: "conteudo de teste",
		Tags: "go,test", Pagina: 0, Ordem: 1,
		Timestamp: "2025-01-01T00:00:00Z", CreatedAt: "2025-01-01T00:00:00Z",
		Hash: "abc123",
	}
	if err := s.InsertDocument(doc); err != nil {
		t.Fatalf("InsertDocument falhou: %v", err)
	}

	got, err := s.GetDocument("test-1")
	if err != nil {
		t.Fatalf("GetDocument falhou: %v", err)
	}
	if got == nil {
		t.Fatal("documento nao encontrado")
	}
	if got.ID != "test-1" {
		t.Fatalf("ID: esperado 'test-1', got %q", got.ID)
	}
	if got.Texto != "conteudo de teste" {
		t.Fatalf("Texto: esperado 'conteudo de teste', got %q", got.Texto)
	}
	if got.Tags != "go,test" {
		t.Fatalf("Tags: esperado 'go,test', got %q", got.Tags)
	}
}

func TestGetDocument_Inexistente(t *testing.T) {
	s := newTestStore(t)
	doc, err := s.GetDocument("id-inexistente")
	if err != nil {
		t.Fatalf("GetDocument falhou: %v", err)
	}
	if doc != nil {
		t.Fatal("esperado nil para documento inexistente")
	}
}

func TestInsertDocument_Replace(t *testing.T) {
	s := newTestStore(t)

	s.InsertDocument(Document{ID: "rep1", Texto: "versao 1", Timestamp: "t1"})
	s.InsertDocument(Document{ID: "rep1", Texto: "versao 2", Timestamp: "t2"})

	doc, _ := s.GetDocument("rep1")
	if doc.Texto != "versao 2" {
		t.Fatalf("esperado 'versao 2', got %q", doc.Texto)
	}
}

func TestDeleteDocument_Remove(t *testing.T) {
	s := newTestStore(t)
	s.InsertDocument(Document{ID: "del1", Texto: "para deletar", Timestamp: "t1"})

	if err := s.DeleteDocument("del1"); err != nil {
		t.Fatalf("DeleteDocument falhou: %v", err)
	}

	doc, _ := s.GetDocument("del1")
	if doc != nil {
		t.Fatal("documento deveria ter sido deletado")
	}
}

func TestDeleteDocumentsByFile(t *testing.T) {
	s := newTestStore(t)
	s.InsertDocument(Document{ID: "f1", Arquivo: "notes/test.md", Texto: "a", Timestamp: "t1"})
	s.InsertDocument(Document{ID: "f2", Arquivo: "notes/test.md", Texto: "b", Timestamp: "t2"})
	s.InsertDocument(Document{ID: "f3", Arquivo: "notes/outro.md", Texto: "c", Timestamp: "t3"})

	if err := s.DeleteDocumentsByFile("notes/test.md"); err != nil {
		t.Fatalf("DeleteDocumentsByFile falhou: %v", err)
	}

	if count := s.GetDocumentCount(); count != 1 {
		t.Fatalf("esperado 1 documento restante, got %d", count)
	}
}

// ── GetDocumentsByFile / GetAllDocuments / GetDocumentCount / GetDistinctFiles ──

func TestGetDocumentsByFile_Ordenado(t *testing.T) {
	s := newTestStore(t)
	s.InsertDocument(Document{ID: "b", Arquivo: "notes/test.md", Ordem: 2, Texto: "segundo", Timestamp: "t1"})
	s.InsertDocument(Document{ID: "a", Arquivo: "notes/test.md", Ordem: 1, Texto: "primeiro", Timestamp: "t2"})

	docs, err := s.GetDocumentsByFile("notes/test.md")
	if err != nil {
		t.Fatalf("GetDocumentsByFile falhou: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("esperado 2 documentos, got %d", len(docs))
	}
	if docs[0].Ordem != 1 || docs[1].Ordem != 2 {
		t.Fatal("documentos deveriam vir ordenados por ordem ASC")
	}
}

func TestGetDocumentsByFile_Inexistente(t *testing.T) {
	s := newTestStore(t)
	docs, err := s.GetDocumentsByFile("notes/nao-existe.md")
	if err != nil {
		t.Fatalf("GetDocumentsByFile falhou: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("esperado slice vazio, got %d", len(docs))
	}
}

func TestGetDocumentCount_Zero(t *testing.T) {
	s := newTestStore(t)
	if c := s.GetDocumentCount(); c != 0 {
		t.Fatalf("banco vazio: esperado 0, got %d", c)
	}
}

func TestGetDocumentCount_Contagem(t *testing.T) {
	s := newTestStore(t)
	s.InsertDocument(Document{ID: "c1", Texto: "a", Timestamp: "t1"})
	s.InsertDocument(Document{ID: "c2", Texto: "b", Timestamp: "t2"})

	if c := s.GetDocumentCount(); c != 2 {
		t.Fatalf("esperado 2, got %d", c)
	}
}

func TestGetDistinctFiles_ListaArquivos(t *testing.T) {
	s := newTestStore(t)
	s.InsertDocument(Document{ID: "d1", Arquivo: "notes/a.md", Texto: "a", Timestamp: "t1"})
	s.InsertDocument(Document{ID: "d2", Arquivo: "notes/a.md", Texto: "b", Timestamp: "t2"})
	s.InsertDocument(Document{ID: "d3", Arquivo: "notes/b.md", Texto: "c", Timestamp: "t3"})

	files, err := s.GetDistinctFiles()
	if err != nil {
		t.Fatalf("GetDistinctFiles falhou: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("esperado 2 arquivos distintos, got %d", len(files))
	}
}

// ── FTS ────────────────────────────────────────────────────────

func TestIndexFTS_EBarraSearch(t *testing.T) {
	s := newTestStore(t)
	if err := s.IndexFTS("fts1", "md", "notes/test.md", "Geral", "conteudo de busca", "go"); err != nil {
		t.Fatalf("IndexFTS falhou: %v", err)
	}

	results, total, err := s.SearchFTS("busca", 0, 10)
	if err != nil {
		t.Fatalf("SearchFTS falhou: %v", err)
	}
	if total == 0 {
		t.Fatal("esperado resultados de busca")
	}
	if len(results) == 0 {
		t.Fatal("esperado pelo menos 1 resultado")
	}
	if results[0].DocID != "fts1" {
		t.Fatalf("DocID: esperado 'fts1', got %q", results[0].DocID)
	}
}

func TestSearchFTS_Vazio_RetornaTodos(t *testing.T) {
	s := newTestStore(t)
	s.IndexFTS("a", "md", "n1.md", "S1", "texto a", "")
	s.IndexFTS("b", "md", "n2.md", "S2", "texto b", "")

	results, total, err := s.SearchFTS("", 0, 10)
	if err != nil {
		t.Fatalf("SearchFTS vazio falhou: %v", err)
	}
	if total != 2 {
		t.Fatalf("esperado 2 resultados, got %d", total)
	}
	if len(results) != 2 {
		t.Fatalf("esperado 2 resultados, got %d", len(results))
	}
}

func TestSearchFTS_ComLimiteEOffset(t *testing.T) {
	s := newTestStore(t)
	s.IndexFTS("x1", "md", "n1.md", "S1", "mesmo termo de busca", "")
	s.IndexFTS("x2", "md", "n2.md", "S2", "mesmo termo de busca tbm", "")

	results, total, err := s.SearchFTS("busca", 0, 1)
	if err != nil {
		t.Fatalf("SearchFTS falhou: %v", err)
	}
	if total != 2 {
		t.Fatalf("total esperado 2, got %d", total)
	}
	if len(results) != 1 {
		t.Fatalf("com size=1, esperado 1 resultado, got %d", len(results))
	}
}

func TestDeleteFTS_Remove(t *testing.T) {
	s := newTestStore(t)
	s.IndexFTS("del", "md", "del.md", "S", "texto", "")

	if err := s.DeleteFTS("del"); err != nil {
		t.Fatalf("DeleteFTS falhou: %v", err)
	}

	_, total, _ := s.SearchFTS("texto", 0, 10)
	if total != 0 {
		t.Fatalf("apos deletar, esperado 0 resultados, got %d", total)
	}
}

func TestDeleteFTSByFile_RemovePorArquivo(t *testing.T) {
	s := newTestStore(t)
	s.IndexFTS("d1", "md", "notes/del.md", "S", "texto", "")
	s.IndexFTS("d2", "md", "notes/del.md", "S", "outro", "")
	s.IndexFTS("d3", "md", "notes/keep.md", "S", "texto", "")

	s.DeleteFTSByFile("notes/del.md")

	_, total, _ := s.SearchFTS("", 0, 100)
	if total != 1 {
		t.Fatalf("esperado 1 documento restante, got %d", total)
	}
}

// ── Popularity ─────────────────────────────────────────────────

func TestIncrementPopularity_Novo(t *testing.T) {
	s := newTestStore(t)
	s.IncrementPopularity("notes/pop.md")

	if c := s.GetPopularity("notes/pop.md"); c != 1 {
		t.Fatalf("esperado 1, got %d", c)
	}
}

func TestIncrementPopularity_Incrementa(t *testing.T) {
	s := newTestStore(t)
	s.IncrementPopularity("notes/pop.md")
	s.IncrementPopularity("notes/pop.md")
	s.IncrementPopularity("notes/pop.md")

	if c := s.GetPopularity("notes/pop.md"); c != 3 {
		t.Fatalf("esperado 3, got %d", c)
	}
}

func TestGetPopularity_Inexistente(t *testing.T) {
	s := newTestStore(t)
	if c := s.GetPopularity("notes/nao-existe.md"); c != 0 {
		t.Fatalf("esperado 0, got %d", c)
	}
}

func TestResetPopularity(t *testing.T) {
	s := newTestStore(t)
	s.IncrementPopularity("notes/reset.md")
	s.ResetPopularity("notes/reset.md")

	if c := s.GetPopularity("notes/reset.md"); c != 0 {
		t.Fatalf("esperado 0 apos reset, got %d", c)
	}
}

func TestGetAllPopularity(t *testing.T) {
	s := newTestStore(t)
	s.IncrementPopularity("notes/a.md")
	s.IncrementPopularity("notes/a.md")
	s.IncrementPopularity("notes/b.md")

	pop, err := s.GetAllPopularity()
	if err != nil {
		t.Fatalf("GetAllPopularity falhou: %v", err)
	}
	if len(pop) != 2 {
		t.Fatalf("esperado 2 entradas, got %d", len(pop))
	}
	if pop["notes/a.md"] != 2 {
		t.Fatalf("notes/a.md: esperado 2, got %d", pop["notes/a.md"])
	}
	if pop["notes/b.md"] != 1 {
		t.Fatalf("notes/b.md: esperado 1, got %d", pop["notes/b.md"])
	}
}

// ── Tags ───────────────────────────────────────────────────────

func TestSetFileTags_EObter(t *testing.T) {
	s := newTestStore(t)
	if err := s.SetFileTags("notes/tag.md", []string{"go", "web"}); err != nil {
		t.Fatalf("SetFileTags falhou: %v", err)
	}

	tags, err := s.GetFileTags("notes/tag.md")
	if err != nil {
		t.Fatalf("GetFileTags falhou: %v", err)
	}
	if len(tags) != 2 || tags[0] != "go" || tags[1] != "web" {
		t.Fatalf("esperado [go web], got %v", tags)
	}
}

func TestSetFileTags_Substitui(t *testing.T) {
	s := newTestStore(t)
	s.SetFileTags("notes/sub.md", []string{"old"})
	s.SetFileTags("notes/sub.md", []string{"new"})

	tags, _ := s.GetFileTags("notes/sub.md")
	if len(tags) != 1 || tags[0] != "new" {
		t.Fatalf("esperado [new], got %v", tags)
	}
}

func TestGetFileTags_SemTags(t *testing.T) {
	s := newTestStore(t)
	tags, err := s.GetFileTags("notes/sem-tags.md")
	if err != nil {
		t.Fatalf("GetFileTags falhou: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("esperado slice vazio, got %v", tags)
	}
}

func TestGetAllTags(t *testing.T) {
	s := newTestStore(t)
	s.SetFileTags("notes/a.md", []string{"go", "web"})
	s.SetFileTags("notes/b.md", []string{"go", "devops"})

	all, err := s.GetAllTags()
	if err != nil {
		t.Fatalf("GetAllTags falhou: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("esperado 3 tags distintas, got %v", all)
	}
}

func TestGetFilesByTag(t *testing.T) {
	s := newTestStore(t)
	s.SetFileTags("notes/a.md", []string{"go"})
	s.SetFileTags("notes/b.md", []string{"go"})
	s.SetFileTags("notes/c.md", []string{"rust"})

	files, err := s.GetFilesByTag("go")
	if err != nil {
		t.Fatalf("GetFilesByTag falhou: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("esperado 2 arquivos com tag 'go', got %d", len(files))
	}
}

func TestAddTagToFile_ERemover(t *testing.T) {
	s := newTestStore(t)

	if err := s.AddTagToFile("notes/x.md", "nova"); err != nil {
		t.Fatalf("AddTagToFile falhou: %v", err)
	}
	tags, _ := s.GetFileTags("notes/x.md")
	if len(tags) != 1 || tags[0] != "nova" {
		t.Fatalf("esperado [nova], got %v", tags)
	}

	s.RemoveTagFromFile("notes/x.md", "nova")
	tags, _ = s.GetFileTags("notes/x.md")
	if len(tags) != 0 {
		t.Fatalf("esperado vazio apos remover, got %v", tags)
	}
}

func TestAddTagToFile_Idempotente(t *testing.T) {
	s := newTestStore(t)
	s.AddTagToFile("notes/y.md", "dup")
	s.AddTagToFile("notes/y.md", "dup")

	tags, _ := s.GetFileTags("notes/y.md")
	if len(tags) != 1 {
		t.Fatalf("tag duplicada nao deveria inserir de novo, got %v", tags)
	}
}

// ── Links ──────────────────────────────────────────────────────

func TestAddLink_EObter(t *testing.T) {
	s := newTestStore(t)
	s.AddLink("notes/a.md", "notes/b.md")

	links, err := s.GetLinks("notes/a.md")
	if err != nil {
		t.Fatalf("GetLinks falhou: %v", err)
	}
	if len(links) != 1 || links[0] != "notes/b.md" {
		t.Fatalf("esperado [notes/b.md], got %v", links)
	}
}

func TestGetLinkCount(t *testing.T) {
	s := newTestStore(t)
	s.AddLink("notes/a.md", "notes/b.md")
	s.AddLink("notes/a.md", "notes/c.md")

	if c := s.GetLinkCount("notes/a.md"); c != 2 {
		t.Fatalf("esperado 2, got %d", c)
	}
}

func TestGetLinkCount_Zero(t *testing.T) {
	s := newTestStore(t)
	if c := s.GetLinkCount("notes/isolado.md"); c != 0 {
		t.Fatalf("esperado 0, got %d", c)
	}
}

func TestBacklinks(t *testing.T) {
	s := newTestStore(t)
	s.AddLink("notes/a.md", "notes/alvo.md")
	s.AddLink("notes/b.md", "notes/alvo.md")

	backlinks, err := s.GetBacklinks("notes/alvo.md")
	if err != nil {
		t.Fatalf("GetBacklinks falhou: %v", err)
	}
	if len(backlinks) != 2 {
		t.Fatalf("esperado 2 backlinks, got %d", len(backlinks))
	}

	if c := s.GetBacklinkCount("notes/alvo.md"); c != 2 {
		t.Fatalf("backlink count: esperado 2, got %d", c)
	}
}

func TestRemoveLink(t *testing.T) {
	s := newTestStore(t)
	s.AddLink("notes/a.md", "notes/b.md")
	s.RemoveLink("notes/a.md", "notes/b.md")

	links, _ := s.GetLinks("notes/a.md")
	if len(links) != 0 {
		t.Fatalf("esperado 0 apos remover, got %v", links)
	}
}

func TestClearLinks(t *testing.T) {
	s := newTestStore(t)
	s.AddLink("notes/a.md", "notes/b.md")
	s.AddLink("notes/a.md", "notes/c.md")
	s.ClearLinks("notes/a.md")

	links, _ := s.GetLinks("notes/a.md")
	if len(links) != 0 {
		t.Fatalf("esperado 0 apos clear, got %v", links)
	}
}

func TestGetAllLinks(t *testing.T) {
	s := newTestStore(t)
	s.AddLink("notes/a.md", "notes/b.md")
	s.AddLink("notes/a.md", "notes/c.md")
	s.AddLink("notes/d.md", "notes/e.md")

	all, err := s.GetAllLinks()
	if err != nil {
		t.Fatalf("GetAllLinks falhou: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("esperado 2 entradas, got %d", len(all))
	}
}

// ── FileMods ───────────────────────────────────────────────────

func TestSetFileMod_EObter(t *testing.T) {
	s := newTestStore(t)
	s.SetFileMod("notes/mod.md", "2025-01-01T00:00:00Z")

	mtime, err := s.GetFileMod("notes/mod.md")
	if err != nil {
		t.Fatalf("GetFileMod falhou: %v", err)
	}
	if mtime != "2025-01-01T00:00:00Z" {
		t.Fatalf("esperado '2025-01-01T00:00:00Z', got %q", mtime)
	}
}

func TestGetFileMod_Inexistente(t *testing.T) {
	s := newTestStore(t)
	mtime, err := s.GetFileMod("notes/inexistente.md")
	if err != nil {
		t.Fatalf("GetFileMod falhou: %v", err)
	}
	if mtime != "" {
		t.Fatalf("esperado string vazia, got %q", mtime)
	}
}

func TestDeleteFileMod(t *testing.T) {
	s := newTestStore(t)
	s.SetFileMod("notes/del.md", "2025-01-01T00:00:00Z")
	s.DeleteFileMod("notes/del.md")

	mtime, _ := s.GetFileMod("notes/del.md")
	if mtime != "" {
		t.Fatal("file mod deveria ter sido deletado")
	}
}

func TestGetAllFileMods(t *testing.T) {
	s := newTestStore(t)
	s.SetFileMod("notes/a.md", "t1")
	s.SetFileMod("notes/b.md", "t2")

	mods, err := s.GetAllFileMods()
	if err != nil {
		t.Fatalf("GetAllFileMods falhou: %v", err)
	}
	if len(mods) != 2 {
		t.Fatalf("esperado 2, got %d", len(mods))
	}
}

// ── GetAllDocuments / GetAllDocumentsByFile ────────────────────

func TestGetAllDocuments_RetornaTodos(t *testing.T) {
	s := newTestStore(t)
	s.InsertDocument(Document{ID: "g1", Arquivo: "notes/a.md", Texto: "a", Timestamp: "t1"})
	s.InsertDocument(Document{ID: "g2", Arquivo: "notes/b.md", Texto: "b", Timestamp: "t2"})

	docs, err := s.GetAllDocuments()
	if err != nil {
		t.Fatalf("GetAllDocuments falhou: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("esperado 2, got %d", len(docs))
	}
}

func TestGetAllDocuments_BancoVazio(t *testing.T) {
	s := newTestStore(t)
	docs, err := s.GetAllDocuments()
	if err != nil {
		t.Fatalf("GetAllDocuments falhou: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("esperado 0, got %d", len(docs))
	}
}

func TestGetAllDocumentsByFile(t *testing.T) {
	s := newTestStore(t)
	s.InsertDocument(Document{ID: "x1", Arquivo: "notes/x.md", Texto: "a", Timestamp: "t1"})
	s.InsertDocument(Document{ID: "x2", Arquivo: "notes/x.md", Texto: "b", Timestamp: "t2"})

	byFile, err := s.GetAllDocumentsByFile()
	if err != nil {
		t.Fatalf("GetAllDocumentsByFile falhou: %v", err)
	}
	if len(byFile) != 1 {
		t.Fatalf("esperado 1 entrada, got %d", len(byFile))
	}
	if len(byFile["notes/x.md"]) != 2 {
		t.Fatalf("esperado 2 documentos para notes/x.md, got %d", len(byFile["notes/x.md"]))
	}
}

// ── Concurrency ────────────────────────────────────────────────

func TestConcurrentWrites(t *testing.T) {
	s := newTestStore(t)
	const numGoroutines = 50
	const numWrites = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(gID int) {
			defer wg.Done()
			for j := 0; j < numWrites; j++ {
				// Insert document
				id := fmt.Sprintf("doc-%d-%d", gID, j)
				doc := Document{
					ID:        id,
					Texto:     "concurrent write test",
					Timestamp: "t1",
				}
				if err := s.InsertDocument(doc); err != nil {
					t.Errorf("InsertDocument error: %v", err)
				}
				// Also update popularity to mix queries
				if err := s.IncrementPopularity("concurrent.md"); err != nil {
					t.Errorf("IncrementPopularity error: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all writes succeeded
	count := s.GetDocumentCount()
	if count != numGoroutines*numWrites {
		t.Fatalf("Expected %d documents, got %d", numGoroutines*numWrites, count)
	}
	pop := s.GetPopularity("concurrent.md")
	if pop != numGoroutines*numWrites {
		t.Fatalf("Expected popularity %d, got %d", numGoroutines*numWrites, pop)
	}
}
