package services

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
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

var binaryCompressedExts = map[string]bool{
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
	".zst":  true,
	".apk":  true,
	".jar":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".heic": true,
	".heif": true,
	".tiff": true,
	".ico":  true,
	".mp3":  true,
	".m4a":  true,
	".aac":  true,
	".flac": true,
	".ogg":  true,
	".opus": true,
	".wav":  true,
	".wma":  true,
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".webm": true,
	".flv":  true,
	".wmv":  true,
	".mpeg": true,
	".mpg":  true,
	".iso":  true,
	".img":  true,
	".dmg":  true,
	".bin":  true,
	".exe":  true,
	".dll":  true,
	".so":   true,
	".dylib": true,
	".pdf":  true,
	".epub": true,
}

func selectCompressionMethod(filename string) uint16 {
	ext := strings.ToLower(filepath.Ext(filename))
	if binaryCompressedExts[ext] {
		return zip.Store
	}
	return zip.Deflate
}

// CreateStream gera o arquivo ZIP enviando o fluxo de dados diretamente para out (ex: http.ResponseWriter).
func (s *BackupService) CreateStream(out io.Writer, full bool) error {
	allNotes, _ := s.notes.GetAllNotes()

	zw := zip.NewWriter(out)
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
		seen[filename] = true
		seen[originalFilename] = true
		seen[zipFilename] = true
	}

	// 2. Arquivos do disco (PDFs, attachments, imagens, notas sem conteúdo no DB, etc.)
	if full {
		_ = filepath.WalkDir(s.docsDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(s.docsDir, path)
			if err != nil {
				return nil
			}
			relPath = filepath.ToSlash(relPath)

			if strings.HasPrefix(relPath, "archives/") || seen[relPath] {
				return nil
			}

			info, err := d.Info()
			var modTime time.Time
			if err == nil {
				modTime = info.ModTime()
			} else if mtimeStr, _ := s.fileMod.GetFileMod(relPath); mtimeStr != "" {
				modTime = repository.ParseMtime(mtimeStr)
			}

			_ = addFileToZip(zw, relPath, path, modTime)
			seen[relPath] = true
			return nil
		})
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("backup: close zip: %w", err)
	}
	return nil
}

// Create gera um ZIP em memória (para uso onde []byte é necessário).
func (s *BackupService) Create(full bool) ([]byte, error) {
	var buf bytes.Buffer
	if err := s.CreateStream(&buf, full); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func addToZip(zw *zip.Writer, name string, data []byte, modTime time.Time) {
	h := &zip.FileHeader{
		Name:   name,
		Method: selectCompressionMethod(name),
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

func addFileToZip(zw *zip.Writer, zipName string, filePath string, modTime time.Time) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	h := &zip.FileHeader{
		Name:   zipName,
		Method: selectCompressionMethod(zipName),
	}
	if !modTime.IsZero() {
		h.SetModTime(modTime)
	} else {
		h.SetModTime(info.ModTime())
	}

	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, file)
	return err
}
