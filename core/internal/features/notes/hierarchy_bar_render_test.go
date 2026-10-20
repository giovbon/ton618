package notes

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	"ton618/core/internal/core/domain"
)

// ---------------------------------------------------------------------------
// Popover "Abaixo desta" (DECISIONS §6.30)
//
// A lista de filhas mistura NOTAS e ARQUIVOS — o `pai` de um PDF/EPUB/ZIP vive
// na tabela file_metadata, não no frontmatter (§6.24). O link era fixo em
// "/editor?file=...", então um PDF filho abria o EDITOR, que não tem conteúdo
// para um binário, e o usuário via uma nota vazia.
//
// O destino tem que sair de domain.NoteOpenTarget — a mesma regra da sidebar,
// do Tabulator e do painel de Consultas.
// ---------------------------------------------------------------------------

// linksDoPopover devolve href → target de cada <a> renderizado no fragmento.
func linksDoPopover(t *testing.T, html string) map[string]string {
	t.Helper()

	tagRe := regexp.MustCompile(`(?s)<a\s([^>]*)>`)
	attrRe := regexp.MustCompile(`([\w-]+)="([^"]*)"`)

	links := make(map[string]string)
	for _, tag := range tagRe.FindAllStringSubmatch(html, -1) {
		attrs := make(map[string]string)
		for _, a := range attrRe.FindAllStringSubmatch(tag[1], -1) {
			attrs[a[1]] = a[2]
		}
		links[attrs["href"]] = attrs["target"]
	}
	return links
}

func TestChildrenBadge_RotaDeCadaFilha(t *testing.T) {
	children := []domain.NoteRef{
		{Filename: "notes/filha.md", DisplayName: "filha"},
		{Filename: "pdfs/manual.pdf", DisplayName: "manual.pdf"},
		{Filename: "epubs/livro.epub", DisplayName: "livro.epub"},
		{Filename: "attachments/x.zip", DisplayName: "x.zip"},
	}

	var buf bytes.Buffer
	if err := ChildrenBadge(children).Render(context.Background(), &buf); err != nil {
		t.Fatalf("Render do ChildrenBadge: %v", err)
	}
	links := linksDoPopover(t, buf.String())

	// O bug em si: filha que é arquivo caindo no editor.
	for _, proibido := range []string{
		"/editor?file=pdfs/manual.pdf",
		"/editor?file=epubs/livro.epub",
		"/editor?file=attachments/x.zip",
	} {
		if _, ok := links[proibido]; ok {
			t.Errorf("filha que é arquivo não pode abrir no editor: %s", proibido)
		}
	}

	casos := []struct {
		href   string
		target string
	}{
		{"/editor?file=notes/filha.md", ""},
		{"/file?name=pdfs/manual.pdf", "_blank"},
		{"/epub/reader?file=epubs/livro.epub", ""},
		{"/file/download?name=attachments/x.zip", "_blank"},
	}
	for _, c := range casos {
		target, ok := links[c.href]
		if !ok {
			t.Errorf("link ausente no popover: %s (renderizado: %v)", c.href, links)
			continue
		}
		if target != c.target {
			t.Errorf("target de %s = %q, want %q", c.href, target, c.target)
		}
	}
}

// TestChildrenBadge_MostraTodasAsFilhas garante que a correção não descartou
// nenhuma filha no caminho (o popover é navegação: contagem e nomes têm que
// continuar batendo com a lista recebida).
func TestChildrenBadge_MostraTodasAsFilhas(t *testing.T) {
	children := []domain.NoteRef{
		{Filename: "notes/filha.md", DisplayName: "filha"},
		{Filename: "pdfs/manual.pdf", DisplayName: "manual.pdf"},
	}

	var buf bytes.Buffer
	if err := ChildrenBadge(children).Render(context.Background(), &buf); err != nil {
		t.Fatalf("Render do ChildrenBadge: %v", err)
	}
	html := buf.String()

	if got := len(linksDoPopover(t, html)); got != len(children) {
		t.Errorf("esperado %d links (uma por filha), got %d", len(children), got)
	}
	for _, c := range children {
		if !bytes.Contains(buf.Bytes(), []byte(c.DisplayName)) {
			t.Errorf("nome de exibição ausente no popover: %q", c.DisplayName)
		}
	}
}
