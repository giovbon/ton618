package notes

import (
	"ton618/core/internal/core/config"
	"ton618/core/internal/core/db"
	"ton618/core/internal/core/services"
)

type HandlerContext struct {
	Cfg     *config.AppConfig
	Store   *db.Store
	Notes   *NoteService
	Backup  *services.BackupService
	Capture *CaptureService

	// BackupFull exporta/restaura o estado COMPLETO (banco + arquivos) — a única
	// opção de backup restaurável. Ver services.FullBackupService.
	BackupFull *services.FullBackupService
}

func NewHandlerContext(cfg *config.AppConfig, store *db.Store, notes *NoteService, backup *services.BackupService, capture *CaptureService) *HandlerContext {
	return &HandlerContext{
		Cfg:     cfg,
		Store:   store,
		Notes:   notes,
		Backup:  backup,
		Capture: capture,
		// Construído aqui (só precisa de store + config) para não mexer na
		// assinatura e nos chamadores de NewHandlerContext.
		BackupFull: services.NewFullBackupService(store, cfg.DBPath, cfg.DocsDir, cfg.StateDir),
	}
}
