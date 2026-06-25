package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"ton618/internal/db"
	"ton618/internal/processor"
	"ton618/internal/service"
	"ton618/internal/template"
	"ton618/internal/template/components"
	"ton618/internal/watcher"
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

	// Redireciona planilhas, desenhos, typst ou mermaid para seus respectivos editores
	isSpreadsheet := false
	isDrawing := false
	isTypst := false
	isMermaid := false
	for _, t := range fileTags {
		if t == "spreadsheet" {
			isSpreadsheet = true
		}
		if t == "drawing" {
			isDrawing = true
		}
		if t == "typst" {
			isTypst = true
		}
		if t == "mermaid" {
			isMermaid = true
		}
	}
	if isSpreadsheet || strings.Contains(content, "type: spreadsheet") {
		http.Redirect(w, r, "/spreadsheet?file="+url.QueryEscape(filename), http.StatusFound)
		return
	}
	if isDrawing || strings.Contains(content, "type: drawing") {
		http.Redirect(w, r, "/drawing?file="+url.QueryEscape(filename), http.StatusFound)
		return
	}
	if isTypst || strings.Contains(content, "type: typst") {
		http.Redirect(w, r, "/typst?file="+url.QueryEscape(filename), http.StatusFound)
		return
	}
	if isMermaid || strings.Contains(content, "type: mermaid") {
		http.Redirect(w, r, "/mermaid?file="+url.QueryEscape(filename), http.StatusFound)
		return
	}

	// Filtra as tags internas para não mostrar na UI do editor
	var userTags []string
	for _, t := range fileTags {
		lt := strings.ToLower(t)
		if lt != "spreadsheet" && lt != "drawing" && lt != "typst" && lt != "mermaid" {
			userTags = append(userTags, t)
		}
	}
	tags = userTags

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

	// Se a nota existe e NÃO for planilha, redireciona para o editor correto
	if content != "" {
		isSpreadsheet := false
		isDrawing := false
		isTypst := false
		isMermaid := false
		for _, t := range fileTags {
			if t == "spreadsheet" {
				isSpreadsheet = true
			}
			if t == "drawing" {
				isDrawing = true
			}
			if t == "typst" {
				isTypst = true
			}
			if t == "mermaid" {
				isMermaid = true
			}
		}
		if isDrawing || strings.Contains(content, "type: drawing") {
			http.Redirect(w, r, "/drawing?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if isTypst || strings.Contains(content, "type: typst") {
			http.Redirect(w, r, "/typst?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if isMermaid || strings.Contains(content, "type: mermaid") {
			http.Redirect(w, r, "/mermaid?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if !isSpreadsheet && !strings.Contains(content, "type: spreadsheet") {
			http.Redirect(w, r, "/editor?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
	}

	// Filtra as tags internas para não mostrar na UI
	var userTags []string
	for _, t := range fileTags {
		lt := strings.ToLower(t)
		if lt != "spreadsheet" && lt != "drawing" && lt != "typst" && lt != "mermaid" {
			userTags = append(userTags, t)
		}
	}
	tags = userTags
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

func (ctx *HandlerContext) HandleDrawing(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		filename = "notes/" + processor.GenerateCUID2() + ".md"
	}

	sanitized := noteFilename(filename)
	if sanitized != filename {
		canonical := "/drawing?file=" + url.QueryEscape(sanitized)
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

	// Se a nota existe e NÃO for desenho, redireciona para o editor correto
	if content != "" {
		isDrawing := false
		isSpreadsheet := false
		isTypst := false
		isMermaid := false
		for _, t := range fileTags {
			if t == "drawing" {
				isDrawing = true
			}
			if t == "spreadsheet" {
				isSpreadsheet = true
			}
			if t == "typst" {
				isTypst = true
			}
			if t == "mermaid" {
				isMermaid = true
			}
		}
		if isSpreadsheet || strings.Contains(content, "type: spreadsheet") {
			http.Redirect(w, r, "/spreadsheet?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if isTypst || strings.Contains(content, "type: typst") {
			http.Redirect(w, r, "/typst?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if isMermaid || strings.Contains(content, "type: mermaid") {
			http.Redirect(w, r, "/mermaid?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
		if !isDrawing && !strings.Contains(content, "type: drawing") {
			http.Redirect(w, r, "/editor?file="+url.QueryEscape(filename), http.StatusFound)
			return
		}
	}

	// Filtra as tags internas para não mostrar na UI
	var userTags []string
	for _, t := range fileTags {
		lt := strings.ToLower(t)
		if lt != "spreadsheet" && lt != "drawing" && lt != "typst" && lt != "mermaid" {
			userTags = append(userTags, t)
		}
	}
	tags = userTags
	allTags, err := ctx.Store.GetAllTags()
	if err != nil {
		allTags = nil
	}
	backlinks, err := ctx.Notes.GetBacklinks(filename)
	if err != nil {
		backlinks = &service.BacklinksResult{}
	}

	data := template.EditorData{
		Title:        "Desenho - " + filename,
		Filename:     filename,
		DisplayName:  template.DisplayName(filename),
		Content:      content,
		Tags:         tags,
		AllTags:      allTags,
		Backlinks:    backlinks,
	}
	template.Drawing(data).Render(r.Context(), w)
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
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	tags, err := ctx.Store.GetAllTags()
	if err != nil {
		tags = nil
	}
	var filtered []string
	for _, t := range tags {
		lt := strings.ToLower(t)
		if lt != "typst" && lt != "drawing" && lt != "spreadsheet" && lt != "mermaid" {
			filtered = append(filtered, t)
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tags": filtered,
	})
}

func (ctx *HandlerContext) HandleGetKeywords(w http.ResponseWriter, r *http.Request) {
	allKeywords, err := ctx.Store.GetAllNotesKeywords()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	freq := make(map[string]int)
	for _, keywords := range allKeywords {
		for _, kw := range keywords {
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

	todos, err := ctx.Store.GetTodosFiltered(typeFilter, statusFilter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	markers, _ := ctx.Store.GetTodoMarkers()
	if markers == nil {
		markers = []db.TodoMarker{}
	}

	var filteredTodos []processor.TodoItem
	for _, t := range todos {
		if searchQuery != "" {
			if !strings.Contains(strings.ToLower(t.Text), searchQuery) &&
			   !strings.Contains(strings.ToLower(t.File), searchQuery) &&
			   !strings.Contains(strings.ToLower(t.Section), searchQuery) {
				continue
			}
		}
		filteredTodos = append(filteredTodos, t)
	}

	// Agrupar preserving insertion order for sections
	fileMap := make(map[string]*components.FileGroup)
	for _, t := range filteredTodos {
		fg, ok := fileMap[t.File]
		if !ok {
			fg = &components.FileGroup{Name: t.File}
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
			fg.Sections = append(fg.Sections, components.SectionGroup{Name: t.Section})
			foundIdx = len(fg.Sections) - 1
		}
		fg.Sections[foundIdx].Todos = append(fg.Sections[foundIdx].Todos, t)
	}

	var sortedFiles []string
	for f := range fileMap {
		sortedFiles = append(sortedFiles, f)
	}
	sort.Strings(sortedFiles)
	
	var finalGroups []components.FileGroup
	for _, f := range sortedFiles {
		finalGroups = append(finalGroups, *fileMap[f])
	}

	components.TodoTree(finalGroups, markers, len(filteredTodos)).Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleTodosPage(w http.ResponseWriter, r *http.Request) {
	markers, _ := ctx.Store.GetTodoMarkers()
	template.Todos("TODOs — TON-618", markers).Render(r.Context(), w)
}

// ── Todo Marker Settings ──

func (ctx *HandlerContext) HandleTodoSettingsPage(w http.ResponseWriter, r *http.Request) {
	// Redireciona para a página inicial — as configurações de marcadores
	// foram movidas para o modal de Configurações (⚙️), aba "Marcadores".
	http.Redirect(w, r, "/", http.StatusSeeOther)
}



func (ctx *HandlerContext) HandleGetAllNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Mtime > notes[j].Mtime
	})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"notes": notes,
		"total": len(notes),
	})
}

func (ctx *HandlerContext) HandleGetSidebar(w http.ResponseWriter, r *http.Request) {
	// Delega para o NoteService que consolida file_mods + notes
	notes, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Mtime > notes[j].Mtime
	})

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	filteredNotes := filterNotes(notes, q)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	components.SidebarTree(filteredNotes, q).Render(r.Context(), w)
}

func filterNotes(notes []service.NoteItem, query string) []service.NoteItem {
	if query == "" {
		return notes
	}

	queryLower := strings.ToLower(query)
	var tagQueries []string
	
	// Extrai as tags da busca (ex: #artigo)
	for _, part := range strings.Fields(queryLower) {
		if strings.HasPrefix(part, "#") {
			tagQueries = append(tagQueries, strings.TrimPrefix(part, "#"))
		}
	}

	nameSearch := ""
	for _, part := range strings.Fields(queryLower) {
		if !strings.HasPrefix(part, "#") {
			nameSearch += part + " "
		}
	}
	nameSearch = strings.TrimSpace(nameSearch)

	matchesTags := func(n service.NoteItem) bool {
		if len(tagQueries) == 0 {
			return true
		}
		for _, tq := range tagQueries {
			found := false
			for _, nt := range n.Tags {
				if strings.ToLower(nt) == tq {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}

	if nameSearch == "" {
		var filtered []service.NoteItem
		for _, n := range notes {
			if matchesTags(n) {
				filtered = append(filtered, n)
			}
		}
		return filtered
	}

	var nameMatches []service.NoteItem

	for _, n := range notes {
		if !matchesTags(n) {
			continue
		}
		filenameLower := strings.ToLower(n.Arquivo)
		if strings.Contains(filenameLower, nameSearch) {
			nameMatches = append(nameMatches, n)
		}
	}

	return nameMatches
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
	w.Header().Set("HX-Trigger", "reload-sidebar")
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

	// Pré-carrega o conteúdo de todas as notas em uma única query para evitar o problema N+1
	notesContent, err := ctx.Store.GetAllNotesContent()
	if err != nil {
		slog.Error("erro ao pré-carregar conteúdo das notas", "error", err)
		notesContent = make(map[string]string)
	}

	newCacheEntries := make(map[string]dbCacheEntry)
	var data []map[string]interface{}
	columnSet := make(map[string]bool)

	// Built-in columns
	columnSet["arquivo"] = true
	columnSet["titulo"] = true
	columnSet["mtime"] = true
	columnSet["tags"] = true

	for _, n := range notes {
		var row map[string]interface{}

		// 1. Tentar obter do cache se a modificação for igual
		ctx.dbCacheMu.RLock()
		cached, exists := ctx.dbCache[n.Arquivo]
		ctx.dbCacheMu.RUnlock()

		if exists && cached.Mtime == n.Mtime {
			// Copia rasa do map para evitar problemas de concorrência ou mutações inesperadas
			row = make(map[string]interface{})
			for k, v := range cached.Row {
				row[k] = v
			}
		} else {
			// 2. Cache miss: busca no mapa pré-carregado em memória
			content := notesContent[n.Arquivo]
			fm, _, err := service.ParseFrontmatter(content)
			if err != nil {
				fm = make(map[string]interface{})
			}

			row = make(map[string]interface{})
			row["arquivo"] = n.Arquivo

			displayMtime := n.Mtime
			if t, err := time.Parse(time.RFC3339, n.Mtime); err == nil {
				displayMtime = t.Local().Format("2006-01-02 15:04:05")
			}
			row["mtime"] = displayMtime

			// Map parsed frontmatter
			for k, v := range fm {
				lowerK := strings.ToLower(k)
				if lowerK != "tags" && lowerK != "title" && lowerK != "titulo" {
					row[k] = v
				}
			}

			// Title logic: o título é o nome da nota (o nome do arquivo com a extensão .md)
			parts := strings.Split(n.Arquivo, "/")
			row["titulo"] = parts[len(parts)-1]

			// Tags
			if len(n.Tags) > 0 {
				row["tags"] = strings.Join(n.Tags, ", ")
			} else {
				row["tags"] = ""
			}

			// Infer robust type for drawings, spreadsheets, typst and mermaid
			isDrawing := false
			isSpreadsheet := false
			isTypst := false
			isMermaid := false
			for _, t := range n.Tags {
				lowerT := strings.ToLower(t)
				if lowerT == "drawing" {
					isDrawing = true
				}
				if lowerT == "spreadsheet" {
					isSpreadsheet = true
				}
				if lowerT == "typst" {
					isTypst = true
				}
				if lowerT == "mermaid" {
					isMermaid = true
				}
			}

			lowerContent := strings.ToLower(content)
			lowerArquivo := strings.ToLower(n.Arquivo)

			if !isDrawing && (strings.Contains(lowerArquivo, "drawing") || strings.Contains(lowerContent, "type: drawing") || strings.Contains(lowerContent, `"type":"excalidraw"`) || strings.Contains(lowerContent, "page:page")) {
				isDrawing = true
			}
			if !isSpreadsheet && (strings.Contains(lowerArquivo, "sheet") || strings.Contains(lowerContent, "type: spreadsheet") || strings.Contains(lowerContent, `"widths"`)) {
				isSpreadsheet = true
			}
			if !isTypst && (strings.Contains(lowerArquivo, "typst") || strings.Contains(lowerContent, "type: typst")) {
				isTypst = true
			}
			if !isMermaid && (strings.Contains(lowerArquivo, "mermaid") || strings.Contains(lowerContent, "type: mermaid")) {
				isMermaid = true
			}

			// Check if frontmatter specified type/Type explicitly
			if !isDrawing && !isSpreadsheet && !isTypst && !isMermaid {
				for _, key := range []string{"type", "Type"} {
					if val, ok := fm[key]; ok {
						if strVal, isStr := val.(string); isStr {
							lowerVal := strings.ToLower(strVal)
							if lowerVal == "drawing" || lowerVal == "desenho" {
								isDrawing = true
								break
							} else if lowerVal == "spreadsheet" || lowerVal == "planilha" {
								isSpreadsheet = true
								break
							} else if lowerVal == "typst" {
								isTypst = true
								break
							} else if lowerVal == "mermaid" {
								isMermaid = true
								break
							}
						}
					}
				}
			}

			if isDrawing {
				row["type"] = "desenho"
				row["Type"] = "desenho"
			} else if isSpreadsheet {
				row["type"] = "planilha"
				row["Type"] = "planilha"
			} else if isTypst {
				row["type"] = "typst"
				row["Type"] = "typst"
			} else if isMermaid {
				row["type"] = "mermaid"
				row["Type"] = "mermaid"
			} else {
				// Check if we have Type (capital T) in row
				var rawType string
				if capType, ok := row["Type"].(string); ok {
					rawType = capType
				} else if lowType, ok := row["type"].(string); ok {
					rawType = lowType
				}

				if rawType != "" {
					switch strings.ToLower(rawType) {
					case "drawing", "desenho":
						row["type"] = "desenho"
						row["Type"] = "desenho"
					case "spreadsheet", "planilha":
						row["type"] = "planilha"
						row["Type"] = "planilha"
					case "typst":
						row["type"] = "typst"
						row["Type"] = "typst"
					case "mermaid":
						row["type"] = "mermaid"
						row["Type"] = "mermaid"
					case "pdf":
						row["type"] = "pdf"
						row["Type"] = "pdf"
					case "attachment", "anexo":
						row["type"] = "anexo"
						row["Type"] = "anexo"
					case "archive", "arquivo":
						row["type"] = "arquivo"
						row["Type"] = "arquivo"
					case "note", "nota":
						row["type"] = "nota"
						row["Type"] = "nota"
					default:
						row["type"] = rawType
						row["Type"] = rawType
					}
				} else {
					arquivo := n.Arquivo
					switch {
					case strings.HasPrefix(arquivo, "pdfs/"):
						row["type"] = "pdf"
						row["Type"] = "pdf"
					case strings.HasPrefix(arquivo, "attachments/"):
						row["type"] = "anexo"
						row["Type"] = "anexo"
					case strings.HasPrefix(arquivo, "archives/"):
						row["type"] = "arquivo"
						row["Type"] = "arquivo"
					default:
						row["type"] = "nota"
						row["Type"] = "nota"
					}
				}
			}

			// Guardar no mapa temporário para atualizar o cache em lote depois
			newCacheEntries[n.Arquivo] = dbCacheEntry{
				Mtime: n.Mtime,
				Row:   row,
			}
		}

		// Adiciona as colunas dinâmicas encontradas nesta linha para o set de colunas global
		for k := range row {
			if k != "tags" && k != "title" && k != "titulo" {
				columnSet[k] = true
			}
		}
		columnSet["type"] = true

		data = append(data, row)
	}

	// Atualiza o cache em lote fora do loop principal
	if len(newCacheEntries) > 0 {
		ctx.dbCacheMu.Lock()
		for k, v := range newCacheEntries {
			ctx.dbCache[k] = v
		}
		ctx.dbCacheMu.Unlock()
	}

	var columns []map[string]interface{}
	// Enforce column order for base columns
	columns = append(columns, map[string]interface{}{"title": "Abrir", "field": "abrir_link", "headerSort": false, "width": 80, "hozAlign": "center"})
	columns = append(columns, map[string]interface{}{"title": "Arquivo", "field": "arquivo", "visible": false})
	columns = append(columns, map[string]interface{}{"title": "Título", "field": "titulo", "editor": "input"})
	columns = append(columns, map[string]interface{}{"title": "Tags", "field": "tags", "editor": "input"})
	columns = append(columns, map[string]interface{}{"title": "Tipo", "field": "type", "editor": false, "width": 110})

	for col := range columnSet {
		lowerCol := strings.ToLower(col)
		if lowerCol != "arquivo" && lowerCol != "titulo" && lowerCol != "tags" && lowerCol != "mtime" && lowerCol != "type" {
			columns = append(columns, map[string]interface{}{
				"title":  strings.ToUpper(col[:1]) + col[1:], // strings.Title is deprecated, simple inline title case
				"field":  col,
				"editor": "input",
			})
		}
	}
	// mtime at the end
	columns = append(columns, map[string]interface{}{"title": "Modificação", "field": "mtime", "editor": false, "visible": false})

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

	// Se for renomeação de título (nome do arquivo)
	if req.Key == "titulo" {
		newValStr, ok := req.Value.(string)
		if !ok || newValStr == "" {
			http.Error(w, "invalid title value", http.StatusBadRequest)
			return
		}

		rawOld := req.File
		rawNew := newValStr

		ext := strings.ToLower(filepath.Ext(rawOld))
		isPdf := ext == ".pdf"
		isZip := ext == ".zip"

		var oldName, newName string

		if isPdf {
			basename := filepath.Base(rawOld)
			newBasename := filepath.Base(rawNew)
			if !strings.HasSuffix(strings.ToLower(newBasename), ".pdf") {
				newBasename += ".pdf"
			}

			subdirs := []string{"pdfs", "notes"}
			found := false
			for _, sd := range subdirs {
				testPath := filepath.Join(ctx.Cfg.DocsDir, sd, basename)
				if _, err := os.Stat(testPath); err == nil {
					oldName = sd + "/" + basename
					newName = sd + "/" + newBasename
					oldPath := testPath
					newPath := filepath.Join(ctx.Cfg.DocsDir, newName)
					if err := os.Rename(oldPath, newPath); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					found = true
					break
				}
			}
			if !found {
				http.Error(w, "file not found", http.StatusNotFound)
				return
			}
		} else if isZip {
			basename := filepath.Base(rawOld)
			newBasename := filepath.Base(rawNew)
			if !strings.HasSuffix(strings.ToLower(newBasename), ".zip") {
				newBasename += ".zip"
			}
			sd := "attachments"
			if strings.HasPrefix(rawOld, "archives/") {
				sd = "archives"
			} else if strings.HasPrefix(rawOld, "attachments/") {
				sd = "attachments"
			} else {
				if _, err := os.Stat(filepath.Join(ctx.Cfg.DocsDir, "archives", basename)); err == nil {
					sd = "archives"
				}
			}
			oldName = sd + "/" + basename
			newName = sd + "/" + newBasename
			oldPath := filepath.Join(ctx.Cfg.DocsDir, oldName)
			newPath := filepath.Join(ctx.Cfg.DocsDir, newName)
			if err := os.Rename(oldPath, newPath); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			// Note: delega para o NoteService
			if err := ctx.Notes.Rename(rawOld, rawNew); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			newName = noteFilename(rawNew)
			oldName = noteFilename(rawOld)
		}

		// Update DB: delete old indexes for PDF/ZIP
		if isPdf || isZip {
			ctx.Store.DeleteDocumentsByFile(oldName)
			ctx.Store.DeleteFTSByFile(oldName)
			ctx.Store.DeleteFileMod(oldName)
			ctx.Store.ResetPopularity(oldName)
			ctx.Store.SetFileTags(oldName, nil)
			ctx.Store.ClearLinks(oldName)
			newPath := filepath.Join(ctx.Cfg.DocsDir, newName)
			info, err := os.Stat(newPath)
			if err == nil {
				watcher.ProcessFile(ctx.Store, watcher.FileEvent{
					Path: newPath, Filename: newName, ModTime: info.ModTime(), Type: "create",
				})
			}
		}

		// Invalidate cache
		ctx.dbCacheMu.Lock()
		delete(ctx.dbCache, req.File)
		if newName != "" {
			delete(ctx.dbCache, newName)
		}
		ctx.dbCacheMu.Unlock()

		w.WriteHeader(http.StatusOK)
		return
	}

	// Guard: ZIPs e PDFs não são notas — nunca devem ser processados como markdown.
	// Editar propriedades como "tags" neles via Notes.Save causaria a criação de um
	// registro "notes/attachments/xxx.zip.md" no banco, corrompendo a listagem.
	ext := strings.ToLower(filepath.Ext(req.File))
	if ext == ".zip" || ext == ".pdf" {
		// Para não-notas, só permitimos atualização de tags via SetFileTags.
		if req.Key == "tags" {
			rawVal, _ := req.Value.(string)
			var tagList []string
			for _, t := range strings.Split(rawVal, ",") {
				t = strings.TrimSpace(t)
				t = strings.TrimPrefix(t, "#")
				if t != "" {
					tagList = append(tagList, t)
				}
			}
			if err := ctx.Store.SetFileTags(req.File, tagList); err != nil {
				http.Error(w, "error updating tags", http.StatusInternalServerError)
				return
			}
		}
		// Outros campos (frontmatter) não se aplicam a ZIPs/PDFs — silenciosamente ignora.
		// Invalidate cache
		ctx.dbCacheMu.Lock()
		delete(ctx.dbCache, req.File)
		ctx.dbCacheMu.Unlock()

		w.WriteHeader(http.StatusOK)
		return
	}

	content, err := ctx.Store.GetNote(req.File)
	if err != nil {
		http.Error(w, "note not found", http.StatusNotFound)
		return
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

	// Invalidate cache
	ctx.dbCacheMu.Lock()
	delete(ctx.dbCache, req.File)
	ctx.dbCacheMu.Unlock()

	w.WriteHeader(http.StatusOK)
}
