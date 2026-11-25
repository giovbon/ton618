// Package extract extrai o texto de documentos (PDF e EPUB) para que ele possa
// ser transformado em uma nota markdown.
//
// EPUB é lido nativamente (é um ZIP com XHTML — archive/zip + x/net/html já são
// dependências do projeto). PDF delega ao binário `pdftotext` (poppler-utils):
// a extração em Go puro tem qualidade irregular com fontes/CMaps incomuns, e o
// objetivo é justamente guardar texto confiável na nota.
package extract

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// MaxTextBytes é o teto de texto extraído de um documento (16 MiB). Livros
// reais ficam bem abaixo disso (0,5–2 MiB); o teto existe para um PDF/EPUB
// patológico não inflar o SQLite/FTS sem limite.
const MaxTextBytes = 16 << 20

var (
	// ErrUnsupported indica uma extensão sem extrator (só PDF e EPUB).
	ErrUnsupported = errors.New("formato não suportado para extração (use PDF ou EPUB)")
	// ErrEmpty indica que o documento não tem texto extraível (ex: PDF
	// escaneado, que precisaria de OCR).
	ErrEmpty = errors.New("nenhum texto encontrado no documento")
	// ErrToolMissing indica que o `pdftotext` não está instalado.
	ErrToolMissing = errors.New("pdftotext não encontrado: instale poppler-utils para extrair PDFs")
)

// Chapter é um capítulo de EPUB (ou o documento inteiro, no caso de PDF).
type Chapter struct {
	Title string
	Text  string
}

// Result é o texto extraído de um documento.
type Result struct {
	// Title é o título do livro (metadados do EPUB) — vazio no PDF.
	Title string
	// Chapters preserva a divisão do documento. PDF traz um único capítulo
	// sem título.
	Chapters []Chapter
	// Truncated informa que o texto passou de MaxTextBytes e foi cortado.
	Truncated bool
}

// Chars devolve o total de caracteres extraídos.
func (r *Result) Chars() int {
	if r == nil {
		return 0
	}
	n := 0
	for _, c := range r.Chapters {
		n += len([]rune(c.Text))
	}
	return n
}

// Text devolve o texto completo (capítulos separados por uma linha em branco).
func (r *Result) Text() string {
	if r == nil {
		return ""
	}
	parts := make([]string, 0, len(r.Chapters))
	for _, c := range r.Chapters {
		if strings.TrimSpace(c.Text) == "" {
			continue
		}
		parts = append(parts, c.Text)
	}
	return strings.Join(parts, "\n\n")
}

// File extrai o texto de acordo com a extensão do arquivo.
func File(ctx context.Context, path string) (*Result, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".epub":
		return EPUB(path)
	case ".pdf":
		return PDF(ctx, path)
	default:
		return nil, ErrUnsupported
	}
}

// truncateUTF8 corta s em no máximo n bytes sem quebrar um caractere UTF-8.
func truncateUTF8(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// capBuffer é um io.Writer que para de acumular depois de limit bytes e marca
// truncated. Retorna len(p) sempre para não abortar o processo produtor (o
// pdftotext terminaria com erro de escrita e perderíamos o texto já lido).
type capBuffer struct {
	buf       []byte
	limit     int
	truncated bool
}

func (c *capBuffer) Write(p []byte) (int, error) {
	if len(c.buf) >= c.limit {
		c.truncated = true
		return len(p), nil
	}
	remain := c.limit - len(c.buf)
	if len(p) > remain {
		c.buf = append(c.buf, p[:remain]...)
		c.truncated = true
		return len(p), nil
	}
	c.buf = append(c.buf, p...)
	return len(p), nil
}

func (c *capBuffer) String() string { return string(c.buf) }

// normalizeText limpa o texto extraído: normaliza quebras de linha, espaços
// não separáveis e espaços repetidos, e evita sequências de linhas vazias.
//
// Mantém a estrutura de linhas (importante para listas e poesia), mas remove o
// ruído típico de HTML/PDF.
func normalizeText(s string) string {
	s = strings.TrimPrefix(s, "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\u00a0", " ")

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, line)
	}

	// Remove linhas vazias nas pontas.
	for len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

func emptyResult(r *Result) error {
	if r == nil || strings.TrimSpace(r.Text()) == "" {
		return ErrEmpty
	}
	return nil
}
