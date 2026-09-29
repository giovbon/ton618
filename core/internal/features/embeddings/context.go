package embeddings

import (
	"ton618/core/internal/core/config"
	"ton618/core/internal/core/db"
)

type HandlerContext struct {
	Cfg   *config.AppConfig
	Store *db.Store

	// Model é o gerenciador do modelo ONNX local (download no boot + serve em
	// /models/*). Opcional: quando nil, o endpoint de status informa ready=true
	// (nada a baixar) — usado por testes e por builds sem modelo local.
	Model *LocalModel
}

func NewHandlerContext(cfg *config.AppConfig, store *db.Store) *HandlerContext {
	return &HandlerContext{
		Cfg:   cfg,
		Store: store,
	}
}

// WithModel injeta o gerenciador do modelo local (chamado no boot).
func (ctx *HandlerContext) WithModel(m *LocalModel) *HandlerContext {
	ctx.Model = m
	return ctx
}
