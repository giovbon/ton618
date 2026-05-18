package api

import (
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/models"
	"net/http"
	"net/http/httptest"
	"testing"

	"strings"
)

func TestPostProcessSearchHitsIsolation(t *testing.T) {
	// Preparar um Hit que tem o termo no "Texto" (corpo) mas não no título/arquivo
	hits := []models.SearchHit{
		{
			Source: models.Document{
				Arquivo: "receita-bolo.md",
				Secao:   "# Ingredientes",
				Texto:   "Use muito açúcar e hayoo para dar sabor.",
			},
		},
	}

	queryTerms := []string{"hayoo"}
	rawQuery := "hayoo"

	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	// 1. MODO COMPACTO: Agora NÃO deve filtrar (o motor de busca dita o que aparece)
	gotCompact := PostProcessSearchHits(hits, queryTerms, rawQuery, true, appState)
	if len(gotCompact) != 1 {
		t.Errorf("Modo Compacto deveria manter hits encontrados pelo motor de busca. Got %d hits", len(gotCompact))
	}

	// 2. MODO NORMAL: Deve manter o hit
	gotNormal := PostProcessSearchHits(hits, queryTerms, rawQuery, false, appState)
	if len(gotNormal) != 1 {
		t.Errorf("Modo Normal deveria ter mantido o hit. Got %d hits", len(gotNormal))
	}
}

func TestPostProcessSearchHitsPartialMatch(t *testing.T) {
	hits := []models.SearchHit{
		{
			Source: models.Document{
				Arquivo: "relatorio-hayoo-final.md",
				Secao:   "# Janeiro",
				Texto:   "Conteúdo qualquer",
			},
		},
	}

	// Busca por parte do título
	queryTerms := []string{"hayoo"}
	rawQuery := "hayoo"

	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	// MODO COMPACTO: Deve encontrar pois "hayoo" está no Arquivo
	got := PostProcessSearchHits(hits, queryTerms, rawQuery, true, appState)
	if len(got) != 1 {
		t.Error("Modo Compacto deveria ter encontrado o hit pelo nome do arquivo")
	}
}

func TestPostProcessSearchHitsHeadingMatch(t *testing.T) {
	hits := []models.SearchHit{
		{
			Source: models.Document{
				Arquivo: "notas.md",
				Secao:   "# Hayoo Setup",
				Texto:   "...",
			},
		},
	}

	queryTerms := []string{"hayoo"}
	rawQuery := "hayoo"

	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	// MODO COMPACTO: Deve encontrar pois "hayoo" está na Seção (Heading)
	got := PostProcessSearchHits(hits, queryTerms, rawQuery, true, appState)
	if len(got) != 1 {
		t.Error("Modo Compacto deveria ter encontrado o hit pelo título da seção (heading)")
	}
}

func TestHandlePing(t *testing.T) {
	ctx := &HandlerContext{}
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	w := httptest.NewRecorder()

	ctx.HandlePing(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Ping: esperado 200, obteve %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "authenticated") {
		t.Error("Resposta do Ping inválida")
	}
}

func TestHandleManualSync(t *testing.T) {
	tmpDir := t.TempDir()
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: tmpDir},
		State: appState,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", nil)
	w := httptest.NewRecorder()

	ctx.HandleManualSync(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Sync: esperado 200, obteve %d", w.Code)
	}
}

func TestBasicAuthMiddleware(t *testing.T) {
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := BasicAuthMiddleware(mockHandler, "user", "pass")

	t.Run("Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Esperado 401, obteve %d", w.Code)
		}
	})

	t.Run("Authorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.SetBasicAuth("user", "pass")
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Esperado 200, obteve %d", w.Code)
		}
	})

	t.Run("AvatarPublic", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/tags/avatar", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Avatar deve ser público: esperado 200, obteve %d", w.Code)
		}
	})

	t.Run("DocsAuthRequired", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/test.pdf", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Docs deve exigir auth: esperado 401, obteve %d", w.Code)
		}
	})
}
