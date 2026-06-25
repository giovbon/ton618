package system

import (
	"sync"

	"ton618/internal/core/config"
	"ton618/internal/core/db"
	"ton618/internal/core/services"
	"ton618/internal/features/notes"
	"ton618/internal/watcher"
)

type dbCacheEntry struct {
	Mtime string
	Row   map[string]interface{}
}

type HandlerContext struct {
	Cfg     *config.AppConfig
	Store   *db.Store
	Watcher *watcher.Watcher
	Backup  *services.BackupService
	Notes   *notes.NoteService

	dbCache   map[string]dbCacheEntry
	dbCacheMu sync.RWMutex
}

func NewHandlerContext(cfg *config.AppConfig, store *db.Store, w *watcher.Watcher, backup *services.BackupService, notes *notes.NoteService) *HandlerContext {
	return &HandlerContext{
		Cfg:     cfg,
		Store:   store,
		Watcher: w,
		Backup:  backup,
		Notes:   notes,
		dbCache: make(map[string]dbCacheEntry),
	}
}
