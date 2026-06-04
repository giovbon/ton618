package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"ton618/internal/db"
	"ton618/internal/processor"
	"ton618/internal/template"
)

// ── Pages ──

func (ctx *HandlerContext) HandleIndex(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":        "TON-618",
		"Query":        r.URL.Query().Get("q"),
		"ContentBlock": "indexContent",
	}
	ctx.render(w, "index.html", data)
}

func (ctx *HandlerContext) HandleEditor(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		filename = "notes/" + processor.GenerateCUID2() + ".md"
	}

	// Se for .zip, redireciona para download em vez de abrir o editor
	if strings.HasSuffix(strings.ToLower(filename), ".zip") {
		http.Redirect(w, r, "/file/download?name="+url.QueryEscape(filename), http.StatusFound)
		return
	}

	// Normaliza o filename: garante prefixo notes/ e extensao .md
	sanitized := noteFilename(filename)

	// Se a URL nao estava normalizada, redireciona para a URL canonica
	if sanitized != filename {
		canonical := "/editor?file=" + url.QueryEscape(sanitized)
		http.Redirect(w, r, canonical, http.StatusFound)
		return
	}

	var content string
	var tags []string

	// Carrega conteudo se a nota ja existe
	if data, err := ctx.Store.GetNote(filename); err == nil && data != "" {
		content = data
		// Incrementa popularidade ao abrir nota existente
		ctx.Store.IncrementPopularity(filename)
	}
	fileTags, err := ctx.Store.GetFileTags(filename)
	if err == nil {
		tags = fileTags
	}

	allTags, err := ctx.Store.GetAllTags()
	if err != nil {
		allTags = nil
	}

	data := map[string]interface{}{
		"Title":        "Editor - " + filename,
		"Filename":     filename,
		"DisplayName":  displayName(filename),
		"Content":      content,
		"Tags":         tags,
		"AllTags":      allTags,
		"LoadTipTap":   true,
		"ContentBlock": "editorContent",
	}
	ctx.render(w, "editor.html", data)
}

// ── API ──

func (ctx *HandlerContext) HandleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"documents": ctx.Store.GetDocumentCount(),
	})
}

func (ctx *HandlerContext) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"up","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
}

// ── Help / Documentation ──

func (ctx *HandlerContext) HandleHelp(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":        "Documentação — TON-618",
		"ContentBlock": "helpContent",
	}
	ctx.render(w, "docs.html", data)
}

func (ctx *HandlerContext) HandleHelpMarkdown(w http.ResponseWriter, r *http.Request) {
	content, err := template.HelpMD()
	if err != nil {
		slog.Error("ler help.md embedado", "error", err)
		http.Error(w, "Documentação não encontrada", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write(content)
}

func (ctx *HandlerContext) HandleGetTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tags, err := ctx.Store.GetAllTags()
	if err != nil {
		tags = nil
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tags": tags,
	})
}

func (ctx *HandlerContext) HandleGetKeywords(w http.ResponseWriter, r *http.Request) {
	notes, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	freq := make(map[string]int)
	for _, n := range notes {
		for _, kw := range n.Keywords {
			freq[kw]++
		}
	}

	type kwEntry struct {
		Word  string `json:"word"`
		Count int    `json:"count"`
	}
	var list []kwEntry
	for w, c := range freq {
		list = append(list, kwEntry{w, c})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count == list[j].Count {
			return list[i].Word < list[j].Word
		}
		return list[i].Count > list[j].Count
	})

	if list == nil {
		list = []kwEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"keywords": list,
		"total":    len(list),
	})
}

func (ctx *HandlerContext) HandleListTodos(w http.ResponseWriter, r *http.Request) {
	// Query parameters: type (all|todo|fixme|bug|task - comma-separated for multiple), status (all|pending|completed)
	rawType := strings.ToUpper(r.URL.Query().Get("type"))
	typeFilter := map[string]bool{}
	if rawType == "" || rawType == "ALL" {
		typeFilter["ALL"] = true
	} else {
		for _, t := range strings.Split(rawType, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				typeFilter[t] = true
			}
		}
	}
	statusFilter := strings.ToLower(r.URL.Query().Get("status"))
	if statusFilter == "" {
		statusFilter = "all"
	}

	// Get all notes
	notes, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var allTodos []map[string]interface{}

	// Get active markers from DB
	activeMarkers, _ := ctx.Store.GetActiveTodoMarkers()
	markerWords := make([]string, 0, len(activeMarkers))
	for _, m := range activeMarkers {
		markerWords = append(markerWords, m.Marker)
	}

	// Extract TODOs from each note
	for _, note := range notes {
		content, err := ctx.Store.GetNote(note.Arquivo)
		if err != nil || content == "" {
			continue
		}

		mtime, err := time.Parse(time.RFC3339, note.Mtime)
		if err != nil {
			mtime = time.Now()
		}

		todos := processor.ExtractTodos(content, note.Arquivo, mtime, markerWords)

		for _, todo := range todos {
			// Apply type filter
			if !typeFilter["ALL"] && !typeFilter[todo.Type] {
				continue
			}
			// Apply status filter
			if statusFilter != "all" && todo.Status != statusFilter {
				continue
			}

			allTodos = append(allTodos, map[string]interface{}{
				"id":      todo.ID,
				"file":    todo.File,
				"section": todo.Section,
				"type":    todo.Type,
				"status":  todo.Status,
				"text":    todo.Text,
				"line":    todo.Line,
				"created": todo.Created.Format(time.RFC3339),
			})
		}
	}

	// Sort by file, then by line (preserves document order, sections appear by appearance)
	sort.Slice(allTodos, func(i, j int) bool {
		iFile := allTodos[i]["file"].(string)
		jFile := allTodos[j]["file"].(string)
		if iFile != jFile {
			return iFile < jFile
		}
		iLine := allTodos[i]["line"].(int)
		jLine := allTodos[j]["line"].(int)
		return iLine < jLine
	})

	if allTodos == nil {
		allTodos = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"todos": allTodos,
		"total": len(allTodos),
	})
}

func (ctx *HandlerContext) HandleTodosPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":        "🎯 TODOs",
		"ContentBlock": "todosContent",
	}
	ctx.render(w, "todos.html", data)
}

// ── Todo Marker Settings ──

func (ctx *HandlerContext) HandleTodoSettingsPage(w http.ResponseWriter, r *http.Request) {
	// Redireciona para a página inicial — as configurações de marcadores
	// foram movidas para o modal de Configurações (⚙️), aba "Marcadores".
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (ctx *HandlerContext) HandleGetTodoMarkers(w http.ResponseWriter, r *http.Request) {
	markers, err := ctx.Store.GetTodoMarkers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if markers == nil {
		markers = []db.TodoMarker{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(markers)
}

func (ctx *HandlerContext) HandleSaveTodoMarkers(w http.ResponseWriter, r *http.Request) {
	var markers []db.TodoMarker
	if err := json.NewDecoder(r.Body).Decode(&markers); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := ctx.Store.SaveTodoMarkers(markers); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (ctx *HandlerContext) HandleGetAllNotes(w http.ResponseWriter, r *http.Request) {
	// Delega para o NoteService que consolida file_mods + notes
	notes, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Mtime > notes[j].Mtime
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"notes": notes,
		"total": len(notes),
	})
}

func (ctx *HandlerContext) HandleManualSync(w http.ResponseWriter, r *http.Request) {
	// Process all notes from the notes table in DB
	allNotes, err := ctx.Store.GetAllNotes()
	if err != nil {
		slog.Error("manual sync: get all notes", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	count := 0
	for filename, mtimeStr := range allNotes {
		content, err := ctx.Store.GetNote(filename)
		if err != nil || content == "" {
			continue
		}
		mtime, err := time.Parse(time.RFC3339, mtimeStr)
		if err != nil {
			mtime = time.Now()
		}
		if err := ctx.reindexNote(filename, content, mtime); err != nil {
			slog.Error("manual sync: reindex note", "file", filename, "error", err)
			continue
		}
		count++
	}

	slog.Info("Manual sync completed", "notes_processed", count)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="text-green-500">✓ Sincronização concluída (` + strconv.Itoa(count) + ` notas processadas)</div>`))
}

func (ctx *HandlerContext) HandleLogin(w http.ResponseWriter, r *http.Request) {
	ctx.renderLogin(w, "login.html", map[string]interface{}{
		"Title": "Login - TON-618",
	})
}
