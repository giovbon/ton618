package search

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/features/notes"
	"ton618/core/internal/processor"
	"ton618/core/internal/watcher"
)

// ── Bulk Delete (Config → Exclusão) ──

func (ctx *HandlerContext) HandleBulkDelete(w http.ResponseWriter, r *http.Request) {
	byTag := r.FormValue("by_tag") == "true"
	tagNamesRaw := strings.TrimSpace(r.FormValue("tag_name"))
	isPreview := r.FormValue("preview") == "true"

	explicitFiles := r.Form["files"]
	filesToDelete := make(map[string]bool)
	firstFilter := true

	if len(explicitFiles) > 0 {
		for _, f := range explicitFiles {
			f = strings.TrimSpace(f)
			if f != "" && !strings.Contains(f, "..") {
				filesToDelete[f] = true
			}
		}
		firstFilter = false
	}

	if len(explicitFiles) == 0 && !byTag {
		http.Error(w, "pelo menos um filtro ou lista de arquivos deve estar ativo", http.StatusBadRequest)
		return
	}

	if byTag {
		if tagNamesRaw == "" {
			http.Error(w, "tag_name obrigatorio", http.StatusBadRequest)
			return
		}
		tagNames := strings.Split(tagNamesRaw, ",")
		tagSet := make(map[string]bool)
		for _, tn := range tagNames {
			tn = strings.TrimSpace(tn)
			if tn == "" {
				continue
			}
			tagFiles, err := ctx.Store.GetFilesByTag(tn)
			if err != nil {
				continue
			}
			for _, f := range tagFiles {
				if notes.IsNoteOrPdf(f) {
					tagSet[f] = true
				}
			}
		}

		if firstFilter {
			filesToDelete = tagSet
		} else {
			for f := range filesToDelete {
				if !tagSet[f] {
					delete(filesToDelete, f)
				}
			}
		}
		firstFilter = false
	}

	if isPreview {
		fileList := make([]string, 0, len(filesToDelete))
		for f := range filesToDelete {
			fileList = append(fileList, f)
		}
		sort.Strings(fileList)
		notes.ArchivePreview(fileList).Render(r.Context(), w)
		return
	}

	if len(filesToDelete) == 0 {
		notes.ArchiveAlert("Nenhuma nota selecionada para exclusão.", false).Render(r.Context(), w)
		return
	}

	deleted := 0
	var errors []string
	for arquivo := range filesToDelete {
		isMd := strings.HasSuffix(strings.ToLower(arquivo), ".md")

		if isMd {
			ctx.Store.DeleteNote(arquivo)
			fullPath := filepath.Join(ctx.Cfg.DocsDir, arquivo)
			os.Remove(fullPath)
		} else {
			fullPath := filepath.Join(ctx.Cfg.DocsDir, arquivo)
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				errors = append(errors, arquivo+": "+err.Error())
				continue
			}
		}

		ctx.Store.DeleteDocumentsByFile(arquivo)
		ctx.Store.DeleteFTSByFile(arquivo)
		ctx.Store.DeleteTodosByFile(arquivo)
		ctx.Store.DeleteFileMod(arquivo)
		ctx.Store.ResetPopularity(arquivo)
		ctx.Store.SetFileTags(arquivo, nil)
		ctx.Store.ClearLinks(arquivo)

		deleted++
	}

	if len(errors) > 0 {
		notes.ArchiveAlert(fmt.Sprintf("%d notas excluídas permanentemente com %d erros.", deleted, len(errors)), false).Render(r.Context(), w)
	} else {
		notes.ArchiveAlert(fmt.Sprintf("%d notas excluídas permanentemente.", deleted), true).Render(r.Context(), w)
	}
}

// ── Bulk Archive (Config → Arquivamento) ──

func (ctx *HandlerContext) HandleBulkArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	files := r.Form["files"]
	if len(files) == 0 {
		http.Error(w, "nenhum arquivo selecionado", http.StatusBadRequest)
		return
	}

	archiveName := processor.GenerateCUID2() + ".zip"
	archiveDir := filepath.Join(ctx.Cfg.DocsDir, "archives")
	os.MkdirAll(archiveDir, 0755)
	archivePath := filepath.Join(archiveDir, archiveName)

	zipFile, err := os.Create(archivePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("erro ao criar archive: %v", err), http.StatusInternalServerError)
		return
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	var archivedFiles []string
	var archiveErrors []string

	for _, arquivo := range files {
		arquivo = strings.TrimSpace(arquivo)
		if arquivo == "" || strings.Contains(arquivo, "..") {
			continue
		}

		isMd := strings.HasSuffix(strings.ToLower(arquivo), ".md")
		var content []byte

		if isMd {
			noteContent, err := ctx.Store.GetNote(arquivo)
			if err != nil || noteContent == "" {
				archiveErrors = append(archiveErrors, fmt.Sprintf("%s: nao encontrado no banco", arquivo))
				continue
			}
			content = []byte(noteContent)
			ctx.Store.DeleteNote(arquivo)
		} else {
			fullPath := filepath.Join(ctx.Cfg.DocsDir, arquivo)
			data, err := os.ReadFile(fullPath)
			if err != nil {
				if os.IsNotExist(err) {
					archiveErrors = append(archiveErrors, fmt.Sprintf("%s: nao encontrado", arquivo))
				} else {
					archiveErrors = append(archiveErrors, fmt.Sprintf("%s: %v", arquivo, err))
				}
				continue
			}
			content = data
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				archiveErrors = append(archiveErrors, fmt.Sprintf("%s: erro ao remover: %v", arquivo, err))
			}
		}

		f, err := zw.Create(arquivo)
		if err != nil {
			archiveErrors = append(archiveErrors, fmt.Sprintf("%s: erro no zip: %v", arquivo, err))
			continue
		}
		if _, err := f.Write(content); err != nil {
			archiveErrors = append(archiveErrors, fmt.Sprintf("%s: erro ao escrever: %v", arquivo, err))
			continue
		}

		ctx.Store.DeleteDocumentsByFile(arquivo)
		ctx.Store.DeleteFTSByFile(arquivo)
		ctx.Store.DeleteTodosByFile(arquivo)
		ctx.Store.DeleteFileMod(arquivo)
		ctx.Store.ResetPopularity(arquivo)
		ctx.Store.SetFileTags(arquivo, nil)
		ctx.Store.ClearLinks(arquivo)

		archivedFiles = append(archivedFiles, arquivo)
	}

	zw.Close()

	ctx.Store.SetFileMod("archives/"+archiveName, time.Now().UTC().Format(time.RFC3339))
	slog.Info("Archive criado", "archive", archiveName, "arquivos", len(archivedFiles))

	if len(archiveErrors) > 0 {
		notes.ArchiveAlert(fmt.Sprintf("%d notas arquivadas no pacote %s com %d erros.", len(archivedFiles), archiveName, len(archiveErrors)), false).Render(r.Context(), w)
	} else {
		notes.ArchiveAlert(fmt.Sprintf("%d notas arquivadas com sucesso (%s).", len(archivedFiles), archiveName), true).Render(r.Context(), w)
	}
}

// ── List Archives ──

func (ctx *HandlerContext) HandleListArchives(w http.ResponseWriter, r *http.Request) {
	archiveDir := filepath.Join(ctx.Cfg.DocsDir, "archives")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		notes.ArchivesList([]domain.ArchiveInfo{}).Render(r.Context(), w)
		return
	}

	var archives []domain.ArchiveInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}

		zipPath := filepath.Join(archiveDir, entry.Name())
		fc := countFilesInZip(zipPath)

		archives = append(archives, domain.ArchiveInfo{
			Name:      entry.Name(),
			Size:      info.Size(),
			Modified:  info.ModTime().Format(time.RFC3339),
			FileCount: fc,
		})
	}

	sort.Slice(archives, func(i, j int) bool {
		return archives[i].Modified > archives[j].Modified
	})

	for i := range archives {
		if t, err := time.Parse(time.RFC3339, archives[i].Modified); err == nil {
			archives[i].Modified = t.Format("02/01/2006 15:04")
		}
	}

	notes.ArchivesList(archives).Render(r.Context(), w)
}

func countFilesInZip(zipPath string) int {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0
	}
	defer r.Close()
	return len(r.File)
}

// ── Restore Archive ──

func (ctx *HandlerContext) HandleRestoreArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	archiveName := strings.TrimSpace(r.FormValue("archive"))
	if archiveName == "" {
		http.Error(w, "archive name required", http.StatusBadRequest)
		return
	}

	if strings.Contains(archiveName, "..") || strings.Contains(archiveName, "/") {
		http.Error(w, "invalid archive name", http.StatusBadRequest)
		return
	}

	archivePath := filepath.Join(ctx.Cfg.DocsDir, "archives", archiveName)

	rZip, err := zip.OpenReader(archivePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("erro ao abrir archive: %v", err), http.StatusInternalServerError)
		return
	}
	defer rZip.Close()

	var restoredFiles []string
	var restoreErrors []string

	for _, f := range rZip.File {
		if strings.Contains(f.Name, "..") {
			restoreErrors = append(restoreErrors, fmt.Sprintf("%s: caminho invalido", f.Name))
			continue
		}

		rc, err := f.Open()
		if err != nil {
			restoreErrors = append(restoreErrors, fmt.Sprintf("%s: erro ao abrir no zip: %v", f.Name, err))
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			restoreErrors = append(restoreErrors, fmt.Sprintf("%s: erro ao ler: %v", f.Name, err))
			continue
		}

		isMd := strings.HasSuffix(strings.ToLower(f.Name), ".md")

		if isMd {
			if err := ctx.Store.SaveNote(f.Name, string(data), time.Now().Format(time.RFC3339)); err != nil {
				restoreErrors = append(restoreErrors, fmt.Sprintf("%s: erro ao salvar no banco: %v", f.Name, err))
				continue
			}
		} else {
			targetPath := filepath.Join(ctx.Cfg.DocsDir, f.Name)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				restoreErrors = append(restoreErrors, fmt.Sprintf("%s: erro ao criar diretorio: %v", f.Name, err))
				continue
			}
			if err := os.WriteFile(targetPath, data, 0644); err != nil {
				restoreErrors = append(restoreErrors, fmt.Sprintf("%s: erro ao criar: %v", f.Name, err))
				continue
			}
		}

		restoredFiles = append(restoredFiles, f.Name)
	}

	for _, arquivo := range restoredFiles {
		isMd := strings.HasSuffix(strings.ToLower(arquivo), ".md")

		if isMd {
			content, err := ctx.Store.GetNote(arquivo)
			if err == nil && content != "" {
				_ = content
			}
		} else {
			fullPath := filepath.Join(ctx.Cfg.DocsDir, arquivo)
			info, err := os.Stat(fullPath)
			if err != nil {
				continue
			}
			ev := watcher.FileEvent{
				Path:     fullPath,
				Filename: arquivo,
				ModTime:  info.ModTime(),
				Type:     "create",
			}
			if err := watcher.ProcessFile(ctx.Store, ev); err != nil {
				slog.Error("reindex archive file", "arquivo", arquivo, "error", err)
			}
		}
	}

	ctx.Store.DeleteDocumentsByFile("archives/" + archiveName)
	ctx.Store.DeleteFTSByFile("archives/" + archiveName)
	ctx.Store.DeleteFileMod("archives/" + archiveName)
	ctx.Store.SetFileTags("archives/"+archiveName, nil)
	ctx.Store.ClearLinks("archives/" + archiveName)
	os.Remove(archivePath)

	slog.Info("Archive restaurado", "archive", archiveName, "arquivos", len(restoredFiles))

	if len(restoreErrors) > 0 {
		notes.ArchiveAlert(fmt.Sprintf("%d notas restauradas com %d erros.", len(restoredFiles), len(restoreErrors)), false).Render(r.Context(), w)
	} else {
		notes.ArchiveAlert(fmt.Sprintf("%d notas restauradas com sucesso. Atualize a página para ver na árvore.", len(restoredFiles)), true).Render(r.Context(), w)
	}
}
