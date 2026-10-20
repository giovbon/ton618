package notes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/domain"
)

// weeklyFilenameForHoje devolve o nome esperado da nota da semana corrente.
func weeklyFilenameForHoje() string {
	return domain.WeeklyNoteFilename(time.Now())
}

// TestHandleWeekly_CriaNotaERedireciona garante que o primeiro acesso cria a
// nota semanal (com a tag canônica) e redireciona para o editor markdown.
func TestHandleWeekly_CriaNotaERedireciona(t *testing.T) {
	ctx := newTestContext(t)
	filename := weeklyFilenameForHoje()

	rec := httptest.NewRecorder()
	ctx.HandleWeekly(rec, httptest.NewRequest(http.MethodGet, "/semana", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("status esperado %d, got %d", http.StatusFound, rec.Code)
	}
	wantLocation := "/editor?file=" + filename
	if got := rec.Header().Get("Location"); got != wantLocation {
		t.Errorf("Location = %q, want %q", got, wantLocation)
	}

	// A nota existe no banco e é reconhecida como semanal.
	content, err := ctx.Store.GetNote(filename)
	if err != nil {
		t.Fatalf("nota semanal não foi criada: %v", err)
	}

	// A nota nasce em BRANCO (só a quebra de linha exigida pelo salvamento).
	if strings.TrimSpace(content) != "" {
		t.Errorf("nota semanal deveria nascer vazia, got %q", content)
	}

	tags, err := ctx.Store.GetFileTags(filename)
	if err != nil {
		t.Fatalf("GetFileTags: %v", err)
	}

	// Sem tag persistida: o tipo "semanal" vem do NOME do arquivo.
	if len(tags) != 0 {
		t.Errorf("nota semanal não deveria ter tags, got %v", tags)
	}

	noteType := domain.DetectNoteType(tags, filename)
	if noteType != domain.NoteTypeSemanal {
		t.Errorf("tipo detectado = %v, want %v (tags=%v)", noteType, domain.NoteTypeSemanal, tags)
	}

	// O editor markdown é a rota correta para o tipo semanal (sem redirect loop).
	if route := noteType.EditorRoute(); route != "/editor" {
		t.Errorf("EditorRoute() = %q, want /editor", route)
	}
}

// TestHandleWeekly_IdempotenteNaMesmaSemana garante que abrir o botão várias
// vezes na mesma semana NÃO cria notas duplicadas nem sobrescreve o conteúdo
// já editado pelo usuário.
func TestHandleWeekly_IdempotenteNaMesmaSemana(t *testing.T) {
	ctx := newTestContext(t)
	filename := weeklyFilenameForHoje()

	// Primeiro acesso: cria a nota.
	ctx.HandleWeekly(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/semana", nil))

	// Usuário edita e salva a nota.
	edited := "# Minha semana\n\n- tarefa anotada\n"
	if err := ctx.Notes.Save(filename, edited, nil); err != nil {
		t.Fatalf("Save da edição: %v", err)
	}

	// Segundo acesso: deve reabrir a mesma nota, preservando o conteúdo.
	rec := httptest.NewRecorder()
	ctx.HandleWeekly(rec, httptest.NewRequest(http.MethodGet, "/semana", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("status esperado %d, got %d", http.StatusFound, rec.Code)
	}

	content, _ := ctx.Store.GetNote(filename)
	if content != edited {
		t.Errorf("conteúdo da nota foi sobrescrito.\ngot:  %q\nwant: %q", content, edited)
	}

	// Não deve existir nenhuma outra nota com nome de semana.
	items, err := ctx.Notes.GetMany()
	if err != nil {
		t.Fatalf("GetMany: %v", err)
	}
	weekly := 0
	for _, it := range items {
		if domain.IsWeeklyNoteFilename(it.Arquivo) {
			weekly++
		}
	}
	if weekly != 1 {
		t.Errorf("esperado exatamente 1 nota semanal, got %d", weekly)
	}
}

// TestHandleWeekly_NomeNaoEhDraft garante que o nome da nota semanal não é
// tratado como rascunho descartável por IsDraftName (o que permitiria renomear
// sobre uma nota existente sem erro, entre outros efeitos colaterais).
func TestHandleWeekly_NomeNaoEhDraft(t *testing.T) {
	filename := weeklyFilenameForHoje()
	if strings.Contains(filename, "..") {
		t.Fatalf("nome inválido gerado: %q", filename)
	}
	if !strings.HasPrefix(filename, "notes/") || !strings.HasSuffix(filename, ".md") {
		t.Errorf("nome fora do padrão notes/*.md: %q", filename)
	}
	// O nome normalizado pelo handler de arquivos deve ser igual ao gerado
	// (senão HandleEditor responderia com redirect em loop).
	if got := NoteFilename(filename); got != filename {
		t.Errorf("NoteFilename(%q) = %q, deveria ser inalterado", filename, got)
	}
}
