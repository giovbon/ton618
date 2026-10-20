package main

import (
	"time"

	"github.com/go-chi/chi/v5"

	"ton618/core/internal/features/appointments"
	"ton618/core/internal/features/notes"
	"ton618/core/internal/features/search"
	"ton618/core/internal/features/system"
	"ton618/core/internal/features/todos"
	"ton618/core/internal/middleware"
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
	mux.Get("/help", sysCtx.HandleHelpPage)
	mux.Get("/login", sysCtx.HandleLogin)

	// API System
	mux.Get("/api/status", sysCtx.HandleStatus)
	mux.Get("/api/health", sysCtx.HandleHealth)
	mux.Get("/api/todos", sysCtx.HandleListTodos)
	mux.Get("/api/settings/ntfy", sysCtx.HandleGetNtfySettings)
	mux.Post("/api/settings/ntfy", sysCtx.HandlePostNtfySettings)
	mux.Get("/api/settings/agenda-notify", sysCtx.HandleGetAgendaNotifyHours)
	mux.Post("/api/settings/agenda-notify", sysCtx.HandlePostAgendaNotifyHours)
	mux.Get("/api/settings/auto-tag", sysCtx.HandleGetAutoTagSettings)
	mux.Post("/api/settings/auto-tag", sysCtx.HandlePostAutoTagSettings)
	mux.Post("/api/settings/auto-tag/apply", sysCtx.HandleApplyAutoTag)
	mux.Get("/api/settings/columns", sysCtx.HandleGetColumnsSettings)
	mux.Post("/api/settings/columns", sysCtx.HandlePostColumnsSettings)
	mux.Get("/api/settings/synonyms", sysCtx.HandleGetSynonymSettings)
	mux.Post("/api/settings/synonyms", sysCtx.HandlePostSynonymSettings)

	// NOTES (Editor e Arquivos)
	mux.Get("/editor", notesCtx.HandleEditor)
	mux.Get("/drawing", notesCtx.HandleDrawing)
	mux.Get("/mindmap", notesCtx.HandleMindmap)
	mux.Get("/tabulator", notesCtx.HandleTabulator)
	mux.Get("/semana", notesCtx.HandleWeekly)

	mux.Get("/file", notesCtx.HandleFile)
	mux.Get("/epub/reader", notesCtx.HandleEpubReader)
	mux.Get("/api/epub/position", notesCtx.HandleGetEpubPosition)
	mux.Post("/api/epub/position", notesCtx.HandlePostEpubPosition)
	mux.Get("/api/tabulator/export", notesCtx.HandleTabulatorExport)
	mux.Post("/api/tabulator/upload", notesCtx.HandleTabulatorUpload)
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
	mux.Post("/api/note/duplicate", notesCtx.HandleDuplicateNote)
	mux.Get("/api/notes", sysCtx.HandleGetAllNotes)
	mux.Get("/api/sidebar", sysCtx.HandleGetSidebar)
	mux.Get("/api/notes/database", sysCtx.HandleGetDatabaseData)
	mux.Get("/api/query", notesCtx.HandleQueryPanel)
	mux.Post("/api/notes/update-property", sysCtx.HandleUpdateNoteProperty)
	mux.Post("/api/sync", sysCtx.HandleManualSync)
	mux.Post("/api/bulk-delete", searchCtx.HandleBulkDelete)
	mux.Post("/api/bulk-archive", searchCtx.HandleBulkArchive)
	mux.Get("/api/archives", searchCtx.HandleListArchives)
	mux.Post("/api/archive/restore", searchCtx.HandleRestoreArchive)
	mux.Get("/api/backup", notesCtx.HandleBackup)
	// Backup TOTAL: exporta banco + arquivos e pode ser restaurado por /api/restore.
	mux.Get("/api/backup/total", notesCtx.HandleFullBackup)
	mux.Post("/api/restore", notesCtx.HandleRestore)

	// TODOS (Markers)
	mux.Get("/api/todo-markers", todosCtx.HandleGetTodoMarkers)
	mux.Post("/api/todo-markers/add", todosCtx.HandleAddTodoMarker)
	mux.Post("/api/todo-markers/update", todosCtx.HandleUpdateTodoMarker)
	mux.Delete("/api/todo-markers/remove", todosCtx.HandleRemoveTodoMarker)
	mux.Post("/api/todo-markers/reset", todosCtx.HandleResetTodoMarkers)
	mux.Get("/api/todos/count", todosCtx.HandleTodoCount)

	// SEARCH (Global Search e Stopwords)
	mux.With(searchLimiter.Middleware).Post("/search", searchCtx.HandleSearch)
	mux.With(searchLimiter.Middleware).Get("/search", searchCtx.HandleSearch)

	// APPOINTMENTS
	mux.Get("/agenda", appointmentsCtx.HandleAgendaPage)
	mux.Get("/api/appointments", appointmentsCtx.HandleGetAppointments)
	mux.Get("/api/appointments/tree", appointmentsCtx.HandleGetAgendaTree)
	mux.Post("/api/appointments/create", appointmentsCtx.HandleCreateAppointment)
	mux.Post("/api/appointments/update", appointmentsCtx.HandleUpdateAppointment)
	mux.Delete("/api/appointments/delete", appointmentsCtx.HandleDeleteAppointment)
	mux.Delete("/api/appointments/purge-old", appointmentsCtx.HandlePurgeOldAppointments)
}
