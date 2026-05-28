package api

import (
	"html/template"
	"net/http"
	"time"

	"ton618/internal/config"
	"ton618/internal/db"
	"ton618/internal/index"
	"ton618/internal/watcher"
)

// HandlerContext agrega todas as dependências dos handlers.
type HandlerContext struct {
	Cfg       *config.AppConfig
	Store     *db.Store
	Watcher   *watcher.Watcher
	Embed     index.EmbeddingProvider
	Templates *template.Template // será populado em main
}

// NewHandlerContext cria o contexto.
func NewHandlerContext(cfg *config.AppConfig, store *db.Store, w *watcher.Watcher, embed index.EmbeddingProvider) *HandlerContext {
	return &HandlerContext{
		Cfg:     cfg,
		Store:   store,
		Watcher: w,
		Embed:   embed,
	}
}

// SetupRoutes registra todas as rotas no ServeMux.
func (ctx *HandlerContext) SetupRoutes(mux *http.ServeMux) {
	// Rate limiters para endpoints pesados
	// Busca FTS: 30 req/min por IP
	searchLimiter := NewRateLimiter(30, time.Minute)
	// API de embedding (Gemini/Ollama): 10 req/min por IP (evita estourar quota)
	embedLimiter := NewRateLimiter(10, time.Minute)

	// Páginas HTML (server-side rendered)
	mux.HandleFunc("GET /", ctx.HandleIndex)
	mux.HandleFunc("GET /editor", ctx.HandleEditor)
	mux.HandleFunc("GET /graph", ctx.HandleGraph)

	mux.HandleFunc("GET /login", ctx.HandleLogin)

	// Busca (HTMX partial) — rate limited
	mux.Handle("POST /search", searchLimiter.Middleware(http.HandlerFunc(ctx.HandleSearch)))
	mux.Handle("GET /search", searchLimiter.Middleware(http.HandlerFunc(ctx.HandleSearch)))

	// Arquivos
	mux.HandleFunc("GET /file", ctx.HandleFile)
	mux.HandleFunc("GET /file/download", ctx.HandleFileDownload)
	mux.HandleFunc("POST /file/save", ctx.HandleFileSave)
	mux.HandleFunc("POST /file/delete", ctx.HandleFileDelete)
	mux.HandleFunc("POST /file/rename", ctx.HandleFileRename)
	mux.HandleFunc("POST /upload", ctx.HandleUpload)
	mux.HandleFunc("POST /api/upload-image", ctx.HandleUploadImage)
	mux.HandleFunc("POST /api/cleanup-images", ctx.HandleCleanupImages)
	mux.HandleFunc("POST /api/merge-notes", ctx.HandleMergeNotes)

	// API
	mux.Handle("POST /api/capture", embedLimiter.Middleware(http.HandlerFunc(ctx.HandleCapture)))
	mux.HandleFunc("GET /api/status", ctx.HandleStatus)
	mux.HandleFunc("GET /api/health", ctx.HandleHealth)
	mux.HandleFunc("GET /api/tags", ctx.HandleGetTags)
	mux.HandleFunc("GET /api/graph/data", ctx.HandleGraphData)
	mux.Handle("POST /api/graph/query", embedLimiter.Middleware(http.HandlerFunc(ctx.HandleGraphQuery)))
	mux.HandleFunc("POST /api/graph/project", ctx.HandleGraphProject)
	mux.HandleFunc("POST /api/upload-attachment", ctx.HandleUploadAttachment)
	mux.HandleFunc("GET /api/notes", ctx.HandleGetAllNotes)
	mux.HandleFunc("POST /api/sync", ctx.HandleManualSync)
	mux.HandleFunc("POST /api/bulk-delete", ctx.HandleBulkDelete)
	mux.HandleFunc("POST /api/bulk-archive", ctx.HandleBulkArchive)
	mux.HandleFunc("GET /api/archives", ctx.HandleListArchives)
	mux.HandleFunc("POST /api/archive/restore", ctx.HandleRestoreArchive)
	mux.HandleFunc("POST /api/toggle-embed", ctx.HandleToggleEmbed)

	// Static files
	fs := http.FileServer(http.Dir(ctx.Cfg.WebDir + "/static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))
}
