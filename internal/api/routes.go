package api

import (
	"html/template"
	"net/http"

	"ton618/internal/config"
	"ton618/internal/db"
	"ton618/internal/semantic"
	"ton618/internal/watcher"
)

// HandlerContext agrega todas as dependências dos handlers.
type HandlerContext struct {
	Cfg       *config.AppConfig
	Store     *db.Store
	Watcher   *watcher.Watcher
	Embed     semantic.EmbeddingProvider
	Templates *template.Template // será populado em main
}

// NewHandlerContext cria o contexto.
func NewHandlerContext(cfg *config.AppConfig, store *db.Store, w *watcher.Watcher, embed semantic.EmbeddingProvider) *HandlerContext {
	return &HandlerContext{
		Cfg:     cfg,
		Store:   store,
		Watcher: w,
		Embed:   embed,
	}
}

// SetupRoutes registra todas as rotas no ServeMux.
func (ctx *HandlerContext) SetupRoutes(mux *http.ServeMux) {
	// Páginas HTML (server-side rendered)
	mux.HandleFunc("GET /", ctx.HandleIndex)
	mux.HandleFunc("GET /editor", ctx.HandleEditor)
	mux.HandleFunc("GET /graph", ctx.HandleGraph)

	mux.HandleFunc("GET /login", ctx.HandleLogin)

	// Busca (HTMX partial)
	mux.HandleFunc("POST /search", ctx.HandleSearch)
	mux.HandleFunc("GET /search", ctx.HandleSearch)

	// Arquivos
	mux.HandleFunc("GET /file", ctx.HandleFile)
	mux.HandleFunc("POST /file/save", ctx.HandleFileSave)
	mux.HandleFunc("POST /file/delete", ctx.HandleFileDelete)
	mux.HandleFunc("POST /file/rename", ctx.HandleFileRename)
	mux.HandleFunc("POST /upload", ctx.HandleUpload)

	// API
	mux.HandleFunc("POST /api/capture", ctx.HandleCapture)
	mux.HandleFunc("GET /api/status", ctx.HandleStatus)
	mux.HandleFunc("GET /api/health", ctx.HandleHealth)
	mux.HandleFunc("GET /api/tags", ctx.HandleGetTags)
	mux.HandleFunc("GET /api/graph/data", ctx.HandleGraphData)
	mux.HandleFunc("POST /api/graph/query", ctx.HandleGraphQuery)
	mux.HandleFunc("POST /api/graph/project", ctx.HandleGraphProject)
	mux.HandleFunc("POST /api/upload-attachment", ctx.HandleUploadAttachment)
	mux.HandleFunc("GET /api/notes", ctx.HandleGetAllNotes)
	mux.HandleFunc("POST /api/sync", ctx.HandleManualSync)

	// Static files
	fs := http.FileServer(http.Dir(ctx.Cfg.WebDir + "/static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))
}
