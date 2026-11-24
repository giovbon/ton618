package notes

import (
	"log/slog"
	"net/http"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/processor"
)

// HandleCalculo cria uma nova nota de cálculo e abre o editor markdown.
//
// A nota nasce com a tag canônica `calculo` (o que faz DetectNoteType
// devolver NoteTypeCalculo) e com um bloco ```calc vazio — o mesmo nó
// calcBlock disponível pelo comando /calculator no editor. Os resultados
// continuam efêmeros (vivem em decorações do ProseMirror) e nunca são
// gravados no markdown.
func (ctx *HandlerContext) HandleCalculo(w http.ResponseWriter, r *http.Request) {
	filename := "notes/calculo-" + processor.GenerateCUID2() + ".md"

	// Semeia a tag ANTES do save: ensureNoteTipoField consulta a tabela tags
	// para gravar o campo `tipo:` correto já na primeira gravação. O conteúdo
	// também traz a tag no frontmatter, então o índice de tags a preserva.
	if err := ctx.Store.SetFileTags(filename, []string{domain.CalculoTag}); err != nil {
		slog.Error("criar nota de cálculo: persistir tag", "file", filename, "error", err)
		http.Error(w, "erro ao criar nota de cálculo", http.StatusInternalServerError)
		return
	}

	if err := ctx.Notes.Save(filename, domain.CalculoNoteEmptyContent, nil); err != nil {
		slog.Error("criar nota de cálculo", "file", filename, "error", err)
		http.Error(w, "erro ao criar nota de cálculo", http.StatusInternalServerError)
		return
	}

	slog.Info("nota de cálculo criada", "file", filename)
	http.Redirect(w, r, "/editor?file="+SafeFileQueryEscape(filename), http.StatusFound)
}
