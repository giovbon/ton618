package main

import (
	"net/http"
	"time"

	"ton618/internal/features/notes"
	"ton618/internal/features/search"
	"ton618/internal/features/system"
	"ton618/internal/features/todos"
	"ton618/internal/middleware"
)

// SetupRoutes registra todas as rotas no ServeMux.
func SetupRoutes(mux *http.ServeMux, sysCtx *system.HandlerContext, notesCtx *notes.HandlerContext, todosCtx *todos.HandlerContext, searchCtx *search.HandlerContext) {
	// Rate limiters para endpoints pesados
	searchLimiter := middleware.NewRateLimiter(30, time.Minute)

	// Páginas HTML (server-side rendered) - SYSTEM
	mux.HandleFunc("GET /", sysCtx.HandleIndex)
	mux.HandleFunc("GET /todos", sysCtx.HandleTodosPage)
	mux.HandleFunc("GET /settings", sysCtx.HandleTodoSettingsPage)
	mux.HandleFunc("GET /database", sysCtx.HandleDatabasePage)
	mux.HandleFunc("GET /help", sysCtx.HandleHelp)
	mux.HandleFunc("GET /login", sysCtx.HandleLogin)

	// API System
	mux.HandleFunc("GET /api/status", sysCtx.HandleStatus)
	mux.HandleFunc("GET /api/health", sysCtx.HandleHealth)
	mux.HandleFunc("GET /api/help/markdown", sysCtx.HandleHelpMarkdown)
	mux.HandleFunc("GET /api/todos", sysCtx.HandleListTodos)

	// NOTES (Editor e Arquivos)
	mux.HandleFunc("GET /editor", sysCtx.HandleEditor)
	mux.HandleFunc("GET /spreadsheet", sysCtx.HandleSpreadsheet)
	mux.HandleFunc("GET /drawing", sysCtx.HandleDrawing)
	mux.HandleFunc("GET /typst", notesCtx.HandleTypst)
	mux.HandleFunc("GET /mermaid", notesCtx.HandleMermaid)
	
	mux.HandleFunc("POST /api/notes/render-typst", notesCtx.HandleTypstRender)
	mux.HandleFunc("GET /api/notes/download-typst-pdf", notesCtx.HandleTypstPDF)

	mux.HandleFunc("GET /file", notesCtx.HandleFile)
	mux.HandleFunc("GET /file/download", notesCtx.HandleFileDownload)
	mux.HandleFunc("POST /file/save", notesCtx.HandleFileSave)
	mux.HandleFunc("POST /api/note/save", notesCtx.HandleNoteSaveJSON)
	mux.HandleFunc("POST /file/delete", notesCtx.HandleFileDelete)
	mux.HandleFunc("POST /api/notes/delete", notesCtx.HandleFileDelete)
	mux.HandleFunc("POST /file/rename", notesCtx.HandleFileRename)
	mux.HandleFunc("POST /api/notes/rename", notesCtx.HandleFileRename)
	mux.HandleFunc("POST /upload", notesCtx.HandleUpload)
	mux.HandleFunc("POST /api/upload-image", notesCtx.HandleUploadImage)
	mux.HandleFunc("POST /api/cleanup-images", notesCtx.HandleCleanupImages)
	mux.HandleFunc("POST /api/upload-attachment", notesCtx.HandleUploadAttachment)
	
	mux.HandleFunc("POST /api/capture", notesCtx.HandleCapture)
	mux.HandleFunc("GET /api/tags", sysCtx.HandleGetTags)
	mux.HandleFunc("GET /api/keywords", sysCtx.HandleGetKeywords)
	mux.HandleFunc("POST /api/note/duplicate", notesCtx.HandleDuplicateNote)
	mux.HandleFunc("GET /api/notes", sysCtx.HandleGetAllNotes)
	mux.HandleFunc("GET /api/sidebar", sysCtx.HandleGetSidebar)
	mux.HandleFunc("GET /api/notes/database", sysCtx.HandleGetDatabaseData)
	mux.HandleFunc("POST /api/notes/update-property", sysCtx.HandleUpdateNoteProperty)
	mux.HandleFunc("POST /api/sync", sysCtx.HandleManualSync)
	mux.HandleFunc("POST /api/bulk-delete", searchCtx.HandleBulkDelete)
	mux.HandleFunc("POST /api/bulk-archive", searchCtx.HandleBulkArchive)
	mux.HandleFunc("GET /api/archives", searchCtx.HandleListArchives)
	mux.HandleFunc("POST /api/archive/restore", searchCtx.HandleRestoreArchive)
	mux.HandleFunc("GET /api/backup", notesCtx.HandleBackup)

	// TODOS (Markers)
	mux.HandleFunc("GET /api/todo-markers", todosCtx.HandleGetTodoMarkers)
	mux.HandleFunc("POST /api/todo-markers/add", todosCtx.HandleAddTodoMarker)
	mux.HandleFunc("POST /api/todo-markers/update", todosCtx.HandleUpdateTodoMarker)
	mux.HandleFunc("DELETE /api/todo-markers/remove", todosCtx.HandleRemoveTodoMarker)
	mux.HandleFunc("POST /api/todo-markers/reset", todosCtx.HandleResetTodoMarkers)

	// SEARCH (Global Search e Stopwords)
	mux.Handle("POST /search", searchLimiter.Middleware(http.HandlerFunc(searchCtx.HandleSearch)))
	mux.Handle("GET /search", searchLimiter.Middleware(http.HandlerFunc(searchCtx.HandleSearch)))
	mux.HandleFunc("GET /api/stopwords", searchCtx.HandleGetStopwords)
	mux.HandleFunc("POST /api/stopwords/add", searchCtx.HandleAddStopword)
	mux.HandleFunc("DELETE /api/stopwords/remove", searchCtx.HandleRemoveStopword)

	// Static files
	fs := http.FileServer(http.Dir(sysCtx.Cfg.WebDir + "/static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))
}
