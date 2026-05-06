package api

import (
	"bytes"
	"encoding/json"
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/models"
	"etl/internal/search"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleGetSettingsMasking(t *testing.T) {
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	appState.SetSettings(models.AppSettings{
		GoogleVisionKey: "chave-secreta-real",
	})

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{StateDir: "/tmp", StateFile: "/tmp/state.json"},
		State: appState,
	}

	// 2. Simular GET /api/settings
	req := httptest.NewRequest("GET", "/api/settings", nil)
	rr := httptest.NewRecorder()

	ctx.HandleGetSettings(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var gotSettings models.AppSettings
	if err := json.NewDecoder(rr.Body).Decode(&gotSettings); err != nil {
		t.Fatal(err)
	}

	// 3. Verificar se a chave está mascarada
	if gotSettings.GoogleVisionKey != KeyMask {
		t.Errorf("Key was not masked: got %v want %v", gotSettings.GoogleVisionKey, KeyMask)
	}
}

func TestHandleSaveSettingsPreservesMask(t *testing.T) {
	realKey := "chave-original-no-servidor"
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	appState.SetSettings(models.AppSettings{
		GoogleVisionKey: realKey,
	})

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{StateDir: "/tmp", StateFile: "/tmp/state.json"},
		State: appState,
	}

	// 2. Simular POST /api/settings com o valor MASCARADO
	payload := models.AppSettings{
		GoogleVisionKey: KeyMask,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/settings", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	ctx.HandleSaveSettings(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// 3. Verificar se a chave original foi mantida (não sobrescrita pela máscara)
	currentKey := ctx.State.GetSettings().GoogleVisionKey

	if currentKey != realKey {
		t.Errorf("Real key was overwritten by mask: got %v want %v", currentKey, realKey)
	}
}

func TestHandleSaveSettingsUpdatesRealKey(t *testing.T) {
	// 1. Configurar estado inicial
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	appState.SetSettings(models.AppSettings{
		GoogleVisionKey: "velha-chave",
	})

	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{StateDir: "/tmp", StateFile: "/tmp/state.json"},
		State: appState,
	}

	// 2. Simular POST /api/settings com uma NOVA chave real
	newKey := "nova-chave-real"
	payload := models.AppSettings{
		GoogleVisionKey: newKey,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/settings", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	ctx.HandleSaveSettings(rr, req)

	// 3. Verificar se a chave foi atualizada
	currentKey := ctx.State.GetSettings().GoogleVisionKey

	if currentKey != newKey {
		t.Errorf("Key was not updated: got %v want %v", currentKey, newKey)
	}
}

func TestHandleWeights(t *testing.T) {
	tempDir := t.TempDir()
	search.InitializeWeights(tempDir)
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	ctx := &HandlerContext{State: appState}

	t.Run("GET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/weights", nil)
		w := httptest.NewRecorder()
		ctx.HandleWeights(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("GET Weights: esperado 200, obteve %d", w.Code)
		}
	})

	t.Run("POST", func(t *testing.T) {
		payload := search.RankingWeights{BaseMultiplier: 5.5}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/weights", bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		ctx.HandleWeights(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("POST Weights: esperado 200, obteve %d", w.Code)
		}
		if search.GetWeights().BaseMultiplier != 5.5 {
			t.Error("Peso base não foi atualizado")
		}
	})

	t.Run("DELETE_Reset", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/weights", nil)
		w := httptest.NewRecorder()
		ctx.HandleWeights(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("DELETE Weights: esperado 200, obteve %d", w.Code)
		}
	})
}
