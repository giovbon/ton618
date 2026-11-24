package notes

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"ton618/core/internal/core/db"
	"ton618/core/internal/watcher"
)

// RenameNonNoteFile renomeia um arquivo físico não-nota (PDF/EPUB/ZIP),
// preservando os metadados externos (ex.: `pai` da tabela file_metadata),
// reindexando o novo caminho e repontando backlinks. oldName e newName são
// caminhos relativos ao docsDir (ex.: "pdfs/relatorio.pdf").
//
// É o único caminho de rename de arquivo: o editor e o Acervo tinham cópias
// quase idênticas, e a do editor perdia o `pai:`.
func RenameNonNoteFile(store *db.Store, svc *NoteService, docsDir, oldName, newName string) error {
	oldPath := filepath.Join(docsDir, oldName)
	newPath := filepath.Join(docsDir, newName)

	// Metadados externos (ex.: `pai`) — lidos ANTES do rename porque
	// DeleteAllFileRecords os remove; restaurados sob o novo caminho.
	oldMeta, _ := store.GetFileMetadata(oldName)

	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}

	if err := store.DeleteAllFileRecords(oldName); err != nil {
		slog.Error("delete old file records on rename", "file", oldName, "error", err)
	}

	info, err := os.Stat(newPath)
	if err == nil {
		watcher.ProcessFile(store, watcher.FileEvent{
			Path: newPath, Filename: newName, ModTime: info.ModTime(), Type: "create",
		})
	}

	for k, v := range oldMeta {
		if err := store.SetFileMetadata(newName, k, v); err != nil {
			slog.Error("restore file metadata on rename", "file", newName, "key", k, "error", err)
		}
	}

	if svc != nil {
		if err := svc.UpdateBacklinksOnRename(oldName, newName); err != nil {
			slog.Error("update backlinks on non-note file rename", "old", oldName, "new", newName, "error", err)
		}
	}

	return nil
}

// WriteRenameError classifica o erro de rename para o cliente: conflito de
// nome e nota inexistente são erro do usuário (400); o resto é 500. Os handlers
// do editor e do Acervo compartilham essa classificação.
func WriteRenameError(w http.ResponseWriter, err error) {
	msg := err.Error()
	if strings.Contains(msg, "já existe uma nota") ||
		strings.Contains(msg, "nota não encontrada") ||
		strings.Contains(msg, "UNIQUE constraint failed") {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	http.Error(w, msg, http.StatusInternalServerError)
}
