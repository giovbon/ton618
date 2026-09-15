package notes

import (
	"log/slog"
	"net/http"
	"time"

	"ton618/core/internal/core/domain"
)

// HandleWeekly abre a nota semanal da semana atual — criando-a na primeira vez.
//
// O nome é DETERMINÍSTICO (ano + semana ISO-8601, ex: notes/2026-S38.md), então
// clicar no botão várias vezes dentro da mesma semana sempre reabre a MESMA nota.
// A nota é criada EM BRANCO, do tipo "semanal" (ícone próprio) — o tipo é derivado
// do nome do arquivo, não de tags. Abre no editor markdown padrão e participa
// normalmente da busca textual/semântica.
func (ctx *HandlerContext) HandleWeekly(w http.ResponseWriter, r *http.Request) {
	filename := domain.WeeklyNoteFilename(time.Now())

	if content, _ := ctx.Store.GetNote(filename); content == "" {
		// A nota nasce EM BRANCO: o tipo "semanal" vem do nome do arquivo
		// (domain.IsWeeklyNoteFilename), então nenhuma tag precisa ser persistida.
		if err := ctx.Notes.Save(filename, domain.WeeklyNoteEmptyContent, nil); err != nil {
			slog.Error("criar nota semanal", "file", filename, "error", err)
			http.Error(w, "erro ao criar nota semanal", http.StatusInternalServerError)
			return
		}
		slog.Info("nota semanal criada", "file", filename)
	}

	http.Redirect(w, r, "/editor?file="+SafeFileQueryEscape(filename), http.StatusFound)
}
