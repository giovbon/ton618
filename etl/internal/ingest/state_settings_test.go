package ingest

import (
	"etl/internal/config"
	"testing"
)

func TestOllamaHostConfig(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{
		StateDir:   tmpDir,
		OllamaHost: "http://default-host:11434",
	}

	appState := NewAppState(cfg)
	defer appState.Close()

	// 1. Testar fallback (deve retornar o host do config se nada for definido)
	effective := appState.GetEffectiveOllamaHost(cfg)
	if effective != cfg.OllamaHost {
		t.Errorf("Esperado fallback para %s, obteve %s", cfg.OllamaHost, effective)
	}

	// 2. Testar fallback quando o host alternativo não responde (endereço inválido/offline)
	// Usando um host obviamente inacessível para forçar o fallback
	unreachableHost := "http://192.0.2.1:11434" // endereço TEST-NET, nunca responde
	settings := appState.GetSettings()
	settings.OllamaHostActive = unreachableHost
	settings.OllamaHosts = []string{unreachableHost}
	appState.SetSettings(settings)

	effective = appState.GetEffectiveOllamaHost(cfg)
	if effective != cfg.OllamaHost {
		t.Errorf("Esperado fallback para %s quando host inacessível, obteve %s", cfg.OllamaHost, effective)
	}

	// 3. Testar override quando host ativo responde (não testável sem mock, mas cobrimos a lógica de ping ausente)
	// O test acima já valida o path de fallback via Ping falho.

	// 3. Testar limpeza de strings (SetSettings deve limpar espaços e vazios)
	settings.OllamaHosts = []string{"  http://limpo:11434  ", "", "   "}
	appState.SetSettings(settings)
	
	savedSettings := appState.GetSettings()
	if len(savedSettings.OllamaHosts) != 1 || savedSettings.OllamaHosts[0] != "http://limpo:11434" {
		t.Errorf("Limpeza de hosts falhou. Obtido: %v", savedSettings.OllamaHosts)
	}

	// 4. Testar persistência
	appState.Save(cfg)
	appState.Close()

	newAppState := NewAppState(cfg)
	defer newAppState.Close()
	newAppState.Load(cfg)

	recovered := newAppState.GetSettings()
	if recovered.OllamaHostActive != unreachableHost {
		t.Errorf("Persistência falhou. Esperado %s, obteve %s", unreachableHost, recovered.OllamaHostActive)
	}
}
