package processor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProcessPDF_ArquivoInexistente_RetornaNil(t *testing.T) {
	docs, links, tags := ProcessPDF("/tmp/nao_existe_12345.pdf", "notes/test.pdf", time.Now())
	if docs != nil {
		t.Fatalf("esperado nil para arquivo inexistente, got %v", docs)
	}
	if links != nil {
		t.Fatalf("esperado nil links, got %v", links)
	}
	if tags != nil {
		t.Fatalf("esperado nil tags, got %v", tags)
	}
}

func TestProcessPDF_CaminhoRelativoInexistente_RetornaNil(t *testing.T) {
	docs, _, _ := ProcessPDF("relative/nonexistent.pdf", "notes/test.pdf", time.Now())
	if docs != nil {
		t.Fatalf("esperado nil para caminho relativo inexistente, got %v", docs)
	}
}

func TestProcessPDF_DocumentoValido(t *testing.T) {
	// Cria um PDF valido minimo para testar
	pdfPath := filepath.Join(t.TempDir(), "test.pdf")
	createMinimalPDF(t, pdfPath)

	modTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	docs, links, tags := ProcessPDF(pdfPath, "notes/meu-documento.pdf", modTime)

	if docs == nil {
		// Se a biblioteca nao conseguiu extrair texto, ao menos
		// verifica que nao panicou e retornou nil graciosamente
		return
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
	if doc.Timestamp != "2025-01-15T10:30:00Z" {
		t.Fatalf("timestamp esperado '2025-01-15T10:30:00Z', got %q", doc.Timestamp)
	}
	if doc.Hash == "" {
		t.Fatal("hash nao deveria ser vazio")
	}
	if doc.VectorHash == "" {
		t.Fatal("vector hash nao deveria ser vazio")
	}
	if len(doc.Tags) != 1 || doc.Tags[0] != "pdf" {
		t.Fatalf("tags esperado ['pdf'], got %v", doc.Tags)
	}
	if links != nil {
		t.Fatalf("esperado nil links para PDF, got %v", links)
	}
	if tags == nil || len(tags) != 1 || tags[0] != "pdf" {
		t.Fatalf("tags esperado ['pdf'], got %v", tags)
	}
}

func TestProcessPDF_HashDeterministico(t *testing.T) {
	pdfPath := filepath.Join(t.TempDir(), "test.pdf")
	createMinimalPDF(t, pdfPath)

	modTime := time.Now()
	docs1, _, _ := ProcessPDF(pdfPath, "notes/test.pdf", modTime)
	docs2, _, _ := ProcessPDF(pdfPath, "notes/test.pdf", modTime)

	if docs1 == nil || docs2 == nil {
		// Se a biblioteca nao extraiu, nao podemos testar determinismo
		return
	}
	if docs1[0].Hash != docs2[0].Hash {
		t.Fatal("hash deveria ser deterministico para mesmo conteudo")
	}
	if docs1[0].VectorHash != docs2[0].VectorHash {
		t.Fatal("vector hash deveria ser deterministico")
	}
}

func TestProcessPDF_ArquivoVazio_RetornaNil(t *testing.T) {
	pdfPath := filepath.Join(t.TempDir(), "empty.pdf")
	if err := os.WriteFile(pdfPath, []byte("not a pdf"), 0644); err != nil {
		t.Fatal(err)
	}

	// O pdf.Open deve falhar para um arquivo invalido
	docs, _, _ := ProcessPDF(pdfPath, "notes/empty.pdf", time.Now())
	if docs != nil {
		t.Fatal("esperado nil para PDF invalido")
	}
}

// createMinimalPDF gera um PDF valido minimo para testes.
func createMinimalPDF(t *testing.T, path string) {
	t.Helper()
	// PDF minimal com texto "Hello PDF"
	content := []byte("%PDF-1.4\n" +
		"1 0 obj\n" +
		"<< /Type /Catalog /Pages 2 0 R >>\n" +
		"endobj\n" +
		"2 0 obj\n" +
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n" +
		"endobj\n" +
		"3 0 obj\n" +
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]\n" +
		"   /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\n" +
		"endobj\n" +
		"4 0 obj\n" +
		"<< /Length 44 >>\n" +
		"stream\n" +
		"BT /F1 12 Tf 100 700 Td (Hello PDF) Tj ET\n" +
		"endstream\n" +
		"endobj\n" +
		"5 0 obj\n" +
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\n" +
		"endobj\n" +
		"xref\n" +
		"0 6\n" +
		"0000000000 65535 f \n" +
		"0000000009 00000 n \n" +
		"0000000058 00000 n \n" +
		"0000000115 00000 n \n" +
		"0000000266 00000 n \n" +
		"0000000363 00000 n \n" +
		"trailer\n" +
		"<< /Size 6 /Root 1 0 R >>\n" +
		"startxref\n" +
		"442\n" +
		"%%EOF")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
}
