package notes

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ton618/internal/processor"
	"ton618/internal/watcher"
)

var compressedExts = map[string]bool{
	// Archives / Compression
	".zip":  true,
	".rar":  true,
	".7z":   true,
	".tar":  true,
	".gz":   true,
	".tgz":  true,
	".bz2":  true,
	".tbz2": true,
	".xz":   true,
	".txz":  true,
	".lzma": true,
	".tlz":  true,
	".z":    true,
	".zst":  true,
	".lz":   true,
	".apk":  true,
	".jar":  true,
	".war":  true,
	".ear":  true,
	// Images
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".heic": true,
	".heif": true,
	".tiff": true,
	".tif":  true,
	".ico":  true,
	// Audio
	".mp3":  true,
	".m4a":  true,
	".aac":  true,
	".flac": true,
	".ogg":  true,
	".opus": true,
	".wav":  true,
	".wma":  true,
	// Video
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".webm": true,
	".flv":  true,
	".wmv":  true,
	".mpeg": true,
	".mpg":  true,
	".m4v":  true,
	".3gp":  true,
	// Disk Images / Binaries / Documents
	".iso":   true,
	".img":   true,
	".dmg":   true,
	".bin":   true,
	".exe":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".deb":   true,
	".rpm":   true,
	".pdf":   true,
	".epub":  true,
}

// ── Helpers de normalizacao ──

// copyFile copies a file from src to dst path, creating parent dirs as needed.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}

// noteFilename garante que o nome do arquivo:
// 1. Tenha extensao .md
// 2. Esteja no diretorio notes/
func NoteFilename(name string) string {
	// Garante extensao .md
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	// Garante prefixo notes/
	if !strings.HasPrefix(name, "notes/") {
		name = "notes/" + name
	}
	return name
}

// isNoteOrPdf checks if a file path belongs to a note, PDF or attachment document.
func IsNoteOrPdf(path string) bool {
	return strings.HasPrefix(path, "notes/") || strings.HasPrefix(path, "pdfs/") || strings.HasPrefix(path, "attachments/")
}

// ── File handlers ──

// HandleFile serves files directly from disk: PDFs inline, everything else as download.
// Markdown notes are now stored in the database and served via the editor.
func (ctx *HandlerContext) HandleFile(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("name")
	if raw == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	// Segurança: só permite subdiretórios conhecidos, previne path traversal
	allowedPrefixes := []string{"notes/", "attachments/", "pdfs/", "archives/", "epubs/"}
	hasPrefix := false
	for _, p := range allowedPrefixes {
		if strings.HasPrefix(raw, p) {
			hasPrefix = true
			break
		}
	}
	if !hasPrefix {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	cleaned := filepath.Clean(raw)
	fullPath, err := safeJoin(ctx.Cfg.DocsDir, cleaned)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	basename := filepath.Base(cleaned)
	ext := strings.ToLower(filepath.Ext(basename))

	if ext == ".pdf" {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "inline; filename=\""+basename+"\"")
	} else if ext == ".epub" {
		w.Header().Set("Content-Type", "application/epub+zip")
		w.Header().Set("Content-Disposition", "inline; filename=\""+basename+"\"")
	} else {
		ct := "application/octet-stream"
		switch ext {
		case ".zip":
			ct = "application/zip"
		case ".png", ".jpg", ".jpeg", ".gif", ".webp":
			ct = "image/" + strings.TrimPrefix(ext, ".")
		case ".mp3", ".wav", ".ogg":
			ct = "audio/" + strings.TrimPrefix(ext, ".")
		case ".mp4", ".webm":
			ct = "video/" + strings.TrimPrefix(ext, ".")
		}
		w.Header().Set("Content-Disposition", "attachment; filename=\""+basename+"\"")
		w.Header().Set("Content-Type", ct)
	}

	http.ServeFile(w, r, fullPath)
}

// HandleFileDownload serve qualquer arquivo do docs/ como download.
// Ex: /file/download?name=attachments/abc.zip → baixa docs/attachments/abc.zip
// Ex: /file/download?name=pdfs/doc.pdf → baixa docs/pdfs/doc.pdf
// Segurança: só permite arquivos dentro de docs/, resolve path traversal via filepath.Base.
func (ctx *HandlerContext) HandleFileDownload(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("name")
	if raw == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	// Segurança: só permite subdiretórios conhecidos, previne path traversal
	allowedPrefixes := []string{"notes/", "attachments/", "pdfs/", "archives/", "epubs/"}
	hasPrefix := false
	for _, p := range allowedPrefixes {
		if strings.HasPrefix(raw, p) {
			hasPrefix = true
			break
		}
	}
	if !hasPrefix {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	// Resolve path traversal
	cleaned := filepath.Clean(raw)
	fullPath, err := safeJoin(ctx.Cfg.DocsDir, cleaned)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	// Verifica se o arquivo existe
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	basename := filepath.Base(cleaned)
	ext := strings.ToLower(filepath.Ext(basename))

	// PDFs: inline; outros: attachment (download)
	if ext == ".pdf" {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "inline; filename=\""+basename+"\"")
	} else if ext == ".epub" {
		w.Header().Set("Content-Type", "application/epub+zip")
		w.Header().Set("Content-Disposition", "inline; filename=\""+basename+"\"")
	} else {
		// Detecta content-type pelo nome
		ct := "application/octet-stream"
		switch ext {
		case ".zip":
			ct = "application/zip"
		case ".png", ".jpg", ".jpeg", ".gif", ".webp":
			ct = "image/" + strings.TrimPrefix(ext, ".")
		case ".mp3", ".wav", ".ogg":
			ct = "audio/" + strings.TrimPrefix(ext, ".")
		case ".mp4", ".webm":
			ct = "video/" + strings.TrimPrefix(ext, ".")
		}
		w.Header().Set("Content-Disposition", "attachment; filename=\""+basename+"\"")
		w.Header().Set("Content-Type", ct)
	}

	http.ServeFile(w, r, fullPath)
}

// doSaveNote é o helper interno compartilhado entre HandleFileSave e HandleNoteSaveJSON.
// Retorna (unchanged bool, err error). unchanged=true quando o conteúdo é idêntico ao salvo.
func (ctx *HandlerContext) doSaveNote(filename, content, tags string) (unchanged bool, err error) {
	if current, e := ctx.Store.GetNote(filename); e == nil && current == content {
		return true, nil
	}
	tagList := strings.Split(tags, ",")
	if e := ctx.Notes.Save(filename, content, tagList); e != nil {
		return false, e
	}
	return false, nil
}

// HandleFileSave saves a note to the database and processes its content.
func (ctx *HandlerContext) HandleFileSave(w http.ResponseWriter, r *http.Request) {
	raw := r.FormValue("filename")
	if raw == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	filename := NoteFilename(raw)
	if _, err := ctx.doSaveNote(filename, r.FormValue("content"), r.FormValue("tags")); err != nil {
		slog.Error("save note", "file", filename, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/editor?file="+SafeFileQueryEscape(filename), http.StatusSeeOther)
}

// HandleNoteSaveJSON salva uma nota e retorna JSON (para chamadas via fetch/XHR).
func (ctx *HandlerContext) HandleNoteSaveJSON(w http.ResponseWriter, r *http.Request) {
	raw := r.FormValue("filename")
	if raw == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	filename := NoteFilename(raw)
	unchanged, err := ctx.doSaveNote(filename, r.FormValue("content"), r.FormValue("tags"))
	if err != nil {
		slog.Error("save note json", "file", filename, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if unchanged || r.FormValue("silent") == "true" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("HX-Redirect", "/editor?file="+SafeFileQueryEscape(filename))
	w.WriteHeader(http.StatusOK)
}

func (ctx *HandlerContext) HandleFileDelete(w http.ResponseWriter, r *http.Request) {
	raw := r.FormValue("filename")
	if raw == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	ft, filename, fullPath, found := resolveFileInfoStrict(ctx.Cfg.DocsDir, raw)
	if !found {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Remove o arquivo físico do disco
	os.Remove(fullPath)

	// Fallback para nomes com caracteres inválidos UTF-8
	if ft == fileTypeNote && strings.Contains(filename, "\uFFFD") {
		allNotes, err := ctx.Store.GetAllNotes()
		if err == nil {
			for dbFile := range allNotes {
				if strings.ToValidUTF8(dbFile, "\uFFFD") == filename {
					if e := ctx.Store.DeleteAllFileRecords(dbFile); e != nil {
						slog.Error("delete all records (fallback utf8)", "file", dbFile, "error", e)
					}
					fPath := filepath.Join(ctx.Cfg.DocsDir, dbFile)
					os.Remove(fPath)
				}
			}
		}
	}

	// Remove from DB (common cleanup for all types, atomically)
	if err := ctx.Store.DeleteAllFileRecords(filename); err != nil {
		slog.Error("delete all file records", "file", filename, "error", err)
	}

	w.Header().Set("HX-Trigger", "reload-sidebar")
	w.WriteHeader(http.StatusOK)
}

func (ctx *HandlerContext) HandleFileRename(w http.ResponseWriter, r *http.Request) {
	rawOld := r.FormValue("old")
	rawNew := r.FormValue("new")
	if rawNew == "" {
		rawNew = r.Header.Get("HX-Prompt")
	}

	if rawOld == "" || rawNew == "" {
		http.Error(w, "old and new required", http.StatusBadRequest)
		return
	}

	ft, oldName, oldFullPath, found := resolveFileInfoStrict(ctx.Cfg.DocsDir, rawOld)
	if !found {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	if ft == fileTypeNote {
		// Note: delega para o NoteService
		if err := ctx.Notes.Rename(rawOld, rawNew); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("HX-Trigger", "reload-sidebar")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Para PDF/ZIP/EPUB: renomeia o arquivo físico
	newBasename := filepath.Base(rawNew)
	extMap := map[fileType]string{
		fileTypePDF:  ".pdf",
		fileTypeEPUB: ".epub",
		fileTypeZip:  ".zip",
	}
	if suffix, ok := extMap[ft]; ok {
		if !strings.HasSuffix(strings.ToLower(newBasename), suffix) {
			newBasename += suffix
		}
	}

	sd := filepath.Dir(oldName)
	newName := sd + "/" + newBasename
	newPath := filepath.Join(ctx.Cfg.DocsDir, newName)

	if err := os.Rename(oldFullPath, newPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update DB: delete old indexes
	if err := ctx.Store.DeleteAllFileRecords(oldName); err != nil {
		slog.Error("delete old file records on rename", "file", oldName, "error", err)
	}

	info, err := os.Stat(newPath)
	if err == nil {
		watcher.ProcessFile(ctx.Store, watcher.FileEvent{
			Path: newPath, Filename: newName, ModTime: info.ModTime(), Type: "create",
		})
	}

	w.Header().Set("HX-Trigger", "reload-sidebar")
	w.WriteHeader(http.StatusOK)
}

// ── Backup ──

// HandleBackup baixa um ZIP com todas as notas (markdown), e opcionalmente PDFs e anexos,
// excluindo a pasta archives/.
func (ctx *HandlerContext) HandleBackup(w http.ResponseWriter, r *http.Request) {
	full := r.URL.Query().Get("full") == "true"
	data, err := ctx.Backup.Create(full)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filename := "ton618-backup-notas-" + processor.GenerateCUID2() + ".zip"
	if full {
		filename = "ton618-backup-completo-" + processor.GenerateCUID2() + ".zip"
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Write(data)
}

// HandleDuplicateNote duplicates an existing note or file, prefixing the name with "copia-".
func (ctx *HandlerContext) HandleDuplicateNote(w http.ResponseWriter, r *http.Request) {
	raw := r.FormValue("filename")
	if raw == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	ft, oldFilename, oldFullPath, found := resolveFileInfoStrict(ctx.Cfg.DocsDir, raw)
	if !found {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	dir := filepath.Dir(oldFilename)
	base := filepath.Base(oldFilename)
	newFilename := filepath.ToSlash(filepath.Join(dir, "copia-"+base))

	if ft == fileTypeNote {
		content, err := ctx.Store.GetNote(oldFilename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if content == "" {
			http.Error(w, "note content not found", http.StatusNotFound)
			return
		}

		// Update title in frontmatter if present
		fm, _, err := ParseFrontmatter(content)
		if err == nil && fm != nil {
			if titleVal, ok := fm["title"]; ok {
				if titleStr, ok := titleVal.(string); ok {
					newContent, err := UpdateFrontmatterProperty(content, "title", "copia-"+titleStr)
					if err == nil {
						content = newContent
					}
				}
			} else if titleVal, ok := fm["titulo"]; ok {
				if titleStr, ok := titleVal.(string); ok {
					newContent, err := UpdateFrontmatterProperty(content, "titulo", "copia-"+titleStr)
					if err == nil {
						content = newContent
					}
				}
			}
		}

		mtime := time.Now().UTC().Format(time.RFC3339)
		if err := ctx.Store.SaveNote(newFilename, content, mtime); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Copy tags
		tags, err := ctx.Store.GetFileTags(oldFilename)
		if err == nil && len(tags) > 0 {
			ctx.Store.SetFileTags(newFilename, tags)
		}

		// Reindex
		if err := ctx.Notes.Save(newFilename, content, nil); err != nil {
			slog.Error("reindex duplicated note", "file", newFilename, "error", err)
		}
	} else {
		// PDF/ZIP/EPUB: copia o arquivo físico
		dstPath := filepath.Join(ctx.Cfg.DocsDir, newFilename)
		if err := copyFile(oldFullPath, dstPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Indexa o arquivo copiado
		info, err := os.Stat(dstPath)
		if err == nil {
			watcher.ProcessFile(ctx.Store, watcher.FileEvent{
				Path: dstPath, Filename: newFilename, ModTime: info.ModTime(), Type: "create",
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":           true,
		"new_filename": newFilename,
	})
}

// HandleEpubReader renders the EPUB reader view.
