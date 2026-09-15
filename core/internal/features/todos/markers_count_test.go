package todos

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/processor"
)

// saveTodos insere tarefas de teste no banco do contexto.
func saveTodos(t *testing.T, ctx *HandlerContext, file string, items ...processor.TodoItem) {
	t.Helper()
	for i := range items {
		items[i].File = file
		if items[i].Section == "" {
			items[i].Section = "Geral"
		}
		if items[i].Created.IsZero() {
			items[i].Created = time.Now().UTC()
		}
		if items[i].Status == "" {
			items[i].Status = "pending"
		}
	}
	if err := ctx.Store.SaveFileTodos(file, items); err != nil {
		t.Fatalf("SaveFileTodos: %v", err)
	}
}

// TestHandleTodoCount_BadgeValores cobre o badge do ícone Task do cabeçalho:
// conta só marcadores ativos e marcados para contar, ignora checkboxes (TASK).
func TestHandleTodoCount_BadgeValores(t *testing.T) {
	ctx := newTestContext(t)

	saveTodos(t, ctx, "notes/a.md",
		processor.TodoItem{ID: "1", Type: "TODO", Text: "fazer A", Line: 1},
		processor.TodoItem{ID: "2", Type: "TODO", Text: "fazer B", Line: 2},
		processor.TodoItem{ID: "3", Type: "DOING", Text: "fazendo C", Line: 3},
		processor.TodoItem{ID: "4", Type: "DONE", Text: "feito D", Line: 4},
		// Checkbox comum: não é marcador, não entra na contagem.
		processor.TodoItem{ID: "5", Type: "TASK", Text: "checkbox solto", Line: 5},
	)

	render := func() string {
		rec := httptest.NewRecorder()
		ctx.HandleTodoCount(rec, httptest.NewRequest(http.MethodGet, "/api/todos/count", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status esperado 200, got %d", rec.Code)
		}
		return rec.Body.String()
	}

	// Com tudo contando: 2 TODO + DOING + DONE = 4 (TASK fora).
	if body := render(); !strings.Contains(body, ">4<") {
		t.Errorf("esperava contagem 4 no badge, body=%q", body)
	}

	// Marca DONE como "não contar" (caso de uso principal).
	markers, _ := ctx.Store.GetTodoMarkers()
	for i := range markers {
		if markers[i].Marker == "DONE" {
			markers[i].CountInBadge = false
		}
	}
	if err := ctx.Store.SaveTodoMarkers(markers); err != nil {
		t.Fatalf("SaveTodoMarkers: %v", err)
	}
	if body := render(); !strings.Contains(body, ">3<") {
		t.Errorf("esperava contagem 3 após excluir DONE, body=%q", body)
	}

	// Desativa o DOING → o marcador deixa de ser detectado, então sai da contagem.
	for i := range markers {
		if markers[i].Marker == "DOING" {
			markers[i].Active = false
		}
	}
	if err := ctx.Store.SaveTodoMarkers(markers); err != nil {
		t.Fatalf("SaveTodoMarkers: %v", err)
	}
	if body := render(); !strings.Contains(body, ">2<") {
		t.Errorf("esperava contagem 2 após desativar DOING, body=%q", body)
	}
}

// TestHandleTodoCount_SemTarefas garante que contagem zero renderiza vazio
// (o container do badge continua no DOM, só sem conteúdo).
func TestHandleTodoCount_SemTarefas(t *testing.T) {
	ctx := newTestContext(t)

	rec := httptest.NewRecorder()
	ctx.HandleTodoCount(rec, httptest.NewRequest(http.MethodGet, "/api/todos/count", nil))

	body := strings.TrimSpace(rec.Body.String())
	if body != "" {
		t.Errorf("esperava badge vazio sem tarefas, got %q", body)
	}
}

// TestHandleTodoCount_ComTarefasSoltas verifica que TASK sozinho não gera badge.
func TestHandleTodoCount_ComTarefasSoltas(t *testing.T) {
	ctx := newTestContext(t)
	saveTodos(t, ctx, "notes/a.md",
		processor.TodoItem{ID: "1", Type: "TASK", Text: "checkbox", Line: 1},
	)

	rec := httptest.NewRecorder()
	ctx.HandleTodoCount(rec, httptest.NewRequest(http.MethodGet, "/api/todos/count", nil))

	if body := strings.TrimSpace(rec.Body.String()); body != "" {
		t.Errorf("checkbox comum não deveria gerar badge, got %q", body)
	}
}

// TestHandleUpdateTodoMarker_CountInBadge garante que o checkbox das
// configurações de marcadores alterna a contagem e devolve o badge já recalculado
// na MESMA resposta (swap out-of-band), sem depender de evento/reload.
func TestHandleUpdateTodoMarker_CountInBadge(t *testing.T) {
	ctx := newTestContext(t)

	// Uma tarefa DONE e uma TODO para a contagem ser distinguível.
	saveTodos(t, ctx, "notes/b.md",
		processor.TodoItem{ID: "1", Type: "DONE", Text: "concluida", Line: 1},
		processor.TodoItem{ID: "2", Type: "TODO", Text: "pendente", Line: 2},
	)

	post := func(url string) string {
		req := httptest.NewRequest(http.MethodPost, url, nil)
		rec := httptest.NewRecorder()
		ctx.HandleUpdateTodoMarker(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status esperado 200, got %d", rec.Code)
		}
		return rec.Body.String()
	}

	// Antes: DONE conta → 2 tarefas no badge.
	if body := post("/api/todo-markers/update?marker=DONE&count_in_badge=false"); !strings.Contains(body, `hx-swap-oob="innerHTML:#todos-badge"`) {
		t.Errorf("resposta deveria trazer o badge como swap out-of-band, body=%q", body)
	} else if !strings.Contains(body, ">1<") {
		t.Errorf("badge OOB deveria mostrar 1 (só TODO), body=%q", body)
	}

	markers, _ := ctx.Store.GetTodoMarkers()
	for _, m := range markers {
		if m.Marker == "DONE" && m.CountInBadge {
			t.Error("DONE deveria ter CountInBadge=false após o update")
		}
		if m.Marker == "TODO" && !m.CountInBadge {
			t.Error("TODO não deveria ter sido afetado")
		}
	}

	// O badge também precisa vir no alvo do mobile.
	if body := post("/api/todo-markers/update?marker=DONE&count_in_badge=true"); !strings.Contains(body, "#mobile-todos-badge") {
		t.Errorf("resposta deveria atualizar o badge mobile, body=%q", body)
	}

	markers, _ = ctx.Store.GetTodoMarkers()
	for _, m := range markers {
		if m.Marker == "DONE" && !m.CountInBadge {
			t.Error("DONE deveria voltar a contar")
		}
	}
}

// TestHandleAddTodoMarker_ContaPorPadrao garante que marcadores novos já entram na contagem.
func TestHandleAddTodoMarker_ContaPorPadrao(t *testing.T) {
	ctx := newTestContext(t)

	form := url.Values{}
	form.Set("marker", "URGENTE")
	form.Set("color", "#ff0000")
	req := httptest.NewRequest(http.MethodPost, "/api/todo-markers/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleAddTodoMarker(rec, req)

	// A resposta traz lista + badge recalculado (OOB).
	if body := rec.Body.String(); !strings.Contains(body, "URGENTE") || !strings.Contains(body, "hx-swap-oob") {
		t.Errorf("resposta deveria ter o marcador novo e o badge OOB, body=%q", body)
	}

	markers, _ := ctx.Store.GetTodoMarkers()
	for _, m := range markers {
		if m.Marker == "URGENTE" && !m.CountInBadge {
			t.Error("marcador novo deveria contar no badge por padrão")
		}
	}
}
