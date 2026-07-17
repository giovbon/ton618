package todos

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"ton618/internal/core/db"
	"ton618/internal/processor"
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
		})
		ctx.Store.SaveTodoMarkers(markers)
	}

	MarkersList(markers).Render(r.Context(), w)
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
			if sortOrderStr != "" {
				if v, err := strconv.Atoi(sortOrderStr); err == nil && v >= 0 {
					markers[i].SortOrder = v
				}
			}
			break
		}
	}

	ctx.Store.SaveTodoMarkers(markers)
	MarkersList(markers).Render(r.Context(), w)
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
	MarkersList(newMarkers).Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleResetTodoMarkers(w http.ResponseWriter, r *http.Request) {
	var defaults []db.TodoMarker
	for _, m := range processor.DefaultTodoMarkers {
		defaults = append(defaults, db.TodoMarker{
			Marker: m.Marker,
			Color:  m.Color,
			Active: m.Active,
		})
	}
	ctx.Store.SaveTodoMarkers(defaults)
	MarkersList(defaults).Render(r.Context(), w)
}
