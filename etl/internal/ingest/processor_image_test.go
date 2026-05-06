package ingest

import (
	"etl/internal/config"
	"testing"
	"time"
)

func TestProcessImage_Structure(t *testing.T) {
	path := "test_image.png"
	filename := "attachments/test_image.png"
	modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()
	docs := ProcessImage(path, filename, modTime, appState)

	if len(docs) != 1 {
		t.Fatalf("Esperava 1 documento, recebeu %d", len(docs))
	}

	doc := docs[0]

	if doc.Tipo != "imagem" {
		t.Errorf("Tipo esperado 'imagem', recebeu '%s'", doc.Tipo)
	}

	if doc.Arquivo != filename {
		t.Errorf("Arquivo esperado '%s', recebeu '%s'", filename, doc.Arquivo)
	}

	if doc.Secao != "Anexos / Imagens" {
		t.Errorf("Secao esperada 'Anexos / Imagens', recebeu '%s'", doc.Secao)
	}

	if doc.Timestamp != "2024-01-01T12:00:00Z" {
		t.Errorf("Timestamp esperado '2024-01-01T12:00:00Z', recebeu '%s'", doc.Timestamp)
	}

	if doc.ID == "" {
		t.Error("ID não deve estar vazio")
	}
}

func TestProcessImage_NoAPIKey(t *testing.T) {
	// Limpamos a chave para simular falta de configuração
	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()
	settings := appState.GetSettings()
	oldKey := settings.GoogleVisionKey
	settings.GoogleVisionKey = ""
	appState.SetSettings(settings)

	defer func() {
		settings.GoogleVisionKey = oldKey
		appState.SetSettings(settings)
	}()

	path := "test_image.png"
	filename := "attachments/test_image.png"
	modTime := time.Now()

	docs := ProcessImage(path, filename, modTime, appState)

	if len(docs) != 1 {
		t.Fatalf("Esperava 1 documento, recebeu %d", len(docs))
	}

	if docs[0].Texto != "" {
		t.Errorf("Esperava texto vazio (sem API key), mas recebeu: %s", docs[0].Texto)
	}

	if docs[0].ID == "" {
		t.Error("ID deve ser gerado mesmo sem OCR")
	}
}
