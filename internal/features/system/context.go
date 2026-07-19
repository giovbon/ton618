package system

import (
	"sync"

	"ton618/internal/core/config"
	"ton618/internal/core/db"
	"ton618/internal/core/services"
	"ton618/internal/features/notes"
)

type dbCacheEntry struct {
	Mtime string
	Row   map[string]interface{}
}

type HandlerContext struct {
	Cfg    *config.AppConfig
	Store  *db.Store
	Backup *services.BackupService
	Notes  *notes.NoteService

	dbCache   map[string]dbCacheEntry
	dbCacheMu sync.RWMutex
}

func NewHandlerContext(cfg *config.AppConfig, store *db.Store, backup *services.BackupService, notes *notes.NoteService) *HandlerContext {
	return &HandlerContext{
		Cfg:     cfg,
		Store:   store,
		Backup:  backup,
		Notes:   notes,
		dbCache: make(map[string]dbCacheEntry),
	}
}
