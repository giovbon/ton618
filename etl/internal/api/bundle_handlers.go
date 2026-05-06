package api

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"etl/internal/ingest"
	"etl/internal/utils"
)

// HandleBundleUpload compacta múltiplos arquivos em um ZIP e cria uma nota MD com TTL.
func (ctx *HandlerContext) HandleBundleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	// Limite de 50MB para o total do upload
	err := r.ParseMultipartForm(50 << 20)
	if err != nil {
		http.Error(w, "Erro ao processar formulário", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "Nenhum arquivo enviado", http.StatusBadRequest)
		return
	}

	bundleID := fmt.Sprintf("arquivos_%s", time.Now().Format("20060102_150405"))
	zipName := bundleID + ".zip"
	mdName := bundleID + ".md"

	// O ZIP fica em 'assets' (não monitorado pelo syncer para documentos)
	// A nota fica em 'notes' (monitorada)
	assetsDir := filepath.Join(ctx.Cfg.DocsDir, "assets")
	notesDir := filepath.Join(ctx.Cfg.DocsDir, "notes")

	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		http.Error(w, "Erro ao criar diretório de assets", http.StatusInternalServerError)
		return
	}
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		http.Error(w, "Erro ao criar diretório de notas", http.StatusInternalServerError)
		return
	}

	zipPath := filepath.Join(assetsDir, zipName)
	zipFile, err := os.Create(zipPath)
	if err != nil {
		http.Error(w, "Erro ao criar arquivo ZIP", http.StatusInternalServerError)
		return
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)

	var fileList []string
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}

		fileName := utils.SlugifyFilename(fileHeader.Filename)
		fileList = append(fileList, fileName)

		f, err := archive.Create(fileName)
		if err != nil {
			file.Close()
			continue
		}

		if _, err := io.Copy(f, file); err != nil {
			file.Close()
			continue
		}
		file.Close()
	}
	archive.Close()

	// Criar nota Markdown com metadados de expiração
	mdPath := filepath.Join(notesDir, mdName)

	lang := ctx.State.GetSettings().Language
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("tags: [%s]\n", utils.GetTag(lang, "arquivos")))
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("# Arquivos: %s\n\n", bundleID))
	sb.WriteString("Arquivos contidos neste pacote:\n")
	for _, f := range fileList {
		sb.WriteString(fmt.Sprintf("- %s\n", f))
	}
	sb.WriteString("\n")
	// Link relativo para a API de arquivos existente
	sb.WriteString(fmt.Sprintf("[📥 Baixar Arquivos Completos](/api/file?name=assets/%s)\n", zipName))

	if err := os.WriteFile(mdPath, []byte(sb.String()), 0644); err != nil {
		http.Error(w, "Erro ao criar nota Markdown", http.StatusInternalServerError)
		return
	}

	// Forçar uma sincronização rápida para que a nota apareça imediatamente
	utils.SafeGo(func() {
		ingest.RunSync(ctx.Cfg, false, "auto", ctx.State)
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "success", "bundle": "%s", "note": "notes/%s"}`, bundleID, mdName)
}
