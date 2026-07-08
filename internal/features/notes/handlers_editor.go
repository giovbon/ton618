package notes

import (
	"net/http"
	"net/url"
	"strings"

	"ton618/internal/core/domain"
	"ton618/internal/processor"
)

// HandleEditor é o handler principal que renderiza o editor de markdown.
// Para notas de tipos especiais (drawing, typst, etc.), redireciona para o editor correto.
func (ctx *HandlerContext) HandleEditor(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		filename = "notes/" + processor.GenerateCUID2() + ".md"
	}

	// ZIPs → download direto
	if strings.HasSuffix(strings.ToLower(filename), ".zip") {
		http.Redirect(w, r, "/file/download?name="+url.QueryEscape(filename), http.StatusFound)
		return
	}

	// Normaliza o filename
	sanitized := NoteFilename(filename)
	if sanitized != filename {
		http.Redirect(w, r, "/editor?file="+url.QueryEscape(sanitized), http.StatusFound)
		return
	}

	nd, _ := ctx.loadNoteData(filename)

	// Redireciona para o editor especializado se necessário
	noteType := domain.DetectNoteType(nd.FileTags, nd.Content, filename)
	if noteType.EditorRoute() != "/editor" {
		http.Redirect(w, r, noteType.EditorRoute()+"?file="+url.QueryEscape(filename), http.StatusFound)
		return
	}

	data := buildEditorData("Editor - "+filename, filename, nd)
	data.AllTags = nd.AllTags
	Editor(data).Render(r.Context(), w)
}

// HandleSpreadsheet renderiza o editor de planilhas.
func (ctx *HandlerContext) HandleSpreadsheet(w http.ResponseWriter, r *http.Request) {
	filename, redirected := ensureNoteFilename(w, r, "/spreadsheet")
	if redirected {
		return
	}

	nd, _ := ctx.loadNoteData(filename)

	if nd.Exists {
		noteType := domain.DetectNoteType(nd.FileTags, nd.Content, filename)
		if redirectIfWrongEditor(w, r, noteType, "/spreadsheet", filename) {
			return
		}
	}

	data := buildEditorData("Planilha - "+filename, filename, nd)
	Spreadsheet(data).Render(r.Context(), w)
}

// HandleDrawing renderiza o editor de desenhos (Excalidraw).
func (ctx *HandlerContext) HandleDrawing(w http.ResponseWriter, r *http.Request) {
	filename, redirected := ensureNoteFilename(w, r, "/drawing")
	if redirected {
		return
	}

	nd, _ := ctx.loadNoteData(filename)

	if nd.Exists {
		noteType := domain.DetectNoteType(nd.FileTags, nd.Content, filename)
		if redirectIfWrongEditor(w, r, noteType, "/drawing", filename) {
			return
		}
	}

	data := buildEditorData("Desenho - "+filename, filename, nd)
	Drawing(data).Render(r.Context(), w)
}
