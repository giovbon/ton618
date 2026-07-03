package notes

import (
	"sync"

	"ton618/internal/core/config"
	"ton618/internal/core/db"
	"ton618/internal/core/services"
	"ton618/internal/watcher"
)

type dbCacheEntry struct {
	Mtime string
	Row   map[string]interface{}
}

type HandlerContext struct {
	Cfg       *config.AppConfig
	Store     *db.Store
	Watcher   *watcher.Watcher
	Notes     *NoteService
	Backup    *services.BackupService
	Capture   *CaptureService
	Typst     *TypstService
	dbCache   map[string]dbCacheEntry
	dbCacheMu sync.RWMutex
}

func NewHandlerContext(cfg *config.AppConfig, store *db.Store, w *watcher.Watcher, notes *NoteService, backup *services.BackupService, capture *CaptureService, typst *TypstService) *HandlerContext {
	return &HandlerContext{
		Cfg:     cfg,
		Store:   store,
		Watcher: w,
		Notes:   notes,
		Backup:  backup,
		Capture: capture,
		Typst:   typst,
		dbCache: make(map[string]dbCacheEntry),
	}
}
