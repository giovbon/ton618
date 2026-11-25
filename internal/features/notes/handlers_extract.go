package notes

import (
	"errors"
	"log/slog"
	"net/http"

	"ton618/core/internal/extract"
	"ton618/core/internal/httputil"
)

// HandleExtractText extrai o texto de um PDF/EPUB e grava o resultado em uma
// nota markdown com link para o documento original.
//
// POST /api/extract-text
//
// Corpo (form ou multipart): file=pdfs/livro.pdf | epubs/livro.epub
// Resposta: { ok, note, url, chars, chapters, truncated }
func (ctx *HandlerContext) HandleExtractText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file := r.FormValue("file")
	result, err := ctx.ExtractDocumentText(r.Context(), file)
	if err != nil {
		slog.Warn("extração de texto falhou", "file", file, "error", err)
		http.Error(w, extractErrorMessage(err), extractErrorStatus(err))
		return
	}

	slog.Info("texto extraído para nota", "file", file, "note", result.Note, "chars", result.Chars, "chapters", result.Chapters, "truncated", result.Truncated)
	httputil.WriteJSON(w, map[string]interface{}{
		"ok":        true,
		"note":      result.Note,
		"url":       result.URL,
		"chars":     result.Chars,
		"chapters":  result.Chapters,
		"truncated": result.Truncated,
	})
}

func extractErrorStatus(err error) int {
	switch {
	case errors.Is(err, ErrInvalidDocument), errors.Is(err, extract.ErrUnsupported):
		return http.StatusBadRequest
	case errors.Is(err, ErrDocumentNotFound):
		return http.StatusNotFound
	case errors.Is(err, extract.ErrEmpty):
		return http.StatusUnprocessableEntity
	case errors.Is(err, extract.ErrToolMissing):
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}

func extractErrorMessage(err error) string {
	switch {
	case errors.Is(err, extract.ErrToolMissing):
		return "Extração de PDF indisponível no servidor (pdftotext/poppler-utils não instalado)."
	case errors.Is(err, extract.ErrEmpty):
		return "Nenhum texto encontrado no documento. Se for um PDF escaneado, é preciso OCR."
	default:
		return err.Error()
	}
}
