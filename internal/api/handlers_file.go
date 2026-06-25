package api

import (
	"archive/zip"
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
	"ton618/internal/service"
	"ton618/internal/template/components"
	"ton618/internal/watcher"
)

var compressedExts = map[string]bool{
	// Archives / Compression
	".zip":   true,
	".rar":   true,
	".7z":    true,
	".tar":   true,
	".gz":    true,
	".tgz":   true,
	".bz2":   true,
	".tbz2":  true,
	".xz":    true,
	".txz":   true,
	".lzma":  true,
	".tlz":   true,
	".z":     true,
	".zst":   true,
	".lz":    true,
	".apk":   true,
	".jar":   true,
	".war":   true,
	".ear":   true,
	// Images
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".gif":   true,
	".webp":  true,
	".heic":  true,
	".heif":  true,
	".tiff":  true,
	".tif":   true,
	".ico":   true,
	// Audio
	".mp3":   true,
	".m4a":   true,
	".aac":   true,
	".flac":  true,
	".ogg":   true,
	".opus":  true,
	".wav":   true,
	".wma":   true,
	// Video
	".mp4":   true,
	".mkv":   true,
	".avi":   true,
	".mov":   true,
	".webm":  true,
	".flv":   true,
	".wmv":   true,
	".mpeg":  true,
	".mpg":   true,
	".m4v":   true,
	".3gp":   true,
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

// noteFilename garante que o nome do arquivo:
// 1. Tenha extensao .md
// 2. Esteja no diretorio notes/
func noteFilename(name string) string {
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
func isNoteOrPdf(path string) bool {
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

	cleaned := filepath.Clean(raw)
	fullPath := filepath.Join(ctx.Cfg.DocsDir, cleaned)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	basename := filepath.Base(cleaned)
	ext := strings.ToLower(filepath.Ext(basename))

	if ext == ".pdf" {
		w.Header().Set("Content-Type", "application/pdf")
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
	allowedPrefixes := []string{"notes/", "attachments/", "pdfs/", "archives/"}
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
	fullPath := filepath.Join(ctx.Cfg.DocsDir, cleaned)

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

// HandleFileSave saves a note to the database and processes its content.
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

	filename := noteFilename(raw)

	// Otimização: verifica se o conteúdo enviado é igual ao atual para evitar I/O
	if currentContent, err := ctx.Store.GetNote(filename); err == nil {
		if currentContent == content {
			http.Redirect(w, r, "/editor?file="+url.QueryEscape(filename), http.StatusSeeOther)
			return
		}
	}

	// Delega para o NoteService (salva + reindexa + tags + links)
	tagList := strings.Split(tags, ",")
	if err := ctx.Notes.Save(filename, content, tagList); err != nil {
		slog.Error("save note", "file", filename, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx.dbCacheMu.Lock()
	delete(ctx.dbCache, filename)
	ctx.dbCacheMu.Unlock()

	http.Redirect(w, r, "/editor?file="+url.QueryEscape(filename), http.StatusSeeOther)
}

// HandleNoteSaveJSON salva uma nota e retorna JSON (para chamadas via fetch/XHR).
func (ctx *HandlerContext) HandleNoteSaveJSON(w http.ResponseWriter, r *http.Request) {
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

	filename := noteFilename(raw)

	if currentContent, err := ctx.Store.GetNote(filename); err == nil {
		if currentContent == content {
			w.Header().Set("HX-Redirect", "/editor?file="+url.QueryEscape(filename))
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	tagList := strings.Split(tags, ",")
	if err := ctx.Notes.Save(filename, content, tagList); err != nil {
		slog.Error("save note json", "file", filename, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx.dbCacheMu.Lock()
	delete(ctx.dbCache, filename)
	ctx.dbCacheMu.Unlock()

	if r.FormValue("silent") == "true" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("HX-Redirect", "/editor?file="+url.QueryEscape(filename))
	w.WriteHeader(http.StatusOK)
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

	if ext == ".pdf" {
		// PDF files: search in pdfs/ or notes/
		basename := filepath.Base(raw)
		subdirs := []string{"pdfs", "notes"}
		found := false
		for _, sd := range subdirs {
			testPath := filepath.Join(ctx.Cfg.DocsDir, sd, basename)
			if _, err := os.Stat(testPath); err == nil {
				filename = sd + "/" + basename
				os.Remove(testPath)
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
		filename = "attachments/" + basename
		fullPath := filepath.Join(ctx.Cfg.DocsDir, "attachments", basename)
		os.Remove(fullPath)
	} else {
		// Note: delete from DB
		filename = noteFilename(raw)
		ctx.Store.DeleteNote(filename)
		// Also remove any file on disk (for backwards compat during migration)
		fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
		os.Remove(fullPath)
	}

	// Remove from DB (common cleanup for all types)
	ctx.Store.DeleteDocumentsByFile(filename)
	ctx.Store.DeleteFTSByFile(filename)
	ctx.Store.DeleteTodosByFile(filename)
	ctx.Store.DeleteFileMod(filename)
	ctx.Store.ResetPopularity(filename)
	ctx.Store.SetFileTags(filename, nil)
	ctx.Store.ClearLinks(filename)

	ctx.dbCacheMu.Lock()
	delete(ctx.dbCache, filename)
	ctx.dbCacheMu.Unlock()

	w.Header().Set("HX-Trigger", "reload-sidebar")
	w.WriteHeader(http.StatusOK)
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
				oldPath := testPath
				newPath := filepath.Join(ctx.Cfg.DocsDir, newName)
				if err := os.Rename(oldPath, newPath); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
	} else if isZip {
		basename := filepath.Base(rawOld)
		newBasename := filepath.Base(rawNew)
		if !strings.HasSuffix(strings.ToLower(newBasename), ".zip") {
			newBasename += ".zip"
		}
		sd := "attachments"
		if strings.HasPrefix(rawOld, "archives/") {
			sd = "archives"
		} else if strings.HasPrefix(rawOld, "attachments/") {
			sd = "attachments"
		} else {
			if _, err := os.Stat(filepath.Join(ctx.Cfg.DocsDir, "archives", basename)); err == nil {
				sd = "archives"
			}
		}
		oldName = sd + "/" + basename
		newName = sd + "/" + newBasename
		oldPath := filepath.Join(ctx.Cfg.DocsDir, oldName)
		newPath := filepath.Join(ctx.Cfg.DocsDir, newName)
		if err := os.Rename(oldPath, newPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Note: delega para o NoteService
		if err := ctx.Notes.Rename(rawOld, rawNew); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		newName = noteFilename(rawNew)
	}

	// Update DB: delete old indexes for PDF/ZIP; notes já foram tratados pelo Notes.Rename
	if isPdf || isZip {
		ctx.Store.DeleteDocumentsByFile(oldName)
		ctx.Store.DeleteFTSByFile(oldName)
		ctx.Store.DeleteFileMod(oldName)
		ctx.Store.ResetPopularity(oldName)
		ctx.Store.SetFileTags(oldName, nil)
		ctx.Store.ClearLinks(oldName)
		newPath := filepath.Join(ctx.Cfg.DocsDir, newName)
		info, err := os.Stat(newPath)
		if err == nil {
			watcher.ProcessFile(ctx.Store, watcher.FileEvent{
				Path: newPath, Filename: newName, ModTime: info.ModTime(), Type: "create",
			})
		}
	}

	ctx.dbCacheMu.Lock()
	delete(ctx.dbCache, rawOld)
	delete(ctx.dbCache, rawNew)
	delete(ctx.dbCache, noteFilename(rawOld))
	delete(ctx.dbCache, noteFilename(rawNew))
	if oldName != "" {
		delete(ctx.dbCache, oldName)
	}
	if newName != "" {
		delete(ctx.dbCache, newName)
	}
	ctx.dbCacheMu.Unlock()

	redirectTarget := "/editor?file=" + url.QueryEscape(newName)
	w.Header().Set("HX-Redirect", redirectTarget)
	w.WriteHeader(http.StatusOK)
}

func (ctx *HandlerContext) HandleUploadAttachment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(126 << 20); err != nil { // 126MB
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "no files uploaded", http.StatusBadRequest)
		return
	}

	// Gera nome legivel pro zip
	zipName := processor.GenerateCUID2() + ".zip"

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
		
		// Determina o método de compressão: usa Store (sem compressão) para arquivos já compactados
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		var method uint16 = zip.Deflate
		if compressedExts[ext] {
			method = zip.Store
		}

		header := &zip.FileHeader{
			Name:   fh.Filename,
			Method: method,
		}
		header.Modified = time.Now()

		w, err := zw.CreateHeader(header)
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

	// Marca como recentemente processado ANTES de registrar no DB,
	// para evitar que o watcher (fsnotify ou pollAll) interfira.
	watcher.MarkRecentlyProcessed(filename)

	// Limpa registros anteriores (segurança, caso haja colisão de nome)
	ctx.Store.DeleteDocumentsByFile(filename)
	ctx.Store.DeleteFTSByFile(filename)

	doc := db.Document{
		ID:        docID,
		Tipo:      "attachment",
		Arquivo:   filename,
		Secao:     "\U0001f4e6 " + zipName,
		Texto:     "Arquivos: " + fileListStr,
		Tags:      "",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Hash:      processor.CalculateHash("att", fileListStr, nil),
	}
	ctx.Store.InsertDocument(doc)
	ctx.Store.IndexFTS(doc.ID, doc.Tipo, doc.Arquivo, doc.Secao, doc.Texto, "")
	ctx.Store.SetFileMod(filename, time.Now().Format(time.RFC3339))

	ctx.dbCacheMu.Lock()
	delete(ctx.dbCache, filename)
	ctx.dbCacheMu.Unlock()

	slog.Info("Anexo ZIP criado", "file", filename, "arquivos", len(files), "tamanho", filepath.Base(zipPath))

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

	// Marca como recentemente processado para evitar race com o watcher
	watcher.MarkRecentlyProcessed(filename)

	// Process the file (index)
	info, _ := os.Stat(fullPath)
	watcher.ProcessFile(ctx.Store, watcher.FileEvent{
		Path: fullPath, Filename: filename, ModTime: info.ModTime(), Type: "create",
	})

	ctx.dbCacheMu.Lock()
	delete(ctx.dbCache, filename)
	ctx.dbCacheMu.Unlock()

	// Redireciona para a pagina inicial (modo compacto)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ── Upload Image (from editor, returns JSON) ──

// HandleUploadImage recebe uma imagem, salva em notes/ e retorna JSON com a URL.
// Diferente do HandleUpload, não redireciona — usado pelo editor via fetch.
func (ctx *HandlerContext) HandleUploadImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.ParseMultipartForm(10 << 20) // 10MB

	file, header, err := r.FormFile("file")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp"

	if !isImage {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "error": "apenas imagens (.png, .jpg, .jpeg, .gif, .webp)",
		})
		return
	}

	// Salva em notes/ com prefixo img_
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	cleanName := strings.ReplaceAll(filepath.Base(header.Filename), " ", "_")
	filename := fmt.Sprintf("notes/img_%s_%s", timestamp, cleanName)

	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	os.MkdirAll(filepath.Dir(fullPath), 0755)

	dst, err := os.Create(fullPath)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	defer dst.Close()

	io.Copy(dst, file)

	// Marca como recentemente processado para evitar race com o watcher
	watcher.MarkRecentlyProcessed(filename)

	// Processa como imagem (cria documento stub, sem FTS)
	info, _ := os.Stat(fullPath)
	watcher.ProcessFile(ctx.Store, watcher.FileEvent{
		Path: fullPath, Filename: filename, ModTime: info.ModTime(), Type: "create",
	})

	ctx.dbCacheMu.Lock()
	delete(ctx.dbCache, filename)
	ctx.dbCacheMu.Unlock()

	imageURL := "/file?name=" + url.QueryEscape(filename)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"filename": filename,
		"url":      imageURL,
	})
}

// ── Cleanup Orphan Images ──

// HandleCleanupImages varre o diretorio notes/ em busca de arquivos img_*
// que não são referenciados por nenhum documento (texto), e os remove
// junto com seus registros no DB (documento stub, file_mod).
func (ctx *HandlerContext) HandleCleanupImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	notesDir := filepath.Join(ctx.Cfg.DocsDir, "notes")
	entries, err := os.ReadDir(notesDir)
	if err != nil {
		components.ArchiveAlert("Erro ao ler diretório de notas.", false).Render(r.Context(), w)
		return
	}

	var removed []string
	var errors []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Só processa arquivos com prefixo img_
		if !strings.HasPrefix(name, "img_") {
			continue
		}
		// Verifica se é extensão de imagem
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".webp" {
			continue
		}

		filename := "notes/" + name
		// Verifica se a imagem é referenciada em algum documento
		count, err := ctx.Store.SearchDocumentText(filename)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: erro ao buscar: %v", name, err))
			continue
		}
		if count > 0 {
			continue // ainda referenciada
		}

		// Remove o arquivo físico
		fullPath := filepath.Join(notesDir, name)
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("%s: erro ao remover: %v", name, err))
			continue
		}

		// Remove registros do DB
		ctx.Store.DeleteDocumentsByFile(filename)
		ctx.Store.DeleteFTSByFile(filename)
		ctx.Store.DeleteFileMod(filename)
		ctx.Store.ResetPopularity(filename)
		ctx.Store.SetFileTags(filename, nil)
		ctx.Store.ClearLinks(filename)

		ctx.dbCacheMu.Lock()
		delete(ctx.dbCache, filename)
		ctx.dbCacheMu.Unlock()

		removed = append(removed, name)
	}

	slog.Info("Limpeza de imagens órfãs", "removidas", len(removed), "erros", len(errors))

	if len(errors) > 0 {
		components.ArchiveAlert(fmt.Sprintf("%d imagens removidas com %d erros.", len(removed), len(errors)), false).Render(r.Context(), w)
	} else {
		components.ArchiveAlert(fmt.Sprintf("%d imagens órfãs removidas com sucesso.", len(removed)), true).Render(r.Context(), w)
	}
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
	var oldFilename, newFilename string
	var isZip, isPdf bool

	if ext == ".pdf" {
		isPdf = true
		basename := filepath.Base(raw)
		subdirs := []string{"pdfs", "notes"}
		found := false
		for _, sd := range subdirs {
			testPath := filepath.Join(ctx.Cfg.DocsDir, sd, basename)
			if _, err := os.Stat(testPath); err == nil {
				oldFilename = sd + "/" + basename
				newFilename = sd + "/copia-" + basename
				
				src, err := os.Open(testPath)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				defer src.Close()

				dstPath := filepath.Join(ctx.Cfg.DocsDir, newFilename)
				if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				dst, err := os.Create(dstPath)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				defer dst.Close()

				if _, err := io.Copy(dst, src); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				found = true
				break
			}
		}
		if !found {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
	} else if ext == ".zip" {
		isZip = true
		basename := filepath.Base(raw)
		oldFilename = "attachments/" + basename
		newFilename = "attachments/copia-" + basename
		oldPath := filepath.Join(ctx.Cfg.DocsDir, "attachments", basename)
		newPath := filepath.Join(ctx.Cfg.DocsDir, "attachments", "copia-"+basename)

		src, err := os.Open(oldPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer src.Close()

		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		dst, err := os.Create(newPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		oldFilename = noteFilename(raw)
		dir := filepath.Dir(oldFilename)
		base := filepath.Base(oldFilename)
		newFilename = filepath.ToSlash(filepath.Join(dir, "copia-"+base))

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
		fm, _, err := service.ParseFrontmatter(content)
		if err == nil && fm != nil {
			if titleVal, ok := fm["title"]; ok {
				if titleStr, ok := titleVal.(string); ok {
					newContent, err := service.UpdateFrontmatterProperty(content, "title", "copia-"+titleStr)
					if err == nil {
						content = newContent
					}
				}
			} else if titleVal, ok := fm["titulo"]; ok {
				if titleStr, ok := titleVal.(string); ok {
					newContent, err := service.UpdateFrontmatterProperty(content, "titulo", "copia-"+titleStr)
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
	}

	// Index/Process new file
	if isPdf || isZip {
		newPath := filepath.Join(ctx.Cfg.DocsDir, newFilename)
		info, err := os.Stat(newPath)
		if err == nil {
			watcher.ProcessFile(ctx.Store, watcher.FileEvent{
				Path: newPath, Filename: newFilename, ModTime: info.ModTime(), Type: "create",
			})
		}
	} else {
		content, _ := ctx.Store.GetNote(newFilename)
		if err := ctx.reindexNote(newFilename, content, time.Now()); err != nil {
			watcher.MarkRecentlyProcessed(newFilename)
			slog.Error("reindex duplicated note", "file", newFilename, "error", err)
		}
	}

	ctx.dbCacheMu.Lock()
	delete(ctx.dbCache, newFilename)
	ctx.dbCacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":           true,
		"new_filename": newFilename,
	})
}
