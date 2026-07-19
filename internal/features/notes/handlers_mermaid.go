package notes

import (
	"net/http"

	"ton618/internal/core/domain"
)

// HandleMermaid renderiza a página do editor split-pane do Mermaid.
func (ctx *HandlerContext) HandleMermaid(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	filename, redirected := ensureNoteFilename(w, r, "/mermaid")
	if redirected {
		return
	}

	nd, _ := ctx.loadNoteData(filename)

	// Conteúdo default para uma nova nota Mermaid com frontmatter
	if !nd.Exists {
		nd.Content = "---\ntype: mermaid\n---\ngraph TD\n    A[Início] --> B(Processamento)\n    B --> C{Decisão}\n    C -->|Sim| D[Resultado 1]\n    C -->|Não| E[Resultado 2]"
	} else {
		noteType := domain.DetectNoteType(nd.FileTags, nd.Content, filename)
		if redirectIfWrongEditor(w, r, noteType, "/mermaid", filename) {
			return
		}
	}

	data := buildEditorData("Mermaid - "+filename, filename, nd)
	Mermaid(data).Render(r.Context(), w)
}
