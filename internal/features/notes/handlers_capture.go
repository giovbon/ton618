package notes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// HandleCapture processa uma URL (artigo web ou video YouTube) e salva como nota.
func (ctx *HandlerContext) HandleCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON invalido", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, "URL obrigatoria", http.StatusBadRequest)
		return
	}

	rawURL := DecodeCaptureURL(req.URL)
	slog.Info("Capturando", "url", rawURL)

	result, err := ctx.Capture.CaptureURL(rawURL)
	if err != nil {
		slog.Error("erro ao capturar", "url", rawURL, "error", err)
		http.Error(w, fmt.Sprintf("Falha ao capturar: %v", err), http.StatusBadGateway)
		return
	}

	if err := ctx.Notes.Save(result.Filename, result.Markdown, nil); err != nil {
		slog.Error("erro ao salvar captura", "file", result.Filename, "error", err)
		http.Error(w, "Erro ao salvar nota", http.StatusInternalServerError)
		return
	}

	slog.Info("Captura salva", "file", result.Filename, "title", result.Title)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"filename": result.Filename,
		"title":    result.Title,
		"url":      "/editor?file=" + result.Filename,
	})
}
