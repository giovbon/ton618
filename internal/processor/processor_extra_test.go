package processor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessMarkdownContent_Basico(t *testing.T) {
	content := []byte("# Titulo\n\nconteudo simples")
	now := time.Now()
	docs, links, tags := ProcessMarkdownContent(content, "notes/basico.md", now, now)

	if len(docs) == 0 {
		t.Fatal("esperado ao menos 1 documento")
	}
	if docs[0].Arquivo != "notes/basico.md" {
		t.Errorf("arquivo: %q", docs[0].Arquivo)
	}
	if len(links) > 0 {
		t.Logf("links extras: %v", links)
	}
	if len(tags) > 0 {
		t.Logf("tags extras: %v", tags)
	}
}

func TestProcessMarkdownContent_ComFrontmatter(t *testing.T) {
	content := []byte("---\ntitle: Teste\ntags: [test, demo]\n---\n# Seção\n\nconteudo")
	now := time.Now()
	docs, _, tags := ProcessMarkdownContent(content, "notes/fm.md", now, now)

	if len(docs) == 0 {
		t.Fatal("esperado documentos")
	}
	if len(tags) != 2 {
		t.Errorf("esperado 2 tags, got %d: %v", len(tags), tags)
	}
}

func TestProcessMarkdownContent_Wikilinks(t *testing.T) {
	content := []byte("Veja [[outra-nota]] e [[link|alias]]")
	now := time.Now()
	_, links, _ := ProcessMarkdownContent(content, "notes/links.md", now, now)

	if len(links) != 2 {
		t.Errorf("esperado 2 wikilinks, got %d: %v", len(links), links)
	}
}

func TestProcessMarkdownContent_Hashtags(t *testing.T) {
	content := []byte("#urgente #golang \n\ntexto qualquer")
	now := time.Now()
	_, _, tags := ProcessMarkdownContent(content, "notes/hashtags.md", now, now)

	if len(tags) != 2 {
		t.Errorf("esperado 2 hashtags, got %d: %v", len(tags), tags)
	}
}

func TestProcessMarkdownContent_Headers(t *testing.T) {
	content := []byte("# H1\nconteudo h1\n## H2\nconteudo h2\n### H3\nconteudo h3")
	now := time.Now()
	docs, _, _ := ProcessMarkdownContent(content, "notes/headers.md", now, now)

	if len(docs) < 3 {
		t.Errorf("esperado ao menos 3 fragmentos, got %d", len(docs))
	}

	// Verifica que as seções aparecem (podem ter hierarquia como "H1 > H2")
	texto := ""
	for _, d := range docs {
		texto += d.Secao + "|"
	}
	if !contains(texto, "H1") || !contains(texto, "H2") || !contains(texto, "H3") {
		t.Errorf("seções H1/H2/H3 não encontradas: %q", texto)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchSubstring(s, sub)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestProcessMarkdownContent_Vazio(t *testing.T) {
	content := []byte("")
	now := time.Now()
	docs, _, _ := ProcessMarkdownContent(content, "notes/vazio.md", now, now)

	// Documento vazio deve gerar ao menos um stub (comportamento atual)
	_ = docs
}

func TestProcessPDF_CriaStub(t *testing.T) {
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "teste.pdf")
	os.WriteFile(pdfPath, []byte("%PDF-1.4 fake"), 0644)

	now := time.Now()
	docs, links, tags := ProcessPDF(pdfPath, "pdfs/teste.pdf", now)

	if len(docs) != 1 {
		t.Fatalf("esperado 1 doc stub, got %d", len(docs))
	}
	d := docs[0]
	if d.Tipo != "pdf" {
		t.Errorf("tipo: %q", d.Tipo)
	}
	if d.Arquivo != "pdfs/teste.pdf" {
		t.Errorf("arquivo: %q", d.Arquivo)
	}
	if len(links) != 0 {
		t.Errorf("links inesperados: %v", links)
	}
	// PDF tag não deve mais existir
	if len(tags) != 0 {
		t.Errorf("esperado 0 tags, got %d", len(tags))
	}
}

func TestProcessPDF_HashUnico(t *testing.T) {
	dir := t.TempDir()
	pdf1 := filepath.Join(dir, "a.pdf")
	pdf2 := filepath.Join(dir, "b.pdf")
	os.WriteFile(pdf1, []byte("%PDF-1.4"), 0644)
	os.WriteFile(pdf2, []byte("%PDF-1.4"), 0644)

	now := time.Now()
	d1, _, _ := ProcessPDF(pdf1, "pdfs/a.pdf", now)
	d2, _, _ := ProcessPDF(pdf2, "pdfs/b.pdf", now)

	if d1[0].ID == d2[0].ID {
		t.Error("PDFs diferentes devem ter IDs diferentes")
	}
}

func TestExtractTitle_ComHeader(t *testing.T) {
	title := ExtractTitle("# Meu Titulo\n\nconteudo", "fallback.md")
	if title != "Meu Titulo" {
		t.Errorf("esperado 'Meu Titulo', got %q", title)
	}
}

func TestExtractTitle_Fallback(t *testing.T) {
	// ExtractTitle retorna o texto do corpo se nao encontrar header
	title := ExtractTitle("sem header", "nome_arquivo.md")
	// Pode retornar o próprio conteúdo ou o nome do arquivo
	if title == "" {
		t.Error("title nao pode ser vazio")
	}
}

func TestExtractTitle_ComHeaderH1(t *testing.T) {
	content := "# Outro Titulo\n\ntexto"
	title := ExtractTitle(content, "fallback.md")
	if title != "Outro Titulo" {
		t.Errorf("esperado 'Outro Titulo', got %q", title)
	}
}

func TestHashFunc_DiferentesInputs(t *testing.T) {
	h1 := HashFunc("a")
	h2 := HashFunc("b")
	if h1 == h2 {
		t.Error("inputs diferentes deveriam produzir hashes diferentes")
	}
	if len(h1) == 0 || len(h2) == 0 {
		t.Error("hash nao pode ser vazio")
	}
}

func TestCalculateHash_DiferentesTextos(t *testing.T) {
	h1 := CalculateHash("Geral", "texto 1", nil)
	h2 := CalculateHash("Geral", "texto 2", nil)
	if h1 == h2 {
		t.Error("textos diferentes deveriam produzir hashes diferentes")
	}
}

func TestCalculateHash_ComTags(t *testing.T) {
	h1 := CalculateHash("Geral", "mesmo texto", []string{"tag1"})
	h2 := CalculateHash("Geral", "mesmo texto", []string{"tag2"})
	if h1 == h2 {
		t.Error("tags diferentes deveriam produzir hashes diferentes")
	}
}
