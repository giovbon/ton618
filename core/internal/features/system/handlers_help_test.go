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

// /help usa a extensão GFM do goldmark: as tabelas de referência do HELP.md
// precisam virar <table> (sem GFM, o pipe vira parágrafo cru).
func TestHandleHelpPage_RenderizaTabelaGFM(t *testing.T) {
	ctx := newTestContext(t)

	dir := t.TempDir()
	helpPath := filepath.Join(dir, "HELP.md")
	content := "| Chave | Valor |\n|---|---|\n| tipo | notas |\n"
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
	for _, want := range []string{"<table>", "<th>Chave</th>", "<td>notas</td>"} {
		if !strings.Contains(body, want) {
			t.Errorf("esperava tabela GFM (%q), body:\n%s", want, body)
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

// O índice do /help é derivado dos títulos: cada seção/subseção ganha um `id`
// e o índice traz um link `#id` para ela.
func TestHandleHelpPage_GeraIndiceComAncoras(t *testing.T) {
	ctx := newTestContext(t)

	dir := t.TempDir()
	helpPath := filepath.Join(dir, "HELP.md")
	content := "# Ajuda\n\nApresentação da ajuda.\n\n## 1. Primeiros passos\n\ntexto\n\n### 1.1 Subir o app\n\nmais texto\n"
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

	// âncoras nos títulos
	for _, want := range []string{`id="1-primeiros-passos"`, `id="1-1-subir-o-app"`} {
		if !strings.Contains(body, want) {
			t.Errorf("esperava âncora no título (%q), body:\n%s", want, body)
		}
	}
	// links do índice
	for _, want := range []string{`href="#1-primeiros-passos"`, `href="#1-1-subir-o-app"`} {
		if !strings.Contains(body, want) {
			t.Errorf("esperava link no índice (%q), body:\n%s", want, body)
		}
	}

	// o índice fica depois da apresentação e antes da primeira seção
	iIntro := strings.Index(body, "Apresentação da ajuda.")
	iNav := strings.Index(body, "help-toc-list")
	iH2 := strings.Index(body, "<h2")
	if iIntro < 0 || iNav < 0 || iH2 < 0 {
		t.Fatalf("não achei introdução/índice/primeira seção (intro=%d nav=%d h2=%d)", iIntro, iNav, iH2)
	}
	if !(iIntro < iNav && iNav < iH2) {
		t.Errorf("ordem errada: introdução=%d índice=%d h2=%d", iIntro, iNav, iH2)
	}

	if !strings.Contains(body, "1 seção · 1 subseção") {
		t.Errorf("esperava o resumo do índice, body:\n%s", body)
	}
}

// Adicionar uma seção ao HELP.md já a faz aparecer no índice na requisição
// seguinte: não existe lista paralela para atualizar.
func TestHandleHelpPage_IndiceAcompanhaNovasSecoes(t *testing.T) {
	ctx := newTestContext(t)

	dir := t.TempDir()
	helpPath := filepath.Join(dir, "HELP.md")
	ctx.Cfg.HelpDocPath = helpPath

	render := func() string {
		t.Helper()
		req := httptest.NewRequest("GET", "/help", nil)
		rr := httptest.NewRecorder()
		ctx.HandleHelpPage(rr, req)
		if rr.Code != 200 {
			t.Fatalf("status %d, body: %s", rr.Code, rr.Body.String())
		}
		return rr.Body.String()
	}

	if err := os.WriteFile(helpPath, []byte("## Alfa\n\ntexto\n"), 0o644); err != nil {
		t.Fatalf("escrever HELP.md: %v", err)
	}
	if body := render(); !strings.Contains(body, "1 seção") {
		t.Errorf("primeira renderização deveria ter 1 seção, body:\n%s", body)
	}

	novo := "## Alfa\n\ntexto\n\n## Beta\n\nmais\n\n### Beta 1\n\ndetalhe\n"
	if err := os.WriteFile(helpPath, []byte(novo), 0o644); err != nil {
		t.Fatalf("reescrever HELP.md: %v", err)
	}
	body := render()
	for _, want := range []string{"2 seções · 1 subseção", `href="#beta"`, `href="#beta-1"`} {
		if !strings.Contains(body, want) {
			t.Errorf("índice não acompanhou a seção nova (%q), body:\n%s", want, body)
		}
	}
}

// O mesmo título repetido não pode produzir duas âncoras iguais — o segundo
// ganha sufixo e os dois links continuam apontando para o lugar certo.
func TestHandleHelpPage_TitulosRepetidosGanhamAncoraUnica(t *testing.T) {
	ctx := newTestContext(t)

	dir := t.TempDir()
	helpPath := filepath.Join(dir, "HELP.md")
	content := "## Repetido\n\na\n\n## Repetido\n\nb\n"
	if err := os.WriteFile(helpPath, []byte(content), 0o644); err != nil {
		t.Fatalf("escrever HELP.md: %v", err)
	}
	ctx.Cfg.HelpDocPath = helpPath

	req := httptest.NewRequest("GET", "/help", nil)
	rr := httptest.NewRecorder()
	ctx.HandleHelpPage(rr, req)

	body := rr.Body.String()
	for _, want := range []string{`id="repetido"`, `id="repetido-2"`, `href="#repetido"`, `href="#repetido-2"`} {
		if !strings.Contains(body, want) {
			t.Errorf("esperava %q, body:\n%s", want, body)
		}
	}
}
