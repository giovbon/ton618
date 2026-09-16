package todos

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"ton618/core/internal/core/db"
	"ton618/core/internal/processor"
)

// ── Todo Markers (HTMX) ──

// defaultMarkerColor é a cor usada quando a enviada não é um `#rrggbb` válido
// (o valor vai para um `style` inline nos templates).
const defaultMarkerColor = "#3b82f6"

func (ctx *HandlerContext) HandleGetTodoMarkers(w http.ResponseWriter, r *http.Request) {
	markers, err := ctx.Store.GetTodoMarkers()
	if err != nil {
		slog.Error("get markers", "error", err)
		http.Error(w, "erro ao carregar marcadores", http.StatusInternalServerError)
		return
	}
	if markers == nil {
		markers = []db.TodoMarker{}
	}
	MarkersList(markers).Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleAddTodoMarker(w http.ResponseWriter, r *http.Request) {
	markerName := strings.ToUpper(strings.TrimSpace(r.FormValue("marker")))

	// O nome vira regex de extração (processor.getTodoRegex) e parâmetro de URL
	// nas ações do painel: validar aqui evita marcador que quebra o extrator.
	if !processor.IsValidTodoMarkerName(markerName) {
		http.Error(w, "nome de marcador inválido (use letras, números, espaço, _ - .; máx. 32)", http.StatusBadRequest)
		return
	}

	color := strings.TrimSpace(r.FormValue("color"))
	if !processor.IsValidTodoMarkerColor(color) {
		color = defaultMarkerColor
	}

	// Insert idempotente: se já existir, nada muda (antes o handler anexava à
	// lista em memória e regravava a tabela inteira).
	if _, err := ctx.Store.AddTodoMarker(db.TodoMarker{
		Marker: markerName,
		Color:  color,
		Active: true,
		// Marcadores novos entram na contagem por padrão (pode ser desmarcado
		// na aba Marcadores das configurações).
		CountInBadge: true,
	}); err != nil {
		slog.Error("add marker", "marker", markerName, "error", err)
		http.Error(w, "erro ao salvar marcador", http.StatusInternalServerError)
		return
	}

	ctx.renderMarkersFromStore(w, r)
}

func (ctx *HandlerContext) HandleUpdateTodoMarker(w http.ResponseWriter, r *http.Request) {
	markerName := strings.TrimSpace(r.URL.Query().Get("marker"))
	if markerName == "" {
		http.Error(w, "marker not specified", http.StatusBadRequest)
		return
	}

	var upd db.TodoMarkerUpdate

	// Os flags vêm na query string (os checkboxes do painel não têm `name`), o
	// restante no corpo do POST. Valores diferentes de true/false são ignorados.
	switch r.URL.Query().Get("active") {
	case "true":
		v := true
		upd.Active = &v
	case "false":
		v := false
		upd.Active = &v
	}
	switch r.URL.Query().Get("count_in_badge") {
	case "true":
		v := true
		upd.CountInBadge = &v
	case "false":
		v := false
		upd.CountInBadge = &v
	}

	if color := strings.TrimSpace(r.FormValue("color")); processor.IsValidTodoMarkerColor(color) {
		upd.Color = &color
	}

	if sortOrderStr := strings.TrimSpace(r.FormValue("sort_order")); sortOrderStr != "" {
		v, err := strconv.Atoi(sortOrderStr)
		if err != nil || v < 0 {
			http.Error(w, "ordem inválida", http.StatusBadRequest)
			return
		}
		upd.SortOrder = &v
	}

	if upd.Active == nil && upd.CountInBadge == nil && upd.Color == nil && upd.SortOrder == nil {
		http.Error(w, "nada para atualizar", http.StatusBadRequest)
		return
	}

	// UPDATE pontual: não toca nos outros marcadores e não perde alteração
	// concorrente (o painel dispara uma requisição por checkbox).
	updated, err := ctx.Store.UpdateTodoMarker(markerName, upd)
	if err != nil {
		slog.Error("update marker", "marker", markerName, "error", err)
		http.Error(w, "erro ao atualizar marcador", http.StatusInternalServerError)
		return
	}
	if !updated {
		// Antes isso era um no-op silencioso: o painel re-renderizava mostrando o
		// valor antigo e o usuário achava que o clique não tinha funcionado.
		http.Error(w, "marcador não encontrado", http.StatusNotFound)
		return
	}

	ctx.renderMarkersFromStore(w, r)
}

func (ctx *HandlerContext) HandleRemoveTodoMarker(w http.ResponseWriter, r *http.Request) {
	markerName := strings.TrimSpace(r.URL.Query().Get("marker"))
	if markerName == "" {
		http.Error(w, "marker not specified", http.StatusBadRequest)
		return
	}

	removed, err := ctx.Store.RemoveTodoMarker(markerName)
	if err != nil {
		slog.Error("remove marker", "marker", markerName, "error", err)
		http.Error(w, "erro ao excluir marcador", http.StatusInternalServerError)
		return
	}
	if !removed {
		http.Error(w, "marcador não encontrado", http.StatusNotFound)
		return
	}

	ctx.renderMarkersFromStore(w, r)
}

func (ctx *HandlerContext) HandleResetTodoMarkers(w http.ResponseWriter, r *http.Request) {
	var defaults []db.TodoMarker
	for _, m := range processor.DefaultTodoMarkers {
		defaults = append(defaults, db.TodoMarker{
			Marker:       m.Marker,
			Color:        m.Color,
			Active:       m.Active,
			CountInBadge: true,
		})
	}
	if err := ctx.Store.SaveTodoMarkers(defaults); err != nil {
		slog.Error("reset markers", "error", err)
		http.Error(w, "erro ao restaurar marcadores", http.StatusInternalServerError)
		return
	}

	ctx.renderMarkersFromStore(w, r)
}

// ── Contagem do badge do cabeçalho ──

// todoBadgeCount calcula a contagem exibida no ícone Task do cabeçalho.
// Entram apenas marcadores ATIVOS e com "Contar" ligado — itens do tipo TASK
// (checkboxes `- [ ]` de markdown) nunca entram, pois não são marcadores.
func (ctx *HandlerContext) todoBadgeCount() int {
	markers, err := ctx.Store.GetActiveTodoMarkers()
	if err != nil {
		slog.Error("get active markers para contagem", "error", err)
		return 0
	}

	var countable []string
	for _, m := range markers {
		if m.CountInBadge {
			countable = append(countable, m.Marker)
		}
	}

	count, err := ctx.Store.CountTodosByMarkers(countable)
	if err != nil {
		slog.Error("contar tarefas do badge", "error", err)
		return 0
	}
	return count
}

// renderMarkers responde com a lista de marcadores + o badge recalculado como
// swap out-of-band. Toda ação que mexe nos marcadores passa por aqui, então a
// contagem do cabeçalho acompanha a mudança na mesma resposta (sem reload e sem
// depender de evento disparado no body).
func (ctx *HandlerContext) renderMarkers(w http.ResponseWriter, r *http.Request, markers []db.TodoMarker) {
	MarkersList(markers).Render(r.Context(), w)
	TodoBadgeOOB(ctx.todoBadgeCount()).Render(r.Context(), w)
}

// renderMarkersFromStore relê os marcadores do banco e renderiza.
// Ler de volta (em vez de reaproveitar a lista em memória do handler) garante que
// a UI mostre o estado REAL: se a escrita falhar, o painel não exibe um valor que
// nunca foi gravado.
func (ctx *HandlerContext) renderMarkersFromStore(w http.ResponseWriter, r *http.Request) {
	markers, err := ctx.Store.GetTodoMarkers()
	if err != nil {
		slog.Error("get markers após escrita", "error", err)
		http.Error(w, "erro ao carregar marcadores", http.StatusInternalServerError)
		return
	}
	if markers == nil {
		markers = []db.TodoMarker{}
	}
	ctx.renderMarkers(w, r, markers)
}

// HandleTodoCount renderiza o badge com a contagem de tarefas do cabeçalho.
func (ctx *HandlerContext) HandleTodoCount(w http.ResponseWriter, r *http.Request) {
	// Contagem dinâmica: nunca deixar o browser servir um valor velho do cache.
	w.Header().Set("Cache-Control", "no-store")
	TodoBadge(ctx.todoBadgeCount()).Render(r.Context(), w)
}
