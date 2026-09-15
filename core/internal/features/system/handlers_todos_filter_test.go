package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/processor"
)

// TestHandleListTodos_ExcluiTasksComuns garante que checkboxes comuns de markdown
// (`- [ ]` / `- [x]`, extraídos como tipo "TASK") NÃO aparecem na listagem de tasks —
// a página trabalha só com os marcadores configurados (TODO/DOING/DONE/custom).
func TestHandleListTodos_ExcluiTasksComuns(t *testing.T) {
	ctx := newTestContext(t)

	now := time.Now().UTC()
	items := []processor.TodoItem{
		{ID: "1", File: "notes/a.md", Section: "Geral", Type: "TODO", Status: "pending", Text: "tarefa de marcador", Line: 1, Created: now},
		{ID: "2", File: "notes/a.md", Section: "Geral", Type: "TASK", Status: "pending", Text: "checkbox comum", Line: 2, Created: now},
		{ID: "3", File: "notes/a.md", Section: "Geral", Type: "DONE", Status: "pending", Text: "marcador concluido", Line: 3, Created: now},
	}
	if err := ctx.Store.SaveFileTodos("notes/a.md", items); err != nil {
		t.Fatalf("SaveFileTodos: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/todos?format=json&status=all", nil)
	rec := httptest.NewRecorder()
	ctx.HandleListTodos(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status esperado 200, got %d (body=%s)", rec.Code, rec.Body.String())
	}

	var resp struct {
		Todos []processor.TodoItem `json:"todos"`
		Count int                  `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v (body=%s)", err, rec.Body.String())
	}

	// Só os dois de marcador (TODO + DONE); o TASK fica de fora.
	if resp.Count != 2 {
		t.Errorf("count = %d, want 2 (tasks comuns devem ser ignoradas)", resp.Count)
	}
	if len(resp.Todos) != 2 {
		t.Fatalf("esperava 2 itens, got %d", len(resp.Todos))
	}
	for _, item := range resp.Todos {
		if strings.EqualFold(item.Type, "TASK") {
			t.Errorf("item do tipo TASK não deveria aparecer na listagem: %+v", item)
		}
	}

	foundMarkerTask := false
	for _, item := range resp.Todos {
		if item.Text == "tarefa de marcador" {
			foundMarkerTask = true
		}
		if item.Text == "checkbox comum" {
			t.Errorf("checkbox comum apareceu na listagem")
		}
	}
	if !foundMarkerTask {
		t.Error("tarefa de marcador sumiu da listagem")
	}
}

// TestHandleListTodos_FiltroPorMarcadorContinuaFuncionando garante que o filtro
// por marcador (aba de filtros da página) segue funcionando após a exclusão do TASK.
func TestHandleListTodos_FiltroPorMarcadorContinuaFuncionando(t *testing.T) {
	ctx := newTestContext(t)

	now := time.Now().UTC()
	items := []processor.TodoItem{
		{ID: "1", File: "notes/a.md", Section: "Geral", Type: "TODO", Status: "pending", Text: "pendente", Line: 1, Created: now},
		{ID: "2", File: "notes/a.md", Section: "Geral", Type: "DOING", Status: "pending", Text: "em andamento", Line: 2, Created: now},
	}
	if err := ctx.Store.SaveFileTodos("notes/a.md", items); err != nil {
		t.Fatalf("SaveFileTodos: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/todos?format=json&status=all&type=DOING", nil)
	rec := httptest.NewRecorder()
	ctx.HandleListTodos(rec, req)

	var resp struct {
		Todos []processor.TodoItem `json:"todos"`
		Count int                  `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}

	if resp.Count != 1 || len(resp.Todos) != 1 {
		t.Fatalf("esperava 1 item DOING, got count=%d len=%d", resp.Count, len(resp.Todos))
	}
	if resp.Todos[0].Type != "DOING" {
		t.Errorf("type = %q, want DOING", resp.Todos[0].Type)
	}
}
