package main

import (
	"bytes"
	"encoding/json"
	"etl/internal/api"
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/search"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIntegrityE2E(t *testing.T) {
	// Setup de ambiente temporário
	tempDocs := t.TempDir()
	tempData := t.TempDir()

	testCfg := config.LoadConfig()
	testCfg.DocsDir = tempDocs
	testCfg.BleveIndexDir = filepath.Join(tempData, "test.bleve")
	testCfg.StateDir = t.TempDir()

	// Inicialização do motor interno para o teste
	if err := search.InitIndex(testCfg.BleveIndexDir); err != nil {
		t.Fatalf("Erro ao inicializar Bleve para teste: %v", err)
	}
	defer search.CloseIndex()

	appState := ingest.NewAppState(testCfg)
	apiCtx := &api.HandlerContext{Cfg: testCfg, State: appState}

	uniqueID := fmt.Sprintf("INTEGRITYCHECK%d", time.Now().Unix())
	filename := "notes/integrity-test.md"
	filePath := filepath.Join(tempDocs, "notes", "integrity-test.md")
	os.MkdirAll(filepath.Join(tempDocs, "notes"), 0755)
	content := fmt.Sprintf("# Teste de Integridade\n\nEste é um segredo %s", uniqueID)

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Erro ao criar arquivo de teste: %v", err)
	}

	// Executa sincronização forçada
	ingest.RunSync(testCfg, true, "test", appState)

	searchPayload := map[string]interface{}{
		"query": map[string]interface{}{
			"term": uniqueID,
		},
		"compact": false,
	}
	body, _ := json.Marshal(searchPayload)

	// Polling para busca inicial (até 5 segundos)
	var resSearch struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
		} `json:"hits"`
	}

	found := false
	for i := 0; i < 10; i++ {
		reqSearch, _ := http.NewRequest("POST", "/api/search", bytes.NewBuffer(body))
		wSearch := httptest.NewRecorder()
		apiCtx.HandleSearch(wSearch, reqSearch)

		if wSearch.Code == http.StatusOK {
			json.Unmarshal(wSearch.Body.Bytes(), &resSearch)
			if resSearch.Hits.Total.Value > 0 {
				found = true
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !found {
		t.Fatalf("Nota não foi encontrada após sincronização (ID Único: %s)", uniqueID)
	} else {
		fmt.Printf("[Integrity] Nota encontrada com sucesso: %d fragmento(s)\n", resSearch.Hits.Total.Value)
	}

	// REMOÇÃO VIA API
	reqDelete, _ := http.NewRequest("DELETE", "/api/file?name="+filename, nil)
	wDelete := httptest.NewRecorder()
	apiCtx.HandleFile(wDelete, reqDelete)

	if wDelete.Code != http.StatusOK {
		t.Errorf("Deleção via API falhou com status %d", wDelete.Code)
	}

	// Polling para verificar expurgo (até 5 segundos)
	expurged := false
	for i := 0; i < 10; i++ {
		search.ClearCache()
		reqSearch, _ := http.NewRequest("POST", "/api/search", bytes.NewBuffer(body))
		wSearchRetry := httptest.NewRecorder()
		apiCtx.HandleSearch(wSearchRetry, reqSearch)

		var resSearchFinal struct {
			Hits struct {
				Total struct {
					Value int `json:"value"`
				} `json:"total"`
			} `json:"hits"`
		}
		json.Unmarshal(wSearchRetry.Body.Bytes(), &resSearchFinal)

		if resSearchFinal.Hits.Total.Value == 0 {
			expurged = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !expurged {
		t.Errorf("FALHA DE INTEGRIDADE: A nota continua aparecendo na busca após a deleção! (%s)", uniqueID)
	} else {
		fmt.Println("[Integrity] SUCESSO: A nota foi completamente expurgada do sistema.")
	}
}
