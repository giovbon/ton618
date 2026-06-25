package search

import (
	"log/slog"
	"net/http"

	"ton618/internal/processor"
	)

// ── Stopwords Customizadas ──

// HandleGetStopwords retorna a lista de stopwords personalizadas em formato HTML (HTMX).
func (ctx *HandlerContext) HandleGetStopwords(w http.ResponseWriter, r *http.Request) {
	words := processor.GetCustomStopwords()
	StopwordsList(words).Render(r.Context(), w)
}

// HandleAddStopword adiciona uma stopword personalizada via formulário HTMX.
func (ctx *HandlerContext) HandleAddStopword(w http.ResponseWriter, r *http.Request) {
	word := r.FormValue("word")

	if word == "" {
		http.Error(w, "palavra não pode ser vazia", http.StatusBadRequest)
		return
	}

	if err := processor.AddCustomStopword(ctx.Cfg.DocsDir, word); err != nil {
		slog.Error("add stopword", "word", word, "error", err)
		http.Error(w, "erro ao adicionar stopword", http.StatusInternalServerError)
		return
	}

	words := processor.GetCustomStopwords()
	StopwordsList(words).Render(r.Context(), w)
}

// HandleRemoveStopword remove uma stopword personalizada via HTMX hx-delete.
func (ctx *HandlerContext) HandleRemoveStopword(w http.ResponseWriter, r *http.Request) {
	word := r.URL.Query().Get("word")
	
	if word == "" {
		http.Error(w, "palavra não especificada", http.StatusBadRequest)
		return
	}

	if err := processor.RemoveCustomStopword(ctx.Cfg.DocsDir, word); err != nil {
		slog.Error("remove stopword", "word", word, "error", err)
		http.Error(w, "erro ao remover stopword", http.StatusInternalServerError)
		return
	}

	words := processor.GetCustomStopwords()
	StopwordsList(words).Render(r.Context(), w)
}
