package main

import (
	"time"

	"github.com/go-chi/chi/v5"

	"ton618/internal/features/appointments"
	"ton618/internal/features/notes"
	"ton618/internal/features/search"
	"ton618/internal/features/system"
	"ton618/internal/features/todos"
	"ton618/internal/middleware"
)

// SetupRoutes registra todas as rotas no chi.Router.
func SetupRoutes(mux chi.Router, sysCtx *system.HandlerContext, notesCtx *notes.HandlerContext, todosCtx *todos.HandlerContext, searchCtx *search.HandlerContext, appointmentsCtx *appointments.HandlerContext) {
	// Rate limiters para endpoints pesados
	searchLimiter := middleware.NewRateLimiter(30, time.Minute)

	// Páginas HTML (server-side rendered) - SYSTEM
	mux.Get("/", sysCtx.HandleIndex)
	mux.Get("/todos", sysCtx.HandleTodosPage)
	mux.Get("/settings", sysCtx.HandleTodoSettingsPage)
	mux.Get("/database", sysCtx.HandleDatabasePage)
	mux.Get("/help", sysCtx.HandleHelp)
	mux.Get("/login", sysCtx.HandleLogin)

	// API System
	mux.Get("/api/status", sysCtx.HandleStatus)
	mux.Get("/api/health", sysCtx.HandleHealth)
	mux.Get("/api/help/markdown", sysCtx.HandleHelpMarkdown)
	mux.Get("/api/todos", sysCtx.HandleListTodos)
	mux.Get("/api/settings/ntfy", sysCtx.HandleGetNtfySettings)
	mux.Post("/api/settings/ntfy", sysCtx.HandlePostNtfySettings)

	// NOTES (Editor e Arquivos)
	mux.Get("/editor", notesCtx.HandleEditor)
	mux.Get("/spreadsheet", notesCtx.HandleSpreadsheet)
	mux.Get("/drawing", notesCtx.HandleDrawing)
	mux.Get("/typst", notesCtx.HandleTypst)
	mux.Get("/mermaid", notesCtx.HandleMermaid)
	mux.Get("/mindmap", notesCtx.HandleMindmap)
	mux.Get("/map", notesCtx.HandleMap)

	mux.Post("/api/notes/render-typst", notesCtx.HandleTypstRender)
	mux.Get("/api/notes/download-typst-pdf", notesCtx.HandleTypstPDF)

	mux.Get("/file", notesCtx.HandleFile)
	mux.Get("/file/download", notesCtx.HandleFileDownload)
	mux.Post("/file/save", notesCtx.HandleFileSave)
	mux.Post("/api/note/save", notesCtx.HandleNoteSaveJSON)
	mux.Post("/file/delete", notesCtx.HandleFileDelete)
	mux.Post("/api/notes/delete", notesCtx.HandleFileDelete)
	mux.Post("/file/rename", notesCtx.HandleFileRename)
	mux.Post("/api/notes/rename", notesCtx.HandleFileRename)
	mux.Post("/upload", notesCtx.HandleUpload)
	mux.Post("/api/upload-image", notesCtx.HandleUploadImage)
	mux.Post("/api/cleanup-images", notesCtx.HandleCleanupImages)
	mux.Post("/api/upload-attachment", notesCtx.HandleUploadAttachment)

	mux.Post("/api/capture", notesCtx.HandleCapture)
	mux.Get("/api/tags", sysCtx.HandleGetTags)
	mux.Get("/api/keywords", sysCtx.HandleGetKeywords)
	mux.Post("/api/note/duplicate", notesCtx.HandleDuplicateNote)
	mux.Get("/api/notes", sysCtx.HandleGetAllNotes)
	mux.Get("/api/sidebar", sysCtx.HandleGetSidebar)
	mux.Get("/api/notes/database", sysCtx.HandleGetDatabaseData)
	mux.Post("/api/notes/update-property", sysCtx.HandleUpdateNoteProperty)
	mux.Post("/api/sync", sysCtx.HandleManualSync)
	mux.Post("/api/bulk-delete", searchCtx.HandleBulkDelete)
	mux.Post("/api/bulk-archive", searchCtx.HandleBulkArchive)
	mux.Get("/api/archives", searchCtx.HandleListArchives)
	mux.Post("/api/archive/restore", searchCtx.HandleRestoreArchive)
	mux.Get("/api/backup", notesCtx.HandleBackup)

	// TODOS (Markers)
	mux.Get("/api/todo-markers", todosCtx.HandleGetTodoMarkers)
	mux.Post("/api/todo-markers/add", todosCtx.HandleAddTodoMarker)
	mux.Post("/api/todo-markers/update", todosCtx.HandleUpdateTodoMarker)
	mux.Delete("/api/todo-markers/remove", todosCtx.HandleRemoveTodoMarker)
	mux.Post("/api/todo-markers/reset", todosCtx.HandleResetTodoMarkers)

	// SEARCH (Global Search e Stopwords)
	mux.With(searchLimiter.Middleware).Post("/search", searchCtx.HandleSearch)
	mux.With(searchLimiter.Middleware).Get("/search", searchCtx.HandleSearch)
	mux.Get("/api/stopwords", searchCtx.HandleGetStopwords)
	mux.Post("/api/stopwords/add", searchCtx.HandleAddStopword)
	mux.Delete("/api/stopwords/remove", searchCtx.HandleRemoveStopword)

	// APPOINTMENTS
	mux.Get("/agenda", appointmentsCtx.HandleAgendaPage)
	mux.Get("/api/appointments", appointmentsCtx.HandleGetAppointments)
	mux.Get("/api/appointments/tree", appointmentsCtx.HandleGetAgendaTree)
	mux.Post("/api/appointments/create", appointmentsCtx.HandleCreateAppointment)
	mux.Post("/api/appointments/update", appointmentsCtx.HandleUpdateAppointment)
	mux.Delete("/api/appointments/delete", appointmentsCtx.HandleDeleteAppointment)
	mux.Post("/api/appointments/purge", appointmentsCtx.HandlePurgeOldAppointments)
}
