package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"ton618/core/internal/core/config"
	"ton618/core/internal/features/embeddings"
)

// newPublicRoutesServer monta um roteador com apenas as rotas públicas
// (registerPublicRoutes) sobre um WEB_DIR e um MODEL_DIR temporários.
func newPublicRoutesServer(t *testing.T, withModel bool) (*httptest.Server, *embeddings.LocalModel) {
	t.Helper()

	webDir := t.TempDir()
	staticDir := filepath.Join(webDir, "static")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatalf("criar static dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "app.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatalf("escrever app.css: %v", err)
	}

	var model *embeddings.LocalModel
	if withModel {
		modelDir := t.TempDir()
		model = embeddings.NewLocalModel(modelDir)
		// Espelha o layout do disco: <ModelDir>/<repo>/<arquivo>
		file := filepath.Join(modelDir, filepath.FromSlash(model.Repo()), "config.json")
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatalf("criar dir do modelo: %v", err)
		}
		if err := os.WriteFile(file, []byte(`{"model":"fake"}`), 0o644); err != nil {
			t.Fatalf("escrever config.json: %v", err)
		}
	}

	cfg := &config.AppConfig{WebDir: webDir, AuthUser: "admin", AuthPass: "ton618"}

	r := chi.NewRouter()
	registerPublicRoutes(r, cfg, model)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, model
}

// TestPublicRoutes_ModeloSemAuth cobre a regressão: a rota /models/* (e o
// ServeHTTP do LocalModel) estava implementada e testada, mas não era montada em
// lugar nenhum do roteador — o Web Worker levava 404 e o browser baixava os
// ~120MB do CDN em toda sessão, ignorando a cópia local.
func TestPublicRoutes_ModeloSemAuth(t *testing.T) {
	srv, model := newPublicRoutesServer(t, true)

	// Sem header Authorization de propósito: é assim que o Web Worker busca.
	resp, err := http.Get(srv.URL + "/models/" + model.Repo() + "/config.json")
	if err != nil {
		t.Fatalf("GET modelo: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /models/<repo>/config.json (sem auth): esperado 200, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ler corpo: %v", err)
	}
	if !strings.Contains(string(body), "fake") {
		t.Fatalf("corpo inesperado: %q", body)
	}

	// O bundle web continua público.
	respCSS, err := http.Get(srv.URL + "/static/app.css")
	if err != nil {
		t.Fatalf("GET static: %v", err)
	}
	defer respCSS.Body.Close()
	if respCSS.StatusCode != http.StatusOK {
		t.Fatalf("GET /static/app.css: esperado 200, got %d", respCSS.StatusCode)
	}
}

// TestPublicRoutes_SemModelo cobre MODEL_DOWNLOAD=false: com o
// download desligado a rota não é montada — e não pode haver panic por handler
// nil, só 404 (o browser então usa o CDN).
func TestPublicRoutes_SemModelo(t *testing.T) {
	srv, _ := newPublicRoutesServer(t, false)

	resp, err := http.Get(srv.URL + "/models/Xenova/qualquer/config.json")
	if err != nil {
		t.Fatalf("GET modelo: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("sem modelo local: esperado 404, got %d", resp.StatusCode)
	}
}
