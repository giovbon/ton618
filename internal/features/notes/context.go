package notes

import (
	"ton618/internal/core/config"
	"ton618/internal/core/db"
	"ton618/internal/core/services"
)

type HandlerContext struct {
	Cfg     *config.AppConfig
	Store   *db.Store
	Notes   *NoteService
	Backup  *services.BackupService
	Capture *CaptureService
	Typst   *TypstService
}

func NewHandlerContext(cfg *config.AppConfig, store *db.Store, notes *NoteService, backup *services.BackupService, capture *CaptureService, typst *TypstService) *HandlerContext {
	return &HandlerContext{
		Cfg:     cfg,
		Store:   store,
		Notes:   notes,
		Backup:  backup,
		Capture: capture,
		Typst:   typst,
	}
}
