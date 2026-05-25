package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ton618/internal/config"
	"ton618/internal/db"
)

// mockEmbedProvider is a no-op embedding provider for testing.
type mockEmbedProvider struct{}

func (m *mockEmbedProvider) Embed(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockEmbedProvider) Dimensions() int {
	return 3
}

// startEmbedWorkersForTest inicia o worker pool com contexto cancelavel
// e reinicia o sync.Once para permitir uso entre testes.
// Retorna uma funcao que drena a fila e aguarda os workers terminarem.
func startEmbedWorkersForTest(t *testing.T) func() {
	t.Helper()
	embedOnce = sync.Once{}
	ctx, cancel := context.WithCancel(context.Background())
	startEmbedWorkers(ctx)
	flush := func() {
		stopEmbedWorkers()
	}
	t.Cleanup(func() {
		cancel()
		stopEmbedWorkers()
	})
	return flush
}

func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	s, err := db.NewStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newTestConfig(t *testing.T) *config.AppConfig {
	t.Helper()
	docsDir := t.TempDir()
	os.MkdirAll(filepath.Join(docsDir, "notes"), 0755)
	os.MkdirAll(filepath.Join(docsDir, "links"), 0755)
	os.MkdirAll(filepath.Join(docsDir, "voice"), 0755)
	return &config.AppConfig{DocsDir: docsDir}
}

func TestShouldEmbed_EmbedAll(t *testing.T) {
	if !shouldEmbed(nil, true) {
		t.Error("embedAll=true deveria retornar true sem tags")
	}
	if !shouldEmbed([]string{}, true) {
		t.Error("embedAll=true deveria retornar true com lista vazia")
	}
}

func TestShouldEmbed_TagEmbed(t *testing.T) {
	if !shouldEmbed([]string{"embed"}, false) {
		t.Error("tag 'embed' deveria retornar true")
	}
	if !shouldEmbed([]string{"golang", "embed", "importante"}, false) {
		t.Error("tag 'embed' em lista deveria retornar true")
	}
}

func TestShouldEmbed_SemTag(t *testing.T) {
	if shouldEmbed([]string{"golang", "programacao"}, false) {
		t.Error("sem tag 'embed' e embedAll=false deveria retornar false")
	}
	if shouldEmbed(nil, false) {
		t.Error("nil tags com embedAll=false deveria retornar false")
	}
}

func TestSupportedExts_CobreFormatos(t *testing.T) {
	exts := []string{".md", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg"}
	for _, ext := range exts {
		if _, ok := supportedExts[ext]; !ok {
			t.Errorf("extensao %q nao esta em supportedExts", ext)
		}
	}
}

func TestMonitoredSubDirs(t *testing.T) {
	expected := []string{"notes", "links", "voice"}
	for _, sub := range expected {
		found := false
		for _, m := range MonitoredSubDirs {
			if m == sub {
				found = true
			}
		}
		if !found {
			t.Errorf("subdiretorio %q nao esta em MonitoredSubDirs", sub)
		}
	}
}

func TestProcessFile_MarkdownSimples(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)

	fp := filepath.Join(cfg.DocsDir, "notes", "teste.md")
	os.WriteFile(fp, []byte("# Título\nconteudo de teste"), 0644)

	ev := FileEvent{
		Path:     fp,
		Filename: "notes/teste.md",
		ModTime:  time.Now(),
		Type:     "modify",
	}

	err := ProcessFile(store, ev, nil, false)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	docs, err := store.GetDocumentsByFile("notes/teste.md")
	if err != nil {
		t.Fatalf("GetDocumentsByFile: %v", err)
	}
	if len(docs) < 1 {
		t.Fatal("documento nao foi indexado")
	}
}

func TestProcessFile_Delete(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)

	// Primeiro insere
	fp := filepath.Join(cfg.DocsDir, "notes", "deleteme.md")
	os.WriteFile(fp, []byte("sera deletado"), 0644)
	ProcessFile(store, FileEvent{Path: fp, Filename: "notes/deleteme.md", ModTime: time.Now(), Type: "modify"}, nil, false)

	// Depois deleta
	ev := FileEvent{Path: fp, Filename: "notes/deleteme.md", Type: "delete"}
	err := ProcessFile(store, ev, nil, false)
	if err != nil {
		t.Fatalf("ProcessFile delete: %v", err)
	}

	docs, _ := store.GetDocumentsByFile("notes/deleteme.md")
	if len(docs) != 0 {
		t.Errorf("documentos ainda existem apos delete: %d docs", len(docs))
	}
}

func TestProcessFile_ExtensaoInvalida(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)

	fp := filepath.Join(cfg.DocsDir, "notes", "teste.txt")
	os.WriteFile(fp, []byte("arquivo txt"), 0644)

	ev := FileEvent{Path: fp, Filename: "notes/teste.txt", ModTime: time.Now(), Type: "modify"}
	err := ProcessFile(store, ev, nil, false)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	// Deve ser ignorado silenciosamente
	docs, _ := store.GetDocumentsByFile("notes/teste.txt")
	if len(docs) != 0 {
		t.Error("extensao invalida nao deveria ser indexada")
	}
}

func TestProcessFile_ArquivoInexistente(t *testing.T) {
	store := newTestStore(t)
	ev := FileEvent{
		Path:     "/caminho/nao/existe.md",
		Filename: "notes/inexistente.md",
		ModTime:  time.Now(),
		Type:     "modify",
	}
	err := ProcessFile(store, ev, nil, false)
	if err != nil {
		t.Fatalf("ProcessFile retornou erro inesperado: %v", err)
	}
}

func TestNewWatcher(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)
	w := NewWatcher(cfg, store)

	if w.cfg != cfg {
		t.Error("cfg nao foi atribuida")
	}
	if w.store != store {
		t.Error("store nao foi atribuida")
	}
	if w.embedAll {
		t.Error("embedAll deveria ser false por padrao")
	}
	if w.events == nil {
		t.Error("events channel nao foi criado")
	}
}

func TestFileEvent_Campos(t *testing.T) {
	now := time.Now()
	ev := FileEvent{
		Path:     "/tmp/teste.md",
		Filename: "notes/teste.md",
		ModTime:  now,
		Type:     "modify",
	}
	if ev.Path != "/tmp/teste.md" {
		t.Error("Path errado")
	}
	if ev.Filename != "notes/teste.md" {
		t.Error("Filename errado")
	}
	if ev.Type != "modify" {
		t.Error("Type errado")
	}
}

// ── ProcessFile — attachment (ZIP) ─────────────────────────────

func TestProcessFile_Attachment_SoRegistraFileMod(t *testing.T) {
	store := newTestStore(t)
	cfg := newTestConfig(t)

	filename := "attachments/teste.zip"
	fullPath := filepath.Join(cfg.DocsDir, filename)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("fake zip"), 0644)

	// Processar como modify
	ev := FileEvent{
		Path:     fullPath,
		Filename: filename,
		ModTime:  time.Now(),
		Type:     "modify",
	}
	if err := ProcessFile(store, ev, nil, false); err != nil {
		t.Fatalf("ProcessFile(attachment): %v", err)
	}

	// Deve ter file_mod
	mods, _ := store.GetAllFileMods()
	if _, ok := mods[filename]; !ok {
		t.Error("attachment deveria estar em file_mods")
	}

	// NAO deve ter documentos (diferente de como o upload cria)
	count := store.GetDocumentCount()
	if count > 0 {
		t.Errorf("ProcessFile attachment nao deveria criar documentos, got %d", count)
	}
}

func TestProcessFile_Attachment_DeleteLimpaFileMod(t *testing.T) {
	store := newTestStore(t)

	filename := "attachments/deletar.zip"
	store.SetFileMod(filename, time.Now().Format(time.RFC3339))

	ev := FileEvent{
		Filename: filename,
		Type:     "delete",
	}
	if err := ProcessFile(store, ev, nil, false); err != nil {
		t.Fatalf("ProcessFile(attachment delete): %v", err)
	}

	mods, _ := store.GetAllFileMods()
	if _, ok := mods[filename]; ok {
		t.Error("file_mods deveria ter sido removido no delete")
	}
}

func TestProcessFile_Attachment_NaoRemoveDocsExistentes(t *testing.T) {
	store := newTestStore(t)
	cfg := newTestConfig(t)

	filename := "attachments/preservar.zip"
	fullPath := filepath.Join(cfg.DocsDir, filename)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("zip"), 0644)

	// Simula documento criado pelo upload handler (como o HandleUploadAttachment faz)
	store.InsertDocument(db.Document{
		ID:      "att-preservar",
		Tipo:    "attachment",
		Arquivo: filename,
		Secao:   "📦 preservar.zip",
		Texto:   "Arquivos: documento.txt",
	})
	store.IndexFTS("att-preservar", "attachment", filename, "📦 preservar.zip", "Arquivos: documento.txt", "")
	store.SetFileMod(filename, time.Now().Format(time.RFC3339))

	// ProcessFile com tipo attachment nao deve deletar o documento
	ev := FileEvent{
		Path:     fullPath,
		Filename: filename,
		ModTime:  time.Now(),
		Type:     "modify",
	}
	if err := ProcessFile(store, ev, nil, false); err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	// Documento deve permanecer
	count := store.GetDocumentCount()
	if count != 1 {
		t.Errorf("documento deveria ter sido preservado (1), got %d", count)
	}

	// file_mods deve estar atualizado
	mods, _ := store.GetAllFileMods()
	if _, ok := mods[filename]; !ok {
		t.Error("file_mods deveria conter o attachment")
	}
}

// ── ProcessFile — Embedding decision (unified) ──────────────────
// Testamos a decisao de embedding via tags (nao o embedding em si,
// que é assincrono). A logica unificada em processFileLocked combina
// doc.Tags (frontmatter) + existingFileTags (banco) e usa shouldEmbed.

func TestProcessFile_Embed_TagEmbedPreservadaAposReprocessamento(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)

	// PDF com tag "embed" no banco (como faria o toggle-embed)
	fp := filepath.Join(cfg.DocsDir, "pdfs", "relatorio.pdf")
	os.MkdirAll(filepath.Dir(fp), 0755)
	os.WriteFile(fp, []byte("%PDF-1.4 fake pdf content"), 0644)
	store.AddTagToFile("pdfs/relatorio.pdf", "embed")

	err := ProcessFile(store, FileEvent{
		Path: fp, Filename: "pdfs/relatorio.pdf", ModTime: time.Now(), Type: "modify",
	}, nil, false)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	// Tag "embed" deve estar preservada
	tags, _ := store.GetFileTags("pdfs/relatorio.pdf")
	hasEmbed := false
	for _, t := range tags {
		if t == "embed" {
			hasEmbed = true
			break
		}
	}
	if !hasEmbed {
		t.Error("tag 'embed' do banco deveria ser preservada apos reprocessamento")
	}
}

func TestProcessFile_Embed_SemTagEmbed_NaoGeraTag(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)

	fp := filepath.Join(cfg.DocsDir, "pdfs", "confidencial.pdf")
	os.MkdirAll(filepath.Dir(fp), 0755)
	os.WriteFile(fp, []byte("%PDF-1.4 fake"), 0644)

	err := ProcessFile(store, FileEvent{
		Path: fp, Filename: "pdfs/confidencial.pdf", ModTime: time.Now(), Type: "modify",
	}, nil, false)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	tags, _ := store.GetFileTags("pdfs/confidencial.pdf")
	for _, tg := range tags {
		if tg == "embed" {
			t.Error("PDF sem tag 'embed' nao deveria ter a tag apos processamento")
		}
	}
}

func TestProcessFile_Embed_FrontmatterTagEmbedGeraFileTag(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)

	content := `---
title: Teste
tags: [embed]
---
Conteudo importante.`
	fp := filepath.Join(cfg.DocsDir, "notes", "importante.md")
	os.MkdirAll(filepath.Dir(fp), 0755)
	os.WriteFile(fp, []byte(content), 0644)

	err := ProcessFile(store, FileEvent{
		Path: fp, Filename: "notes/importante.md", ModTime: time.Now(), Type: "modify",
	}, nil, false)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	// A tag "embed" do frontmatter vira file-level tag
	tags, _ := store.GetFileTags("notes/importante.md")
	hasEmbed := false
	for _, t := range tags {
		if t == "embed" {
			hasEmbed = true
			break
		}
	}
	if !hasEmbed {
		t.Error("tag 'embed' do frontmatter deveria virar file-level tag")
	}
}

func TestProcessFile_Embed_FrontmatterSemEmbed_NaoGeraTag(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)

	content := `---
title: Rascunho
tags: [rascunho]
---
Conteudo qualquer.`
	fp := filepath.Join(cfg.DocsDir, "notes", "rascunho.md")
	os.MkdirAll(filepath.Dir(fp), 0755)
	os.WriteFile(fp, []byte(content), 0644)

	err := ProcessFile(store, FileEvent{
		Path: fp, Filename: "notes/rascunho.md", ModTime: time.Now(), Type: "modify",
	}, nil, false)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	tags, _ := store.GetFileTags("notes/rascunho.md")
	for _, tg := range tags {
		if tg == "embed" {
			t.Error("markdown sem tag 'embed' no frontmatter nao deveria ter a tag")
		}
	}
}

func TestProcessFile_Embed_ToggleTagNoBancoPreservadaMesmoSemFrontmatter(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)

	// Markdown SEM tag "embed" no frontmatter
	content := `---
title: Teste
tags: [rascunho]
---
Conteudo.`
	fp := filepath.Join(cfg.DocsDir, "notes", "toggle-test.md")
	os.MkdirAll(filepath.Dir(fp), 0755)
	os.WriteFile(fp, []byte(content), 0644)

	// Mas com tag "embed" no banco (vinda do toggle-embed)
	store.AddTagToFile("notes/toggle-test.md", "embed")

	err := ProcessFile(store, FileEvent{
		Path: fp, Filename: "notes/toggle-test.md", ModTime: time.Now(), Type: "modify",
	}, nil, false)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	// A tag do banco deve sobreviver ao reprocessamento
	tags, _ := store.GetFileTags("notes/toggle-test.md")
	hasEmbed := false
	for _, t := range tags {
		if t == "embed" {
			hasEmbed = true
			break
		}
	}
	if !hasEmbed {
		t.Error("tag 'embed' do toggle-embed deveria ser preservada")
	}
}

func TestProcessFile_Embed_ImagemIndexada(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)

	fp := filepath.Join(cfg.DocsDir, "notes", "foto.png")
	os.MkdirAll(filepath.Dir(fp), 0755)
	os.WriteFile(fp, []byte("fake png"), 0644)

	err := ProcessFile(store, FileEvent{
		Path: fp, Filename: "notes/foto.png", ModTime: time.Now(), Type: "modify",
	}, nil, true) // embedAll=true
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	// A imagem deve ter sido indexada como documento
	if store.GetDocumentCount() == 0 {
		t.Error("imagem deveria ter sido indexada com embedAll=true")
	}
}
