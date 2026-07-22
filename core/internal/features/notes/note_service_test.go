package notes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/db"
	"ton618/core/internal/processor"
)

// ── Mocks ──

type mockFileOps struct {
	deleteAllFileRecordsFn      func(filename string) error
	getFilesModsAndTagsFn       func() ([]db.FileModTag, error)
	getNotesNeedingMarkmapTagFn func() ([]string, error)
	getActiveTodoMarkersFn      func() ([]db.TodoMarker, error)
	replaceFileIndexesFn        func(ctx context.Context, filename string, docs []processor.Document, links []string, tags []string, todos []processor.TodoItem, modTime time.Time) error
}

func (m *mockFileOps) DeleteAllFileRecords(filename string) error {
	if m.deleteAllFileRecordsFn != nil {
		return m.deleteAllFileRecordsFn(filename)
	}
	return nil
}
func (m *mockFileOps) GetFilesModsAndTags() ([]db.FileModTag, error) {
	if m.getFilesModsAndTagsFn != nil {
		return m.getFilesModsAndTagsFn()
	}
	return nil, nil
}
func (m *mockFileOps) GetNotesNeedingMarkmapTag() ([]string, error) {
	if m.getNotesNeedingMarkmapTagFn != nil {
		return m.getNotesNeedingMarkmapTagFn()
	}
	return nil, nil
}
func (m *mockFileOps) GetActiveTodoMarkers() ([]db.TodoMarker, error) {
	if m.getActiveTodoMarkersFn != nil {
		return m.getActiveTodoMarkersFn()
	}
	return nil, nil
}
func (m *mockFileOps) ReplaceFileIndexes(ctx context.Context, filename string, docs []processor.Document, links []string, tags []string, todos []processor.TodoItem, modTime time.Time) error {
	if m.replaceFileIndexesFn != nil {
		return m.replaceFileIndexesFn(ctx, filename, docs, links, tags, todos, modTime)
	}
	return nil
}

type mockNoteStore struct {
	getNoteFn      func(filename string) (string, error)
	saveNoteFn     func(filename, content, mtime string) error
	deleteNoteFn   func(filename string) error
	renameNoteFn   func(old, new string) error
	getAllNotesFn  func() (map[string]string, error)
	getNoteMtimeFn func(filename string) (string, error)
	noteExistsFn   func(filename string) bool
}

func (m *mockNoteStore) GetNote(filename string) (string, error) {
	if m.getNoteFn != nil {
		return m.getNoteFn(filename)
	}
	return "", nil
}
func (m *mockNoteStore) SaveNote(filename, content, mtime string) error {
	if m.saveNoteFn != nil {
		return m.saveNoteFn(filename, content, mtime)
	}
	return nil
}
func (m *mockNoteStore) DeleteNote(filename string) error {
	if m.deleteNoteFn != nil {
		return m.deleteNoteFn(filename)
	}
	return nil
}
func (m *mockNoteStore) RenameNote(old, new string) error {
	if m.renameNoteFn != nil {
		return m.renameNoteFn(old, new)
	}
	return nil
}
func (m *mockNoteStore) GetAllNotes() (map[string]string, error) {
	if m.getAllNotesFn != nil {
		return m.getAllNotesFn()
	}
	return nil, nil
}
func (m *mockNoteStore) GetNoteMtime(filename string) (string, error) {
	if m.getNoteMtimeFn != nil {
		return m.getNoteMtimeFn(filename)
	}
	return "", nil
}
func (m *mockNoteStore) NoteExists(filename string) bool {
	if m.noteExistsFn != nil {
		return m.noteExistsFn(filename)
	}
	return false
}

type mockTagStore struct {
	setFileTagsFn func(arquivo string, tags []string) error
	getFileTagsFn func(arquivo string) ([]string, error)
	getAllTagsFn  func() ([]string, error)
}

func (m *mockTagStore) SetFileTags(arquivo string, tags []string) error {
	if m.setFileTagsFn != nil {
		return m.setFileTagsFn(arquivo, tags)
	}
	return nil
}
func (m *mockTagStore) GetFileTags(arquivo string) ([]string, error) {
	if m.getFileTagsFn != nil {
		return m.getFileTagsFn(arquivo)
	}
	return nil, nil
}
func (m *mockTagStore) GetAllTags() ([]string, error) {
	if m.getAllTagsFn != nil {
		return m.getAllTagsFn()
	}
	return nil, nil
}

type mockLinkStore struct {
	addLinkFn         func(fromFile, toFile string) error
	clearLinksFn      func(fromFile string) error
	getBacklinksFn    func(toFile string) ([]string, error)
	getLinksFn        func(fromFile string) ([]string, error)
	getLinksByFilesFn func(fromFiles []string, exclude map[string]bool) ([]string, error)
}

func (m *mockLinkStore) AddLink(fromFile, toFile string) error {
	if m.addLinkFn != nil {
		return m.addLinkFn(fromFile, toFile)
	}
	return nil
}
func (m *mockLinkStore) ClearLinks(fromFile string) error {
	if m.clearLinksFn != nil {
		return m.clearLinksFn(fromFile)
	}
	return nil
}
func (m *mockLinkStore) GetBacklinks(toFile string) ([]string, error) {
	if m.getBacklinksFn != nil {
		return m.getBacklinksFn(toFile)
	}
	return nil, nil
}
func (m *mockLinkStore) GetLinks(fromFile string) ([]string, error) {
	if m.getLinksFn != nil {
		return m.getLinksFn(fromFile)
	}
	return nil, nil
}
func (m *mockLinkStore) GetLinksByFiles(fromFiles []string, exclude map[string]bool) ([]string, error) {
	if m.getLinksByFilesFn != nil {
		return m.getLinksByFilesFn(fromFiles, exclude)
	}
	return nil, nil
}

type mockPopStore struct {
	getPopularityFn       func(arquivo string) int
	incrementPopularityFn func(arquivo string) error
	resetPopularityFn     func(arquivo string) error
}

func (m *mockPopStore) GetPopularity(arquivo string) int {
	if m.getPopularityFn != nil {
		return m.getPopularityFn(arquivo)
	}
	return 0
}
func (m *mockPopStore) IncrementPopularity(arquivo string) error {
	if m.incrementPopularityFn != nil {
		return m.incrementPopularityFn(arquivo)
	}
	return nil
}
func (m *mockPopStore) ResetPopularity(arquivo string) error {
	if m.resetPopularityFn != nil {
		return m.resetPopularityFn(arquivo)
	}
	return nil
}

type mockFileModStore struct {
	getFileModFn     func(arquivo string) (string, error)
	setFileModFn     func(arquivo, mtime string) error
	deleteFileModFn  func(arquivo string) error
	getAllFileModsFn func() (map[string]string, error)
}

func (m *mockFileModStore) GetFileMod(arquivo string) (string, error) {
	if m.getFileModFn != nil {
		return m.getFileModFn(arquivo)
	}
	return "", nil
}
func (m *mockFileModStore) SetFileMod(arquivo, mtime string) error {
	if m.setFileModFn != nil {
		return m.setFileModFn(arquivo, mtime)
	}
	return nil
}
func (m *mockFileModStore) DeleteFileMod(arquivo string) error {
	if m.deleteFileModFn != nil {
		return m.deleteFileModFn(arquivo)
	}
	return nil
}
func (m *mockFileModStore) GetAllFileMods() (map[string]string, error) {
	if m.getAllFileModsFn != nil {
		return m.getAllFileModsFn()
	}
	return nil, nil
}

// ── Helpers ──

func newMockService(t *testing.T, mocks ...interface{}) (*NoteService, string) {
	t.Helper()
	docsDir := t.TempDir()

	svc := &NoteService{
		store:   &mockFileOps{},
		notes:   &mockNoteStore{},
		tags:    &mockTagStore{},
		links:   &mockLinkStore{},
		pop:     &mockPopStore{},
		fileMod: &mockFileModStore{},
		docsDir: docsDir,
	}

	for _, m := range mocks {
		switch v := m.(type) {
		case *mockFileOps:
			svc.store = v
		case *mockNoteStore:
			svc.notes = v
		case *mockTagStore:
			svc.tags = v
		case *mockLinkStore:
			svc.links = v
		case *mockPopStore:
			svc.pop = v
		case *mockFileModStore:
			svc.fileMod = v
		}
	}

	return svc, docsDir
}

func writeNoteFile(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

// ── Tests: Save ──

func TestNoteService_Save_NormalizesFilename(t *testing.T) {
	var savedFilename string
	fileOps := &mockFileOps{
		replaceFileIndexesFn: func(_ context.Context, filename string, _ []processor.Document, _ []string, _ []string, _ []processor.TodoItem, _ time.Time) error {
			savedFilename = filename
			return nil
		},
	}
	notes := &mockNoteStore{
		saveNoteFn: func(filename, content, mtime string) error { return nil },
	}

	svc, _ := newMockService(t, fileOps, notes)

	if err := svc.Save("teste", "# Conteudo", nil); err != nil {
		t.Fatalf("Save: %v", err)
	}

	expected := "notes/teste.md"
	if savedFilename != expected {
		t.Errorf("esperado %q, got %q", expected, savedFilename)
	}
}

func TestNoteService_Save_AddsExtension(t *testing.T) {
	var savedFilename string
	fileOps := &mockFileOps{
		replaceFileIndexesFn: func(_ context.Context, filename string, _ []processor.Document, _ []string, _ []string, _ []processor.TodoItem, _ time.Time) error {
			savedFilename = filename
			return nil
		},
	}
	notes := &mockNoteStore{
		saveNoteFn: func(filename, content, mtime string) error { return nil },
	}

	svc, _ := newMockService(t, fileOps, notes)

	if err := svc.Save("notes/nota.md", "# Conteudo", nil); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if savedFilename != "notes/nota.md" {
		t.Errorf("esperado notes/nota.md, got %q", savedFilename)
	}
}

func TestNoteService_Save_ReplaceFileIndexesError(t *testing.T) {
	expectedErr := errors.New("index error")
	fileOps := &mockFileOps{
		replaceFileIndexesFn: func(_ context.Context, _ string, _ []processor.Document, _ []string, _ []string, _ []processor.TodoItem, _ time.Time) error {
			return expectedErr
		},
	}
	notes := &mockNoteStore{
		saveNoteFn: func(filename, content, mtime string) error { return nil },
	}

	svc, _ := newMockService(t, fileOps, notes)

	err := svc.Save("teste", "# Conteudo", nil)
	if err == nil {
		t.Fatal("esperado erro, got nil")
	}
	if !strings.Contains(err.Error(), "index error") {
		t.Errorf("esperado 'index error', got %v", err)
	}
}

// ── Tests: Delete ──

func TestNoteService_Delete_NormalizesAndCallsStore(t *testing.T) {
	var deletedFilename string
	fileOps := &mockFileOps{
		deleteAllFileRecordsFn: func(filename string) error {
			deletedFilename = filename
			return nil
		},
	}

	svc, docsDir := newMockService(t, fileOps)

	// Cria o arquivo físico para que os.Remove não falhe
	writeNoteFile(t, filepath.Join(docsDir, "notes/delete-me.md"), "# vai ser deletado")

	if err := svc.Delete("delete-me"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if deletedFilename != "notes/delete-me.md" {
		t.Errorf("esperado notes/delete-me.md, got %q", deletedFilename)
	}

	// Verifica que o arquivo foi removido
	if _, err := os.Stat(filepath.Join(docsDir, "notes/delete-me.md")); !os.IsNotExist(err) {
		t.Error("arquivo deveria ter sido removido do disco")
	}
}

func TestNoteService_Delete_WithPrefix(t *testing.T) {
	var deletedFilename string
	fileOps := &mockFileOps{
		deleteAllFileRecordsFn: func(filename string) error {
			deletedFilename = filename
			return nil
		},
	}

	svc, docsDir := newMockService(t, fileOps)
	writeNoteFile(t, filepath.Join(docsDir, "notes/delete-me.md"), "# vai ser deletado")

	if err := svc.Delete("notes/delete-me.md"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if deletedFilename != "notes/delete-me.md" {
		t.Errorf("esperado notes/delete-me.md, got %q", deletedFilename)
	}
}

// ── Tests: Rename ──

func TestNoteService_Rename_SameName(t *testing.T) {
	svc, _ := newMockService(t)
	err := svc.Rename("notes/igual.md", "notes/igual.md")
	if err != nil {
		t.Errorf("renomear para o mesmo nome deve retornar nil, got %v", err)
	}
}

func TestNoteService_Rename_NormalizesAndRenames(t *testing.T) {
	var renamedOld, renamedNew string
	var deletedOld string

	notes := &mockNoteStore{
		renameNoteFn: func(old, new string) error {
			renamedOld = old
			renamedNew = new
			return nil
		},
	}
	links := &mockLinkStore{
		getBacklinksFn: func(_ string) ([]string, error) { return nil, nil },
	}
	fileOps := &mockFileOps{
		deleteAllFileRecordsFn: func(filename string) error {
			deletedOld = filename
			return nil
		},
		replaceFileIndexesFn: func(_ context.Context, _ string, _ []processor.Document, _ []string, _ []string, _ []processor.TodoItem, _ time.Time) error {
			return nil
		},
	}

	svc, docsDir := newMockService(t, fileOps, notes, links)
	writeNoteFile(t, filepath.Join(docsDir, "notes/old-name.md"), "# Nota antiga")

	if err := svc.Rename("old-name", "new-name"); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	if renamedOld != "notes/old-name.md" {
		t.Errorf("esperado notes/old-name.md, got %q", renamedOld)
	}
	if renamedNew != "notes/new-name.md" {
		t.Errorf("esperado notes/new-name.md, got %q", renamedNew)
	}
	if deletedOld != "notes/old-name.md" {
		t.Errorf("esperado notes/old-name.md, got %q", deletedOld)
	}
}

func TestNoteService_Rename_UpdatesBacklinks(t *testing.T) {
	notes := &mockNoteStore{
		renameNoteFn: func(old, new string) error { return nil },
		getNoteFn: func(filename string) (string, error) {
			if filename == "notes/ref.md" {
				return "# Referencia [[old-name]]", nil
			}
			return "", nil
		},
	}
	links := &mockLinkStore{
		getBacklinksFn: func(_ string) ([]string, error) {
			return []string{"notes/ref.md"}, nil
		},
	}
	fileOps := &mockFileOps{
		deleteAllFileRecordsFn: func(_ string) error { return nil },
		replaceFileIndexesFn: func(_ context.Context, _ string, _ []processor.Document, _ []string, _ []string, _ []processor.TodoItem, _ time.Time) error {
			return nil
		},
	}

	svc, docsDir := newMockService(t, fileOps, notes, links)
	writeNoteFile(t, filepath.Join(docsDir, "notes/old-name.md"), "# Nota antiga")
	writeNoteFile(t, filepath.Join(docsDir, "notes/ref.md"), "# Referencia [[old-name]]")
	writeNoteFile(t, filepath.Join(docsDir, "notes/new-name.md"), "# Nota nova")

	if err := svc.Rename("old-name", "new-name"); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	// Verifica que o arquivo fisico foi renomeado
	if _, err := os.Stat(filepath.Join(docsDir, "notes/old-name.md")); !os.IsNotExist(err) {
		t.Error("old-name.md ainda existe no disco")
	}
	if _, err := os.Stat(filepath.Join(docsDir, "notes/new-name.md")); os.IsNotExist(err) {
		t.Error("new-name.md deveria existir no disco")
	}
}

func TestNoteService_UpdateBacklinksOnRename_ZipFile(t *testing.T) {
	var savedNoteContent string
	var savedNoteFile string

	notes := &mockNoteStore{
		getAllNotesFn: func() (map[string]string, error) {
			return map[string]string{"notes/referenciadora.md": "2026-01-01T00:00:00Z"}, nil
		},
		getNoteFn: func(filename string) (string, error) {
			if filename == "notes/referenciadora.md" {
				return "Link 1: [[meuarquivo.zip]]\nLink 2: [[attachments/meuarquivo.zip|Rótulo]]\nLink 3: [Baixar](/file/download?name=attachments/meuarquivo.zip)", nil
			}
			return "", nil
		},
		saveNoteFn: func(filename, content, mtime string) error {
			savedNoteFile = filename
			savedNoteContent = content
			return nil
		},
	}
	links := &mockLinkStore{
		getBacklinksFn: func(to string) ([]string, error) {
			return []string{"notes/referenciadora.md"}, nil
		},
	}
	fileOps := &mockFileOps{
		replaceFileIndexesFn: func(_ context.Context, _ string, _ []processor.Document, _ []string, _ []string, _ []processor.TodoItem, _ time.Time) error {
			return nil
		},
	}

	svc, _ := newMockService(t, fileOps, notes, links)

	oldZip := "attachments/meuarquivo.zip"
	newZip := "attachments/novo_arquivo.zip"

	if err := svc.UpdateBacklinksOnRename(oldZip, newZip); err != nil {
		t.Fatalf("UpdateBacklinksOnRename: %v", err)
	}

	if savedNoteFile != "notes/referenciadora.md" {
		t.Errorf("esperado salvar notes/referenciadora.md, got %q", savedNoteFile)
	}

	expectedContent := "Link 1: [[novo_arquivo.zip]]\nLink 2: [[attachments/novo_arquivo.zip|Rótulo]]\nLink 3: [Baixar](/file/download?name=attachments/novo_arquivo.zip)"
	if savedNoteContent != expectedContent {
		t.Errorf("conteúdo inesperado após renomear zip:\nesperado:\n%s\nobtido:\n%s", expectedContent, savedNoteContent)
	}
}

func TestNoteService_UpdateBacklinksOnRename_AllNoteTypes(t *testing.T) {
	var savedNoteContent string

	notes := &mockNoteStore{
		getAllNotesFn: func() (map[string]string, error) {
			return map[string]string{"notes/main.md": "2026-01-01T00:00:00Z"}, nil
		},
		getNoteFn: func(filename string) (string, error) {
			if filename == "notes/main.md" {
				return "Desenho: [[meu-desenho]] | PDF: [[manual.pdf]] | EPUB: [Livro](/epub/reader?file=epubs/oldbook.epub)", nil
			}
			return "", nil
		},
		saveNoteFn: func(filename, content, mtime string) error {
			savedNoteContent = content
			return nil
		},
	}
	fileOps := &mockFileOps{
		replaceFileIndexesFn: func(_ context.Context, _ string, _ []processor.Document, _ []string, _ []string, _ []processor.TodoItem, _ time.Time) error {
			return nil
		},
	}

	svc, _ := newMockService(t, fileOps, notes)

	// 1. Renomeia nota de desenho
	if err := svc.UpdateBacklinksOnRename("notes/meu-desenho.md", "notes/desenho-v2.md"); err != nil {
		t.Fatalf("rename drawing: %v", err)
	}
	if !strings.Contains(savedNoteContent, "[[desenho-v2]]") {
		t.Errorf("esperado link [[desenho-v2]], obtido: %s", savedNoteContent)
	}

	// 2. Renomeia PDF
	if err := svc.UpdateBacklinksOnRename("pdfs/manual.pdf", "pdfs/manual-novo.pdf"); err != nil {
		t.Fatalf("rename pdf: %v", err)
	}
	if !strings.Contains(savedNoteContent, "[[manual-novo.pdf]]") {
		t.Errorf("esperado link [[manual-novo.pdf]], obtido: %s", savedNoteContent)
	}

	// 3. Renomeia EPUB
	if err := svc.UpdateBacklinksOnRename("epubs/oldbook.epub", "epubs/newbook.epub"); err != nil {
		t.Fatalf("rename epub: %v", err)
	}
	if !strings.Contains(savedNoteContent, "file=epubs/newbook.epub") {
		t.Errorf("esperado URL file=epubs/newbook.epub, obtido: %s", savedNoteContent)
	}
}

// ── Tests: GetMany ──

func TestNoteService_GetMany_ReturnsItems(t *testing.T) {
	fileOps := &mockFileOps{
		getFilesModsAndTagsFn: func() ([]db.FileModTag, error) {
			return []db.FileModTag{
				{Arquivo: "notes/nota1.md", Mtime: "2026-01-01T00:00:00Z", Tags: "tag1,tag2"},
				{Arquivo: "notes/nota2.md", Mtime: "2026-01-02T00:00:00Z", Tags: ""},
			}, nil
		},
	}

	svc, _ := newMockService(t, fileOps)

	items, err := svc.GetMany()
	if err != nil {
		t.Fatalf("GetMany: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("esperado 2 items, got %d", len(items))
	}

	if items[0].Arquivo != "notes/nota1.md" {
		t.Errorf("esperado notes/nota1.md, got %q", items[0].Arquivo)
	}
	if len(items[0].Tags) != 2 {
		t.Errorf("esperado 2 tags, got %d", len(items[0].Tags))
	}
	if items[0].Type != "nota" {
		t.Errorf("esperado 'nota', got %q", items[0].Type)
	}
}

func TestNoteService_GetMany_PropagatesError(t *testing.T) {
	expectedErr := errors.New("db error")
	fileOps := &mockFileOps{
		getFilesModsAndTagsFn: func() ([]db.FileModTag, error) {
			return nil, expectedErr
		},
	}

	svc, _ := newMockService(t, fileOps)

	_, err := svc.GetMany()
	if err != expectedErr {
		t.Errorf("esperado %v, got %v", expectedErr, err)
	}
}

// ── Tests: GetBacklinks ──

func TestNoteService_GetBacklinks_NoLinks(t *testing.T) {
	links := &mockLinkStore{
		getBacklinksFn: func(_ string) ([]string, error) { return nil, nil },
	}

	svc, _ := newMockService(t, links)

	result, err := svc.GetBacklinks("notes/teste.md")
	if err != nil {
		t.Fatalf("GetBacklinks: %v", err)
	}

	if len(result.Level1) != 0 {
		t.Errorf("esperado Level1 vazio, got %d", len(result.Level1))
	}
}

func TestNoteService_GetBacklinks_TwoLevels(t *testing.T) {
	links := &mockLinkStore{
		getBacklinksFn: func(_ string) ([]string, error) {
			return []string{"notes/a.md", "notes/b.md"}, nil
		},
	}

	svc, _ := newMockService(t, links)

	result, err := svc.GetBacklinks("notes/teste.md")
	if err != nil {
		t.Fatalf("GetBacklinks: %v", err)
	}

	if len(result.Level1) != 2 {
		t.Errorf("esperado 2 Level1, got %d", len(result.Level1))
	}
}

func TestNoteService_GetBacklinks_FiltersCurrentNote(t *testing.T) {
	links := &mockLinkStore{
		getBacklinksFn: func(_ string) ([]string, error) {
			return []string{"notes/a.md"}, nil
		},
	}

	svc, _ := newMockService(t, links)

	result, err := svc.GetBacklinks("notes/teste.md")
	if err != nil {
		t.Fatalf("GetBacklinks: %v", err)
	}

	if len(result.Level1) != 1 || result.Level1[0] != "notes/a.md" {
		t.Errorf("esperado Level1=[notes/a.md], got %v", result.Level1)
	}
}

// ── Tests: processAndSave (via Save) ──

func TestNoteService_Save_CallsReplaceFileIndexes(t *testing.T) {
	var called bool
	fileOps := &mockFileOps{
		replaceFileIndexesFn: func(_ context.Context, _ string, _ []processor.Document, links []string, tags []string, _ []processor.TodoItem, _ time.Time) error {
			called = true
			return nil
		},
	}
	notes := &mockNoteStore{
		saveNoteFn: func(filename, content, mtime string) error { return nil },
	}

	svc, _ := newMockService(t, fileOps, notes)

	if err := svc.Save("teste", "# Titulo\n\nConteudo com [[link]].", []string{"tag1"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if !called {
		t.Error("ReplaceFileIndexes não foi chamado")
	}
}

// ── Tests: SyncDatabase ──

func TestNoteService_SyncDatabase_ProcessesPendingNotes(t *testing.T) {
	var processed string
	fileOps := &mockFileOps{
		getNotesNeedingMarkmapTagFn: func() ([]string, error) { return nil, nil },
		replaceFileIndexesFn: func(_ context.Context, filename string, _ []processor.Document, _ []string, _ []string, _ []processor.TodoItem, _ time.Time) error {
			processed = filename
			return nil
		},
	}
	notes := &mockNoteStore{
		getAllNotesFn: func() (map[string]string, error) {
			return map[string]string{"notes/pendente.md": "2026-01-01T00:00:00Z"}, nil
		},
		getNoteFn: func(filename string) (string, error) {
			return "# Pendente", nil
		},
	}
	fileMod := &mockFileModStore{
		getFileModFn: func(_ string) (string, error) { return "", nil },
	}

	svc, _ := newMockService(t, fileOps, notes, fileMod)

	if err := svc.SyncDatabase(); err != nil {
		t.Fatalf("SyncDatabase: %v", err)
	}

	if processed != "notes/pendente.md" {
		t.Errorf("esperado notes/pendente.md, got %q", processed)
	}
}

// ── Tests: Type detection ──

func TestNoteService_GetMany_DetectsNoteType(t *testing.T) {
	fileOps := &mockFileOps{
		getFilesModsAndTagsFn: func() ([]db.FileModTag, error) {
			return []db.FileModTag{
				{Arquivo: "notes/drawing.md", Mtime: "", Tags: "drawing"},
				{Arquivo: "pdfs/doc.pdf", Mtime: "", Tags: ""},
				{Arquivo: "notes/typst.md", Mtime: "", Tags: "typst"},
			}, nil
		},
	}

	svc, _ := newMockService(t, fileOps)

	items, err := svc.GetMany()
	if err != nil {
		t.Fatalf("GetMany: %v", err)
	}

	tests := []struct {
		filename string
		wantType string
	}{
		{"notes/drawing.md", "desenho"},
		{"pdfs/doc.pdf", "pdf"},
		{"notes/typst.md", "typst"},
	}

	for _, tt := range tests {
		found := false
		for _, item := range items {
			if item.Arquivo == tt.filename {
				found = true
				if item.Type != tt.wantType {
					t.Errorf("%s: esperado type=%q, got %q", tt.filename, tt.wantType, item.Type)
				}
				break
			}
		}
		if !found {
			t.Errorf("%s: não encontrado nos resultados", tt.filename)
		}
	}
}
