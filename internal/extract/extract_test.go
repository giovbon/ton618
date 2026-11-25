package extract

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// buildEPUB monta um EPUB de teste. chapters é uma lista de (href, xhtml) na
// ordem em que devem ser gravados; spine define a ordem de leitura pelo id.
func buildEPUB(t *testing.T, dir, title string, manifest map[string]string, spine []string, chapters map[string]string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	epubPath := filepath.Join(dir, "livro.epub")
	f, err := os.Create(epubPath)
	if err != nil {
		t.Fatalf("create epub: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	write := func(name, content string, store bool) {
		method := zip.Deflate
		if store {
			method = zip.Store
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}

	write("mimetype", "application/epub+zip", true)
	write("META-INF/container.xml", `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`, false)

	var manifestXML, spineXML strings.Builder
	for id, href := range manifest {
		manifestXML.WriteString(`<item id="` + id + `" href="` + href + `" media-type="application/xhtml+xml"/>`)
	}
	for _, id := range spine {
		spineXML.WriteString(`<itemref idref="` + id + `"/>`)
	}
	opf := `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>` + title + `</dc:title></metadata>
  <manifest>` + manifestXML.String() + `</manifest>
  <spine>` + spineXML.String() + `</spine>
</package>`
	write("OEBPS/content.opf", opf, false)

	for name, content := range chapters {
		write("OEBPS/"+name, content, false)
	}
	return epubPath
}

func TestEPUB_ExtraiCapitulosNaOrdemDoSpine(t *testing.T) {
	dir := t.TempDir()
	epubPath := buildEPUB(t, dir, "Meu Livro",
		map[string]string{
			"c1": "cap1.xhtml",
			"c2": "cap2.xhtml",
		},
		[]string{"c2", "c1"}, // ordem invertida em relação ao nome dos arquivos
		map[string]string{
			"cap1.xhtml": `<html><head><title>Ignorar</title></head><body><h1>Primeiro</h1><p>Texto do primeiro capítulo.</p></body></html>`,
			"cap2.xhtml": `<html><body><h1>Segundo</h1><p>Texto do segundo capítulo.</p><p>Com <b>negrito</b> e <i>itálico</i>.</p></body></html>`,
		})

	res, err := EPUB(epubPath)
	if err != nil {
		t.Fatalf("EPUB: %v", err)
	}
	if res.Title != "Meu Livro" {
		t.Errorf("título = %q, want %q", res.Title, "Meu Livro")
	}
	if len(res.Chapters) != 2 {
		t.Fatalf("capítulos = %d, want 2", len(res.Chapters))
	}
	if res.Chapters[0].Title != "Segundo" || res.Chapters[1].Title != "Primeiro" {
		t.Errorf("ordem dos capítulos = %q, %q", res.Chapters[0].Title, res.Chapters[1].Title)
	}
	if !strings.Contains(res.Chapters[0].Text, "Texto do segundo capítulo.") {
		t.Errorf("texto do cap 1 ausente: %q", res.Chapters[0].Text)
	}
	if !strings.Contains(res.Chapters[0].Text, "negrito") || !strings.Contains(res.Chapters[0].Text, "itálico") {
		t.Errorf("inline perdido: %q", res.Chapters[0].Text)
	}
	// O h1 usado como título não pode aparecer duplicado no corpo.
	if strings.Contains(res.Chapters[0].Text, "Segundo") {
		t.Errorf("título do capítulo duplicado no texto: %q", res.Chapters[0].Text)
	}
	if res.Truncated {
		t.Error("não deveria estar truncado")
	}
	if res.Chars() == 0 {
		t.Error("Chars() = 0")
	}
}

func TestEPUB_SemContainerUsaFallback(t *testing.T) {
	epubPath := filepath.Join(t.TempDir(), "solto.epub")
	f, err := os.Create(epubPath)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	zw := zip.NewWriter(f)
	for name, content := range map[string]string{
		"b.xhtml": `<html><body><h1>B</h1><p>Segundo.</p></body></html>`,
		"a.xhtml": `<html><body><h1>A</h1><p>Primeiro.</p></body></html>`,
	} {
		w, _ := zw.Create(name)
		w.Write([]byte(content))
	}
	zw.Close()
	f.Close()

	res, err := EPUB(epubPath)
	if err != nil {
		t.Fatalf("EPUB: %v", err)
	}
	if len(res.Chapters) != 2 {
		t.Fatalf("capítulos = %d, want 2", len(res.Chapters))
	}
	if res.Chapters[0].Title != "A" || res.Chapters[1].Title != "B" {
		t.Errorf("ordem do fallback = %q, %q", res.Chapters[0].Title, res.Chapters[1].Title)
	}
}

func TestEPUB_SemTextoRetornaErrEmpty(t *testing.T) {
	dir := t.TempDir()
	epubPath := buildEPUB(t, dir, "Vazio",
		map[string]string{"c1": "cap1.xhtml"},
		[]string{"c1"},
		map[string]string{"cap1.xhtml": `<html><body><script>ignorar()</script></body></html>`})

	_, err := EPUB(epubPath)
	if !errors.Is(err, ErrEmpty) {
		t.Fatalf("err = %v, want ErrEmpty", err)
	}
}

func TestPDF_UsaSaidaDoPdftotext(t *testing.T) {
	oldLook, oldRun := lookPath, runPDFToText
	defer func() { lookPath, runPDFToText = oldLook, oldRun }()

	lookPath = func(string) (string, error) { return "/usr/bin/pdftotext", nil }
	var gotPath string
	runPDFToText = func(_ context.Context, path string) ([]byte, string, bool, error) {
		gotPath = path
		return []byte("Primeira linha\fSegunda página\n\n\nFim"), "", false, nil
	}

	res, err := PDF(context.Background(), "pdfs/teste.pdf")
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}
	if gotPath != "pdfs/teste.pdf" {
		t.Errorf("caminho repassado = %q", gotPath)
	}
	if len(res.Chapters) != 1 {
		t.Fatalf("capítulos = %d, want 1", len(res.Chapters))
	}
	text := res.Chapters[0].Text
	if !strings.Contains(text, "Primeira linha") || !strings.Contains(text, "Segunda página") {
		t.Errorf("texto faltando: %q", text)
	}
	if strings.Contains(text, "\f") {
		t.Error("form feed deveria ter sido normalizado")
	}
	if strings.Contains(text, "\n\n\n") {
		t.Errorf("linhas vazias demais: %q", text)
	}
}

func TestPDF_TruncaNoTeto(t *testing.T) {
	oldLook, oldRun := lookPath, runPDFToText
	defer func() { lookPath, runPDFToText = oldLook, oldRun }()

	lookPath = func(string) (string, error) { return "/usr/bin/pdftotext", nil }
	runPDFToText = func(context.Context, string) ([]byte, string, bool, error) {
		return []byte("texto"), "", true, nil
	}

	res, err := PDF(context.Background(), "pdfs/grande.pdf")
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}
	if !res.Truncated {
		t.Error("Truncated deveria ser true")
	}
}

func TestPDF_SemBinarioRetornaErrToolMissing(t *testing.T) {
	oldLook := lookPath
	defer func() { lookPath = oldLook }()

	lookPath = func(string) (string, error) { return "", errors.New("not found") }

	_, err := PDF(context.Background(), "pdfs/teste.pdf")
	if !errors.Is(err, ErrToolMissing) {
		t.Fatalf("err = %v, want ErrToolMissing", err)
	}
}

func TestFile_FormatoNaoSuportado(t *testing.T) {
	_, err := File(context.Background(), "notes/qualquer.md")
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestCapBuffer_LimitaSaida(t *testing.T) {
	buf := &capBuffer{limit: 4}
	n, err := buf.Write([]byte("abcdef"))
	if err != nil || n != 6 {
		t.Fatalf("Write = %d, %v", n, err)
	}
	if buf.String() != "abcd" {
		t.Errorf("buf = %q", buf.String())
	}
	if !buf.truncated {
		t.Error("truncated deveria ser true")
	}

	buf.Write([]byte("x"))
	if buf.String() != "abcd" || !buf.truncated {
		t.Errorf("escrita após o limite alterou o buffer: %q", buf.String())
	}
}

func TestTruncateUTF8_NaoQuebraRuna(t *testing.T) {
	s := "ação"
	// "aç" = 3 bytes; cortar em 2 cai no meio do "ç" (0xC3 0xA7).
	got := truncateUTF8(s, 2)
	if !utf8.ValidString(got) {
		t.Fatalf("resultado inválido: %q", got)
	}
	if got != "a" {
		t.Errorf("got = %q, want %q", got, "a")
	}
}

func TestNormalizeText_ColapsaEspacosELinhasVazias(t *testing.T) {
	got := normalizeText("\n\n  Olá   mundo \n\n\n\n segunda \t linha  \n")
	want := "Olá mundo\n\nsegunda linha"
	if got != want {
		t.Errorf("got = %q, want %q", got, want)
	}
}
