package ingest

import (
	"encoding/json"
	"log/slog"
	"strings"

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

// GetEffectiveOllamaHost retorna o host Ollama a ser usado.
// Se um host alternativo estiver configurado (OllamaHostActive), verifica
// se ele está acessível com um ping rápido. Caso contrário, usa o fallback
// definido no docker-compose (cfg.OllamaHost).
func (s *AppState) GetEffectiveOllamaHost(cfg *config.AppConfig) string {
	s.settingsMu.RLock()
	configuredHost := s.settings.OllamaHostActive
	s.settingsMu.RUnlock()

	if configuredHost == "" {
		return cfg.OllamaHost
	}

	// Verifica se o PC alternativo está ativo antes de usá-lo
	if err := semantic.Ping(configuredHost); err != nil {
		slog.Warn("[Ollama] Host alternativo inacessível, usando fallback do docker-compose",
			"configured", configuredHost,
			"fallback", cfg.OllamaHost,
			"reason", err.Error())
		return cfg.OllamaHost
	}

	slog.Debug("[Ollama] Host alternativo respondeu, usando-o", "host", configuredHost)
	return configuredHost
}

func (s *AppState) SetSettings(settings models.AppSettings) {
	// Limpar strings vazias ou espaços extras das URLs
	var cleanedHosts []string
	for _, h := range settings.OllamaHosts {
		h = strings.TrimSpace(h)
		if h != "" {
			cleanedHosts = append(cleanedHosts, h)
		}
	}
	settings.OllamaHosts = cleanedHosts
	settings.OllamaHostActive = strings.TrimSpace(settings.OllamaHostActive)

	s.settingsMu.Lock()
	s.settings = settings
	s.settingsMu.Unlock()

	s.db.Update(func(tx *bolt.Tx) error {
		val, _ := json.Marshal(settings)
		return tx.Bucket(bucketSettings).Put([]byte("current"), val)
	})
}
