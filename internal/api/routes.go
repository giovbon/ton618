package api

import (
	"net/http"
	"sync"
	"time"

	"ton618/internal/config"
	"ton618/internal/db"
	"ton618/internal/service"
	"ton618/internal/watcher"
)

// dbCacheEntry guarda a linha pré-formatada do banco de dados para evitar ler e parsear a nota repetidamente.
type dbCacheEntry struct {
	Mtime string
	Row   map[string]interface{}
}

// HandlerContext agrega todas as dependências dos handlers.
type HandlerContext struct {
	Cfg       *config.AppConfig
	Store     *db.Store
	Watcher   *watcher.Watcher

	// Serviços (lógica de negócio separada dos handlers HTTP)
	Backup *service.BackupService
	Notes  *service.NoteService

	// Cache do banco de dados de notas (/database)
	dbCache   map[string]dbCacheEntry
	dbCacheMu sync.RWMutex
}

// NewHandlerContext cria o contexto.
func NewHandlerContext(cfg *config.AppConfig, store *db.Store, w *watcher.Watcher) *HandlerContext {
	return &HandlerContext{
		Cfg:     cfg,
		Store:   store,
		Watcher: w,
		// Backup e Notes são injetados depois (possuem dependências cíclicas)
		Backup:  service.NewBackupService(store, store, cfg.DocsDir),
		Notes:   service.NewNoteService(store, store, store, store, store, store, cfg.DocsDir),
		dbCache: make(map[string]dbCacheEntry),
	}
}

// SetupRoutes registra todas as rotas no ServeMux.
func (ctx *HandlerContext) SetupRoutes(mux *http.ServeMux) {
	// Rate limiters para endpoints pesados
	// Busca FTS: 30 req/min por IP
	searchLimiter := NewRateLimiter(30, time.Minute)

	// Páginas HTML (server-side rendered)
	mux.HandleFunc("GET /", ctx.HandleIndex)
	mux.HandleFunc("GET /editor", ctx.HandleEditor)
	mux.HandleFunc("GET /spreadsheet", ctx.HandleSpreadsheet)
	mux.HandleFunc("GET /drawing", ctx.HandleDrawing)
	mux.HandleFunc("GET /typst", ctx.HandleTypst)
	mux.HandleFunc("GET /mermaid", ctx.HandleMermaid)
	mux.HandleFunc("GET /todos", ctx.HandleTodosPage)
	mux.HandleFunc("GET /database", ctx.HandleDatabasePage)
	mux.HandleFunc("GET /help", ctx.HandleHelp)
	mux.HandleFunc("POST /api/notes/render-typst", ctx.HandleTypstRender)
	mux.HandleFunc("GET /api/notes/download-typst-pdf", ctx.HandleTypstPDF)

	mux.HandleFunc("GET /login", ctx.HandleLogin)

	// Busca (HTMX partial) — rate limited
	mux.Handle("POST /search", searchLimiter.Middleware(http.HandlerFunc(ctx.HandleSearch)))
	mux.Handle("GET /search", searchLimiter.Middleware(http.HandlerFunc(ctx.HandleSearch)))

	// Arquivos
	mux.HandleFunc("GET /file", ctx.HandleFile)
	mux.HandleFunc("GET /file/download", ctx.HandleFileDownload)
	mux.HandleFunc("POST /file/save", ctx.HandleFileSave)
	mux.HandleFunc("POST /api/note/save", ctx.HandleNoteSaveJSON)
	mux.HandleFunc("POST /file/delete", ctx.HandleFileDelete)
	mux.HandleFunc("POST /file/rename", ctx.HandleFileRename)
	mux.HandleFunc("POST /upload", ctx.HandleUpload)
	mux.HandleFunc("POST /api/upload-image", ctx.HandleUploadImage)
	mux.HandleFunc("POST /api/cleanup-images", ctx.HandleCleanupImages)

	// API
	mux.HandleFunc("POST /api/capture", ctx.HandleCapture)
	mux.HandleFunc("GET /api/status", ctx.HandleStatus)
	mux.HandleFunc("GET /api/health", ctx.HandleHealth)
	mux.HandleFunc("GET /api/help/markdown", ctx.HandleHelpMarkdown)
	mux.HandleFunc("GET /api/tags", ctx.HandleGetTags)
	mux.HandleFunc("GET /api/keywords", ctx.HandleGetKeywords)
	mux.HandleFunc("GET /api/todos", ctx.HandleListTodos)
	// Todo Markers
	mux.HandleFunc("GET /api/todo-markers", ctx.HandleGetTodoMarkers)
	mux.HandleFunc("POST /api/todo-markers/add", ctx.HandleAddTodoMarker)
	mux.HandleFunc("POST /api/todo-markers/update", ctx.HandleUpdateTodoMarker)
	mux.HandleFunc("DELETE /api/todo-markers/remove", ctx.HandleRemoveTodoMarker)
	mux.HandleFunc("POST /api/todo-markers/reset", ctx.HandleResetTodoMarkers)

	// Settings page
	mux.HandleFunc("GET /settings", ctx.HandleTodoSettingsPage)

	mux.HandleFunc("POST /api/upload-attachment", ctx.HandleUploadAttachment)
	mux.HandleFunc("POST /api/note/duplicate", ctx.HandleDuplicateNote)
	mux.HandleFunc("GET /api/notes", ctx.HandleGetAllNotes)
	mux.HandleFunc("GET /api/sidebar", ctx.HandleGetSidebar)
	mux.HandleFunc("GET /api/notes/database", ctx.HandleGetDatabaseData)
	mux.HandleFunc("POST /api/notes/update-property", ctx.HandleUpdateNoteProperty)
	mux.HandleFunc("POST /api/sync", ctx.HandleManualSync)
	mux.HandleFunc("POST /api/bulk-delete", ctx.HandleBulkDelete)
	mux.HandleFunc("POST /api/bulk-archive", ctx.HandleBulkArchive)
	mux.HandleFunc("GET /api/archives", ctx.HandleListArchives)
	mux.HandleFunc("POST /api/archive/restore", ctx.HandleRestoreArchive)
	mux.HandleFunc("GET /api/backup", ctx.HandleBackup)

	// Stopwords customizadas
	mux.HandleFunc("GET /api/stopwords", ctx.HandleGetStopwords)
	mux.HandleFunc("POST /api/stopwords/add", ctx.HandleAddStopword)
	mux.HandleFunc("DELETE /api/stopwords/remove", ctx.HandleRemoveStopword)

	// Static files
	fs := http.FileServer(http.Dir(ctx.Cfg.WebDir + "/static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))
}
