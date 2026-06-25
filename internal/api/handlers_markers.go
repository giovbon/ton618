package api

import (
	"log/slog"
	"net/http"
	"strings"

	"ton618/internal/db"
	"ton618/internal/template/components"
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
	components.MarkersList(markers).Render(r.Context(), w)
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

	components.MarkersList(markers).Render(r.Context(), w)
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
			break
		}
	}

	ctx.Store.SaveTodoMarkers(markers)
	components.MarkersList(markers).Render(r.Context(), w)
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
	components.MarkersList(newMarkers).Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleResetTodoMarkers(w http.ResponseWriter, r *http.Request) {
	defaults := []db.TodoMarker{
		{Marker: "TODO", Color: "#3b82f6", Active: true},
		{Marker: "FIXME", Color: "#f59e0b", Active: true},
		{Marker: "BUG", Color: "#ef4444", Active: true},
		{Marker: "HACK", Color: "#8b5cf6", Active: false},
		{Marker: "NOTE", Color: "#06b6d4", Active: false},
		{Marker: "OPTIMIZE", Color: "#10b981", Active: false},
		{Marker: "REVIEW", Color: "#f97316", Active: false},
	}
	ctx.Store.SaveTodoMarkers(defaults)
	components.MarkersList(defaults).Render(r.Context(), w)
}
