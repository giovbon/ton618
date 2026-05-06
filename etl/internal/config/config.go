package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const AppVersion = "1.0.0"

type AppConfig struct {
	DocsDir         string
	BleveIndexDir   string
	PollIntervalSec time.Duration
	Port            string
	WebDir          string
	StateDir        string
	StateFile       string
	AuthUser        string
	AuthPass        string
	OllamaHost      string
	OllamaModel     string
	SemanticEnable  bool
}

func LoadConfig() *AppConfig {
	docsDir := getEnv("DOCS_DIR", "./docs")
	stateDir := getEnv("STATE_DIR", "./state")

	return &AppConfig{
		DocsDir:         docsDir,
		BleveIndexDir:   getEnv("BLEVE_INDEX_DIR", "./data/ton618.bleve"),
		PollIntervalSec: time.Duration(getEnvAsInt("POLL_INTERVAL_SEC", 30)) * time.Second,
		Port:            getEnv("PORT", "6180"),
		WebDir:          getEnv("WEB_DIR", "./web/dist"),
		StateDir:        stateDir,
		StateFile:       filepath.Join(stateDir, "state.json"),
		AuthUser:        getEnv("AUTH_USER", "admin"),
		AuthPass:        getEnv("AUTH_PASS", "ton618_secret"),
		OllamaHost:      getEnv("OLLAMA_HOST", "http://192.168.15.6:11434"),
		OllamaModel:     getEnv("OLLAMA_MODEL", "nomic-embed-text"),
		SemanticEnable:  getEnv("SEMANTIC_ENABLE", "true") == "true",
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		var i int
		fmt.Sscanf(value, "%d", &i)
		return i
	}
	return fallback
}
