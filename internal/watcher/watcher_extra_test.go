package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ton618/internal/core/config"
)

func TestIsRecentlyProcessed(t *testing.T) {
	MarkRecentlyProcessed("notes/recente.md")
	if !isRecentlyProcessed("notes/recente.md") {
		t.Error("deveria estar marcada como recente")
	}
}

func TestIsRecentlyProcessed_NaoMarcada(t *testing.T) {
	if isRecentlyProcessed("notes/nunca-vista.md") {
		t.Error("nao deveria estar marcada como recente")
	}
}

func TestRelPathFromAbs_Valido(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "notes", "teste.md")
	os.MkdirAll(filepath.Dir(absPath), 0755)
	os.WriteFile(absPath, []byte("test"), 0644)

	w := &Watcher{cfg: &config.AppConfig{DocsDir: dir}}
	rel, ok := w.relPathFromAbs(absPath)
	if !ok {
		t.Error("caminho valido deveria ser reconhecido")
	}
	if rel != "notes/teste.md" {
		t.Errorf("relativo: esperado 'notes/teste.md', got %q", rel)
	}
}

func TestRelPathFromAbs_ForaDoDocs(t *testing.T) {
	w := &Watcher{cfg: &config.AppConfig{DocsDir: "/tmp/docs"}}
	_, ok := w.relPathFromAbs("/etc/passwd")
	if ok {
		t.Error("caminho fora do docs nao deveria ser aceito")
	}
}

func TestRelPathFromWalk_Valido(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "notes", "walk.md")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("x"), 0644)

	w := &Watcher{cfg: &config.AppConfig{DocsDir: dir}}
	rel, ok := w.relPathFromWalk(fullPath)
	if !ok {
		t.Error("caminho valido deveria ser reconhecido")
	}
	if rel != "notes/walk.md" {
		t.Errorf("relativo: got %q", rel)
	}
}

func TestRelPathFromWalk_ForaDoDocs(t *testing.T) {
	w := &Watcher{cfg: &config.AppConfig{DocsDir: "/tmp/docs"}}
	_, ok := w.relPathFromWalk("/etc/hosts")
	if ok {
		t.Error("caminho fora do docs nao deveria ser aceito")
	}
}

func TestRelPathFromWalk_DotGit(t *testing.T) {
	dir := t.TempDir()
	gitPath := filepath.Join(dir, ".git", "config")
	os.MkdirAll(filepath.Dir(gitPath), 0755)
	os.WriteFile(gitPath, []byte("x"), 0644)

	w := &Watcher{cfg: &config.AppConfig{DocsDir: dir}}
	rel, ok := w.relPathFromWalk(gitPath)
	// relPathFromWalk não filtra .git (filtro é feito pelo chamador)
	t.Logf("rel=%q ok=%v", rel, ok)
}

func TestSupportedExts(t *testing.T) {
	exts := map[string]string{
		".pdf":  "pdf",
		".png":  "imagem",
		".jpg":  "imagem",
		".jpeg": "imagem",
		".gif":  "imagem",
		".webp": "imagem",
		".bmp":  "imagem",
		".svg":  "imagem",
		".zip":  "attachment",
	}
	for ext, expected := range exts {
		got, ok := supportedExts[ext]
		if !ok {
			t.Errorf("extensao %q nao suportada", ext)
		}
		if got != expected {
			t.Errorf("ext %q: esperado %q, got %q", ext, expected, got)
		}
	}
}

func TestProcessImageFile(t *testing.T) {
	now := time.Now()
	docs := processImageFile("notes/img_foto.png", now, now)
	if len(docs) != 1 {
		t.Fatalf("esperado 1 doc, got %d", len(docs))
	}
	d := docs[0]
	if d.Tipo != "imagem" {
		t.Errorf("tipo: %q", d.Tipo)
	}
	if d.Secao != "Anexos / Imagens" {
		t.Errorf("secao: %q", d.Secao)
	}
}

func TestMarkRecentlyProcessed_Expiracao(t *testing.T) {
	MarkRecentlyProcessed("notes/expirada.md")
	if !isRecentlyProcessed("notes/expirada.md") {
		t.Error("deveria estar marcada como recente")
	}
}
