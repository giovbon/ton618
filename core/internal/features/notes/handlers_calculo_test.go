package notes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ton618/core/internal/core/domain"
)

// calculoFilenameFromLocation extrai o arquivo criado a partir do redirect
// para o editor (`/editor?file=notes/calculo-....md`).
func calculoFilenameFromLocation(t *testing.T, location string) string {
	t.Helper()
	const prefix = "/editor?file="
	if !strings.HasPrefix(location, prefix) {
		t.Fatalf("Location inesperada: %q", location)
	}
	filename := strings.TrimPrefix(location, prefix)
	if !strings.HasPrefix(filename, "notes/calculo-") || !strings.HasSuffix(filename, ".md") {
		t.Fatalf("nome de nota de cálculo inesperado: %q", filename)
	}
	return filename
}

// TestHandleCalculo_CriaNotaERedireciona garante que o acesso cria uma nota
// com a tag canônica e um bloco ```calc, redirecionando para o editor markdown.
func TestHandleCalculo_CriaNotaERedireciona(t *testing.T) {
	ctx := newTestContext(t)

	rec := httptest.NewRecorder()
	ctx.HandleCalculo(rec, httptest.NewRequest(http.MethodGet, "/calculo", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("status esperado %d, got %d", http.StatusFound, rec.Code)
	}
	location := rec.Header().Get("Location")
	filename := calculoFilenameFromLocation(t, location)

	content, err := ctx.Store.GetNote(filename)
	if err != nil {
		t.Fatalf("nota de cálculo não foi criada: %v", err)
	}

	fm, body, ferr := ParseFrontmatter(content)
	if ferr != nil {
		t.Fatalf("frontmatter da nota de cálculo inválido: %v", ferr)
	}
	if fm == nil || fm["tipo"] != "calculo" {
		t.Errorf("nota de cálculo deveria ter `tipo: calculo`, got %q", content)
	}
	if !strings.Contains(body, "```calc") {
		t.Errorf("corpo deveria conter um bloco ```calc, got %q", body)
	}

	tags, err := ctx.Store.GetFileTags(filename)
	if err != nil {
		t.Fatalf("GetFileTags: %v", err)
	}
	if len(tags) != 1 || tags[0] != domain.CalculoTag {
		t.Errorf("tags = %v, want [%s]", tags, domain.CalculoTag)
	}

	noteType := domain.DetectNoteType(tags, filename)
	if noteType != domain.NoteTypeCalculo {
		t.Errorf("tipo detectado = %v, want %v (tags=%v)", noteType, domain.NoteTypeCalculo, tags)
	}
	if route := noteType.EditorRoute(); route != "/editor" {
		t.Errorf("EditorRoute() = %q, want /editor", route)
	}
}

// TestHandleCalculo_CadaAcessoCriaNotaNova garante que o atalho sempre cria uma
// nova nota de cálculo (diferente do antigo + Semana, que reabria a mesma).
func TestHandleCalculo_CadaAcessoCriaNotaNova(t *testing.T) {
	ctx := newTestContext(t)

	rec1 := httptest.NewRecorder()
	ctx.HandleCalculo(rec1, httptest.NewRequest(http.MethodGet, "/calculo", nil))
	rec2 := httptest.NewRecorder()
	ctx.HandleCalculo(rec2, httptest.NewRequest(http.MethodGet, "/calculo", nil))

	f1 := calculoFilenameFromLocation(t, rec1.Header().Get("Location"))
	f2 := calculoFilenameFromLocation(t, rec2.Header().Get("Location"))
	if f1 == f2 {
		t.Fatalf("dois acessos criaram a mesma nota: %q", f1)
	}

	for _, f := range []string{f1, f2} {
		if _, err := ctx.Store.GetNote(f); err != nil {
			t.Errorf("nota %q não existe: %v", f, err)
		}
	}
}

// TestHandleCalculo_PreservaEdicao garante que salvar conteúdo na nota não
// perde a tag/tipo de cálculo nas gravações seguintes.
func TestHandleCalculo_PreservaEdicao(t *testing.T) {
	ctx := newTestContext(t)

	rec := httptest.NewRecorder()
	ctx.HandleCalculo(rec, httptest.NewRequest(http.MethodGet, "/calculo", nil))
	filename := calculoFilenameFromLocation(t, rec.Header().Get("Location"))

	edited := "---\ntipo: calculo\ntags: [calculo]\n---\n\n# Orçamento\n\n```calc\npreco = 10\nqtd = 3\npreco * qtd\n```\n"
	if err := ctx.Notes.Save(filename, edited, nil); err != nil {
		t.Fatalf("Save da edição: %v", err)
	}

	content, err := ctx.Store.GetNote(filename)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if !strings.Contains(content, "preco * qtd") {
		t.Errorf("edição perdida no save: %q", content)
	}

	tags, err := ctx.Store.GetFileTags(filename)
	if err != nil {
		t.Fatalf("GetFileTags: %v", err)
	}
	if domain.DetectNoteType(tags, filename) != domain.NoteTypeCalculo {
		t.Errorf("nota deixou de ser do tipo cálculo após edição (tags=%v)", tags)
	}
}
