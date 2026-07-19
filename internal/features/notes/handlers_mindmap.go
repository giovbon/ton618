package notes

import (
	"net/http"

	"ton618/internal/core/domain"
)

// HandleMindmap renderiza a página do editor split-pane do Markmap.
func (ctx *HandlerContext) HandleMindmap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	filename, redirected := ensureNoteFilename(w, r, "/mindmap")
	if redirected {
		return
	}

	nd, _ := ctx.loadNoteData(filename)

	// Conteúdo default para um novo mapa mental com frontmatter
	if !nd.Exists {
		nd.Content = "---\ntype: markmap\n---\n# Meu Markmap\n\n- Tópico Principal\n  - Subtópico 1\n  - Subtópico 2\n- Outro Tópico"
	} else {
		noteType := domain.DetectNoteType(nd.FileTags, nd.Content, filename)
		if redirectIfWrongEditor(w, r, noteType, "/mindmap", filename) {
			return
		}
	}

	data := buildEditorData("Markmap - "+filename, filename, nd)
	Mindmap(data).Render(r.Context(), w)
}
