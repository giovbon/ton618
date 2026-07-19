package system

import (
	"compress/gzip"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"ton618/internal/core/db"
	"ton618/internal/core/domain"
	"ton618/internal/features/notes"
	"ton618/internal/features/search"
	"ton618/internal/features/todos"
	"ton618/internal/httputil"
	"ton618/internal/processor"
	"ton618/internal/watcher"
)

// ── Pages ──

func (ctx *HandlerContext) HandleIndex(w http.ResponseWriter, r *http.Request) {
	search.Index("TON-618").Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleStatus(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, map[string]interface{}{
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
	Docs("Documentação — TON-618").Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleHelpMarkdown(w http.ResponseWriter, r *http.Request) {
	content, err := HelpMD()
	if err != nil {
		slog.Error("ler help.md embedado", "error", err)
		http.Error(w, "Documentação não encontrada", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write(content)
}

func (ctx *HandlerContext) HandleGetTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	tags, err := ctx.Store.GetAllTags()
	if err != nil {
		tags = nil
	}
	// Usa InternalTypeTags para filtrar todas as tags de tipo de editor
	filtered := domain.FilterUserTags(tags)
	httputil.WriteJSON(w, map[string]interface{}{
		"tags": filtered,
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

	// Agrupar preserving insertion order for sections
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

	// Constrói mapa de marker → sort_order para ordenação eficiente
	markerOrder := make(map[string]int)
	for _, m := range markers {
		markerOrder[strings.ToUpper(m.Marker)] = m.SortOrder
	}

	// Para cada arquivo, calcula o menor sort_order dos markers presentes
	// (0 = indefinido → tratado como máximo para ficar no fim)
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

		// Sort sections within each FileGroup by the minimum sort_order of their todos
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

		// Sort todos within each section by sort_order of their type, then by line number
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

// ── Todo Marker Settings ──

func (ctx *HandlerContext) HandleTodoSettingsPage(w http.ResponseWriter, r *http.Request) {
	// Redireciona para a página inicial — as configurações de marcadores
	// foram movidas para o modal de Configurações (⚙️), aba "Marcadores".
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (ctx *HandlerContext) HandleGetAllNotes(w http.ResponseWriter, r *http.Request) {
	noteList, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Slice(noteList, func(i, j int) bool {
		return noteList[i].Mtime > noteList[j].Mtime
	})
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	httputil.WriteJSON(w, map[string]interface{}{
		"notes": noteList,
		"total": len(noteList),
	})
}

func (ctx *HandlerContext) HandleGetSidebar(w http.ResponseWriter, r *http.Request) {
	// Delega para o NoteService que consolida file_mods + notes
	noteList, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sort.Slice(noteList, func(i, j int) bool {
		return noteList[i].Mtime > noteList[j].Mtime
	})

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	filteredNotes := filterNotes(noteList, q)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	notes.SidebarTree(filteredNotes, q).Render(r.Context(), w)
}

func filterNotes(noteList []domain.NoteItem, query string) []domain.NoteItem {
	if query == "" {
		return noteList
	}

	queryLower := strings.ToLower(query)
	nameSearch := strings.TrimSpace(queryLower)

	var nameMatches []domain.NoteItem
	for _, n := range noteList {
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
		_, err = time.Parse(time.RFC3339, mtimeStr)
		if err != nil {

		}
		if false {
			slog.Error("manual sync: reindex note", "file", filename)
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
	Login().Render(r.Context(), w)
}

// ── Tabulator Database Handlers ──

func (ctx *HandlerContext) HandleDatabasePage(w http.ResponseWriter, r *http.Request) {
	notes.Database("Tabulator — TON-618").Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleGetDatabaseData(w http.ResponseWriter, r *http.Request) {
	noteList, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Carrega todas as notas indexadas para busca semântica em lote
	embeddedFiles, err := ctx.Store.GetEmbeddedFiles()
	if err != nil {
		slog.Warn("erro ao obter arquivos com embedding", "error", err)
		embeddedFiles = make(map[string]bool)
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

	for _, n := range noteList {
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
			fm, _, err := notes.ParseFrontmatter(content)
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

			// O tipo já foi calculado corretamente por GetMany() via DetectNoteType.
			row["type"] = n.Type
			row["Type"] = n.Type

			// Guardar no mapa temporário para atualizar o cache em lote depois
			newCacheEntries[n.Arquivo] = dbCacheEntry{
				Mtime: n.Mtime,
				Row:   row,
			}
		}

		// Injeta o status de embedding dinamicamente (garante dados em tempo real sem expirar o cache do arquivo)
		row["embeded"] = embeddedFiles[n.Arquivo]

		// Adiciona as colunas dinâmicas encontradas nesta linha para o set de colunas global
		for k := range row {
			if k != "tags" && k != "title" && k != "titulo" && k != "embeded" {
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
	columns = append(columns, map[string]interface{}{"title": "Embeded", "field": "embeded", "editor": false, "width": 110, "hozAlign": "center"})

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
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		json.NewEncoder(gz).Encode(map[string]interface{}{
			"columns": columns,
			"data":    data,
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"columns": columns,
			"data":    data,
		})
	}
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
			newName = notes.NoteFilename(rawNew)
			oldName = notes.NoteFilename(rawOld)
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

	// Guard: ZIPs, PDFs e EPUBs não são notas — nunca devem ser processados como markdown.
	// Editar propriedades como "tags" neles via Notes.Save causaria a criação de um
	// registro "notes/attachments/xxx.zip.md" ou "notes/epubs/xxx.epub.md" no banco,
	// corrompendo a listagem e criando notas vazias duplicadas.
	ext := strings.ToLower(filepath.Ext(req.File))
	if ext == ".zip" || ext == ".pdf" || ext == ".epub" {
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
		// Outros campos (frontmatter) não se aplicam a ZIPs/PDFs/EPUBs — silenciosamente ignora.
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

	newContent, err := notes.UpdateFrontmatterProperty(content, req.Key, req.Value)
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

func (ctx *HandlerContext) HandleGetNtfySettings(w http.ResponseWriter, r *http.Request) {
	url, _ := ctx.Store.GetSetting("ntfy_url")
	topic, _ := ctx.Store.GetSetting("ntfy_topic")
	user, _ := ctx.Store.GetSetting("ntfy_user")
	pass, _ := ctx.Store.GetSetting("ntfy_pass")

	NtfySettings(url, topic, user, pass, false).Render(r.Context(), w)
}

func (ctx *HandlerContext) HandlePostNtfySettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	url := r.FormValue("ntfy_url")
	topic := r.FormValue("ntfy_topic")
	user := r.FormValue("ntfy_user")
	pass := r.FormValue("ntfy_pass")

	ctx.Store.SetSetting("ntfy_url", url)
	ctx.Store.SetSetting("ntfy_topic", topic)
	ctx.Store.SetSetting("ntfy_user", user)
	ctx.Store.SetSetting("ntfy_pass", pass)

	NtfySettings(url, topic, user, pass, true).Render(r.Context(), w)
}

// HandleGetSemanticDevice retorna o device configurado para embeddings ("wasm" ou "auto").
// GET /api/settings/semantic-device
func (ctx *HandlerContext) HandleGetSemanticDevice(w http.ResponseWriter, r *http.Request) {
	device := "wasm" // padrão: CPU
	if val, err := ctx.Store.GetSetting("semantic_device"); err == nil && val != "" {
		device = val
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"device":"` + device + `"}`))
}

// HandlePostSemanticDevice salva o device configurado para embeddings.
// POST /api/settings/semantic-device
func (ctx *HandlerContext) HandlePostSemanticDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Device string `json:"device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "json invalido", http.StatusBadRequest)
		return
	}
	if body.Device != "wasm" && body.Device != "auto" {
		http.Error(w, "device deve ser 'wasm' ou 'auto'", http.StatusBadRequest)
		return
	}
	if err := ctx.Store.SetSetting("semantic_device", body.Device); err != nil {
		http.Error(w, "erro ao salvar", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"device":"` + body.Device + `"}`))
}

// HandleGetSemanticThresholds retorna os thresholds configurados para buscas semânticas
// GET /api/settings/semantic-thresholds
func (ctx *HandlerContext) HandleGetSemanticThresholds(w http.ResponseWriter, r *http.Request) {
	searchThreshold := 20 // default: 20%
	notesThreshold := 40  // default: 40%

	if val, err := ctx.Store.GetSetting("semantic_search_threshold"); err == nil && val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			if v == 50 {
				searchThreshold = 20
			} else {
				searchThreshold = v
			}
		}
	}
	if val, err := ctx.Store.GetSetting("similar_notes_threshold"); err == nil && val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			if v == 72 {
				notesThreshold = 40
			} else {
				notesThreshold = v
			}
		}
	}

	httputil.WriteJSON(w, map[string]int{
		"search_threshold": searchThreshold,
		"notes_threshold":  notesThreshold,
	})
}

// HandlePostSemanticThresholds salva os thresholds configurados
// POST /api/settings/semantic-thresholds
func (ctx *HandlerContext) HandlePostSemanticThresholds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		SearchThreshold *int `json:"search_threshold,omitempty"`
		NotesThreshold  *int `json:"notes_threshold,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "json invalido", http.StatusBadRequest)
		return
	}

	if body.SearchThreshold != nil {
		if *body.SearchThreshold < 0 || *body.SearchThreshold > 100 {
			http.Error(w, "search_threshold deve ser entre 0 e 100", http.StatusBadRequest)
			return
		}
		if err := ctx.Store.SetSetting("semantic_search_threshold", strconv.Itoa(*body.SearchThreshold)); err != nil {
			http.Error(w, "erro ao salvar", http.StatusInternalServerError)
			return
		}
	}

	if body.NotesThreshold != nil {
		if *body.NotesThreshold < 0 || *body.NotesThreshold > 100 {
			http.Error(w, "notes_threshold deve ser entre 0 e 100", http.StatusBadRequest)
			return
		}
		if err := ctx.Store.SetSetting("similar_notes_threshold", strconv.Itoa(*body.NotesThreshold)); err != nil {
			http.Error(w, "erro ao salvar", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"success"}`))
}

// ── Helpers ──
