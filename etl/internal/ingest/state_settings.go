package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"etl/internal/config"
	"etl/internal/models"
	"etl/internal/semantic"

	bolt "go.etcd.io/bbolt"
)

// Settings

func (s *AppState) GetSettings() models.AppSettings {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	return s.settings
}

func (s *AppState) SetSettings(settings models.AppSettings) {
	s.settingsMu.Lock()
	s.settings = settings
	s.settingsMu.Unlock()

	s.db.Update(func(tx *bolt.Tx) error {
		val, _ := json.Marshal(settings)
		return tx.Bucket(bucketSettings).Put([]byte("current"), val)
	})
}

// IsSemanticEnabled retorna true se busca semantica deve funcionar.
func (s *AppState) IsSemanticEnabled() bool {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	return s.settings.SemanticEnable
}

// GetEmbeddingFunc retorna a funcao de embedding Ollama memoizada.
// A closure e recriada apenas quando modelo, host ou dimensao mudam,
// preservando o HTTP client e connection pooling entre chamadas.
func (s *AppState) GetEmbeddingFunc(cfg *config.AppConfig) func(context.Context, string) ([]float32, error) {
	s.settingsMu.RLock()
	settings := s.settings
	s.settingsMu.RUnlock()

	dimension := settings.EmbeddingDimension
	if dimension <= 0 {
		dimension = 512
	}

	cacheKey := fmt.Sprintf("%s@%s:%d", cfg.OllamaModel, cfg.OllamaHost, dimension)

	s.embCacheMu.Lock()
	defer s.embCacheMu.Unlock()

	if s.embCacheKey == cacheKey && s.embCacheFunc != nil {
		return s.embCacheFunc
	}

	s.embCacheKey = cacheKey
	s.embCacheFunc = semantic.NewOllamaEmbedding(cfg.OllamaModel, cfg.OllamaHost, dimension)
	slog.Info("[Embedding] Funcao de embedding criada/cache atualizado",
		"model", cfg.OllamaModel, "host", cfg.OllamaHost, "dimension", dimension)
	return s.embCacheFunc
}
