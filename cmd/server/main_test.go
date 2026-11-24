package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"ton618/core/internal/core/config"
)

// newPublicRoutesServer monta um roteador com apenas as rotas públicas
// (registerPublicRoutes) sobre um WEB_DIR temporário.
func newPublicRoutesServer(t *testing.T) *httptest.Server {
	t.Helper()

	webDir := t.TempDir()
	staticDir := filepath.Join(webDir, "static")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatalf("criar static dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "app.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatalf("escrever app.css: %v", err)
	}

	cfg := &config.AppConfig{WebDir: webDir, AuthUser: "admin", AuthPass: "ton618"}

	r := chi.NewRouter()
	registerPublicRoutes(r, cfg)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// TestPublicRoutes_StaticoSemAuth garante que o bundle web continua servido SEM
// autenticação — a página de login também depende dele.
func TestPublicRoutes_StaticoSemAuth(t *testing.T) {
	srv := newPublicRoutesServer(t)

	resp, err := http.Get(srv.URL + "/static/app.css")
	if err != nil {
		t.Fatalf("GET static: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /static/app.css: esperado 200, got %d", resp.StatusCode)
	}
}
