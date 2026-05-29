package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
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
		filename = "notes/novo.md"
	}

	// Se for .zip, redireciona para download em vez de abrir o editor
	if strings.HasSuffix(strings.ToLower(filename), ".zip") {
		http.Redirect(w, r, "/file/download?name="+url.QueryEscape(filename), http.StatusFound)
		return
	}

	// Normaliza o filename: garante prefixo notes/ e extensao .md
	sanitized := sanitizeFilename(filename)

	// Se a URL nao estava normalizada, redireciona para a URL canonica
	if sanitized != filename {
		canonical := "/editor?file=" + url.QueryEscape(sanitized)
		http.Redirect(w, r, canonical, http.StatusFound)
		return
	}

	var content string
	var tags []string

	// So ignora conteudo para o template exato "notes/novo.md"
	if filename != "notes/novo.md" {
		fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
		if data, err := os.ReadFile(fullPath); err == nil {
			content = string(data)
		}
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

func (ctx *HandlerContext) HandleGetAllNotes(w http.ResponseWriter, r *http.Request) {
	from, _ := strconv.Atoi(r.URL.Query().Get("from"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 || size > 200 {
		size = 50
	}

	mods, total, err := ctx.Store.GetFileModsPaginated(from, size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type noteItem struct {
		Arquivo string   `json:"arquivo"`
		Tags    []string `json:"tags"`
		Mtime   string   `json:"mtime"`
	}

	var notes []noteItem
	for arquivo, mtime := range mods {
		tags, _ := ctx.Store.GetFileTags(arquivo)
		notes = append(notes, noteItem{
			Arquivo: arquivo,
			Tags:    tags,
			Mtime:   mtime,
		})
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Mtime > notes[j].Mtime
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"notes": notes,
		"total": total,
		"from":  from,
		"size":  size,
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
