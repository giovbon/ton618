package ingest

import (
	"etl/internal/config"
	"path/filepath"
	"testing"
)

func TestStatePersistence(t *testing.T) {
	// 1. Setup
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.json")
	cfg := &config.AppConfig{
		StateDir:  tmpDir,
		StateFile: stateFile,
	}

	appState := NewAppState(cfg)

	// 2. Modificar estado inicial
	appState.SetHash("file1", "hash123")
	appState.SetFileTags("file1", []string{"tag-teste"})

	settings := appState.GetSettings()
	settings.SemanticEnable = false // Desligar IA para o teste
	settings.Language = "en-US"
	appState.SetSettings(settings)

	// 3. Salvar
	appState.Save(cfg)
	appState.Close()

	// 4. Criar NOVO AppState e carregar
	newAppState := NewAppState(cfg)
	defer newAppState.Close()
	newAppState.Load(cfg)

	// 5. Validar se os dados sobreviveram à "viagem"
	hash, _ := newAppState.GetHash("file1")
	if hash != "hash123" {
		t.Errorf("Hash não recuperado corretamente. Esperado hash123, obteve %s", hash)
	}

	recoveredSettings := newAppState.GetSettings()
	if recoveredSettings.SemanticEnable != false {
		t.Error("Configuração SemanticEnable não sobreviveu ao Load()")
	}
	if recoveredSettings.Language != "en-US" {
		t.Errorf("Language incorreta. Esperado 'en-US', obteve %s", recoveredSettings.Language)
	}

	tags := newAppState.GetFileTags("file1")
	if len(tags) == 0 || tags[0] != "tag-teste" {
		t.Error("Tags não foram recuperadas corretamente")
	}
}

func TestMetadataPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{
		StateDir: tmpDir,
	}

	appState := NewAppState(cfg)
	defer appState.Close()

	meta := map[string]interface{}{
		"status":   "done",
		"priority": 10.0,
		"tags":     []interface{}{"a", "b"},
	}

	appState.SetFileMetadata("test.md", meta)

	// Simula reabertura do banco
	appState.Close()
	newAppState := NewAppState(cfg)
	defer newAppState.Close()
	newAppState.Load(cfg)

	recovered := newAppState.GetFileMetadata("test.md")
	if recovered == nil {
		t.Fatal("Metadados não foram recuperados")
	}

	if recovered["status"] != "done" {
		t.Errorf("Esperado status 'done', obteve '%v'", recovered["status"])
	}

	if recovered["priority"] != 10.0 {
		t.Errorf("Esperado prioridade 10.0, obteve '%v'", recovered["priority"])
	}
}
