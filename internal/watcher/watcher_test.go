package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ton618/internal/config"
	"ton618/internal/db"
)

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

func TestSupportedExts_CobreFormatos(t *testing.T) {
	exts := []string{".pdf", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".zip"}
	for _, ext := range exts {
		if _, ok := supportedExts[ext]; !ok {
			t.Errorf("extensao %q nao esta em supportedExts", ext)
		}
	}
}

func TestMonitoredSubDirs(t *testing.T) {
	expected := []string{"pdfs", "attachments", "archives"}
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

func TestProcessFile_ExtensaoInvalida(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)

	fp := filepath.Join(cfg.DocsDir, "notes", "teste.txt")
	os.WriteFile(fp, []byte("arquivo txt"), 0644)

	ev := FileEvent{Path: fp, Filename: "notes/teste.txt", ModTime: time.Now(), Type: "modify"}
	err := ProcessFile(store, ev)
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	// Deve ser ignorado silenciosamente
	docs, _ := store.GetDocumentsByFile("notes/teste.txt")
	if len(docs) != 0 {
		t.Error("extensao invalida nao deveria ser indexada")
	}
}

func TestProcessFile_SkippedExtension(t *testing.T) {
	store := newTestStore(t)
	ev := FileEvent{
		Path:     "/caminho/nao/existe.txt",
		Filename: "notes/inexistente.txt",
		ModTime:  time.Now(),
		Type:     "modify",
	}
	err := ProcessFile(store, ev)
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
	if err := ProcessFile(store, ev); err != nil {
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
	if err := ProcessFile(store, ev); err != nil {
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
	if err := ProcessFile(store, ev); err != nil {
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

func TestProcessFile_Embed_ImagemIndexada(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)

	fp := filepath.Join(cfg.DocsDir, "notes", "foto.png")
	os.MkdirAll(filepath.Dir(fp), 0755)
	os.WriteFile(fp, []byte("fake png"), 0644)

	err := ProcessFile(store, FileEvent{
		Path: fp, Filename: "notes/foto.png", ModTime: time.Now(), Type: "modify",
	})
	if err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	// A imagem deve ter sido indexada como documento
	if store.GetDocumentCount() == 0 {
		t.Error("imagem deveria ter sido indexada")
	}
}

// ── Regressão: tags removidas do frontmatter devem sumir ──────

func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

// ── MarkRecentlyProcessed / isRecentlyProcessed ───────────────────

func TestRecentlyProcessed_MarksAndReturnsTrue(t *testing.T) {
	MarkRecentlyProcessed("notes/test.md")
	if !isRecentlyProcessed("notes/test.md") {
		t.Error("should return true immediately after marking")
	}
}

func TestRecentlyProcessed_UnknownFile_ReturnsFalse(t *testing.T) {
	if isRecentlyProcessed("notes/unknown.md") {
		t.Error("unknown file should return false")
	}
}

func TestRecentlyProcessed_Expires(t *testing.T) {
	// isRecentlyProcessed deletes the entry after checking within 3s,
	// so calling it twice should return false the second time.
	MarkRecentlyProcessed("notes/test.md")
	isRecentlyProcessed("notes/test.md") // first call — returns true and deletes
	if isRecentlyProcessed("notes/test.md") {
		t.Error("second call should return false (entry was deleted)")
	}
}

// ── Events() ─────────────────────────────────────────────────────

func TestEvents_ChannelNaoNulo(t *testing.T) {
	store := newTestStore(t)
	cfg := newTestConfig(t)
	w := NewWatcher(cfg, store)
	if w.Events() == nil {
		t.Error("Events() should not return nil channel")
	}
}

// ── ProcessBatch ─────────────────────────────────────────────────

// ── relPathFromAbs / relPathFromWalk ─────────────────────────────

func TestRelPathFromAbs_Relativo(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)
	w := NewWatcher(cfg, store)

	absPath := filepath.Join(cfg.DocsDir, "notes", "test.md")
	rel, ok := w.relPathFromAbs(absPath)
	if !ok {
		t.Fatal("relPathFromAbs should succeed")
	}
	if rel != "notes/test.md" {
		t.Errorf("expected 'notes/test.md', got %q", rel)
	}
}

func TestRelPathFromAbs_ForaDoDocDir(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)
	w := NewWatcher(cfg, store)

	_, ok := w.relPathFromAbs("/tmp/outside.md")
	if ok {
		t.Error("should return false for path outside DocsDir")
	}
}

func TestRelPathFromWalk_Relativo(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)
	w := NewWatcher(cfg, store)

	absPath := filepath.Join(cfg.DocsDir, "notes", "test.md")
	rel, ok := w.relPathFromWalk(absPath)
	if !ok {
		t.Fatal("relPathFromWalk should succeed")
	}
	if rel != "notes/test.md" {
		t.Errorf("expected 'notes/test.md', got %q", rel)
	}
}

// ── PollAll ──────────────────────────────────────────────────────

func TestPollAll_IndexaArquivosNovos(t *testing.T) {
	cfg := newTestConfig(t)
	store := newTestStore(t)
	w := NewWatcher(cfg, store)

	// PollAll scans monitored directories (notes/ no longer monitored)
	// Use attachments/ with images since they get indexed as documents
	fp1 := filepath.Join(cfg.DocsDir, "attachments", "poll-test1.png")
	fp2 := filepath.Join(cfg.DocsDir, "attachments", "poll-test2.png")
	os.MkdirAll(filepath.Dir(fp1), 0755)
	os.MkdirAll(filepath.Dir(fp2), 0755)
	os.WriteFile(fp1, []byte("fake png"), 0644)
	os.WriteFile(fp2, []byte("fake png"), 0644)

	w.PollAll()

	// Imagens são indexadas como documentos stub
	if c := store.GetDocumentCount(); c != 2 {
		t.Errorf("PollAll should index 2 files, got %d", c)
	}
}

// ── processFileLocked — attachment recovery ─────────────────────

func TestProcessFile_AttachmentRecovery(t *testing.T) {
	store := newTestStore(t)
	cfg := newTestConfig(t)

	filename := "attachments/recover.zip"
	fullPath := filepath.Join(cfg.DocsDir, filename)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("fake zip for recovery test"), 0644)

	// Set up file_mod so it looks like the file was already indexed
	store.SetFileMod(filename, time.Now().Format(time.RFC3339))

	ev := FileEvent{
		Path:     fullPath,
		Filename: filename,
		ModTime:  time.Now(),
		Type:     "modify",
	}
	if err := ProcessFile(store, ev); err != nil {
		t.Fatalf("ProcessFile: %v", err)
	}

	// Should have recovered the document
	count := store.GetDocumentCount()
	if count != 1 {
		t.Errorf("expected 1 recovered document, got %d", count)
	}

	// Verify the tag was set
	tags, _ := store.GetFileTags(filename)
	if len(tags) != 1 || tags[0] != "zip" {
		t.Errorf("expected [zip] tags for recovered attachment, got %v", tags)
	}
}

// ── Start — context cancellation ────────────────────────────────

func TestWatcher_StartCancellation(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.PollIntervalSec = time.Hour // avoid panic from NewTicker(0)
	store := newTestStore(t)
	w := NewWatcher(cfg, store)

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)

	// Should not panic when we cancel
	cancel()

	// Allow goroutines to finish
	time.Sleep(50 * time.Millisecond)

	// If we got here, test passes
}
