package system

import (
	"bytes"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// resolveHelpDocPath encontra o arquivo markdown do help (/help). Ordem de
// prioridade:
//
//  1. HELP_DOC explícito (se existir);
//  2. HELP.md na raiz do repositório — inferida do WebDir (core/web → raiz);
//  3. HELP.md no diretório de trabalho.
//
// Isso permite que \"tudo o que for escrito na raiz\" (HELP.md) vire o help do
// sistema, incluindo edições em runtime (basta reiniciar/recarregar a página).
func (ctx *HandlerContext) resolveHelpDocPath() string {
	if p := ctx.Cfg.HelpDocPath; p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if ws := ctx.Cfg.WebDir; ws != "" {
		root := filepath.Join(filepath.Dir(filepath.Dir(ws)), "HELP.md")
		if _, err := os.Stat(root); err == nil {
			return root
		}
	}
	if _, err := os.Stat("HELP.md"); err == nil {
		return "HELP.md"
	}
	return ""
}

// HandleHelpPage renderiza a página /help: lê o HELP.md da raiz e o converte
// para HTML no servidor (goldmark), injetado via @templ.Raw.
func (ctx *HandlerContext) HandleHelpPage(w http.ResponseWriter, r *http.Request) {
	path := ctx.resolveHelpDocPath()
	if path == "" {
		HelpPage("Ajuda — TON-618",
			"<p><em>Arquivo de ajuda não encontrado.</em></p>"+
				"<p>Crie um arquivo <code>HELP.md</code> na raiz do projeto, ou aponte "+
				"<code>HELP_DOC</code> para o seu caminho.</p>", nil, "").Render(r.Context(), w)
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		slog.Error("ajuda: erro ao ler arquivo", "path", path, "error", err)
		http.Error(w, "erro ao ler arquivo de ajuda", http.StatusInternalServerError)
		return
	}

	// GFM habilita tabelas, riscado, listas de tarefas e autolinks — o HELP.md
	// usa tabelas de referência em quase todas as seções.
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))

	// O documento é interpretado uma vez: a mesma árvore serve para gerar o
	// índice (e os `id` das âncoras) e para renderizar o HTML final.
	doc := md.Parser().Parse(text.NewReader(content))
	toc := helpHeadingIDs(doc, content)

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, content, doc); err != nil {
		slog.Error("ajuda: erro ao converter markdown", "error", err)
		http.Error(w, "erro ao renderizar ajuda", http.StatusInternalServerError)
		return
	}

	// O índice entra entre a apresentação e a primeira seção (antes do
	// primeiro <h2>), que é o ponto natural de leitura.
	intro, body := buf.String(), ""
	if i := strings.Index(intro, "<h2"); i >= 0 {
		intro, body = intro[:i], intro[i:]
	}

	HelpPage("Ajuda — TON-618", intro, toc, body).Render(r.Context(), w)
}
