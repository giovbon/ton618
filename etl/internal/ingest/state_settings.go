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
// Deprecated: use GetEmbeddingProvider.
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

// GetEmbeddingProvider retorna o provider de embeddings configurado.
// Respeita as settings salvas (provider, api key, modelo) com fallback para Ollama.
// O resultado e memoizado para preservar HTTP client e connection pooling.
func (s *AppState) GetEmbeddingProvider(cfg *config.AppConfig) semantic.EmbeddingProvider {
	s.settingsMu.RLock()
	settings := s.settings
	s.settingsMu.RUnlock()

	// Cache key leva em conta settings + config
	cacheKey := fmt.Sprintf("provider|%s|%s|%s|%s|%d",
		settings.EmbeddingProvider,
		settings.EmbeddingModel,
		settings.EmbeddingBaseURL,
		cfg.OllamaHost,
		settings.EmbeddingDimension,
	)

	s.embCacheMu.Lock()
	defer s.embCacheMu.Unlock()

	// Se o cache for valido, reusa
	if s.embCacheKey == cacheKey && s.embCacheProvider != nil {
		return s.embCacheProvider
	}

	s.embCacheProvider = semantic.NewEmbeddingProvider(&settings, cfg.OllamaHost, cfg.OllamaModel)
	s.embCacheKey = cacheKey
	return s.embCacheProvider
}
