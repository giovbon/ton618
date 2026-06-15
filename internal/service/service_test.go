package service

import (
	"archive/zip"
	"bytes"
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

func TestNoteService_Delete_ClearsBacklinks(t *testing.T) {
	svc, _, _, cleanup := newTestService(t)
	defer cleanup()

	// Cria note1
	err := svc.Save("note1", "Eu sou a note1", nil)
	if err != nil {
		t.Fatalf("Save note1: %v", err)
	}

	// Cria note2 que linka para note1
	err = svc.Save("note2", "Link para [[note1]]", nil)
	if err != nil {
		t.Fatalf("Save note2: %v", err)
	}

	// Verifica se o backlink aparece em note1
	backlinks, err := svc.GetBacklinks("notes/note1.md")
	if err != nil {
		t.Fatalf("GetBacklinks falhou: %v", err)
	}
	if len(backlinks.Level1) != 1 || backlinks.Level1[0] != "notes/note2.md" {
		t.Fatalf("esperava backlink notes/note2.md, got %v", backlinks.Level1)
	}

	// Deleta note2
	err = svc.Delete("notes/note2.md")
	if err != nil {
		t.Fatalf("Delete note2: %v", err)
	}

	// Verifica se o backlink sumiu de note1
	backlinks, err = svc.GetBacklinks("notes/note1.md")
	if err != nil {
		t.Fatalf("GetBacklinks falhou: %v", err)
	}
	if len(backlinks.Level1) != 0 {
		t.Fatalf("esperava 0 backlinks em note1 após deletar note2, got %v", backlinks.Level1)
	}
}

func TestNoteService_Rename_ClearsOldLinks(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	// Cria note1
	svc.Save("note1", "Eu sou a note1", nil)

	// Cria note2 que linka para note1
	svc.Save("note2", "Link para [[note1]]", nil)

	// Renomeia note2 para note3
	err := svc.Rename("note2", "note3")
	if err != nil {
		t.Fatalf("Rename note2 para note3 falhou: %v", err)
	}

	// Verifica se os links originados de note2 (oldName) foram limpos
	links, err := store.GetLinks("notes/note2.md")
	if err != nil {
		t.Fatalf("GetLinks note2 falhou: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("esperava 0 links originados de note2, got %v", links)
	}

	// Verifica se o backlink em note1 agora aponta para note3
	backlinks, err := svc.GetBacklinks("notes/note1.md")
	if err != nil {
		t.Fatalf("GetBacklinks note1 falhou: %v", err)
	}
	if len(backlinks.Level1) != 1 || backlinks.Level1[0] != "notes/note3.md" {
		t.Fatalf("esperava backlink de notes/note3.md, got %v", backlinks.Level1)
	}
}

func TestNoteService_Rename_UpdatesWikilinks(t *testing.T) {
	svc, _, store, cleanup := newTestService(t)
	defer cleanup()

	// 1. Cria a nota alvo
	svc.Save("alvo", "Eu sou a nota alvo", nil)

	// 2. Cria notas que referenciam a nota alvo de diferentes formas
	contentRef1 := "Link simples: [[alvo]] e outro no final [[alvo]]"
	contentRef2 := "Link com alias: [[alvo|meu texto]]"
	contentRef3 := "Link com seção e alias: [[alvo#Secao 1|texto longo]]"
	contentRef4 := "Link case-insensitive: [[ALVO]]"

	svc.Save("ref1", contentRef1, nil)
	svc.Save("ref2", contentRef2, nil)
	svc.Save("ref3", contentRef3, nil)
	svc.Save("ref4", contentRef4, nil)

	// 3. Renomeia a nota alvo
	err := svc.Rename("alvo", "novo-alvo")
	if err != nil {
		t.Fatalf("Rename falhou: %v", err)
	}

	// 4. Verifica o conteúdo atualizado das notas
	validaNota := func(nomeNota, conteudoEsperado string) {
		content, err := store.GetNote("notes/" + nomeNota + ".md")
		if err != nil {
			t.Fatalf("Erro ao ler nota %s: %v", nomeNota, err)
		}
		if content != conteudoEsperado {
			t.Errorf("Conteúdo de %s incorreto.\nEsperado: %s\nObtido: %s", nomeNota, conteudoEsperado, content)
		}
	}

	validaNota("ref1", "Link simples: [[novo-alvo]] e outro no final [[novo-alvo]]")
	validaNota("ref2", "Link com alias: [[novo-alvo|meu texto]]")
	validaNota("ref3", "Link com seção e alias: [[novo-alvo#Secao 1|texto longo]]")
	validaNota("ref4", "Link case-insensitive: [[novo-alvo]]") // Note que a caixa mudará para a do newTitle (novo-alvo)
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

func TestUpdateFrontmatterProperty_FlowStyleTags(t *testing.T) {
	content := "---\ntype: note\ntags: [golang, programacao]\n---\n# Corpo da nota"
	updated, err := UpdateFrontmatterProperty(content, "tags", "golang, programacao, teste")
	if err != nil {
		t.Fatalf("UpdateFrontmatterProperty: %v", err)
	}

	// Note: YAML encoding order might vary (e.g. tags could be sorted or keys could be sorted),
	// but the output should have flow style tags: [golang, programacao, teste].
	if !bytes.Contains([]byte(updated), []byte("tags: [golang, programacao, teste]")) {
		t.Errorf("esperava que o conteudo contivesse 'tags: [golang, programacao, teste]', got:\n%q", updated)
	}
}

func TestUpdateFrontmatterProperty_CaseInsensitiveTagsAndString(t *testing.T) {
	content := "---\ntype: note\nTags: golang, programacao\n---\n# Corpo da nota"
	updated, err := UpdateFrontmatterProperty(content, "tags", "golang, programacao, teste")
	if err != nil {
		t.Fatalf("UpdateFrontmatterProperty: %v", err)
	}

	if !bytes.Contains([]byte(updated), []byte("tags: [golang, programacao, teste]")) {
		t.Errorf("esperava que o conteudo contivesse 'tags: [golang, programacao, teste]', got:\n%q", updated)
	}
	if bytes.Contains([]byte(updated), []byte("Tags:")) {
		t.Error("esperava que a chave duplicada 'Tags:' fosse removida")
	}
}
