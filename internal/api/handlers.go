package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ton618/internal/db"
	"ton618/internal/processor"
	"ton618/internal/service"
	"ton618/internal/template"
)

// ── Pages ──

func (ctx *HandlerContext) HandleIndex(w http.ResponseWriter, r *http.Request) {
	template.Index("TON-618").Render(r.Context(), w)
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

	// Redireciona planilhas para o editor de planilhas
	isSpreadsheet := false
	for _, t := range tags {
		if t == "spreadsheet" {
			isSpreadsheet = true
			break
		}
	}
	if isSpreadsheet || strings.Contains(content, "type: spreadsheet") {
		http.Redirect(w, r, "/spreadsheet?file="+url.QueryEscape(filename), http.StatusFound)
		return
	}

	allTags, err := ctx.Store.GetAllTags()
	if err != nil {
		allTags = nil
	}

	// Busca backlinks de 2 níveis
	backlinks, err := ctx.Notes.GetBacklinks(filename)
	if err != nil {
		slog.Error("get backlinks", "file", filename, "error", err)
		backlinks = &service.BacklinksResult{}
	}

	data := template.EditorData{
		Title:        "Editor - " + filename,
		Filename:     filename,
		DisplayName:  template.DisplayName(filename),
		Content:      content,
		Tags:         tags,
		AllTags:      allTags,
		Backlinks:    backlinks,
	}
	template.Editor(data).Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleSpreadsheet(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		filename = "notes/" + processor.GenerateCUID2() + ".md"
	}

	sanitized := noteFilename(filename)
	if sanitized != filename {
		canonical := "/spreadsheet?file=" + url.QueryEscape(sanitized)
		http.Redirect(w, r, canonical, http.StatusFound)
		return
	}

	var content string
	var tags []string

	if data, err := ctx.Store.GetNote(filename); err == nil && data != "" {
		content = data
		ctx.Store.IncrementPopularity(filename)
	}
	fileTags, err := ctx.Store.GetFileTags(filename)
	if err == nil {
		tags = fileTags
	}

	// Se a nota existe e NÃO for planilha, redireciona para o editor de texto
	if content != "" {
		isSpreadsheet := false
		for _, t := range tags {
			if t == "spreadsheet" {
				isSpreadsheet = true
				break
			}
		}
		if !isSpreadsheet && !strings.Contains(content, "type: spreadsheet") {
			http.Redirect(w, r, "/editor?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
	}
	allTags, err := ctx.Store.GetAllTags()
	if err != nil {
		allTags = nil
	}
	backlinks, err := ctx.Notes.GetBacklinks(filename)
	if err != nil {
		backlinks = &service.BacklinksResult{}
	}

	data := template.EditorData{
		Title:        "Planilha - " + filename,
		Filename:     filename,
		DisplayName:  template.DisplayName(filename),
		Content:      content,
		Tags:         tags,
		AllTags:      allTags,
		Backlinks:    backlinks,
	}
	template.Spreadsheet(data).Render(r.Context(), w)
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
	template.Docs("Documentação — TON-618").Render(r.Context(), w)
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
	rawType := strings.ToUpper(r.URL.Query().Get("type"))
	typeFilter := map[string]bool{}
	if rawType != "" && rawType != "ALL" {
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

	todos, err := ctx.Store.GetTodosFiltered(typeFilter, statusFilter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var allTodos []map[string]interface{}
	for _, todo := range todos {
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"todos": allTodos,
		"total": len(allTodos),
	})
}

func (ctx *HandlerContext) HandleTodosPage(w http.ResponseWriter, r *http.Request) {
	template.Todos("TODOs — TON-618").Render(r.Context(), w)
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
	template.Login().Render(r.Context(), w)
}

// ── Tabulator Database Handlers ──

func (ctx *HandlerContext) HandleDatabasePage(w http.ResponseWriter, r *http.Request) {
	template.Database("Banco de Dados — TON-618").Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleGetDatabaseData(w http.ResponseWriter, r *http.Request) {
	notes, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var data []map[string]interface{}
	columnSet := make(map[string]bool)

	// Built-in columns
	columnSet["arquivo"] = true
	columnSet["titulo"] = true
	columnSet["mtime"] = true
	columnSet["tags"] = true

	for _, n := range notes {
		content, err := ctx.Store.GetNote(n.Arquivo)
		if err != nil {
			continue
		}
		fm, body, err := service.ParseFrontmatter(content)
		if err != nil {
			fm = make(map[string]interface{})
			body = content
		}

		row := make(map[string]interface{})
		row["arquivo"] = n.Arquivo
		row["mtime"] = n.Mtime
		
		// Map parsed frontmatter
		for k, v := range fm {
			if k != "tags" && k != "title" && k != "titulo" {
				columnSet[k] = true
				row[k] = v
			}
		}

		// Title logic
		if t, ok := fm["title"]; ok && t != "" {
			row["titulo"] = t
		} else if t, ok := fm["titulo"]; ok && t != "" {
			row["titulo"] = t
		} else {
			// Check if it's a spreadsheet
			isSpreadsheet := false
			if typeVal, ok := fm["type"]; ok && typeVal == "spreadsheet" {
				isSpreadsheet = true
			} else if tagsVal, ok := fm["tags"]; ok {
				if tagsSlice, ok := tagsVal.([]interface{}); ok {
					for _, tg := range tagsSlice {
						if tgStr, ok := tg.(string); ok && tgStr == "spreadsheet" {
							isSpreadsheet = true
							break
						}
					}
				}
			}
			for _, t := range n.Tags {
				if t == "spreadsheet" {
					isSpreadsheet = true
					break
				}
			}

			if isSpreadsheet || strings.HasPrefix(strings.TrimSpace(body), "{") {
				parts := strings.Split(n.Arquivo, "/")
				row["titulo"] = strings.TrimSuffix(parts[len(parts)-1], ".md")
			} else {
				t := processor.ExtractTitle(body, n.Arquivo)
				// Remove wikilinks brackets [[title]] -> title
				t = strings.ReplaceAll(t, "[[", "")
				t = strings.ReplaceAll(t, "]]", "")
				// Remove markdown links [title](url) -> title
				re := regexp.MustCompile(`\[([^\]]+)\]\([^\)]+\)`)
				t = re.ReplaceAllString(t, "$1")
				row["titulo"] = strings.TrimSpace(t)
			}
		}

		// Tags
		if len(n.Tags) > 0 {
			row["tags"] = strings.Join(n.Tags, ", ")
		} else {
			row["tags"] = ""
		}

		// Ensure all rows have a "type" field — default to "note" if not set in frontmatter
		if _, hasType := row["type"]; !hasType {
			// Infer type from path when not declared
			arquivo := n.Arquivo
			switch {
			case strings.HasPrefix(arquivo, "pdfs/"):
				row["type"] = "pdf"
			case strings.HasPrefix(arquivo, "attachments/"):
				row["type"] = "attachment"
			default:
				row["type"] = "note"
			}
		}
		columnSet["type"] = true

		data = append(data, row)
	}

	var columns []map[string]interface{}
	// Enforce column order for base columns
	columns = append(columns, map[string]interface{}{"title": "Abrir", "field": "abrir_link", "headerSort": false, "width": 80, "hozAlign": "center"})
	columns = append(columns, map[string]interface{}{"title": "Arquivo", "field": "arquivo", "visible": false})
	columns = append(columns, map[string]interface{}{"title": "Título", "field": "titulo", "editor": "input"})
	columns = append(columns, map[string]interface{}{"title": "Tags", "field": "tags", "editor": "input"})
	columns = append(columns, map[string]interface{}{"title": "Type", "field": "type", "editor": false, "width": 110})

	for col := range columnSet {
		if col != "arquivo" && col != "titulo" && col != "tags" && col != "mtime" && col != "type" {
			columns = append(columns, map[string]interface{}{
				"title":  strings.ToUpper(col[:1]) + col[1:], // strings.Title is deprecated, simple inline title case
				"field":  col,
				"editor": "input",
			})
		}
	}
	// mtime at the end
	columns = append(columns, map[string]interface{}{"title": "Modificação", "field": "mtime", "editor": false})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"columns": columns,
		"data":    data,
	})
}

type UpdatePropertyRequest struct {
	File  string      `json:"file"`
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

func (ctx *HandlerContext) HandleUpdateNoteProperty(w http.ResponseWriter, r *http.Request) {
	var req UpdatePropertyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.File == "" || req.Key == "" {
		http.Error(w, "file and key are required", http.StatusBadRequest)
		return
	}

	content, err := ctx.Store.GetNote(req.File)
	if err != nil {
		http.Error(w, "note not found", http.StatusNotFound)
		return
	}

	// For standard properties, map appropriately
	if req.Key == "titulo" {
		req.Key = "title" // internally save as title
	}

	newContent, err := service.UpdateFrontmatterProperty(content, req.Key, req.Value)
	if err != nil {
		http.Error(w, "error updating frontmatter", http.StatusInternalServerError)
		return
	}

	// Resave to trigger reindex
	if err := ctx.Notes.Save(req.File, newContent, nil); err != nil {
		http.Error(w, "error saving note", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
