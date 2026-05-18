package api

import (
	"encoding/json"
	"fmt"
	"io"
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
	"ton618/internal/search"
	"ton618/internal/watcher"
)

// ── Helpers de normalizacao ──

// sanitizeFilename garante que o nome do arquivo:
// 1. Nao tenha subdiretorios (strips tudo antes de /)
// 2. Tenha extensao .md
// 3. Esteja no diretorio notes/
func sanitizeFilename(name string) string {
	// Remove qualquer prefixo de diretorio
	base := filepath.Base(name)
	// Garante extensao .md
	if !strings.HasSuffix(base, ".md") {
		base += ".md"
	}
	// Forca prefixo notes/
	if !strings.HasPrefix(base, "notes/") {
		base = "notes/" + base
	}
	return base
}

// displayName retorna apenas o nome do arquivo (sem diretorio) para exibicao.
func displayName(name string) string {
	base := filepath.Base(name)
	return base
}

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
		filename = "notes/novo.md"
	}

	var content string
	var tags []string

	// Se for o template "novo.md" ou "novo-*", ignora conteudo existente no disco
	// para evitar que o auto-save polua o template de nova nota.
	if !strings.HasPrefix(filename, "notes/novo") {
		fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
		if data, err := os.ReadFile(fullPath); err == nil {
			content = string(data)
		}
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

func (ctx *HandlerContext) HandleGraph(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":        "Mapa Semântico - TON-618",
		"ContentBlock": "graphContent",
	}
	ctx.render(w, "graph.html", data)
}

// ── Search (HTMX partial) ──

func (ctx *HandlerContext) HandleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("q")
	if query == "" && r.Method == "POST" {
		body, _ := io.ReadAll(r.Body)
		query = string(body)
		// parse form-encoded or simple string
		if strings.HasPrefix(query, "q=") {
			query = strings.TrimPrefix(query, "q=")
		}
	}

	from, _ := strconv.Atoi(r.FormValue("from"))
	size, _ := strconv.Atoi(r.FormValue("size"))
	if size <= 0 {
		size = 20
	}

	results, err := search.Search(r.Context(), ctx.Store, query, from, size,
		ctx.Store.GetLinkCount, ctx.Store.GetPopularity)
	if err != nil {
		slog.Error("search error", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Track popularity
	for _, hit := range results.Hits {
		ctx.Store.IncrementPopularity(hit.Doc.Arquivo)
	}

	// Build template data
	type resultItem struct {
		ID         string
		Arquivo    string
		Secao      string
		Tags       []string
		Snippet    string
		Score      float64
		Tipo       string
		Timestamp  string
		IsIndexing bool
	}

	var items []resultItem
	for _, hit := range results.Hits {
		// Clean snippet: strip HTML, show context around query
		snippet := hit.Doc.Texto
		// Strip basic HTML tags
		snippet = strings.ReplaceAll(snippet, "<p>", "")
		snippet = strings.ReplaceAll(snippet, "</p>", " ")
		snippet = strings.ReplaceAll(snippet, "<br>", " ")
		snippet = strings.ReplaceAll(snippet, "<br/>", " ")
		snippet = strings.ReplaceAll(snippet, "<strong>", "")
		snippet = strings.ReplaceAll(snippet, "</strong>", "")
		snippet = strings.ReplaceAll(snippet, "<em>", "")
		snippet = strings.ReplaceAll(snippet, "</em>", "")
		snippet = strings.ReplaceAll(snippet, "<code>", "")
		snippet = strings.ReplaceAll(snippet, "</code>", "")
		snippet = strings.ReplaceAll(snippet, "<pre>", "")
		snippet = strings.ReplaceAll(snippet, "</pre>", "")
		snippet = strings.ReplaceAll(snippet, "<h1>", "")
		snippet = strings.ReplaceAll(snippet, "</h1>", " - ")
		snippet = strings.ReplaceAll(snippet, "<h2>", "")
		snippet = strings.ReplaceAll(snippet, "</h2>", " - ")
		snippet = strings.ReplaceAll(snippet, "<h3>", "")
		snippet = strings.ReplaceAll(snippet, "</h3>", " - ")
		snippet = strings.ReplaceAll(snippet, "<ul>", "")
		snippet = strings.ReplaceAll(snippet, "</ul>", "")
		snippet = strings.ReplaceAll(snippet, "<li>", "  ")
		snippet = strings.ReplaceAll(snippet, "</li>", "")
		snippet = strings.ReplaceAll(snippet, "<a[^>]*>", "")
		snippet = strings.ReplaceAll(snippet, "</a>", "")
		// Normalize whitespace
		snippet = strings.Join(strings.Fields(snippet), " ")

		// Extract context window around the query term (first occurrence)
		queryLower := strings.ToLower(query)
		snippetLower := strings.ToLower(snippet)
		if pos := strings.Index(snippetLower, queryLower); pos >= 0 {
			start := pos - 80
			if start < 0 {
				start = 0
			}
			end := pos + len(query) + 120
			if end > len(snippet) {
				end = len(snippet)
			}
			snippet = snippet[start:end]
			if start > 0 {
				snippet = "... " + snippet
			}
			if end < len(snippet) {
				snippet = snippet + " ..."
			}
		} else if len(snippet) > 250 {
			snippet = snippet[:250] + "..."
		}
		tags := db.TagsToSlice(hit.Doc.Tags)
		items = append(items, resultItem{
			ID:         hit.Doc.ID,
			Arquivo:    hit.Doc.Arquivo,
			Secao:      hit.Doc.Secao,
			Tags:       tags,
			Snippet:    snippet,
			Score:      hit.FinalScore,
			Tipo:       hit.Doc.Tipo,
			Timestamp:  hit.Doc.Timestamp,
			IsIndexing: hit.Doc.IsIndexing,
		})
	}

	data := map[string]interface{}{
		"Query":   query,
		"Results": items,
		"Total":   results.Total,
	}

	// HTMX: return only the results partial
	w.Header().Set("Content-Type", "text/html")
	ctx.renderPartial(w, "search_results.html", data)
}

// ── File handlers ──

func (ctx *HandlerContext) HandleFile(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("name")
	if raw == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	filename := sanitizeFilename(raw)
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	http.ServeFile(w, r, fullPath)
}

func (ctx *HandlerContext) HandleFileSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := r.FormValue("filename")
	content := r.FormValue("content")
	tags := r.FormValue("tags")

	if raw == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	// Normaliza: remove subdiretorios, garante .md e prefixo notes/
	filename := sanitizeFilename(raw)

	// Garante que o diretorio notes/ existe
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update tags
	if tags != "" {
		tagList := strings.Split(tags, ",")
		var cleanTags []string
		for _, t := range tagList {
			t = strings.TrimSpace(t)
			if t != "" {
				cleanTags = append(cleanTags, t)
			}
		}
		ctx.Store.SetFileTags(filename, cleanTags)
	}

	// Process immediately (sync)
	ev := watcher.FileEvent{
		Path:     fullPath,
		Filename: filename,
		ModTime:  time.Now(),
		Type:     "modify",
	}
	if err := watcher.ProcessFile(ctx.Store, ev, ctx.Embed, ctx.Cfg.EmbeddingAll); err != nil {
		slog.Error("process file after save", "error", err)
	}

	// Redirect back to editor
	http.Redirect(w, r, "/editor?file="+url.QueryEscape(filename), http.StatusSeeOther)
}

func (ctx *HandlerContext) HandleFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := r.FormValue("filename")
	if raw == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	filename := sanitizeFilename(raw)
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Remove from DB
	ctx.Store.DeleteDocumentsByFile(filename)
	ctx.Store.DeleteFTSByFile(filename)
	ctx.Store.DeleteEmbedding(filename)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (ctx *HandlerContext) HandleFileRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawOld := r.FormValue("old")
	rawNew := r.FormValue("new")

	if rawOld == "" || rawNew == "" {
		http.Error(w, "old and new required", http.StatusBadRequest)
		return
	}

	oldName := sanitizeFilename(rawOld)
	newName := sanitizeFilename(rawNew)

	if oldName == newName {
		http.Redirect(w, r, "/editor?file="+newName, http.StatusSeeOther)
		return
	}

	oldPath := filepath.Join(ctx.Cfg.DocsDir, oldName)
	newPath := filepath.Join(ctx.Cfg.DocsDir, newName)

	if err := os.Rename(oldPath, newPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update DB: delete old, re-index new
	ctx.Store.DeleteDocumentsByFile(oldName)
	ctx.Store.DeleteFTSByFile(oldName)

	info, err := os.Stat(newPath)
	if err == nil {
		watcher.ProcessFile(ctx.Store, watcher.FileEvent{
			Path: newPath, Filename: newName, ModTime: info.ModTime(), Type: "create",
		}, ctx.Embed, ctx.Cfg.EmbeddingAll)
	}

	http.Redirect(w, r, "/editor?file="+url.QueryEscape(newName), http.StatusSeeOther)
}

func (ctx *HandlerContext) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.ParseMultipartForm(10 << 20) // 10MB

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Forca diretorio notes independente do subdir enviado
	filename := "notes/" + filepath.Base(header.Filename)
	// Garante .md
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)

	os.MkdirAll(filepath.Dir(fullPath), 0755)

	dst, err := os.Create(fullPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	io.Copy(dst, file)

	// Process
	info, _ := os.Stat(fullPath)
	watcher.ProcessFile(ctx.Store, watcher.FileEvent{
		Path: fullPath, Filename: filename, ModTime: info.ModTime(), Type: "create",
	}, ctx.Embed, ctx.Cfg.EmbeddingAll)

	http.Redirect(w, r, "/editor?file="+filename, http.StatusSeeOther)
}

// ── API ──

func (ctx *HandlerContext) HandleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	embCount := ctx.Store.GetEmbeddingCount()
	fmt.Fprintf(w, `{"status":"ok","documents":%d,"embeddings":%d}`, ctx.Store.GetDocumentCount(), embCount)
}

func (ctx *HandlerContext) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"up","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
}

func (ctx *HandlerContext) HandleGetTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tags, err := ctx.Store.GetAllTags()
	if err != nil {
		tags = nil
	}
	fmt.Fprintf(w, `{"tags":[`)
	for i, t := range tags {
		if i > 0 {
			w.Write([]byte(","))
		}
		fmt.Fprintf(w, `"%s"`, t)
	}
	w.Write([]byte(`]}`))
}

func (ctx *HandlerContext) HandleGraphData(w http.ResponseWriter, r *http.Request) {
	embeddings, _ := ctx.Store.GetAllEmbeddings()
	links, _ := ctx.Store.GetAllLinks()

	type node struct {
		ID    string  `json:"id"`
		Title string  `json:"title"`
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
	}
	type link struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}

	// Map doc IDs to filenames by consulting the documents table
	var nodes []node
	for docID, nv := range embeddings {
		// Busca o documento para obter o nome real do arquivo
		doc, _ := ctx.Store.GetDocument(docID)
		fileName := ""
		if doc != nil && doc.Arquivo != "" {
			fileName = doc.Arquivo
		}

		// Se nao achou o documento, tenta usar o titulo salvo
		display := nv.Title
		if fileName != "" {
			// Usa o nome do arquivo como ID (para abrir a nota) e como titulo
			parts := strings.Split(fileName, "/")
			shortName := strings.TrimSuffix(parts[len(parts)-1], ".md")
			if len(parts) > 1 {
				shortName = parts[len(parts)-1] + " (" + parts[len(parts)-1] + ")"
			}
			display = shortName
		} else if display == "" {
			display = docID
			if len(display) > 12 {
				display = display[:12] + "..."
			}
		}

		nodes = append(nodes, node{
			ID:    fileName, // O ID real é o filename, nao o hash
			Title: display,
			X:     nv.X,
			Y:     nv.Y,
		})
	}

	var edgeList []link
	for fromFile, toFiles := range links {
		for _, toFile := range toFiles {
			edgeList = append(edgeList, link{Source: fromFile, Target: toFile})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	result := map[string]interface{}{
		"nodes": nodes,
		"links": edgeList,
	}
	json.NewEncoder(w).Encode(result)
}

func (ctx *HandlerContext) HandleGetAllNotes(w http.ResponseWriter, r *http.Request) {
	mods, err := ctx.Store.GetAllFileMods()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type noteItem struct {
		Arquivo  string   `json:"arquivo"`
		Tags     []string `json:"tags"`
		Mtime    string   `json:"mtime"`
		Embedded bool     `json:"embedded"`
	}

	var notes []noteItem
	for arquivo, mtime := range mods {
		tags, _ := ctx.Store.GetFileTags(arquivo)
		embedded := ctx.Store.HasFileEmbedding(arquivo)
		notes = append(notes, noteItem{
			Arquivo:  arquivo,
			Tags:     tags,
			Mtime:    mtime,
			Embedded: embedded,
		})
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Mtime > notes[j].Mtime
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"notes": notes,
	})
}

func (ctx *HandlerContext) HandleManualSync(w http.ResponseWriter, r *http.Request) {
	// Trigger poll
	ctx.Watcher.PollAll()
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="text-green-500">✓ Sincronização iniciada</div>`))
}

func (ctx *HandlerContext) HandleLogin(w http.ResponseWriter, r *http.Request) {
	ctx.renderLogin(w, "login.html", map[string]interface{}{
		"Title": "Login - TON-618",
	})
}
