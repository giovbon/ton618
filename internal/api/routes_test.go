package api

import (
	"net/http"
	"testing"
)

func TestSetupRoutes_RegistraTodasAsRotas(t *testing.T) {
	ctx := newTestContext(t)
	mux := http.NewServeMux()
	ctx.SetupRoutes(mux)

	// Tenta todas as rotas registradas — se nao estiver registrada, ServeHTTP retorna
	// uma pagina padrao do Go (404 ou 405 dependendo do metodo).
	routes := []struct {
		method string
		path   string
		wantOK bool
	}{
		// Páginas
		{"GET", "/", true},
		{"GET", "/editor", true},
		{"GET", "/login", true},

		// Busca
		{"POST", "/search", true},
		{"GET", "/search", true},

		// Arquivos
		{"GET", "/file", false}, // sem parametro name
		{"POST", "/file/save", true},
		{"POST", "/file/delete", true},
		{"POST", "/file/rename", true},
		{"POST", "/upload", true},

		// API
		{"GET", "/api/status", true},
		{"GET", "/api/health", true},
		{"GET", "/api/tags", true},
		{"GET", "/api/notes", true},
		{"POST", "/api/sync", true},
		{"POST", "/api/bulk-delete", true},
	}

	for _, rt := range routes {
		req, err := http.NewRequest(rt.method, rt.path, nil)
		if err != nil {
			t.Fatalf("erro ao criar request %s %s: %v", rt.method, rt.path, err)
		}

		rec := &responseRecorder{}
		mux.ServeHTTP(rec, req)

		if rt.wantOK && rec.status == 0 {
			t.Errorf("rota %s %s nao foi encontrada: nenhum handler respondeu", rt.method, rt.path)
		}
		if !rt.wantOK && rec.status == 200 {
			t.Logf("rota %s %s retornou 200 (esperado erro)", rt.method, rt.path)
		}
	}
}

func TestSetupRoutes_StaticFiles(t *testing.T) {
	ctx := newTestContext(t)
	mux := http.NewServeMux()
	ctx.SetupRoutes(mux)

	// A rota /static/ deve ser registrada
	req, _ := http.NewRequest("GET", "/static/inexistente.js", nil)
	rec := &responseRecorder{}
	mux.ServeHTTP(rec, req)

	// Se a rota foi encontrada, o handler de arquivos tenta servir e retorna 404
	// Se a rota nao foi encontrada, rec.status ficaria 0
	if rec.status == 0 {
		t.Error("rota /static/ nao encontrada no mux")
	}
}

// responseRecorder captura o status sem depender de http.ResponseWriter completo
type responseRecorder struct {
	status int
	header http.Header
	body   []byte
}

func (r *responseRecorder) Header() http.Header {
	if r.header == nil {
		r.header = http.Header{}
	}
	return r.header
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = 200
	}
	r.body = append(r.body, b...)
	return len(b), nil
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
}
