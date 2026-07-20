package processor

import (
	"path/filepath"
	"strings"
	"time"
)

// ProcessPDF cria um registro mínimo para o PDF no índice FTS.
// Diferente do comportamento anterior, NÃO extrai o texto completo do PDF
// para evitar poluir a busca global com conteúdo de livros inteiros.
// O PDF continua aparecendo na busca compacta (por nome de arquivo) e
// pode ser encontrado na busca global pelo título.
func ProcessPDF(path, filename string, modTime time.Time) ([]Document, []string, []string) {
	baseName := strings.TrimSuffix(filepath.Base(filename), ".pdf")

	doc := Document{
		ID:         HashFunc("pdf-" + filename),
		Tipo:       "pdf",
		Arquivo:    filename,
		Secao:      "\U0001f4d5 " + baseName,
		Texto:      "", // Sem extração de texto — apenas pesquisável por título/nome
		Timestamp:  modTime.UTC().Format(time.RFC3339),
		Created:    modTime.UTC().Format(time.RFC3339),
		Hash:       CalculateHash("pdf", baseName, nil),
		Tags:       nil,
	}

	return []Document{doc}, nil, nil
}
