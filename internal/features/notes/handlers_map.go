package notes

import (
	"fmt"
	"net/http"
	"net/url"

	"ton618/internal/core/domain"
	"ton618/internal/processor"
)

func (ctx *HandlerContext) HandleMap(w http.ResponseWriter, r *http.Request) {
	fileParam := r.URL.Query().Get("file")
	if fileParam == "" {
		// Mapas usam um prefixo especial "mapa-" para identificação via nome
		slug := processor.GenerateCUID2()
		fileParam = fmt.Sprintf("notes/mapa-%s.md", slug)
		http.Redirect(w, r, "/map?file="+url.QueryEscape(fileParam), http.StatusFound)
		return
	}

	nd, _ := ctx.loadNoteData(fileParam)

	// Nota existente com tipo incorreto → redireciona para o editor correto
	if nd.Exists {
		noteType := domain.DetectNoteType(nd.FileTags, nd.Content, fileParam)
		if redirectIfWrongEditor(w, r, noteType, "/map", fileParam) {
			return
		}
	}

	data := buildEditorData(domain.DisplayName(fileParam)+" — TON-618", fileParam, nd)
	MapNote(data).Render(r.Context(), w)
}
