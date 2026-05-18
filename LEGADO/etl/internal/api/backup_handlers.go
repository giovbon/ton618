package api

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
)

func (ctx *HandlerContext) HandleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	// 1. Configurar headers para download de arquivo
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("ton618_backup_%s.zip", timestamp)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// 2. Criar o Zip Writer apontando para o ResponseBody
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// 3. Caminhar pela pasta docs e adicionar ao ZIP
	docsDir := ctx.Cfg.DocsDir
	err := filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Criar o header do arquivo dentro do zip usando o caminho relativo
		relPath, err := filepath.Rel(docsDir, path)
		if err != nil {
			return err
		}

		// Ignorar arquivos ocultos ou de sistema se necessário
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		f, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		// Abrir o arquivo real e copiar para o zip
		fileToZip, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fileToZip.Close()

		_, err = io.Copy(f, fileToZip)
		return err
	})

	if err != nil {
		slog.Error("Erro ao gerar backup", "error", err)
		// Aqui o header já pode ter sido enviado, então o erro vai corromper o zip,
		// o que é o comportamento esperado se houver falha no meio do stream.
	}
}

func (ctx *HandlerContext) HandleGetBackupSize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	var totalSize int64
	err := filepath.Walk(ctx.Cfg.DocsDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		http.Error(w, "Erro ao calcular tamanho", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"totalSize": %d}`, totalSize)
}
