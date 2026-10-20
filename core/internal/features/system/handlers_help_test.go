package system

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// /help lê o HELP.md (resolvido por HELP_DOC, ou na raiz) e o converte para
// HTML no servidor (goldmark), renderizando no corpo do article.prose.
func TestHandleHelpPage_RenderizaMarkdown(t *testing.T) {
	ctx := newTestContext(t)

	dir := t.TempDir()
	helpPath := filepath.Join(dir, "HELP.md")
	content := "# Título da Ajuda\n\nUm parágrafo com **negrito** e um `código`.\n\n## Seção\n\n- item 1\n- item 2\n"
	if err := os.WriteFile(helpPath, []byte(content), 0o644); err != nil {
		t.Fatalf("escrever HELP.md: %v", err)
	}
	ctx.Cfg.HelpDocPath = helpPath

	req := httptest.NewRequest("GET", "/help", nil)
	rr := httptest.NewRecorder()
	ctx.HandleHelpPage(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status %d, body: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{"<h1", "Título da Ajuda", "<strong>negrito</strong>", "<code>código</code>", "<h2", "<li>item 1</li>"} {
		if !strings.Contains(body, want) {
			t.Errorf("esperava %q no HTML renderizado, body:\n%s", want, body)
		}
	}
}

// Sem arquivo de ajuda resolvível, a página não quebra: mostra uma mensagem
// amigável no lugar (e com status 200).
func TestHandleHelpPage_SemArquivo_NaoQuebra(t *testing.T) {
	ctx := newTestContext(t)
	// Sem HelpDocPath e sem HELP.md resolvível no cwd.

	req := httptest.NewRequest("GET", "/help", nil)
	rr := httptest.NewRecorder()
	ctx.HandleHelpPage(rr, req)

	if rr.Code != 200 {
		t.Fatalf("sem arquivo deveria renderizar 200 com msg amigável, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Arquivo de ajuda não encontrado") {
		t.Errorf("esperava mensagem amigável, body:\n%s", rr.Body.String())
	}
}
