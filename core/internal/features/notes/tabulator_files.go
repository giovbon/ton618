package notes

import (
	"log/slog"
	"os"
	"path/filepath"

	"ton618/core/internal/processor"
)

// datasetRefCounter conta quantas outras notas ainda referenciam um caminho de
// datasets/. É implementado por *db.Store; declarado como interface estreita
// aqui para o NoteService (que depende da interface fileOps) também poder usar.
type datasetRefCounter interface {
	CountOtherNotesReferencing(path, except string) (int, error)
}

// ReconcileTabulatorFiles apaga os arquivos de datasets/ que a nota
// `selfFilename` referenciava em `oldContent` e deixou de referenciar em
// `newContent` — célula esvaziada (×), arquivo trocado, registro ou coluna
// excluídos, ou a própria nota removida (newContent vazio).
//
// Um arquivo só é removido quando nenhuma OUTRA nota ainda o referencia: notas
// duplicadas apontam para os mesmos arquivos, e a guarda evita apagar um arquivo
// em uso pela cópia. Devolve quantos arquivos foram removidos.
//
// Best-effort: falhas apenas geram log (a nota/índice já está consistente, e um
// arquivo órfão é inofensivo). Nunca toca em caminhos fora de datasets/ — a
// extração só devolve caminhos validados por processor.IsTabulatorFilePath.
func ReconcileTabulatorFiles(store datasetRefCounter, docsDir, selfFilename, oldContent, newContent string) int {
	old := processor.ExtractTabulatorFiles(oldContent)
	if len(old) == 0 {
		return 0
	}

	kept := make(map[string]bool)
	for _, p := range processor.ExtractTabulatorFiles(newContent) {
		kept[p] = true
	}

	removed := 0
	for _, p := range old {
		if kept[p] {
			continue
		}
		if store != nil {
			n, err := store.CountOtherNotesReferencing(p, selfFilename)
			if err != nil {
				// Não deu para verificar: preserva o arquivo (errar para o lado seguro).
				slog.Warn("verificar referências de arquivo de célula Tabulator", "path", p, "error", err)
				continue
			}
			if n > 0 {
				continue
			}
		}
		if err := os.Remove(filepath.Join(docsDir, filepath.FromSlash(p))); err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("remover arquivo de célula Tabulator", "path", p, "error", err)
			}
			continue
		}
		removed++
	}

	if removed > 0 {
		slog.Info("arquivos de célula Tabulator removidos", "nota", selfFilename, "count", removed)
	}
	return removed
}
