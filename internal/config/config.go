package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AppConfig struct {
	DocsDir           string
	DBPath            string
	PollIntervalSec   time.Duration
	Port              string
	WebDir            string
	StateDir          string
	AuthUser          string
	AuthPass          string
	OllamaHost        string
	OllamaModel       string
	EmbeddingProvider string // "gemini", "ollama", "openai"
	EmbeddingAPIKey   string
	EmbeddingModel    string
	EmbeddingDim      int
	EmbeddingAll      bool
}

func Load() *AppConfig {
	cfg := &AppConfig{
		DocsDir:           getEnv("DOCS_DIR", "./docs"),
		DBPath:            getEnv("DB_PATH", "./data/ton618.db"),
		PollIntervalSec:   time.Duration(getEnvAsInt("POLL_INTERVAL_SEC", 30)) * time.Second,
		Port:              getEnv("PORT", "6180"),
		WebDir:            getEnv("WEB_DIR", "./web"),
		StateDir:          getEnv("STATE_DIR", "./data"),
		AuthUser:          getEnv("AUTH_USER", "admin"),
		AuthPass:          getEnv("AUTH_PASS", "ton618"),
		OllamaHost:        getEnv("OLLAMA_HOST", "http://localhost:11434"),
		OllamaModel:       getEnv("OLLAMA_MODEL", "nomic-embed-text"),
		EmbeddingProvider: getEnv("EMBEDDING_PROVIDER", "gemini"),
		EmbeddingAPIKey:   getEnv("EMBEDDING_API_KEY", ""),
		EmbeddingModel:   getEnv("EMBEDDING_MODEL", "gemini-embedding-2"),
		EmbeddingDim:     getEnvAsInt("EMBEDDING_DIM", 768),
		EmbeddingAll:     getEnvAsBool("EMBEDDING_ALL", false),
	}

	// Resolve caminhos relativos para absolutos (essencial no Windows)
	if absDir, err := filepath.Abs(cfg.DocsDir); err == nil {
		cfg.DocsDir = absDir
	}
	if absDB, err := filepath.Abs(cfg.DBPath); err == nil {
		cfg.DBPath = absDB
	}
	if absWeb, err := filepath.Abs(cfg.WebDir); err == nil {
		cfg.WebDir = absWeb
	}
	if absState, err := filepath.Abs(cfg.StateDir); err == nil {
		cfg.StateDir = absState
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		l := strings.ToLower(v)
		return l == "true" || l == "1" || l == "yes"
	}
	return fallback
}

func (c *AppConfig) EnsureDirs() error {
	for _, dir := range []string{c.DocsDir, c.StateDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	// Create monitored subdirectories
	for _, sub := range []string{"notes", "links", "voice"} {
		if err := os.MkdirAll(filepath.Join(c.DocsDir, sub), 0755); err != nil {
			return err
		}
	}
	return nil
}
