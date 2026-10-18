package system

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// A aba "Colunas" deve listar todos os campos dinâmicos do frontmatter presente
// em pelo menos uma nota — incluindo apenas os campos em um bloco `---` válido.
func TestHandleGetColumnsSettings_ListaCamposDoFrontmatter(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/a.md", "---\noasis: teste\norigem: web\n---\n# A", "")

	req := httptest.NewRequest("GET", "/api/settings/columns", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetColumnsSettings(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status: %d, body: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, field := range []string{"oasis", "origem"} {
		if !strings.Contains(body, `value="`+field+`"`) {
			t.Errorf("esperava checkbox para o campo %q, body:\n%s", field, body)
		}
	}
}

// Texto no corpo que não esteja em um bloco frontmatter `---` NÃO pode virar
// campo de coluna (ParseFrontmatter exige o bloco no topo).
func TestHandleGetColumnsSettings_IgnoraTextoForaDoBloco(t *testing.T) {
	ctx := newTestContext(t)
	// "oasis" solto no corpo (sem bloco ---) não é frontmatter.
	saveTestNote(t, ctx, "notes/b.md", "# A\n\noasis: teste", "")

	req := httptest.NewRequest("GET", "/api/settings/columns", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetColumnsSettings(rr, req)

	if strings.Contains(rr.Body.String(), `value="oasis"`) {
		t.Error("oasis solto no corpo (fora do bloco ---) não deveria virar checkbox")
	}
}

// Salvar com um campo desmarcado deve: persistir a allowlist (db_columns) e
// remover o campo do payload de colunas do Tabulator no próximo fetch.
func TestHandlePostColumnsSettings_DesmarcarRemoveColuna(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/a.md", "---\noasis: teste\norigem: web\n---\n# A", "")

	// Submete marcando apenas "origem" (oasis fica desmarcada = não enviada).
	form := url.Values{}
	form.Add("columns", "origem")

	req := httptest.NewRequest("POST", "/api/settings/columns", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	ctx.HandlePostColumnsSettings(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status %d, body: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, `value="oasis" checked`) {
		t.Error("oasis deveria ser renderizada desmarcada após salvar")
	}
	if !strings.Contains(body, `value="origem" checked`) {
		t.Error("origem deveria permanecer marcada após salvar")
	}

	// A allowlist persistida não pode conter "oasis".
	saved, _ := ctx.Store.GetSetting("db_columns")
	if saved != "origem" {
		t.Errorf("db_columns deveria ser %q, got %q", "origem", saved)
	}

	// O payload do Tabulator deve omitir a coluna oasis daqui em diante.
	resp := fetchDatabase(t, ctx)
	if _, ok := treeColumn(resp.Columns, "oasis"); ok {
		t.Error("oasis não deveria mais virar coluna após salvar")
	}
	if _, ok := treeColumn(resp.Columns, "origem"); !ok {
		t.Error("origem deveria continuar coluna (está na allowlist)")
	}
}
