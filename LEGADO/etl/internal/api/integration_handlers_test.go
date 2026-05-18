package api

import (
	"etl/internal/config"
	"etl/internal/ingest"
	"net/http"
	"net/http/httptest"

	"strings"
	"testing"
)

func TestYouTubeHelpers(t *testing.T) {
	t.Run("isYouTubeURL", func(t *testing.T) {
		if !isYouTubeURL("https://www.youtube.com/watch?v=123") {
			t.Error("Deveria identificar URL do YouTube")
		}
		if isYouTubeURL("https://google.com") {
			t.Error("Não deveria identificar URL do Google como YouTube")
		}
	})

	t.Run("extractVideoID", func(t *testing.T) {
		id := extractVideoID("https://www.youtube.com/watch?v=abc-123_XYZ")
		if id != "abc-123_XYZ" {
			t.Errorf("Esperado abc-123_XYZ, obteve %s", id)
		}
		id2 := extractVideoID("https://youtu.be/shortid?t=10")
		if id2 != "shortid" {
			t.Errorf("Esperado shortid, obteve %s", id2)
		}
		id3 := extractVideoID("https://not-youtube.com/v/123")
		if id3 != "" {
			t.Error("Deveria retornar vazio para não-YouTube")
		}
		id4 := extractVideoID("invalid-url")
		if id4 != "" {
			t.Error("Deveria retornar vazio para URL inválida")
		}
	})
}

func TestHandleLink_Failures(t *testing.T) {
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{},
		State: appState,
	}

	t.Run("FailingReadability", func(t *testing.T) {
		// URL que não existe ou falha no timeout
		req := httptest.NewRequest(http.MethodPost, "/api/link", strings.NewReader(`{"url": "http://localhost:1234/ghost"}`))
		w := httptest.NewRecorder()
		ctx.HandleLink(w, req)
		if w.Code == http.StatusOK {
			t.Error("Não deveria retornar OK para URL inexistente")
		}
	})

	t.Run("FailingYouTubeID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/link", strings.NewReader(`{"url": "https://youtube.com/watch"}`))
		w := httptest.NewRecorder()
		ctx.HandleLink(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("YouTube sem ID: esperado 400, obteve %d", w.Code)
		}
	})
}

func TestHandleLink_Validation(t *testing.T) {
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{},
		State: appState,
	}

	t.Run("EmptyURL", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/link", strings.NewReader(`{"url": ""}`))
		w := httptest.NewRecorder()
		ctx.HandleLink(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("URL vazia: esperado 400, obteve %d", w.Code)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/link", strings.NewReader(`{bad json`))
		w := httptest.NewRecorder()
		ctx.HandleLink(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("JSON inválido: esperado 400, obteve %d", w.Code)
		}
	})

	t.Run("WrongMethod", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/link", nil)
		w := httptest.NewRecorder()
		ctx.HandleLink(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Método errado: esperado 405, obteve %d", w.Code)
		}
	})
}
