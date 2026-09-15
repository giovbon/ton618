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

func (ctx *HandlerContext) HandleGetTodoMarkers(w http.ResponseWriter, r *http.Request) {
	markers, err := ctx.Store.GetTodoMarkers()
	if err != nil {
		slog.Error("get markers", "error", err)
	}
	if markers == nil {
		markers = []db.TodoMarker{}
	}
	MarkersList(markers).Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleAddTodoMarker(w http.ResponseWriter, r *http.Request) {
	markerName := strings.ToUpper(strings.TrimSpace(r.FormValue("marker")))
	color := r.FormValue("color")

	if markerName == "" {
		http.Error(w, "marker cannot be empty", http.StatusBadRequest)
		return
	}

	markers, _ := ctx.Store.GetTodoMarkers()
	if markers == nil {
		markers = []db.TodoMarker{}
	}

	// Verifica se já existe para não duplicar
	exists := false
	for _, m := range markers {
		if m.Marker == markerName {
			exists = true
			break
		}
	}

	if !exists {
		markers = append(markers, db.TodoMarker{
			Marker: markerName,
			Color:  color,
			Active: true,
			// Marcadores novos entram na contagem por padrão (pode ser desmarcado
			// na aba Marcadores das configurações).
			CountInBadge: true,
		})
		ctx.Store.SaveTodoMarkers(markers)
	}

	ctx.renderMarkers(w, r, markers)
}

func (ctx *HandlerContext) HandleUpdateTodoMarker(w http.ResponseWriter, r *http.Request) {
	markerName := r.URL.Query().Get("marker")
	if markerName == "" {
		http.Error(w, "marker not specified", http.StatusBadRequest)
		return
	}

	markers, _ := ctx.Store.GetTodoMarkers()

	color := r.FormValue("color")
	activeStr := r.URL.Query().Get("active")
	countStr := r.URL.Query().Get("count_in_badge")
	sortOrderStr := r.FormValue("sort_order")

	for i, m := range markers {
		if m.Marker == markerName {
			if color != "" {
				markers[i].Color = color
			}
			if activeStr == "true" {
				markers[i].Active = true
			} else if activeStr == "false" {
				markers[i].Active = false
			}
			// Se este marcador entra na contagem do badge do cabeçalho.
			if countStr == "true" {
				markers[i].CountInBadge = true
			} else if countStr == "false" {
				markers[i].CountInBadge = false
			}
			if sortOrderStr != "" {
				if v, err := strconv.Atoi(sortOrderStr); err == nil && v >= 0 {
					markers[i].SortOrder = v
				}
			}
			break
		}
	}

	ctx.Store.SaveTodoMarkers(markers)
	ctx.renderMarkers(w, r, markers)
}

func (ctx *HandlerContext) HandleRemoveTodoMarker(w http.ResponseWriter, r *http.Request) {
	markerName := r.URL.Query().Get("marker")
	if markerName == "" {
		http.Error(w, "marker not specified", http.StatusBadRequest)
		return
	}

	markers, _ := ctx.Store.GetTodoMarkers()
	var newMarkers []db.TodoMarker

	for _, m := range markers {
		if m.Marker != markerName {
			newMarkers = append(newMarkers, m)
		}
	}

	ctx.Store.SaveTodoMarkers(newMarkers)
	ctx.renderMarkers(w, r, newMarkers)
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
	ctx.Store.SaveTodoMarkers(defaults)
	ctx.renderMarkers(w, r, defaults)
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

// HandleTodoCount renderiza o badge com a contagem de tarefas do cabeçalho.
func (ctx *HandlerContext) HandleTodoCount(w http.ResponseWriter, r *http.Request) {
	// Contagem dinâmica: nunca deixar o browser servir um valor velho do cache.
	w.Header().Set("Cache-Control", "no-store")
	TodoBadge(ctx.todoBadgeCount()).Render(r.Context(), w)
}
