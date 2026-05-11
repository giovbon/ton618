package api

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

	"etl/internal/ingest"
	"etl/internal/search"
	"etl/internal/utils"
)

// HandleRename renomeia um arquivo .md: cria novo com mesmo conteúdo, deleta o antigo.
func (ctx *HandlerContext) HandleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	fromRel := r.URL.Query().Get("from")
	toRel := r.URL.Query().Get("to")

	if fromRel == "" || toRel == "" {
		http.Error(w, "Parâmetros 'from' e 'to' são obrigatórios", http.StatusBadRequest)
		return
	}
	for _, p := range []string{fromRel, toRel} {
		absDocs, _ := filepath.Abs(ctx.Cfg.DocsDir)
		target, _ := filepath.Abs(filepath.Join(ctx.Cfg.DocsDir, p))
		if !strings.HasPrefix(target, absDocs) {
			http.Error(w, "Caminho inválido ou tentativa de traversal", http.StatusForbidden)
			return
		}
		if ext := strings.ToLower(filepath.Ext(p)); ext != ".md" {
			http.Error(w, "Apenas arquivos .md podem ser renomeados", http.StatusForbidden)
			return
		}
	}

	fromPath := filepath.Join(ctx.Cfg.DocsDir, fromRel)
	toPath := filepath.Join(ctx.Cfg.DocsDir, toRel)

	content, err := os.ReadFile(fromPath)
	if err != nil {
		http.Error(w, "Arquivo original não encontrado", http.StatusNotFound)
		return
	}

	if fromRel != toRel {
		if _, err := os.Stat(toPath); err == nil {
			http.Error(w, "Já existe um arquivo com este nome", http.StatusConflict)
			return
		}
	}

	if err := os.MkdirAll(filepath.Dir(toPath), 0755); err != nil {
		http.Error(w, "Erro ao criar diretório destino", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(toPath, content, 0644); err != nil {
		http.Error(w, "Erro ao criar arquivo destino", http.StatusInternalServerError)
		return
	}

	// Sincronização Síncrona do Índice (Deleção do antigo)
	ctx.Coordinator.Lock() // <--- TRAVA O VIGILANTE
	defer ctx.Coordinator.Unlock()

	os.Remove(fromPath)
	search.InvalidateFile(fromRel)
	search.InvalidateFile(toRel)
	slog.Info("Note renamed", "from", fromRel, "to", toRel)

	// Bug 2 Fix: coletar e limpar estado do arquivo ANTIGO antes de deletar do Bleve
	deletedIDs := ingest.CollectBleveIDsForFile(ctx.Cfg, fromRel)
	ingest.DeleteFileFromBleve(ctx.Cfg, fromRel)
	ctx.State.DeleteVectorHash(fromRel)
	ctx.State.DeleteFileTags(fromRel)
	ctx.State.DeleteFileSemanticLinks(fromRel)
	ctx.State.DeleteHashesByIDs(deletedIDs)
	ctx.State.DeleteFileMod(fromPath)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"newName": "%s"}`, toRel)

	if ctx.Coordinator != nil {
		ctx.Coordinator.Push(toRel, ingest.JobFileUpdate, false)
	}
}

func (ctx *HandlerContext) HandleFile(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("name")
	if relPath == "" {
		http.Error(w, "Nome do arquivo necessário", http.StatusBadRequest)
		return
	}

	absDocs, _ := filepath.Abs(ctx.Cfg.DocsDir)
	fullPath := filepath.Join(ctx.Cfg.DocsDir, relPath)
	absTarget, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absTarget, absDocs) {
		http.Error(w, "Caminho de arquivo inválido ou tentativa de traversal", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Se não encontrar no caminho direto, tenta em subdiretórios padrão
		targetPath := fullPath
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			basename := filepath.Base(relPath)
			subDirs := []string{"pdfs", "attachments", "notes", "links", ""}
			for _, sd := range subDirs {
				testPath := filepath.Join(ctx.Cfg.DocsDir, sd, basename)
				if _, err := os.Stat(testPath); err == nil {
					targetPath = testPath
					break
				}
			}
		}

		ext := strings.ToLower(filepath.Ext(targetPath))
		switch ext {
		case ".pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Cache-Control", "public, max-age=86400") // 24h
		case ".png":
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "public, max-age=86400") // 24h
		case ".jpg", ".jpeg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "public, max-age=86400") // 24h
		case ".zip":
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Cache-Control", "public, max-age=86400") // 24h
		default:
			w.Header().Set("Content-Type", "text/markdown")
			w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
		}

		http.ServeFile(w, r, targetPath)

	case http.MethodPost:
		var req struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.Error("Erro ao decodificar JSON no POST", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slog.Info("Received POST for file", "file", relPath, "contentLength", len(req.Content))

		if relPath == "" {
			relPath = req.Name
			if relPath == "" {
				http.Error(w, "Nome do arquivo necessário", http.StatusBadRequest)
				return
			}
			fullPath = filepath.Join(ctx.Cfg.DocsDir, relPath)
		}

		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			http.Error(w, "Erro ao criar diretório: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		search.InvalidateFile(relPath)
		if ctx.Coordinator != nil {
			ctx.Coordinator.Push(relPath, ingest.JobFileUpdate, false)
		} else {
			// Fallback síncrono (usado em testes sem coordenador)
			docs, _, _, _, _ := ingest.ProcessMarkdown(fullPath, relPath, time.Now(), ctx.State)
			for _, doc := range docs {
				search.IndexDocument(doc.ID, doc)
				ctx.State.SetHash(doc.ID, doc.Hash)
			}
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		ext := strings.ToLower(filepath.Ext(relPath))
		isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg"
		if ext != ".md" && ext != ".pdf" && !isImage {
			http.Error(w, "Apenas arquivos .md, .pdf ou imagens podem ser excluídos", http.StatusForbidden)
			return
		}

		removeErr := os.Remove(fullPath)
		if removeErr != nil {
			basename := filepath.Base(relPath)
			subdirs := []string{"pdfs", "notes", "links", "attachments"}
			found := false
			for _, sd := range subdirs {
				testPath := filepath.Join(ctx.Cfg.DocsDir, sd, basename)
				if err := os.Remove(testPath); err == nil {
					found = true
					relPath = filepath.Join(sd, basename)
					fullPath = testPath
					break
				}
			}
			if !found {
				if !os.IsNotExist(removeErr) {
					http.Error(w, "Erro ao excluir arquivo físico", http.StatusInternalServerError)
					return
				}
			}
		}

		// Sincronizar exclusão nos motores de busca antes de responder 200 OK
		slog.Info("Iniciando exclusão síncrona", "tag", "API", "file", relPath)

		ctx.Coordinator.Lock() // <--- TRAVA O VIGILANTE
		defer ctx.Coordinator.Unlock()

		deletedIDs := ingest.CollectBleveIDsForFile(ctx.Cfg, relPath)
		slog.Info("Identified Bleve fragments", "count", len(deletedIDs), "file", relPath)

		ingest.DeleteFileFromBleve(ctx.Cfg, relPath)
		ctx.State.DeleteVectorHash(relPath)

		// Lógica de Deletar em Cascata (excluir anexos vinculados se for uma nota)
		if strings.HasSuffix(relPath, ".md") {
			links := ctx.State.GetFileLinks(relPath)
			for _, link := range links {
				// Só apagar automaticamente se estiver em pastas de mídia (attachments/, pdfs/ ou assets/)
				if strings.HasPrefix(link, "attachments/") || strings.HasPrefix(link, "pdfs/") || strings.HasPrefix(link, "assets/") {
					targetPath := filepath.Join(ctx.Cfg.DocsDir, link)
					if err := os.Remove(targetPath); err == nil {
						slog.Info("Cascade deletion", "link", link, "parent", relPath)
						// Limpar rastros do arquivo deletado em cascata
						ingest.DeleteFileFromBleve(ctx.Cfg, link)
						ctx.State.DeleteVectorHash(link)
						ctx.State.DeleteFileTags(link)
						ctx.State.DeleteFileMod(targetPath)
						ctx.State.DeleteFileMetadata(link)
					}
				}
			}
			ctx.State.DeleteFileLinks(relPath)
		}

		// Limpar state completo do arquivo deletado
		ctx.State.DeleteFileTags(relPath)
		ctx.State.DeleteFileSemanticLinks(relPath)
		ctx.State.DeleteHashesByIDs(deletedIDs)
		ctx.State.DeleteFileMod(fullPath)
		ctx.State.Save(ctx.Cfg)
		ctx.State.RebuildKnownTagsCache()

		// Invalida o cache APÓS todas as deleções estarem concluídas
		search.InvalidateFile(relPath)

		w.WriteHeader(http.StatusOK)
		slog.Info("File deleted successfully", "file", relPath)

	default:
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
	}
}

func (ctx *HandlerContext) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	r.ParseMultipartForm(20 << 20)

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Erro ao recuperar arquivo", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(handler.Filename))
	isPdf := ext == ".pdf"
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg"

	if !isPdf && !isImage {
		http.Error(w, "Apenas arquivos PDF ou Imagens (.png, .jpg) são permitidos", http.StatusForbidden)
		return
	}

	if isImage && handler.Size > (10<<20) {
		http.Error(w, "Imagens devem ter no máximo 10MB", http.StatusRequestEntityTooLarge)
		return
	}

	cleanName := utils.SlugifyFilename(handler.Filename)
	subDir := "pdfs"
	if isImage {
		subDir = "attachments"
	}

	targetDir := filepath.Join(ctx.Cfg.DocsDir, subDir)
	os.MkdirAll(targetDir, 0755)

	destPath := filepath.Join(targetDir, cleanName)
	dst, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "Erro ao salvar arquivo no servidor", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Erro ao copiar conteúdo do arquivo", http.StatusInternalServerError)
		return
	}

	if isImage {
		relFilename := subDir + "/" + cleanName
		utils.SafeGo(func() {
			ctx.runOCRInBackground(destPath, relFilename)
		})
	} else if strings.HasSuffix(strings.ToLower(cleanName), ".pdf") {
		relFilename := subDir + "/" + cleanName
		utils.SafeGo(func() {
			ctx.runPDFProcessingInBackground(destPath, relFilename)
		})
	}

	search.InvalidateFile(subDir + "/" + cleanName)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "success", "filename": "%s"}`, cleanName)
}

func (ctx *HandlerContext) runOCRInBackground(imagePath, relFilename string) {
	slog.Info("Starting OCR background processing", "file", relFilename)

	docs := ingest.ProcessImage(imagePath, relFilename, time.Now(), ctx.State)
	if len(docs) > 0 {
		// Não indexamos a imagem original, apenas a nota gerada
		ingest.UpdateStateAfterOCR(imagePath, relFilename, docs, ctx.State)
		slog.Info("Image processed, creating note", "file", relFilename)

		// Criar nota Markdown automática vinculada à imagem
		ctx.createOCRNote(relFilename, docs[0].Texto)
	}
	search.InvalidateFile(relFilename)
}

func (ctx *HandlerContext) createOCRNote(imageRelPath, text string) {
	notesDir := filepath.Join(ctx.Cfg.DocsDir, "notes")
	os.MkdirAll(notesDir, 0755)

	basename := filepath.Base(imageRelPath)
	cleanName := strings.TrimSuffix(basename, filepath.Ext(basename))
	noteName := fmt.Sprintf("ocr_%s_%s.md", time.Now().Format("20060102_150405"), cleanName)
	notePath := filepath.Join(notesDir, noteName)

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("tags: [imagem]\n")
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("# OCR: %s\n\n", cleanName))
	sb.WriteString(fmt.Sprintf("![%s](/api/file?name=%s)\n\n", cleanName, imageRelPath))
	sb.WriteString("## Texto Extraído\n\n")
	sb.WriteString(text)

	if err := os.WriteFile(notePath, []byte(sb.String()), 0644); err != nil {
		slog.Error("Erro ao criar nota MD", "error", err)
	} else if ctx.Coordinator != nil {
		ctx.Coordinator.Push(noteName, ingest.JobFileUpdate, false)
	}
}

func (ctx *HandlerContext) runPDFProcessingInBackground(pdfPath, relFilename string) {
	slog.Info("Starting PDF background processing", "file", relFilename)

	docs := ingest.ProcessPDF(pdfPath, relFilename, time.Now(), ctx.State)
	if len(docs) > 0 {
		// Não indexamos o PDF original, apenas a nota gerada
		slog.Info("PDF processed, creating note", "file", relFilename)

		// Unir o texto de todas as páginas para a nota principal
		var fullText strings.Builder
		for i, doc := range docs {
			if i > 0 {
				fullText.WriteString("\n\n---\n\n")
			}
			fullText.WriteString(doc.Texto)
		}

		ctx.createPDFNote(relFilename, fullText.String())
	}
	search.InvalidateFile(relFilename)
}

func (ctx *HandlerContext) createPDFNote(pdfRelPath, text string) {
	notesDir := filepath.Join(ctx.Cfg.DocsDir, "notes")
	os.MkdirAll(notesDir, 0755)

	basename := filepath.Base(pdfRelPath)
	cleanName := strings.TrimSuffix(basename, filepath.Ext(basename))
	noteName := fmt.Sprintf("doc_%s_%s.md", time.Now().Format("20060102_150405"), cleanName)
	notePath := filepath.Join(notesDir, noteName)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# PDF: %s\n\n", cleanName))
	sb.WriteString(fmt.Sprintf("[📖 Abrir Documento PDF](/docs/%s)\n\n", pdfRelPath))
	sb.WriteString("## Conteúdo Extraído\n\n")
	sb.WriteString(text)

	if err := os.WriteFile(notePath, []byte(sb.String()), 0644); err != nil {
		slog.Error("Erro ao criar nota MD", "error", err)
	} else if ctx.Coordinator != nil {
		ctx.Coordinator.Push(noteName, ingest.JobFileUpdate, false)
	}
}
