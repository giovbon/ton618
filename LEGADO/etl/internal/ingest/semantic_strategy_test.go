package ingest

import (
	"etl/internal/config"
	"etl/internal/models"
	"etl/internal/search"
	"path/filepath"
	"testing"
)

func TestSendToEngines_SemanticStrategy(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "strategy.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()
	cfg := &config.AppConfig{SemanticEnable: true}

	// Documentos de teste
	docEmbed := models.Document{ID: "doc-embed", Tipo: "markdown", Tags: []string{"embed"}, Texto: "com embed"}
	docNormal := models.Document{ID: "doc-normal", Tipo: "markdown", Tags: []string{"ajuda"}, Texto: "normal"}
	docNoEmbed := models.Document{ID: "doc-no-embed", Tipo: "markdown", Tags: []string{"no-embed"}, Texto: "com no-embed"}
	docImg := models.Document{ID: "doc-img", Tipo: "image", Texto: "imagem"}

	docs := []models.Document{docEmbed, docNormal, docNoEmbed, docImg}

	t.Run("Whitelist Strategy (Default)", func(t *testing.T) {
		appState.settings.SemanticStrategy = "whitelist"
		appState.settings.SemanticEnable = true

		// Precisamos de uma forma de rastrear o que foi enviado para vetores.
		// Como o search.BatchAddDocumentVectors é um efeito colateral,
		// vamos verificar se ele não causou panics e focar na lógica de decisão.
		// Em um ambiente ideal, mockaríamos a search engine.

		// Para este teste, vamos apenas garantir que a função executa sem erros
		// e cobre os caminhos lógicos.
		SendToEngines(cfg, docs, docs, appState)
	})

	t.Run("Blacklist Strategy", func(t *testing.T) {
		appState.settings.SemanticStrategy = "blacklist"
		appState.settings.SemanticEnable = true

		SendToEngines(cfg, docs, docs, appState)
	})

	t.Run("Semantic Disabled", func(t *testing.T) {
		appState.settings.SemanticEnable = false
		SendToEngines(cfg, docs, docs, appState)
	})
}
