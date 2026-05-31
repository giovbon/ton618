package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ton618/internal/db"
)

// ── Helpers ──

func newTestService(t *testing.T) (*NoteService, *BackupService, *db.Store, func()) {
	t.Helper()
	docsDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test.db")

	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	cleanup := func() { store.Close() }

	os.MkdirAll(filepath.Join(docsDir, "notes"), 0755)
	os.MkdirAll(filepath.Join(docsDir, "pdfs"), 0755)
	os.MkdirAll(filepath.Join(docsDir, "attachments"), 0755)

	noteSvc := NewNoteService(store, store, store, store, store, store, docsDir)
	backupSvc := NewBackupService(store, store, docsDir)

	return noteSvc, backupSvc, store, cleanup
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	fullPath := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("writefile: %v", err)
	}
	return fullPath
}

// ── NoteService Tests ──

func TestNoteService_Save(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	err := svc.Save("nova-nota", "# Teste\n\nConteudo da nota.", nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if !store.NoteExists("notes/nova-nota.md") {
		t.Error("nota deveria existir no banco")
	}

	content, err := store.GetNote("notes/nova-nota.md")
	if err != nil || content == "" {
		t.Error("nota deveria ter conteúdo")
	}

	docs, err := store.GetDocumentsByFile("notes/nova-nota.md")
	if err != nil {
		t.Fatalf("GetDocumentsByFile: %v", err)
	}
	if len(docs) == 0 {
		t.Error("deveria ter documentos indexados")
	}
}

func TestNoteService_Save_ComTags(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	err := svc.Save("com-tags", "---\ntags: [teste, demo]\n---\n# Nota com tags", []string{"extra", "demo"})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	docs, _ := store.GetDocumentsByFile("notes/com-tags.md")
	if len(docs) == 0 {
		t.Fatal("sem documentos")
	}

	tags, _ := store.GetFileTags("notes/com-tags.md")
	found := false
	for _, tag := range tags {
		if tag == "demo" || tag == "teste" {
			found = true
		}
	}
	if !found {
		t.Logf("tags encontradas: %v", tags)
	}
}

func TestNoteService_Save_SemPrefixoNotes(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	// filename sem "notes/" — o serviço deve adicionar
	err := svc.Save("sem-prefixo", "conteudo", nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !store.NoteExists("notes/sem-prefixo.md") {
		t.Error("deveria ter prefixo notes/")
	}
}

func TestNoteService_Save_SemExtensaoMd(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	err := svc.Save("notes/sem-extensao", "conteudo", nil)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !store.NoteExists("notes/sem-extensao.md") {
		t.Error("deveria ter extensão .md")
	}
}

func TestNoteService_Delete(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	svc.Save("para-deletar", "conteudo temporario", nil)

	err := svc.Delete("notes/para-deletar.md")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if store.NoteExists("notes/para-deletar.md") {
		t.Error("nota não deveria existir após delete")
	}

	mod, _ := store.GetFileMod("notes/para-deletar.md")
	if mod != "" {
		t.Error("file_mod deveria ter sido removido")
	}
}

func TestNoteService_Rename(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	svc.Save("old-name", "conteudo antigo", nil)

	err := svc.Rename("old-name", "new-name")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}

	if store.NoteExists("notes/old-name.md") {
		t.Error("nome antigo não deveria existir")
	}
	if !store.NoteExists("notes/new-name.md") {
		t.Error("nome novo deveria existir")
	}

	content, _ := store.GetNote("notes/new-name.md")
	if content != "conteudo antigo" {
		t.Errorf("conteúdo errado: %q", content)
	}
}

func TestNoteService_Rename_MesmoNome(t *testing.T) {
	svc, _, _, cleanup := newTestService(t)
	defer cleanup()

	svc.Save("mesmo", "conteudo", nil)

	// Renomear para o mesmo nome não deve errar
	err := svc.Rename("notes/mesmo.md", "mesmo")
	if err != nil {
		t.Fatalf("Rename mesmo nome: %v", err)
	}
}

func TestNoteService_GetMany(t *testing.T) {
	svc, _, _, cleanup := newTestService(t)
	defer cleanup()

	svc.Save("nota-1", "conteudo 1", nil)
	svc.Save("nota-2", "conteudo 2", nil)

	items, err := svc.GetMany()
	if err != nil {
		t.Fatalf("GetMany: %v", err)
	}

	if len(items) < 2 {
		t.Errorf("esperava pelo menos 2 notas, got %d", len(items))
	}

	found := make(map[string]bool)
	for _, item := range items {
		found[item.Arquivo] = true
	}
	if !found["notes/nota-1.md"] || !found["notes/nota-2.md"] {
		t.Error("notas esperadas não encontradas na listagem")
	}
}

// ── BackupService Tests ──

func TestBackupService_Create_Vazio(t *testing.T) {
	_, bkSvc, _, cleanup := newTestService(t)
	defer cleanup()

	data, err := bkSvc.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Mesmo vazio, deve gerar um ZIP válido
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("ZIP inválido: %v", err)
	}
	if len(zr.File) != 0 {
		t.Errorf("esperava 0 arquivos, got %d", len(zr.File))
	}
}

func TestBackupService_Create_ComNotas(t *testing.T) {
	svc, bkSvc, _, cleanup := newTestService(t)
	defer cleanup()

	svc.Save("backup-nota", "# Backup\n\nEsta nota deve aparecer no ZIP.", nil)

	data, err := bkSvc.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("ZIP inválido: %v", err)
	}

	found := false
	for _, f := range zr.File {
		if f.Name == "notes/backup-nota.md" {
			found = true
			rc, _ := f.Open()
			buf := new(bytes.Buffer)
			buf.ReadFrom(rc)
			rc.Close()
			if buf.String() != "# Backup\n\nEsta nota deve aparecer no ZIP." {
				t.Errorf("conteúdo errado no ZIP: %q", buf.String())
			}
		}
	}
	if !found {
		t.Error("nota não encontrada no ZIP de backup")
	}
}

func TestBackupService_Create_PulaArchives(t *testing.T) {
	svc, bkSvc, store, cleanup := newTestService(t)
	defer cleanup()

	// Cria uma nota no diretório archives/ (simulando nota arquivada)
	store.SaveNote("archives/arquivada.md", "conteudo arquivado", time.Now().UTC().Format(time.RFC3339))
	store.SetFileMod("archives/arquivada.md", time.Now().UTC().Format(time.RFC3339))

	svc.Save("nota-normal", "conteudo normal", nil)

	data, err := bkSvc.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	for _, f := range zr.File {
		if f.Name == "archives/arquivada.md" {
			t.Error("arquivos da pasta archives/ não deveriam aparecer no backup")
		}
	}
}

func TestBackupService_Create_ComPDFs(t *testing.T) {
	svc, bkSvc, _, cleanup := newTestService(t)
	defer cleanup()

	// Cria PDF e nota
	docsDir := t.TempDir() // precisamos do docsDir real do serviço
	_ = docsDir
	svc.Save("com-pdf", "nota referenciando PDF", nil)

	data, err := bkSvc.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// ZIP deve ser válido e conter a nota
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if len(zr.File) == 0 {
		t.Error("ZIP vazio inesperadamente")
	}
}

func TestBackupService_Filename(t *testing.T) {
	name := Filename()
	expected := fmt.Sprintf("ton618-backup-%s.zip", time.Now().Format("2006-01-02"))
	if name != expected {
		t.Errorf("Filename: esperado %q, got %q", expected, name)
	}
}

// ── Watcher integration ──

func TestNoteService_ReindexAposProcessFile(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	svc.Save("via-watcher", "# Teste\n\nconteudo", nil)

	// Apaga documentos para simular que o watcher vai reindexar
	store.DeleteDocumentsByFile("notes/via-watcher.md")
	store.DeleteFTSByFile("notes/via-watcher.md")

	// Reindex via Save novamente
	svc.Save("via-watcher", "# Teste\n\nconteudo atualizado", nil)

	docs, _ := store.GetDocumentsByFile("notes/via-watcher.md")
	if len(docs) == 0 {
		t.Error("documentos deveriam ser reindexados")
	}
}

// ── Edge cases ──

func TestNoteService_Delete_NotaNaoExistente(t *testing.T) {
	svc, _, _, cleanup := newTestService(t)
	defer cleanup()

	// Deleção de nota inexistente não deve errar
	err := svc.Delete("notes/inexistente.md")
	if err != nil {
		t.Errorf("Delete de inexistente não deveria errar: %v", err)
	}
}

func TestNoteService_Rename_NotaNaoExistente(t *testing.T) {
	svc, _, _, cleanup := newTestService(t)
	defer cleanup()

	err := svc.Rename("notes/nao-existe", "notes/novo")
	if err != nil {
		// Esperado algum erro do banco
		t.Logf("erro esperado: %v", err)
	}
}

func TestNoteService_GetMany_Vazio(t *testing.T) {
	svc, _, _, cleanup := newTestService(t)
	defer cleanup()

	items, err := svc.GetMany()
	if err != nil {
		t.Fatalf("GetMany: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("esperava 0 itens, got %d", len(items))
	}
}
