package notes

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/extract"
)

// writeTestEPUB grava um EPUB mínimo (container + OPF + capítulos) no docsDir.
func writeTestEPUB(t *testing.T, docsDir, relPath, title string, chapters [][2]string) {
	t.Helper()
	fullPath := filepath.Join(docsDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(fullPath)
	if err != nil {
		t.Fatalf("create epub: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	write := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		w.Write([]byte(content))
	}

	mt, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	mt.Write([]byte("application/epub+zip"))
	write("META-INF/container.xml", `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`)

	var manifest, spine strings.Builder
	for i, ch := range chapters {
		id := "c" + string(rune('1'+i))
		manifest.WriteString(`<item id="` + id + `" href="` + ch[0] + `" media-type="application/xhtml+xml"/>`)
		spine.WriteString(`<itemref idref="` + id + `"/>`)
		write("OEBPS/"+ch[0], `<html><body><h1>`+ch[1]+`</h1><p>Conteúdo de `+ch[1]+`.</p></body></html>`)
	}
	write("OEBPS/content.opf", `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>`+title+`</dc:title></metadata>
  <manifest>`+manifest.String()+`</manifest><spine>`+spine.String()+`</spine>
</package>`)

	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
}

func TestValidateExtractableDocument(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"pdfs/livro.pdf", "pdfs/livro.pdf", false},
		{"epubs/livro.epub", "epubs/livro.epub", false},
		{`pdfs\livro.pdf`, "pdfs/livro.pdf", false},
		{"pdfs/sub/livro.PDF", "pdfs/sub/livro.PDF", false},
		{"", "", true},
		{"notes/nota.md", "", true},
		{"pdfs/livro.txt", "", true},
		{"../pdfs/livro.pdf", "", true},
		{"/etc/passwd", "", true},
		{"attachments/livro.pdf", "", true},
	}
	for _, tt := range tests {
		got, err := validateExtractableDocument(tt.in)
		if tt.wantErr {
			if !errors.Is(err, ErrInvalidDocument) {
				t.Errorf("%q: err = %v, want ErrInvalidDocument", tt.in, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: erro inesperado: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q: got %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractDocumentText_EPUBCriaNotaComLinkEFonte(t *testing.T) {
	ctx := newTestContext(t)
	writeTestEPUB(t, ctx.Cfg.DocsDir, "epubs/meu-livro.epub", "Meu Livro", [][2]string{
		{"cap1.xhtml", "Capítulo Um"},
		{"cap2.xhtml", "Capítulo Dois"},
	})

	res, err := ctx.ExtractDocumentText(context.Background(), "epubs/meu-livro.epub")
	if err != nil {
		t.Fatalf("ExtractDocumentText: %v", err)
	}
	if res.Note != "notes/meu-livro-texto.md" {
		t.Errorf("note = %q", res.Note)
	}
	if res.URL != "/editor?file=notes/meu-livro-texto.md" {
		t.Errorf("url = %q", res.URL)
	}
	if res.Chapters != 2 || res.Chars == 0 {
		t.Errorf("res = %+v", res)
	}

	content, err := ctx.Store.GetNote(res.Note)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	for _, want := range []string{
		"tags: [" + extractTag + "]",
		"# Meu Livro",
		"Abrir EPUB",
		"/epub/reader?file=epubs/meu-livro.epub",
		"## Capítulo Um",
		"## Capítulo Dois",
		"Conteúdo de Capítulo Dois.",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("nota não contém %q:\n%s", want, content)
		}
	}
	if got := extractNoteSource(content); got != "epubs/meu-livro.epub" {
		t.Errorf("fonte = %q, want epubs/meu-livro.epub", got)
	}

	if !ctx.Store.NoteExists(res.Note) {
		t.Error("nota não foi gravada no banco")
	}
}

func TestExtractDocumentText_ReextraiSobrescrevendoMesmaNota(t *testing.T) {
	ctx := newTestContext(t)
	writeTestEPUB(t, ctx.Cfg.DocsDir, "epubs/livro.epub", "Livro", [][2]string{{"c1.xhtml", "Original"}})

	first, err := ctx.ExtractDocumentText(context.Background(), "epubs/livro.epub")
	if err != nil {
		t.Fatalf("1ª extração: %v", err)
	}

	writeTestEPUB(t, ctx.Cfg.DocsDir, "epubs/livro.epub", "Livro", [][2]string{{"c1.xhtml", "Atualizado"}})
	second, err := ctx.ExtractDocumentText(context.Background(), "epubs/livro.epub")
	if err != nil {
		t.Fatalf("2ª extração: %v", err)
	}

	if second.Note != first.Note {
		t.Errorf("reextração criou outra nota: %q != %q", second.Note, first.Note)
	}
	content, _ := ctx.Store.GetNote(second.Note)
	if !strings.Contains(content, "Conteúdo de Atualizado.") {
		t.Errorf("nota não foi atualizada:\n%s", content)
	}
}

func TestExtractDocumentText_NomeOcupadoPorOutraOrigemGeraSufixo(t *testing.T) {
	ctx := newTestContext(t)
	writeTestEPUB(t, ctx.Cfg.DocsDir, "epubs/livro.epub", "Livro", [][2]string{{"c1.xhtml", "Cap"}})

	// Nota homônima de OUTRO documento: não pode ser sobrescrita.
	other := "---\ntags: [texto-extraido]\nfonte: \"epubs/outro.epub\"\n---\n\n# Outro\n"
	if err := ctx.Notes.Save("notes/livro-texto.md", other, nil); err != nil {
		t.Fatalf("Save: %v", err)
	}

	res, err := ctx.ExtractDocumentText(context.Background(), "epubs/livro.epub")
	if err != nil {
		t.Fatalf("ExtractDocumentText: %v", err)
	}
	if res.Note != "notes/livro-texto-2.md" {
		t.Errorf("note = %q, want notes/livro-texto-2.md", res.Note)
	}
	preserved, _ := ctx.Store.GetNote("notes/livro-texto.md")
	if extractNoteSource(preserved) != "epubs/outro.epub" {
		t.Errorf("nota de outra origem foi alterada:\n%s", preserved)
	}
}

func TestExtractDocumentText_NomeNaoViraMapaMental(t *testing.T) {
	ctx := newTestContext(t)
	writeTestEPUB(t, ctx.Cfg.DocsDir, "epubs/Mindmap Guide.epub", "Guia", [][2]string{{"c1.xhtml", "Cap"}})

	res, err := ctx.ExtractDocumentText(context.Background(), "epubs/Mindmap Guide.epub")
	if err != nil {
		t.Fatalf("ExtractDocumentText: %v", err)
	}
	if res.Note != "notes/doc-guide-texto.md" {
		t.Errorf("note = %q, want notes/doc-guide-texto.md", res.Note)
	}
	if got := domain.DetectNoteType(nil, res.Note); got != domain.NoteTypeMarkdown {
		t.Errorf("tipo detectado = %v, want %v", got, domain.NoteTypeMarkdown)
	}
}

func TestExtractDocumentText_DocumentoInexistente(t *testing.T) {
	ctx := newTestContext(t)
	_, err := ctx.ExtractDocumentText(context.Background(), "pdfs/nao-existe.pdf")
	if !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("err = %v, want ErrDocumentNotFound", err)
	}
}

func TestExtractDocumentText_PDFViaBinario(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script de pdftotext falso usa shell POSIX")
	}
	ctx := newTestContext(t)

	// Instala um `pdftotext` falso no PATH para não depender do poppler.
	binDir := t.TempDir()
	script := "#!/bin/sh\nprintf 'Linha um do PDF\\nLinha dois do PDF\\n'\n"
	if err := os.WriteFile(filepath.Join(binDir, "pdftotext"), []byte(script), 0755); err != nil {
		t.Fatalf("script: %v", err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)

	pdfPath := filepath.Join(ctx.Cfg.DocsDir, "pdfs", "relatorio.pdf")
	os.MkdirAll(filepath.Dir(pdfPath), 0755)
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 fake"), 0644); err != nil {
		t.Fatalf("pdf: %v", err)
	}

	res, err := ctx.ExtractDocumentText(context.Background(), "pdfs/relatorio.pdf")
	if err != nil {
		t.Fatalf("ExtractDocumentText: %v", err)
	}
	if res.Note != "notes/relatorio-texto.md" {
		t.Errorf("note = %q", res.Note)
	}
	content, _ := ctx.Store.GetNote(res.Note)
	if !strings.Contains(content, "Linha um do PDF") {
		t.Errorf("texto do PDF ausente:\n%s", content)
	}
	if !strings.Contains(content, "/file?name=pdfs/relatorio.pdf") {
		t.Errorf("link para o PDF ausente:\n%s", content)
	}
}

func TestBuildExtractNoteMarkdown_TruncadoEEscapaLink(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 30, 0, 0, time.UTC)
	md := buildExtractNoteMarkdown("pdfs/meu arquivo #1.pdf", "Título", &extract.Result{
		Chapters:  []extract.Chapter{{Text: "corpo"}},
		Truncated: true,
	}, now)

	if !strings.Contains(md, urlEscapeForTest("pdfs/meu arquivo #1.pdf")) {
		t.Errorf("URL do documento não foi escapada:\n%s", md)
	}
	if !strings.Contains(md, "truncado") {
		t.Errorf("aviso de truncamento ausente:\n%s", md)
	}
	if !strings.Contains(md, "25/11/2026 10:30") {
		t.Errorf("timestamp ausente:\n%s", md)
	}
}

func urlEscapeForTest(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "%2F", "/")
}

func TestHandleExtractText_Sucesso(t *testing.T) {
	ctx := newTestContext(t)
	writeTestEPUB(t, ctx.Cfg.DocsDir, "epubs/livro.epub", "Livro", [][2]string{{"c1.xhtml", "Cap"}})

	form := url.Values{"file": {"epubs/livro.epub"}}
	req := httptest.NewRequest("POST", "/api/extract-text", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleExtractText(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp["ok"] != true || resp["note"] != "notes/livro-texto.md" {
		t.Errorf("resposta = %v", resp)
	}
}

func TestHandleExtractText_Erros(t *testing.T) {
	ctx := newTestContext(t)

	tests := []struct {
		name   string
		file   string
		status int
	}{
		{"caminho inválido", "notes/nota.md", http.StatusBadRequest},
		{"inexistente", "pdfs/nao-existe.pdf", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{"file": {tt.file}}
			req := httptest.NewRequest("POST", "/api/extract-text", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			ctx.HandleExtractText(rec, req)
			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tt.status, rec.Body.String())
			}
		})
	}
}
