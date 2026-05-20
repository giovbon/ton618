package watcher

import (
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
