package todos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/db"
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

// TestHandleTodoCount_NoStore garante que o browser nunca sirva uma contagem
// velha do cache (a URL do badge também leva cache-buster `?r=`).
func TestHandleTodoCount_NoStore(t *testing.T) {
	ctx := newTestContext(t)

	rec := httptest.NewRecorder()
	ctx.HandleTodoCount(rec, httptest.NewRequest(http.MethodGet, "/api/todos/count", nil))

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
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

// ── Regressões do painel de marcadores ──

// postMarker dispara uma ação do painel e devolve status + corpo.
func postMarker(t *testing.T, ctx *HandlerContext, method, target string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	switch {
	case strings.HasPrefix(target, "/api/todo-markers/update"):
		ctx.HandleUpdateTodoMarker(rec, req)
	case strings.HasPrefix(target, "/api/todo-markers/remove"):
		ctx.HandleRemoveTodoMarker(rec, req)
	case strings.HasPrefix(target, "/api/todo-markers/reset"):
		ctx.HandleResetTodoMarkers(rec, req)
	default:
		t.Fatalf("alvo não suportado no teste: %s", target)
	}
	return rec.Code, rec.Body.String()
}

// TestHandleAddTodoMarker_NomeInvalido garante que um nome que quebraria o
// extrator (regex) ou as URLs do painel é recusado com 400 e NÃO é gravado.
func TestHandleAddTodoMarker_NomeInvalido(t *testing.T) {
	ctx := newTestContext(t)

	invalidos := []string{"A(B", "A[B", "A&B", "A:B", "TASK", "task", "", strings.Repeat("A", 33)}

	for _, nome := range invalidos {
		t.Run(nome, func(t *testing.T) {
			form := url.Values{}
			form.Set("marker", nome)
			form.Set("color", "#ff0000")
			req := httptest.NewRequest(http.MethodPost, "/api/todo-markers/add", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()

			ctx.HandleAddTodoMarker(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("nome %q: status = %d, want 400", nome, rec.Code)
			}
			markers, _ := ctx.Store.GetTodoMarkers()
			for _, m := range markers {
				if m.Marker == strings.ToUpper(nome) && nome != "" {
					t.Errorf("nome inválido %q não deveria ser gravado", nome)
				}
			}
		})
	}

	// A configuração original continua intacta depois das tentativas inválidas.
	markers, _ := ctx.Store.GetTodoMarkers()
	if len(markers) != len(processor.DefaultTodoMarkers) {
		t.Errorf("marcadores padrão sumiram: %+v", markers)
	}
}

// TestHandleAddTodoMarker_CorInvalidaUsaPadrao cobre a cor indo para um style inline.
func TestHandleAddTodoMarker_CorInvalidaUsaPadrao(t *testing.T) {
	ctx := newTestContext(t)

	form := url.Values{}
	form.Set("marker", "COR")
	form.Set("color", "red; background-image:url(x)")
	req := httptest.NewRequest(http.MethodPost, "/api/todo-markers/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleAddTodoMarker(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	markers, _ := ctx.Store.GetTodoMarkers()
	for _, m := range markers {
		if m.Marker == "COR" {
			if m.Color != defaultMarkerColor {
				t.Errorf("cor inválida deveria cair no padrão %s, got %q", defaultMarkerColor, m.Color)
			}
			return
		}
	}
	t.Fatal("marcador COR não foi criado")
}

// TestHandleAddTodoMarker_Idempotente garante que adicionar o mesmo marcador
// duas vezes não altera a cor já configurada nem duplica linhas.
func TestHandleAddTodoMarker_Idempotente(t *testing.T) {
	ctx := newTestContext(t)

	add := func(cor string) {
		form := url.Values{}
		form.Set("marker", "DUP")
		form.Set("color", cor)
		req := httptest.NewRequest(http.MethodPost, "/api/todo-markers/add", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		ctx.HandleAddTodoMarker(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("add: status = %d", rec.Code)
		}
	}

	add("#112233")
	add("#445566")

	markers, _ := ctx.Store.GetTodoMarkers()
	ocorrencias := 0
	for _, m := range markers {
		if m.Marker == "DUP" {
			ocorrencias++
			if m.Color != "#112233" {
				t.Errorf("cor original deveria ser preservada, got %q", m.Color)
			}
		}
	}
	if ocorrencias != 1 {
		t.Errorf("marcador DUP deveria aparecer 1x, got %d", ocorrencias)
	}
}

// TestHandleUpdateTodoMarker_MarcadorInexistente garante 404 (em vez do no-op
// silencioso antigo) e que a configuração dos demais marcadores não é tocada.
func TestHandleUpdateTodoMarker_MarcadorInexistente(t *testing.T) {
	ctx := newTestContext(t)

	antes, _ := ctx.Store.GetTodoMarkers()

	code, _ := postMarker(t, ctx, http.MethodPost, "/api/todo-markers/update?marker=NAOEXISTE&count_in_badge=false")
	if code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", code)
	}

	depois, _ := ctx.Store.GetTodoMarkers()
	if len(depois) != len(antes) {
		t.Fatalf("quantidade de marcadores mudou: %d → %d", len(antes), len(depois))
	}
	for i := range antes {
		if antes[i] != depois[i] {
			t.Errorf("marcador %s foi alterado por um update de marcador inexistente", antes[i].Marker)
		}
	}
}

// TestHandleUpdateTodoMarker_SemCampos garante 400 quando a requisição não traz nada.
func TestHandleUpdateTodoMarker_SemCampos(t *testing.T) {
	ctx := newTestContext(t)

	code, _ := postMarker(t, ctx, http.MethodPost, "/api/todo-markers/update?marker=TODO")
	if code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", code)
	}

	// O estado do marcador continua o mesmo.
	markers, _ := ctx.Store.GetTodoMarkers()
	for _, m := range markers {
		if m.Marker == "TODO" && (!m.Active || !m.CountInBadge) {
			t.Errorf("TODO não deveria ter sido alterado: %+v", m)
		}
	}
}

// TestHandleRemoveTodoMarker_MarcadorInexistente garante 404 e nenhuma perda de config.
func TestHandleRemoveTodoMarker_MarcadorInexistente(t *testing.T) {
	ctx := newTestContext(t)

	antes, _ := ctx.Store.GetTodoMarkers()

	code, _ := postMarker(t, ctx, http.MethodDelete, "/api/todo-markers/remove?marker=NAOEXISTE")
	if code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", code)
	}

	depois, _ := ctx.Store.GetTodoMarkers()
	if len(depois) != len(antes) {
		t.Errorf("marcadores foram apagados por engano: %d → %d", len(antes), len(depois))
	}
}

// TestHandleRemoveTodoMarker_NaoApagaOsOutros é a regressão central do
// read-modify-write: excluir 1 marcador não pode zerar a configuração.
func TestHandleRemoveTodoMarker_NaoApagaOsOutros(t *testing.T) {
	ctx := newTestContext(t)

	form := url.Values{}
	form.Set("marker", "TEMP")
	form.Set("color", "#123456")
	req := httptest.NewRequest(http.MethodPost, "/api/todo-markers/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleAddTodoMarker(rec, req)

	antes, _ := ctx.Store.GetTodoMarkers()
	if len(antes) != len(processor.DefaultTodoMarkers)+1 {
		t.Fatalf("esperava %d marcadores, got %d", len(processor.DefaultTodoMarkers)+1, len(antes))
	}

	code, body := postMarker(t, ctx, http.MethodDelete, "/api/todo-markers/remove?marker=TEMP")
	if code != http.StatusOK {
		t.Fatalf("remove: status = %d (body=%s)", code, body)
	}

	depois, _ := ctx.Store.GetTodoMarkers()
	if len(depois) != len(processor.DefaultTodoMarkers) {
		t.Fatalf("esperava os padrão restantes (%d), got %d (%+v)", len(processor.DefaultTodoMarkers), len(depois), depois)
	}
	for _, m := range depois {
		if m.Marker == "TEMP" {
			t.Error("TEMP deveria ter sido removido")
		}
	}

	// O badge continua sendo enviado (OOB) na resposta da remoção.
	if !strings.Contains(body, `hx-swap-oob="innerHTML:#todos-badge"`) {
		t.Errorf("resposta do remove deveria trazer o badge OOB, body=%q", body)
	}
}

// TestMarkersList_EscapaMarcadorNaURL garante que um marcador com caractere
// reservado de query string (% & # +) não quebra as ações do painel.
// Sem url.QueryEscape o clique no checkbox "Contar" não fazia nada: `&` cortava
// o parâmetro, `#` truncava a URL e `+` chegava como espaço no servidor.
func TestMarkersList_EscapaMarcadorNaURL(t *testing.T) {
	var sb strings.Builder
	err := MarkersList([]db.TodoMarker{
		{Marker: "A&B", Color: "#3b82f6", Active: true, CountInBadge: true},
		{Marker: "C++", Color: "#3b82f6", Active: true, CountInBadge: false},
	}).Render(context.Background(), &sb)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	body := sb.String()

	for _, esperado := range []string{"marker=A%26B", "marker=C%2B%2B"} {
		if !strings.Contains(body, esperado) {
			t.Errorf("esperava %q no HTML do painel", esperado)
		}
	}
	for _, proibido := range []string{"marker=A&B", "marker=C++"} {
		if strings.Contains(body, proibido) {
			t.Errorf("URL do painel não pode conter %q (quebra a query string)", proibido)
		}
	}
}
