package search

import (
	"ton618/core/internal/core/config"
	"ton618/core/internal/core/db"
)

type HandlerContext struct {
	Cfg   *config.AppConfig
	Store *db.Store
}

func NewHandlerContext(cfg *config.AppConfig, store *db.Store) *HandlerContext {
	return &HandlerContext{
		Cfg:   cfg,
		Store: store,
	}
}
