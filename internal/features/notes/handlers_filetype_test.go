package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeJoin(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("Caminho valido dentro do baseDir", func(t *testing.T) {
		res, err := safeJoin(tempDir, "notes/minha_nota.md")
		if err != nil {
			t.Fatalf("safeJoin erro inesperado: %v", err)
		}
		expected := filepath.Join(tempDir, "notes/minha_nota.md")
		if res != expected {
			t.Errorf("got %q, want %q", res, expected)
		}
	})

	t.Run("Path traversal com ../", func(t *testing.T) {
		_, err := safeJoin(tempDir, "../../../etc/passwd")
		if err == nil {
			t.Error("safeJoin deveria ter retornado erro para path traversal")
		}
		if !strings.Contains(err.Error(), "path traversal detectado") {
			t.Errorf("mensagem de erro inesperada: %v", err)
		}
	})
}

func TestResolveFileInfo(t *testing.T) {
	tempDir := t.TempDir()

	// Criar arquivos de teste em disco
	os.MkdirAll(filepath.Join(tempDir, "pdfs"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "epubs"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "images"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "attachments"), 0755)

	os.WriteFile(filepath.Join(tempDir, "pdfs", "doc.pdf"), []byte("pdf content"), 0644)
	os.WriteFile(filepath.Join(tempDir, "epubs", "book.epub"), []byte("epub content"), 0644)
	os.WriteFile(filepath.Join(tempDir, "images", "foto.png"), []byte("png content"), 0644)

	t.Run("PDF existente", func(t *testing.T) {
		ft, filename, fullPath, found := resolveFileInfo(tempDir, "doc.pdf")
		if ft != fileTypePDF {
			t.Errorf("got fileType %v, want PDF", ft)
		}
		if filename != "pdfs/doc.pdf" {
			t.Errorf("got filename %q, want pdfs/doc.pdf", filename)
		}
		if !found {
			t.Error("esperava found = true")
		}
		if !strings.HasSuffix(fullPath, "pdfs/doc.pdf") {
			t.Errorf("fullPath inesperado: %s", fullPath)
		}
	})

	t.Run("EPUB", func(t *testing.T) {
		ft, filename, _, found := resolveFileInfo(tempDir, "book.epub")
		if ft != fileTypeEPUB {
			t.Errorf("got fileType %v, want EPUB", ft)
		}
		if filename != "epubs/book.epub" {
			t.Errorf("got filename %q, want epubs/book.epub", filename)
		}
		if !found {
			t.Error("esperava found = true")
		}
	})

	t.Run("Imagem existente em images/", func(t *testing.T) {
		ft, filename, _, found := resolveFileInfo(tempDir, "foto.png")
		if ft != fileTypeImage {
			t.Errorf("got fileType %v, want Image", ft)
		}
		if filename != "images/foto.png" {
			t.Errorf("got filename %q, want images/foto.png", filename)
		}
		if !found {
			t.Error("esperava found = true")
		}
	})

	t.Run("Nota Markdown (nao exige existencia em disco)", func(t *testing.T) {
		ft, filename, _, found := resolveFileInfo(tempDir, "notes/minha_nota.md")
		if ft != fileTypeNote {
			t.Errorf("got fileType %v, want Note", ft)
		}
		if filename != "notes/minha_nota.md" {
			t.Errorf("got filename %q, want notes/minha_nota.md", filename)
		}
		if !found {
			t.Error("esperava found = true")
		}
	})
}

func TestResolveFileInfoStrict(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("PDF inexistente deve retornar found = false", func(t *testing.T) {
		_, _, _, found := resolveFileInfoStrict(tempDir, "inexistente.pdf")
		if found {
			t.Error("esperava found = false para PDF inexistente")
		}
	})

	t.Run("Nota Markdown mesmo inexistente em disco deve retornar found = true", func(t *testing.T) {
		ft, filename, _, found := resolveFileInfoStrict(tempDir, "notes/nova_nota.md")
		if ft != fileTypeNote {
			t.Errorf("got fileType %v, want Note", ft)
		}
		if filename != "notes/nova_nota.md" {
			t.Errorf("got filename %q, want notes/nova_nota.md", filename)
		}
		if !found {
			t.Error("esperava found = true para nota markdown")
		}
	})
}
