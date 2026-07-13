package processor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProcessPDF_ArquivoInexistente_AindaCriaStub(t *testing.T) {
	docs, links, tags := ProcessPDF("/tmp/nao_existe_12345.pdf", "notes/test.pdf", time.Now())
	if docs == nil {
		t.Fatal("esperado documento stub mesmo para arquivo inexistente")
	}
	if len(docs) != 1 {
		t.Fatalf("esperado 1 documento, got %d", len(docs))
	}
	if docs[0].Texto != "" {
		t.Fatal("texto do stub deve ser vazio — sem extracao de conteudo")
	}
	if docs[0].Arquivo != "notes/test.pdf" {
		t.Fatalf("arquivo esperado 'notes/test.pdf', got %q", docs[0].Arquivo)
	}
	if links != nil {
		t.Fatalf("esperado nil links, got %v", links)
	}
	if len(tags) != 0 {
		t.Fatalf("tags esperado vazio, got %v", tags)
	}
}

func TestProcessPDF_CaminhoRelativoInexistente_AindaCriaStub(t *testing.T) {
	docs, _, _ := ProcessPDF("relative/nonexistent.pdf", "notes/test.pdf", time.Now())
	if docs == nil {
		t.Fatal("esperado documento stub mesmo para caminho inexistente")
	}
	if docs[0].Texto != "" {
		t.Fatal("texto do stub deve ser vazio")
	}
}

func TestProcessPDF_DocumentoValido(t *testing.T) {
	modTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	docs, links, tags := ProcessPDF("/caminho/qualquer/test.pdf", "notes/meu-documento.pdf", modTime)

	if docs == nil {
		t.Fatal("esperado documento stub")
	}
	if len(docs) != 1 {
		t.Fatalf("esperado 1 documento, got %d", len(docs))
	}

	doc := docs[0]
	if doc.Arquivo != "notes/meu-documento.pdf" {
		t.Fatalf("esperado arquivo 'notes/meu-documento.pdf', got %q", doc.Arquivo)
	}
	if doc.Tipo != "pdf" {
		t.Fatalf("esperado tipo 'pdf', got %q", doc.Tipo)
	}
	if !strings.Contains(doc.Secao, "meu-documento") {
		t.Fatalf("secao deveria conter o nome do arquivo, got %q", doc.Secao)
	}
	if doc.Texto != "" {
		t.Fatal("texto do stub deve ser vazio — sem extracao de conteudo")
	}
	if doc.Timestamp != "2025-01-15T10:30:00Z" {
		t.Fatalf("timestamp esperado '2025-01-15T10:30:00Z', got %q", doc.Timestamp)
	}
	if doc.Hash == "" {
		t.Fatal("hash nao deveria ser vazio")
	}
	if len(doc.Tags) != 0 {
		t.Fatalf("tags esperado vazio, got %v", doc.Tags)
	}
	if links != nil {
		t.Fatalf("esperado nil links para PDF, got %v", links)
	}
	if len(tags) != 0 {
		t.Fatalf("tags esperado vazio, got %v", tags)
	}
}

func TestProcessPDF_HashDeterministico(t *testing.T) {
	modTime := time.Now()
	docs1, _, _ := ProcessPDF("/caminho/qualquer/test.pdf", "notes/test.pdf", modTime)
	docs2, _, _ := ProcessPDF("/caminho/qualquer/test.pdf", "notes/test.pdf", modTime)

	if docs1 == nil || docs2 == nil {
		t.Fatal("esperado documento stub")
	}
	if docs1[0].Hash != docs2[0].Hash {
		t.Fatal("hash deveria ser deterministico para mesmo conteudo")
	}
}

func TestProcessPDF_ArquivoNaoPDF_AindaCriaStub(t *testing.T) {
	pdfPath := filepath.Join(t.TempDir(), "empty.pdf")
	if err := os.WriteFile(pdfPath, []byte("not a pdf"), 0644); err != nil {
		t.Fatal(err)
	}

	// O PDF invalido ainda gera um stub — não há mais extração de texto
	docs, _, _ := ProcessPDF(pdfPath, "notes/empty.pdf", time.Now())
	if docs == nil {
		t.Fatal("esperado documento stub mesmo para PDF invalido")
	}
	if docs[0].Texto != "" {
		t.Fatal("texto do stub deve ser vazio")
	}
	if docs[0].Arquivo != "notes/empty.pdf" {
		t.Fatalf("arquivo esperado 'notes/empty.pdf', got %q", docs[0].Arquivo)
	}
}
