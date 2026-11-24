package system

import "testing"

// A coluna nativa "_actions" (excluir registro) precisa chegar ao Tabulator
// sempre visível, fixa e por último — o database.js a transforma no botão "×"
// que exclui a nota da linha. Regressão: se ela sumir do payload, o Acervo
// perde a ação de excluir.
func TestHandleGetDatabaseData_ActionColumn(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/a.md", "# A", "")

	resp := fetchDatabase(t, ctx)
	col, ok := treeColumn(resp.Columns, "_actions")
	if !ok {
		t.Fatal("coluna nativa _actions não encontrada no payload")
	}

	if w, _ := col["width"].(float64); w != 44 {
		t.Errorf("width = %v, quer 44", col["width"])
	}
	if w, _ := col["minWidth"].(float64); w != 44 {
		t.Errorf("minWidth = %v, quer 44", col["minWidth"])
	}
	if hs, _ := col["headerSort"].(bool); hs {
		t.Error("headerSort deveria ser false")
	}
	if rs, _ := col["resizable"].(bool); rs {
		t.Error("resizable deveria ser false")
	}
	if vis, ok := col["visible"].(bool); !ok || !vis {
		t.Errorf("visible = %v, quer true", col["visible"])
	}
	if al, _ := col["hozAlign"].(string); al != "center" {
		t.Errorf("hozAlign = %q, quer center", al)
	}

	last := resp.Columns[len(resp.Columns)-1]
	if f, _ := last["field"].(string); f != "_actions" {
		t.Errorf("_actions deveria ser a última coluna, última = %q", f)
	}
}
