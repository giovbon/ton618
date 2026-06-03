package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"ton618/internal/processor"
)

// ── Stopwords Customizadas ──

// HandleGetStopwords retorna a lista de stopwords personalizadas.
func (ctx *HandlerContext) HandleGetStopwords(w http.ResponseWriter, r *http.Request) {
	words := processor.GetCustomStopwords()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stopwords": words,
		"total":     len(words),
	})
}

// HandleAddStopword adiciona uma stopword personalizada.
func (ctx *HandlerContext) HandleAddStopword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Word string `json:"word"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"requisição inválida"}`, http.StatusBadRequest)
		return
	}

	if req.Word == "" {
		http.Error(w, `{"error":"palavra não pode ser vazia"}`, http.StatusBadRequest)
		return
	}

	if err := processor.AddCustomStopword(ctx.Cfg.DocsDir, req.Word); err != nil {
		slog.Error("add stopword", "word", req.Word, "error", err)
		http.Error(w, `{"error":"erro ao adicionar stopword"}`, http.StatusInternalServerError)
		return
	}

	words := processor.GetCustomStopwords()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"stopwords": words,
		"total":    len(words),
	})
}

// HandleRemoveStopword remove uma stopword personalizada.
func (ctx *HandlerContext) HandleRemoveStopword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Word string `json:"word"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"requisição inválida"}`, http.StatusBadRequest)
		return
	}

	if err := processor.RemoveCustomStopword(ctx.Cfg.DocsDir, req.Word); err != nil {
		slog.Error("remove stopword", "word", req.Word, "error", err)
		http.Error(w, `{"error":"erro ao remover stopword"}`, http.StatusInternalServerError)
		return
	}

	words := processor.GetCustomStopwords()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"stopwords": words,
		"total":    len(words),
	})
}
