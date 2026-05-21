package processor

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

// ProcessPDF extrai o texto de um arquivo PDF e retorna um documento
// para indexacao no FTS5 e geracao de embeddings.
func ProcessPDF(path, filename string, modTime time.Time) ([]Document, []string, []string) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, nil, nil
	}
	defer f.Close()

	totalPages := r.NumPage()
	var fullText strings.Builder

	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		page := r.Page(pageNum)
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		fullText.WriteString(" ")
		fullText.WriteString(strings.TrimSpace(text))
	}

	text := strings.TrimSpace(fullText.String())
	if text == "" {
		return nil, nil, nil
	}

	baseName := strings.TrimSuffix(filepath.Base(filename), ".pdf")

	doc := Document{
		ID:         HashFunc("pdf-" + filename),
		Tipo:       "pdf",
		Arquivo:    filename,
		Secao:      "\U0001f4d5 " + baseName,
		Texto:      text,
		Timestamp:  modTime.UTC().Format(time.RFC3339),
		Created:    modTime.UTC().Format(time.RFC3339),
		Hash:       CalculateHash("pdf", baseName, nil),
		VectorHash: CalculateVectorHash("pdf", text),
		Tags:       []string{"pdf"},
	}

	return []Document{doc}, nil, []string{"pdf"}
}
