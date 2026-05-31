package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveNote_Insert(t *testing.T) {
	s := newTestStore(t)
	if err := s.SaveNote("notes/teste.md", "conteudo", "2025-01-01T00:00:00Z"); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	got, err := s.GetNote("notes/teste.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if got != "conteudo" {
		t.Errorf("esperado 'conteudo', got %q", got)
	}
}

func TestSaveNote_Update(t *testing.T) {
	s := newTestStore(t)
	s.SaveNote("notes/upd.md", "v1", "2025-01-01T00:00:00Z")
	s.SaveNote("notes/upd.md", "v2", "2025-01-02T00:00:00Z")
	got, _ := s.GetNote("notes/upd.md")
	if got != "v2" {
		t.Errorf("esperado 'v2', got %q", got)
	}
}

func TestGetNote_NaoExistente(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetNote("notes/nao-existe.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if got != "" {
		t.Errorf("esperado '', got %q", got)
	}
}

func TestDeleteNote(t *testing.T) {
	s := newTestStore(t)
	s.SaveNote("notes/del.md", "x", "2025-01-01T00:00:00Z")
	s.DeleteNote("notes/del.md")
	if s.NoteExists("notes/del.md") {
		t.Error("nota deveria ter sido deletada")
	}
}

func TestRenameNote(t *testing.T) {
	s := newTestStore(t)
	s.SaveNote("notes/old.md", "conteudo", "2025-01-01T00:00:00Z")
	if err := s.RenameNote("notes/old.md", "notes/new.md"); err != nil {
		t.Fatalf("RenameNote: %v", err)
	}
	if s.NoteExists("notes/old.md") {
		t.Error("nome antigo nao deveria existir")
	}
	if !s.NoteExists("notes/new.md") {
		t.Error("nome novo deveria existir")
	}
}

func TestGetAllNotes(t *testing.T) {
	s := newTestStore(t)
	s.SaveNote("notes/a.md", "a", "2025-01-01T00:00:00Z")
	s.SaveNote("notes/b.md", "b", "2025-01-02T00:00:00Z")
	all, err := s.GetAllNotes()
	if err != nil {
		t.Fatalf("GetAllNotes: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("esperado 2 notes, got %d", len(all))
	}
}

func TestGetAllNotesPaginated(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("notes/p%d.md", i)
		s.SaveNote(name, "x", "2025-01-01T00:00:00Z")
	}
	mods, total, err := s.GetAllNotesPaginated(0, 2)
	if err != nil {
		t.Fatalf("GetAllNotesPaginated: %v", err)
	}
	if total != 5 {
		t.Errorf("total: esperado 5, got %d", total)
	}
	if len(mods) != 2 {
		t.Errorf("page size: esperado 2, got %d", len(mods))
	}
}

func TestGetNoteMtime(t *testing.T) {
	s := newTestStore(t)
	s.SaveNote("notes/mtime.md", "x", "2025-06-15T10:30:00Z")
	got, err := s.GetNoteMtime("notes/mtime.md")
	if err != nil {
		t.Fatalf("GetNoteMtime: %v", err)
	}
	if got != "2025-06-15T10:30:00Z" {
		t.Errorf("mtime errado: %q", got)
	}
}

func TestGetNoteMtime_NaoExistente(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetNoteMtime("notes/nao-existe.md")
	if err != nil {
		t.Fatalf("GetNoteMtime: %v", err)
	}
	if got != "" {
		t.Errorf("esperado '', got %q", got)
	}
}

func TestNoteExists_True(t *testing.T) {
	s := newTestStore(t)
	s.SaveNote("notes/existe.md", "x", "2025-01-01T00:00:00Z")
	if !s.NoteExists("notes/existe.md") {
		t.Error("deveria existir")
	}
}

func TestNoteExists_False(t *testing.T) {
	s := newTestStore(t)
	if s.NoteExists("notes/nao-existe.md") {
		t.Error("nao deveria existir")
	}
}

func TestMigrateNotesFromDisk(t *testing.T) {
	s := newTestStore(t)
	docsDir := t.TempDir()
	notesDir := filepath.Join(docsDir, "notes")
	os.MkdirAll(notesDir, 0755)
	os.WriteFile(filepath.Join(notesDir, "migrado.md"), []byte("# Migrado\n\nconteudo"), 0644)
	os.WriteFile(filepath.Join(notesDir, "outro.md"), []byte("outro conteudo"), 0644)
	count, err := s.MigrateNotesFromDisk(docsDir)
	if err != nil {
		t.Fatalf("MigrateNotesFromDisk: %v", err)
	}
	if count != 2 {
		t.Errorf("esperado 2 migracoes, got %d", count)
	}
	if !s.NoteExists("notes/migrado.md") {
		t.Error("nota migrada deveria existir")
	}
}

func TestSetFileMod_Update(t *testing.T) {
	s := newTestStore(t)
	s.SetFileMod("notes/fm-upd.md", "2025-01-01T00:00:00Z")
	s.SetFileMod("notes/fm-upd.md", "2025-06-01T00:00:00Z")
	got, _ := s.GetFileMod("notes/fm-upd.md")
	if got != "2025-06-01T00:00:00Z" {
		t.Errorf("esperado update, got %q", got)
	}
}

func TestGetBacklinkCount(t *testing.T) {
	s := newTestStore(t)
	s.AddLink("notes/a.md", "notes/target.md")
	count := s.GetBacklinkCount("notes/target.md")
	if count != 1 {
		t.Errorf("esperado 1, got %d", count)
	}
}

func TestAddLink_Duplicado(t *testing.T) {
	s := newTestStore(t)
	s.AddLink("notes/from.md", "notes/to.md")
	s.AddLink("notes/from.md", "notes/to.md")
	count := s.GetLinkCount("notes/from.md")
	if count != 1 {
		t.Errorf("duplicado: esperado 1, got %d", count)
	}
}

func TestSearchFTS_SemResultados(t *testing.T) {
	s := newTestStore(t)
	_, total, err := s.SearchFTS("inexistente", 0, 10)
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if total != 0 {
		t.Errorf("esperado 0 resultados, got %d", total)
	}
}

func TestSearchFTSLike_Resultado(t *testing.T) {
	s := newTestStore(t)
	s.InsertDocument(Document{ID: "doc-like", Arquivo: "notes/like.md", Secao: "Geral", Texto: "palavra_chave especial"})
	s.IndexFTS("doc-like", "md", "notes/like.md", "Geral", "palavra_chave especial", "")
	results, total, err := s.SearchFTSLike("especial", 0, 10)
	if err != nil {
		t.Fatalf("SearchFTSLike: %v", err)
	}
	if total != 1 {
		t.Errorf("esperado 1, got %d", total)
	}
	if len(results) > 0 && results[0].DocID != "doc-like" {
		t.Errorf("doc_id errado: %q", results[0].DocID)
	}
}

func TestSearchFTSLike_SemResultados(t *testing.T) {
	s := newTestStore(t)
	_, total, _ := s.SearchFTSLike("zzzzzz", 0, 10)
	if total != 0 {
		t.Errorf("esperado 0, got %d", total)
	}
}

func TestSaveNote_ContentVazio(t *testing.T) {
	s := newTestStore(t)
	s.SaveNote("notes/vazio.md", "", time.Now().UTC().Format(time.RFC3339))
	got, _ := s.GetNote("notes/vazio.md")
	if got != "" {
		t.Errorf("esperado '', got %q", got)
	}
}

func TestGetAllNotes_Vazio(t *testing.T) {
	s := newTestStore(t)
	all, _ := s.GetAllNotes()
	if len(all) != 0 {
		t.Errorf("esperado 0, got %d", len(all))
	}
}

func TestGetAllLinks_Vazio(t *testing.T) {
	s := newTestStore(t)
	all, _ := s.GetAllLinks()
	if len(all) != 0 {
		t.Errorf("esperado 0, got %d", len(all))
	}
}

func TestGetDistinctFiles_Vazio(t *testing.T) {
	s := newTestStore(t)
	files, _ := s.GetDistinctFiles()
	if len(files) != 0 {
		t.Errorf("esperado 0, got %d", len(files))
	}
}

func TestGetDocument_NaoExistente_DB(t *testing.T) {
	s := newTestStore(t)
	doc, err := s.GetDocument("nao-existe")
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if doc != nil {
		t.Error("esperado nil para documento inexistente")
	}
}
