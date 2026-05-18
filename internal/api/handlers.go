package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ton618/internal/db"
	"ton618/internal/search"
	"ton618/internal/watcher"
)

// ── Pages ──

func (ctx *HandlerContext) HandleIndex(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "TON-618",
		"Query": r.URL.Query().Get("q"),
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

	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	if data, err := os.ReadFile(fullPath); err == nil {
		content = string(data)
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
		"Title":      "Editor - " + filename,
		"Filename":   filename,
		"Content":    content,
		"Tags":       tags,
		"AllTags":    allTags,
		"LoadTipTap": true,
	}
	ctx.render(w, "editor.html", data)
}

func (ctx *HandlerContext) HandleGraph(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Mapa Semântico - TON-618",
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
		snippet := hit.Doc.Texto
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
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
	filename := r.URL.Query().Get("name")
	if filename == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	http.ServeFile(w, r, fullPath)
}

func (ctx *HandlerContext) HandleFileSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := r.FormValue("filename")
	content := r.FormValue("content")
	tags := r.FormValue("tags")

	if filename == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	// Ensure filename has .md extension
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}

	// Ensure directory exists
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
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
	if err := watcher.ProcessFile(ctx.Store, ev); err != nil {
		slog.Error("process file after save", "error", err)
	}

	// Redirect back to editor
	http.Redirect(w, r, "/editor?file="+filename, http.StatusSeeOther)
}

func (ctx *HandlerContext) HandleFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := r.FormValue("filename")
	if filename == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

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

	oldName := r.FormValue("old")
	newName := r.FormValue("new")

	if oldName == "" || newName == "" {
		http.Error(w, "old and new required", http.StatusBadRequest)
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
		})
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
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

	subdir := r.FormValue("subdir")
	if subdir == "" {
		subdir = "notes"
	}

	filename := filepath.Join(subdir, header.Filename)
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)

	dir := filepath.Dir(fullPath)
	os.MkdirAll(dir, 0755)

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
	})

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

	var nodes []node
	for id, nv := range embeddings {
		// Use filename as display name
		parts := strings.Split(id, "/")
		display := id
		if len(parts) > 0 {
			display = parts[len(parts)-1]
		}
		if nv.Title != "" {
			display = nv.Title
		}
		nodes = append(nodes, node{
			ID:    id,
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

func (ctx *HandlerContext) HandleManualSync(w http.ResponseWriter, r *http.Request) {
	// Trigger poll
	ctx.Watcher.PollAll()
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="text-green-500">✓ Sincronização iniciada</div>`))
}
