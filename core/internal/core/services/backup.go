package services

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ton618/core/internal/repository"

	"gopkg.in/yaml.v3"
)

// BackupService gera backups ZIP de todos os dados (exceto archives/).
type BackupService struct {
	notes   repository.NoteStore
	fileMod repository.FileModStore
	docsDir string
}

// NewBackupService cria o serviço de backup.
func NewBackupService(notes repository.NoteStore, fm repository.FileModStore, docsDir string) *BackupService {
	return &BackupService{notes: notes, fileMod: fm, docsDir: docsDir}
}

// SpreadsheetPayload define o JSON interno de dados de uma planilha.
type SpreadsheetPayload struct {
	Data   [][]interface{} `json:"data"`
	Widths []interface{}   `json:"widths"`
}

// parseFrontmatterBody separa o frontmatter (bloco YAML entre ---) e o corpo.
func parseFrontmatterBody(content string) (string, string) {
	text := strings.TrimSpace(content)
	if !strings.HasPrefix(text, "---") {
		return "", content
	}

	parts := strings.SplitN(text, "---", 3)
	if len(parts) >= 3 {
		return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
	}
	return "", content
}

// detectNoteTypeFromFrontmatter faz o parse YAML do frontmatter e retorna o valor da chave "type".
// Diferente de parseFrontmatterBody (que só separa texto), esta função usa unmarshal YAML
// para evitar falsos positivos com strings.Contains (ex: "description: 'type: drawing'").
func detectNoteTypeFromFrontmatter(content string) string {
	fm, _ := parseFrontmatterBody(content)
	if fm == "" {
		return ""
	}
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(fm), &data); err != nil {
		return ""
	}
	if t, ok := data["type"]; ok {
		if s, ok := t.(string); ok {
			return s
		}
	}
	return ""
}

// jsonToCSV converte a estrutura JSON da planilha do jspreadsheet em um arquivo CSV.
func jsonToCSV(jsonStr string) ([]byte, error) {
	var payload SpreadsheetPayload
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		// Fallback para tentar ler diretamente como matriz
		var direct [][]interface{}
		if err2 := json.Unmarshal([]byte(jsonStr), &direct); err2 == nil {
			payload.Data = direct
		} else {
			return nil, err
		}
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	for _, row := range payload.Data {
		record := make([]string, len(row))
		for i, cell := range row {
			if cell == nil {
				record[i] = ""
			} else {
				record[i] = fmt.Sprintf("%v", cell)
			}
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), nil
}

// Create gera um ZIP com todas as notas, PDFs e anexos (se full for verdadeiro).
func (s *BackupService) Create(full bool) ([]byte, error) {
	allNotes, _ := s.notes.GetAllNotes()
	allMods, _ := s.fileMod.GetAllFileMods()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	seen := make(map[string]bool)

	// 1. Notas do DB — conteúdo markdown
	for filename, mtimeStr := range allNotes {
		if strings.HasPrefix(filename, "archives/") {
			continue
		}
		content, err := s.notes.GetNote(filename)
		if err != nil || content == "" {
			continue
		}

		originalFilename := filename
		if !strings.HasSuffix(filename, ".md") {
			filename += ".md"
			originalFilename = filename
		}

		_, body := parseFrontmatterBody(content)
		noteType := detectNoteTypeFromFrontmatter(content)

		zipFilename := filename
		var zipData []byte

		switch noteType {
		case "drawing":
			// Desenho -> .excalidraw
			zipFilename = strings.TrimSuffix(filename, ".md") + ".excalidraw"
			zipData = []byte(body)
		case "spreadsheet":
			// Planilha -> .csv
			zipFilename = strings.TrimSuffix(filename, ".md") + ".csv"
			if csvData, csvErr := jsonToCSV(body); csvErr == nil {
				zipData = csvData
			} else {
				zipData = []byte(body)
			}
		case "mermaid":
			// Diagrama Mermaid -> .mmd (salva apenas o corpo)
			zipFilename = strings.TrimSuffix(filename, ".md") + ".mmd"
			zipData = []byte(body)
		default:
			// Markdown normal
			zipData = []byte(content)
		}

		addToZip(zw, zipFilename, zipData, repository.ParseMtime(mtimeStr))
		seen[originalFilename] = true
	}

	// 2. Arquivos do disco (PDFs, attachments, notas sem conteúdo no DB)
	if full {
		for filename := range allMods {
			if strings.HasPrefix(filename, "archives/") || seen[filename] {
				continue
			}
			fullPath := filepath.Join(s.docsDir, filename)
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			mtimeStr, _ := s.fileMod.GetFileMod(filename)
			addToZip(zw, filename, data, repository.ParseMtime(mtimeStr))
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("backup: close zip: %w", err)
	}
	return buf.Bytes(), nil
}

func addToZip(zw *zip.Writer, name string, data []byte, modTime time.Time) {
	h := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	if !modTime.IsZero() {
		h.SetModTime(modTime)
	}
	w, err := zw.CreateHeader(h)
	if err != nil {
		return
	}
	w.Write(data)
}
