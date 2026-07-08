package notes

import (
	"log/slog"
	"net/http"
	"net/url"

	"ton618/internal/core/domain"
	"ton618/internal/processor"
)

// noteLoadResult agrupa todos os dados necessários para renderizar qualquer editor de nota.
type noteLoadResult struct {
	Content   string
	FileTags  []string // tags brutas incluindo tags internas de tipo
	UserTags  []string // tags filtradas para display (sem tags internas)
	AllTags   []string
	Backlinks *domain.BacklinksResult
	Exists    bool // true se o conteúdo foi carregado do banco (nota existente)
}

// loadNoteData carrega todos os dados comuns necessários para renderizar qualquer editor.
// Centraliza: GetNote + IncrementPopularity + GetFileTags + FilterUserTags +
// GetAllTags + GetBacklinks.
func (ctx *HandlerContext) loadNoteData(filename string) (noteLoadResult, error) {
	var r noteLoadResult

	if data, err := ctx.Store.GetNote(filename); err == nil && data != "" {
		r.Content = data
		r.Exists = true
		ctx.Store.IncrementPopularity(filename)
	}

	fileTags, _ := ctx.Store.GetFileTags(filename)
	r.FileTags = fileTags
	r.UserTags = domain.FilterUserTags(fileTags)

	allTags, _ := ctx.Store.GetAllTags()
	r.AllTags = allTags

	backlinks, err := ctx.Notes.GetBacklinks(filename)
	if err != nil {
		slog.Error("get backlinks", "file", filename, "error", err)
		backlinks = &domain.BacklinksResult{}
	}
	r.Backlinks = backlinks

	return r, nil
}

// ensureNoteFilename normaliza o filename da query string e redireciona se necessário.
// Retorna o filename normalizado e true se um redirect foi enviado.
func ensureNoteFilename(w http.ResponseWriter, r *http.Request, route string) (string, bool) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		filename = "notes/" + processor.GenerateCUID2() + ".md"
	}
	sanitized := NoteFilename(filename)
	if sanitized != filename {
		http.Redirect(w, r, route+"?file="+url.QueryEscape(sanitized), http.StatusFound)
		return sanitized, true
	}
	return sanitized, false
}

// redirectIfWrongEditor redireciona para o editor correto se o tipo da nota
// não corresponder à rota atual. Retorna true se um redirect foi enviado.
// Só deve ser chamado quando a nota já existe (nd.Exists == true).
func redirectIfWrongEditor(w http.ResponseWriter, r *http.Request, noteType domain.NoteType, currentRoute, filename string) bool {
	correctRoute := noteType.EditorRoute()
	if correctRoute != currentRoute {
		http.Redirect(w, r, correctRoute+"?file="+url.QueryEscape(filename), http.StatusFound)
		return true
	}
	return false
}

// buildEditorData constrói o EditorData a partir de um noteLoadResult.
func buildEditorData(title, filename string, nd noteLoadResult) domain.EditorData {
	return domain.EditorData{
		Title:       title,
		Filename:    filename,
		DisplayName: domain.DisplayName(filename),
		Content:     nd.Content,
		Tags:        nd.UserTags,
		AllTags:     nd.AllTags,
		Backlinks:   nd.Backlinks,
	}
}
