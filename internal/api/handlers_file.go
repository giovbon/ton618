package api

import (
	"archive/zip"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ton618/internal/db"
	"ton618/internal/processor"
	"ton618/internal/watcher"
)

// ── Helpers de normalizacao ──

// sanitizeFilename garante que o nome do arquivo:
// 1. Nao tenha subdiretorios (strips tudo antes de /)
// 2. Tenha extensao .md
// 3. Esteja no diretorio notes/
func sanitizeFilename(name string) string {
	// Remove qualquer prefixo de diretorio
	base := filepath.Base(name)
	// Garante extensao .md
	if !strings.HasSuffix(base, ".md") {
		base += ".md"
	}
	// Forca prefixo notes/
	if !strings.HasPrefix(base, "notes/") {
		base = "notes/" + base
	}
	return base
}

// displayName retorna apenas o nome do arquivo (sem diretorio e sem .md) para exibicao.
func displayName(name string) string {
	base := filepath.Base(name)
	return strings.TrimSuffix(base, ".md")
}

// isNoteOrPdf checks if a file path belongs to a note, PDF or attachment document.
func isNoteOrPdf(path string) bool {
	return strings.HasPrefix(path, "notes/") || strings.HasPrefix(path, "pdfs/") || strings.HasPrefix(path, "attachments/")
}

// ── File handlers ──

func (ctx *HandlerContext) HandleFile(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("name")
	if raw == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(raw))
	isPdf := ext == ".pdf"

	if isPdf {
		// PDF pode estar em pdfs/ ou notes/
		basename := filepath.Base(raw)
		subdirs := []string{"pdfs", "notes"}
		for _, sd := range subdirs {
			testPath := filepath.Join(ctx.Cfg.DocsDir, sd, basename)
			if _, err := os.Stat(testPath); err == nil {
				ctx.Store.IncrementPopularity(sd + "/" + basename)
				w.Header().Set("Content-Type", "application/pdf")
				w.Header().Set("Content-Disposition", "inline")
				http.ServeFile(w, r, testPath)
				return
			}
		}
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Anexos (ZIPs): serve como download
	// NOTA: usa filepath.Base para evitar path traversal.
	// O prefixo "attachments/" é forçado para manter a organização.
	if strings.HasPrefix(raw, "attachments/") || strings.HasPrefix(raw, "attachments\\") {
		basename := filepath.Base(raw)
		fullPath := filepath.Join(ctx.Cfg.DocsDir, "attachments", basename)
		if _, err := os.Stat(fullPath); err == nil {
			w.Header().Set("Content-Disposition", "attachment; filename=\""+basename+"\"")
			w.Header().Set("Content-Type", "application/zip")
			http.ServeFile(w, r, fullPath)
			return
		}
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Para arquivos .md e imagens, usa o comportamento anterior
	filename := sanitizeFilename(raw)
	ctx.Store.IncrementPopularity(filename)
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	http.ServeFile(w, r, fullPath)
}

func (ctx *HandlerContext) HandleFileSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := r.FormValue("filename")
	content := r.FormValue("content")
	tags := r.FormValue("tags")

	if raw == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	// Normaliza: remove subdiretorios, garante .md e prefixo notes/
	filename := sanitizeFilename(raw)

	// Garante que o diretorio notes/ existe
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update tags
	if tags != "" {
		tagList := strings.Split(tags, ",")
		var cleanTags []string
		for _, t := range tagList {
			t = strings.TrimSpace(t)
			if t != "" {
				cleanTags = append(cleanTags, t)
			}
		}
		ctx.Store.SetFileTags(filename, cleanTags)
	}

	// Process immediately (sync)
	ev := watcher.FileEvent{
		Path:     fullPath,
		Filename: filename,
		ModTime:  time.Now(),
		Type:     "modify",
	}
	if err := watcher.ProcessFile(ctx.Store, ev, ctx.Embed, ctx.Cfg.EmbeddingAll); err != nil {
		slog.Error("process file after save", "error", err)
	}

	// Mark as recently processed to prevent watcher from reprocessing
	watcher.MarkRecentlyProcessed(filename)

	// Redirect back to editor
	http.Redirect(w, r, "/editor?file="+url.QueryEscape(filename), http.StatusSeeOther)
}

func (ctx *HandlerContext) HandleFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := r.FormValue("filename")
	if raw == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(raw))
	var filename string
	var fullPath string

	if ext == ".pdf" {
		// PDF files: search in pdfs/ or notes/
		basename := filepath.Base(raw)
		subdirs := []string{"pdfs", "notes"}
		found := false
		for _, sd := range subdirs {
			testPath := filepath.Join(ctx.Cfg.DocsDir, sd, basename)
			if err := os.Remove(testPath); err == nil {
				filename = sd + "/" + basename
				fullPath = testPath
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
	} else if ext == ".zip" {
		// ZIP attachments: stored in attachments/
		basename := filepath.Base(raw)
		testPath := filepath.Join(ctx.Cfg.DocsDir, "attachments", basename)
		if err := os.Remove(testPath); err == nil {
			filename = "attachments/" + basename
			fullPath = testPath
		} else if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		filename = sanitizeFilename(raw)
		fullPath = filepath.Join(ctx.Cfg.DocsDir, filename)
	}
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Remove from DB
	ctx.Store.DeleteEmbeddingsByFile(filename)
	ctx.Store.DeleteDocumentsByFile(filename)
	ctx.Store.DeleteFTSByFile(filename)
	ctx.Store.DeleteFileMod(filename)
	ctx.Store.ResetPopularity(filename)
	ctx.Store.SetFileTags(filename, nil)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (ctx *HandlerContext) HandleFileRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawOld := r.FormValue("old")
	rawNew := r.FormValue("new")

	if rawOld == "" || rawNew == "" {
		http.Error(w, "old and new required", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(rawOld))
	isPdf := ext == ".pdf"
	isZip := ext == ".zip"

	var oldName, newName string
	var oldPath, newPath string

	if isPdf {
		// Para PDFs, busca o arquivo em pdfs/ ou notes/
		basename := filepath.Base(rawOld)
		newBasename := filepath.Base(rawNew)
		if !strings.HasSuffix(strings.ToLower(newBasename), ".pdf") {
			newBasename += ".pdf"
		}

		subdirs := []string{"pdfs", "notes"}
		found := false
		for _, sd := range subdirs {
			testPath := filepath.Join(ctx.Cfg.DocsDir, sd, basename)
			if _, err := os.Stat(testPath); err == nil {
				oldName = sd + "/" + basename
				newName = sd + "/" + newBasename
				oldPath = testPath
				newPath = filepath.Join(ctx.Cfg.DocsDir, newName)
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
	} else if isZip {
		// Para Zips, busca em attachments/
		basename := filepath.Base(rawOld)
		newBasename := filepath.Base(rawNew)
		if !strings.HasSuffix(strings.ToLower(newBasename), ".zip") {
			newBasename += ".zip"
		}
		oldName = "attachments/" + basename
		newName = "attachments/" + newBasename
		oldPath = filepath.Join(ctx.Cfg.DocsDir, oldName)
		newPath = filepath.Join(ctx.Cfg.DocsDir, newName)
	} else {
		oldName = sanitizeFilename(rawOld)
		newName = sanitizeFilename(rawNew)
		if oldName == newName {
			http.Redirect(w, r, "/editor?file="+newName, http.StatusSeeOther)
			return
		}
		oldPath = filepath.Join(ctx.Cfg.DocsDir, oldName)
		newPath = filepath.Join(ctx.Cfg.DocsDir, newName)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update DB: delete old, re-index new
	ctx.Store.DeleteDocumentsByFile(oldName)
	ctx.Store.DeleteFTSByFile(oldName)
	ctx.Store.DeleteEmbeddingsByFile(oldName)

	info, err := os.Stat(newPath)
	if err == nil {
		watcher.ProcessFile(ctx.Store, watcher.FileEvent{
			Path: newPath, Filename: newName, ModTime: info.ModTime(), Type: "create",
		}, ctx.Embed, ctx.Cfg.EmbeddingAll)
	}

	redirectTarget := "/editor?file=" + url.QueryEscape(newName)
	http.Redirect(w, r, redirectTarget, http.StatusSeeOther)
}

func (ctx *HandlerContext) HandleUploadAttachment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "no files uploaded", http.StatusBadRequest)
		return
	}

	// Gera nome aleatorio pro zip
	randBytes := make([]byte, 4)
	rand.Read(randBytes)
	zipName := fmt.Sprintf("%x.zip", randBytes)

	// Diretorio de anexos
	attachDir := filepath.Join(ctx.Cfg.DocsDir, "attachments")
	os.MkdirAll(attachDir, 0755)
	zipPath := filepath.Join(attachDir, zipName)

	// Cria o ZIP
	zipFile, err := os.Create(zipPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	zw := zip.NewWriter(zipFile)
	var fileList []map[string]string
	var listText strings.Builder

	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			continue
		}
		w, err := zw.Create(fh.Filename)
		if err != nil {
			src.Close()
			continue
		}
		io.Copy(w, src)
		src.Close()
		listText.WriteString(fh.Filename + " ")
		fileList = append(fileList, map[string]string{
			"name": fh.Filename,
			"size": fmt.Sprintf("%d", fh.Size),
		})
	}
	zw.Close()
	zipFile.Close()

	// Cria documento FTS com a lista de arquivos (pesquisavel)
	filename := "attachments/" + zipName
	docID := processor.HashFunc("att-" + zipName)
	fileListStr := listText.String()

	doc := db.Document{
		ID:        docID,
		Tipo:      "attachment",
		Arquivo:   filename,
		Secao:     "\U0001f4e6 " + zipName,
		Texto:     "Arquivos: " + fileListStr,
		Tags:      "",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Hash:      processor.CalculateHash("att", zipName, nil),
	}
	ctx.Store.InsertDocument(doc)
	ctx.Store.IndexFTS(doc.ID, doc.Tipo, doc.Arquivo, doc.Secao, doc.Texto, "")
	ctx.Store.SetFileTags(filename, []string{"zip"})
	ctx.Store.SetFileMod(filename, time.Now().Format(time.RFC3339))

	// Redireciona pra lista compacta (mesmo comportamento do upload de PDF)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (ctx *HandlerContext) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.ParseMultipartForm(10 << 20) // 10MB

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	isPdf := ext == ".pdf"
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg"

	if !isPdf && !isImage {
		http.Error(w, "apenas arquivos PDF ou imagens (.png, .jpg) sao permitidos", http.StatusForbidden)
		return
	}

	var filename string
	if isPdf {
		filename = "pdfs/" + filepath.Base(header.Filename)
		// Garante extensao .pdf
		if !strings.HasSuffix(filename, ".pdf") {
			filename += ".pdf"
		}
	} else {
		// Imagem: salva em notes/ com prefixo img_ para evitar conflito
		timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
		cleanName := strings.ReplaceAll(filepath.Base(header.Filename), " ", "_")
		filename = fmt.Sprintf("notes/img_%s_%s", timestamp, cleanName)
	}

	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	os.MkdirAll(filepath.Dir(fullPath), 0755)

	dst, err := os.Create(fullPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	io.Copy(dst, file)

	// Process the file (index, embed)
	info, _ := os.Stat(fullPath)
	watcher.ProcessFile(ctx.Store, watcher.FileEvent{
		Path: fullPath, Filename: filename, ModTime: info.ModTime(), Type: "create",
	}, ctx.Embed, ctx.Cfg.EmbeddingAll)

	// Redireciona para a pagina inicial (modo compacto)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
