package watcher

import (
	"testing"
	"time"
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
		".epub": "epub",
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
