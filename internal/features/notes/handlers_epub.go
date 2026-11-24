package notes

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"ton618/core/internal/httputil"
)

// HandleEpubReader renders the EPUB reader view.
func (ctx *HandlerContext) HandleEpubReader(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	if file == "" {
		http.Error(w, "file parameter required", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	EpubReader(file).Render(r.Context(), w)
}

// HandleGetEpubPosition returns the saved position of an EPUB file.
// GET /api/epub/position?file=<file>
func (ctx *HandlerContext) HandleGetEpubPosition(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	if file == "" {
		http.Error(w, "file parameter required", http.StatusBadRequest)
		return
	}
	key := "epub_position:" + file
	position, err := ctx.Store.GetSetting(key)
	if err != nil {
		slog.Error("get epub position", "file", file, "error", err)
		position = ""
	}
	httputil.WriteJSON(w, map[string]string{
		"position": position,
	})
}

// HandlePostEpubPosition saves the reading position of an EPUB file.
// POST /api/epub/position
func (ctx *HandlerContext) HandlePostEpubPosition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		File     string `json:"file"`
		Position string `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.File == "" || req.Position == "" {
		http.Error(w, "file and position are required", http.StatusBadRequest)
		return
	}
	key := "epub_position:" + req.File
	if err := ctx.Store.SetSetting(key, req.Position); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
