package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	// Limpar variáveis de ambiente que podem afetar o teste
	os.Unsetenv("DOCS_DIR")
	os.Unsetenv("STATE_DIR")
	os.Unsetenv("POLL_INTERVAL_SEC")

	cfg := LoadConfig()

	if cfg.DocsDir != "./docs" {
		t.Errorf("Esperado DocsDir ./docs, obteve %s", cfg.DocsDir)
	}
	if cfg.BleveIndexDir != "./data/ton618.bleve" {
		t.Errorf("Esperado BleveIndexDir ./data/ton618.bleve, obteve %s", cfg.BleveIndexDir)
	}
	if cfg.PollIntervalSec != 30*time.Second {
		t.Errorf("Esperado PollInterval 30s, obteve %v", cfg.PollIntervalSec)
	}
}

func TestLoadConfigOverride(t *testing.T) {
	os.Setenv("DOCS_DIR", "/tmp/pkm-docs")
	os.Setenv("POLL_INTERVAL_SEC", "60")
	defer os.Unsetenv("DOCS_DIR")
	defer os.Unsetenv("POLL_INTERVAL_SEC")

	cfg := LoadConfig()

	if cfg.DocsDir != "/tmp/pkm-docs" {
		t.Errorf("Esperado DocsDir /tmp/pkm-docs, obteve %s", cfg.DocsDir)
	}
	if cfg.PollIntervalSec != 60*time.Second {
		t.Errorf("Esperado PollInterval 60s, obteve %v", cfg.PollIntervalSec)
	}
}

func TestStateFileConstruction(t *testing.T) {
	os.Setenv("STATE_DIR", "/custom/state")
	defer os.Unsetenv("STATE_DIR")

	cfg := LoadConfig()

	expectedStateFile := "/custom/state/state.json"
	if cfg.StateFile != expectedStateFile {
		t.Errorf("Esperado StateFile %s, obteve %s", expectedStateFile, cfg.StateFile)
	}
}
