package system

import (
	"net/http"
	"sort"
	"strings"

	"ton618/core/internal/core/db"
	"ton618/core/internal/features/todos"
	"ton618/core/internal/httputil"
	"ton618/core/internal/processor"
)

// ── Handlers de Listagem e Páginas de Tarefas (Todos) ──

func (ctx *HandlerContext) HandleListTodos(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	types := r.Form["type"]
	typeFilter := map[string]bool{}

	for _, t := range types {
		t = strings.ToUpper(strings.TrimSpace(t))
		if t != "" && t != "ALL" {
			typeFilter[t] = true
		}
	}
	// Fallback para o antigo formato separado por vírgula via GET
	if len(types) == 0 {
		rawType := strings.ToUpper(r.URL.Query().Get("type"))
		if rawType != "" && rawType != "ALL" {
			for _, t := range strings.Split(rawType, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					typeFilter[t] = true
				}
			}
		}
	}

	statusFilter := strings.ToLower(r.FormValue("status"))
	if statusFilter == "" {
		statusFilter = "all"
	}

	searchQuery := strings.ToLower(r.FormValue("q"))

	todoList, err := ctx.Store.GetTodosFiltered(typeFilter, statusFilter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	markers, _ := ctx.Store.GetTodoMarkers()
	if markers == nil {
		markers = []db.TodoMarker{}
	}

	var filteredTodos []processor.TodoItem
	for _, t := range todoList {
		if strings.EqualFold(t.Type, "TASK") {
			continue
		}
		if searchQuery != "" {
			if !strings.Contains(strings.ToLower(t.Text), searchQuery) &&
				!strings.Contains(strings.ToLower(t.File), searchQuery) &&
				!strings.Contains(strings.ToLower(t.Section), searchQuery) {
				continue
			}
		}
		filteredTodos = append(filteredTodos, t)
	}

	if r.URL.Query().Get("format") == "json" || r.Header.Get("Accept") == "application/json" {
		httputil.WriteJSON(w, map[string]interface{}{
			"todos": filteredTodos,
			"count": len(filteredTodos),
		})
		return
	}

	fileMap := make(map[string]*todos.FileGroup)
	for _, t := range filteredTodos {
		fg, ok := fileMap[t.File]
		if !ok {
			fg = &todos.FileGroup{Name: t.File}
			fileMap[t.File] = fg
		}
		fg.Count++

		foundIdx := -1
		for i := range fg.Sections {
			if fg.Sections[i].Name == t.Section {
				foundIdx = i
				break
			}
		}
		if foundIdx == -1 {
			fg.Sections = append(fg.Sections, todos.SectionGroup{Name: t.Section})
			foundIdx = len(fg.Sections) - 1
		}
		fg.Sections[foundIdx].Todos = append(fg.Sections[foundIdx].Todos, t)
	}

	markerOrder := make(map[string]int)
	for _, m := range markers {
		markerOrder[strings.ToUpper(m.Marker)] = m.SortOrder
	}

	const maxOrder = 999999
	fileMinOrder := make(map[string]int)
	for fname, fg := range fileMap {
		min := maxOrder
		for _, sec := range fg.Sections {
			for _, t := range sec.Todos {
				ord, ok := markerOrder[strings.ToUpper(t.Type)]
				if !ok || ord == 0 {
					ord = maxOrder
				}
				if ord < min {
					min = ord
				}
			}
		}
		fileMinOrder[fname] = min
	}

	var sortedFiles []string
	for f := range fileMap {
		sortedFiles = append(sortedFiles, f)
	}
	sort.Slice(sortedFiles, func(i, j int) bool {
		oi := fileMinOrder[sortedFiles[i]]
		oj := fileMinOrder[sortedFiles[j]]
		if oi != oj {
			return oi < oj
		}
		return sortedFiles[i] < sortedFiles[j]
	})

	var finalGroups []todos.FileGroup
	for _, f := range sortedFiles {
		fg := fileMap[f]

		sort.Slice(fg.Sections, func(i, j int) bool {
			minI := maxOrder
			for _, t := range fg.Sections[i].Todos {
				ord, ok := markerOrder[strings.ToUpper(t.Type)]
				if !ok || ord == 0 {
					ord = maxOrder
				}
				if ord < minI {
					minI = ord
				}
			}
			minJ := maxOrder
			for _, t := range fg.Sections[j].Todos {
				ord, ok := markerOrder[strings.ToUpper(t.Type)]
				if !ok || ord == 0 {
					ord = maxOrder
				}
				if ord < minJ {
					minJ = ord
				}
			}
			if minI != minJ {
				return minI < minJ
			}
			return fg.Sections[i].Name < fg.Sections[j].Name
		})

		for sIdx := range fg.Sections {
			sort.Slice(fg.Sections[sIdx].Todos, func(i, j int) bool {
				ti := fg.Sections[sIdx].Todos[i]
				tj := fg.Sections[sIdx].Todos[j]
				ordI, okI := markerOrder[strings.ToUpper(ti.Type)]
				if !okI || ordI == 0 {
					ordI = maxOrder
				}
				ordJ, okJ := markerOrder[strings.ToUpper(tj.Type)]
				if !okJ || ordJ == 0 {
					ordJ = maxOrder
				}
				if ordI != ordJ {
					return ordI < ordJ
				}
				return ti.Line < tj.Line
			})
		}

		finalGroups = append(finalGroups, *fg)
	}

	todos.TodoTree(finalGroups, markers, len(filteredTodos)).Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleTodosPage(w http.ResponseWriter, r *http.Request) {
	markers, _ := ctx.Store.GetTodoMarkers()
	todos.Todos("Task — TON-618", markers).Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleTodoSettingsPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
