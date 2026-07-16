package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// safeJoin resolve o caminho e verifica se ele está contido no diretório base,
// prevenindo path traversal (ex: notes/../../../etc/passwd).
func safeJoin(baseDir, target string) (string, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolver diretório base: %w", err)
	}
	absTarget, err := filepath.Abs(filepath.Join(baseDir, target))
	if err != nil {
		return "", fmt.Errorf("resolver caminho: %w", err)
	}
	// Garante que o caminho resolvido começa com o diretório base
	if !strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) && absTarget != absBase {
		return "", fmt.Errorf("path traversal detectado: %s", target)
	}
	return absTarget, nil
}

// fileType enumera os tipos de arquivo manipulados pelos handlers.
type fileType int

const (
	fileTypeNote fileType = iota // Nota markdown em notes/
	fileTypePDF                  // PDF em pdfs/ ou notes/
	fileTypeEPUB                 // EPUB em epubs/
	fileTypeZip                  // ZIP/attachment em attachments/ (ou archives/)
)

// resolveFileInfo determina o tipo, nome lógico e caminho completo de um arquivo.
// raw é o nome vindo do formulário (ex: "doc.pdf", "notes/nota.md", "anexo.zip").
func resolveFileInfo(docsDir, raw string) (ft fileType, filename, fullPath string, found bool) {
	ext := strings.ToLower(filepath.Ext(raw))

	switch ext {
	case ".pdf":
		basename := filepath.Base(raw)
		subdirs := []string{"pdfs", "notes"}
		for _, sd := range subdirs {
			testPath := filepath.Join(docsDir, sd, basename)
			if _, err := os.Stat(testPath); err == nil {
				return fileTypePDF, sd + "/" + basename, testPath, true
			}
		}
		return fileTypePDF, "", "", false

	case ".epub":
		basename := filepath.Base(raw)
		filename = "epubs/" + basename
		fullPath = filepath.Join(docsDir, "epubs", basename)
		return fileTypeEPUB, filename, fullPath, true

	case ".zip":
		basename := filepath.Base(raw)
		// Tenta attachments/ primeiro; se não existir, tenta archives/
		sd := "attachments"
		if strings.HasPrefix(raw, "archives/") {
			sd = "archives"
		} else if !strings.HasPrefix(raw, "attachments/") {
			// raw veio sem prefixo: verifica se existe em archives/
			if _, err := os.Stat(filepath.Join(docsDir, "archives", basename)); err == nil {
				sd = "archives"
			}
		}
		filename = sd + "/" + basename
		fullPath = filepath.Join(docsDir, sd, basename)
		return fileTypeZip, filename, fullPath, true

	default:
		// Nota markdown — sanitiza o nome para prevenir path traversal
		base := filepath.Base(raw)
		name := strings.TrimSuffix(base, ".md")
		filename = NoteFilename(name)
		fullPath, err := safeJoin(docsDir, filename)
		if err != nil {
			return fileTypeNote, "", "", false
		}
		// Não verifica existência — a nota pode estar só no DB
		return fileTypeNote, filename, fullPath, true
	}
}

// resolveFileInfoStrict como resolveFileInfo, mas retorna found=false se o arquivo
// não existir em disco (usado para operações que exigem o arquivo físico).
func resolveFileInfoStrict(docsDir, raw string) (ft fileType, filename, fullPath string, found bool) {
	ft, filename, fullPath, found = resolveFileInfo(docsDir, raw)
	if !found {
		return
	}
	// Para notas markdown, verifica existência em disco
	if ft == fileTypeNote {
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return ft, filename, fullPath, false
		}
	}
	return
}
