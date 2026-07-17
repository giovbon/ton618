package notes

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ton618/internal/core/db"
	"ton618/internal/httputil"
	"ton618/internal/processor"
	"ton618/internal/watcher"
)

// HandleUploadAttachment handles generic file uploads (attachments) and packages them into a ZIP.
func (ctx *HandlerContext) HandleUploadAttachment(w http.ResponseWriter, r *http.Request) {
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

	slog.Info("Anexo ZIP criado", "file", filename, "arquivos", len(files), "tamanho", filepath.Base(zipPath))

	// Redireciona pra lista compacta (mesmo comportamento do upload de PDF)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleUpload handles PDF/EPUB/image file uploads.
func (ctx *HandlerContext) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(126 << 20); err != nil { // 126MB
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	isPdf := ext == ".pdf"
	isEpub := ext == ".epub"
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg"

	if !isPdf && !isEpub && !isImage {
		http.Error(w, "apenas arquivos PDF, EPUB ou imagens (.png, .jpg, .jpeg) são permitidos", http.StatusForbidden)
		return
	}

	var filename string
	if isPdf {
		filename = "pdfs/" + filepath.Base(header.Filename)
		// Garante extensao .pdf
		if !strings.HasSuffix(filename, ".pdf") {
			filename += ".pdf"
		}
	} else if isEpub {
		filename = "epubs/" + filepath.Base(header.Filename)
		// Garante extensao .epub
		if !strings.HasSuffix(filename, ".epub") {
			filename += ".epub"
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

	if _, err := io.Copy(dst, file); err != nil {
		slog.Error("write uploaded file", "path", fullPath, "error", err)
		os.Remove(fullPath)
		http.Error(w, "write error", http.StatusInternalServerError)
		return
	}

	// Marca como recentemente processado para evitar race com o watcher
	watcher.MarkRecentlyProcessed(filename)

	// Process the file (index)
	info, _ := os.Stat(fullPath)
	watcher.ProcessFile(ctx.Store, watcher.FileEvent{
		Path: fullPath, Filename: filename, ModTime: info.ModTime(), Type: "create",
	})

	// Redireciona para a pagina inicial (modo compacto)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleUploadImage recebe uma imagem, salva em notes/ e retorna JSON com a URL.
// Diferente do HandleUpload, não redireciona — usado pelo editor via fetch.
func (ctx *HandlerContext) HandleUploadImage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB
		httputil.WriteJSON(w, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.WriteJSON(w, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp"

	if !isImage {
		httputil.WriteJSON(w, map[string]interface{}{
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
		httputil.WriteJSON(w, map[string]interface{}{
			"ok": false, "error": err.Error(),
		})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		slog.Error("write uploaded image", "path", fullPath, "error", err)
		os.Remove(fullPath)
		httputil.WriteJSON(w, map[string]interface{}{
			"ok": false, "error": "write error",
		})
		return
	}

	// Marca como recentemente processado para evitar race com o watcher
	watcher.MarkRecentlyProcessed(filename)

	// Processa como imagem (cria documento stub, sem FTS)
	info, _ := os.Stat(fullPath)
	watcher.ProcessFile(ctx.Store, watcher.FileEvent{
		Path: fullPath, Filename: filename, ModTime: info.ModTime(), Type: "create",
	})

	imageURL := "/file?name=" + SafeFileQueryEscape(filename)

	httputil.WriteJSON(w, map[string]interface{}{
		"ok":       true,
		"filename": filename,
		"url":      imageURL,
	})
}

// HandleCleanupImages varre o diretorio notes/ em busca de arquivos img_*
// que não são referenciados por nenhum documento (texto), e os remove
// junto com seus registros no DB (documento stub, file_mod).
func (ctx *HandlerContext) HandleCleanupImages(w http.ResponseWriter, r *http.Request) {
	notesDir := filepath.Join(ctx.Cfg.DocsDir, "notes")
	entries, err := os.ReadDir(notesDir)
	if err != nil {
		ArchiveAlert("Erro ao ler diretório de notas.", false).Render(r.Context(), w)
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

		removed = append(removed, name)
	}

	slog.Info("Limpeza de imagens órfãs", "removidas", len(removed), "erros", len(errors))

	if len(errors) > 0 {
		ArchiveAlert(fmt.Sprintf("%d imagens removidas com %d erros.", len(removed), len(errors)), false).Render(r.Context(), w)
	} else {
		ArchiveAlert(fmt.Sprintf("%d imagens órfãs removidas com sucesso.", len(removed)), true).Render(r.Context(), w)
	}
}
