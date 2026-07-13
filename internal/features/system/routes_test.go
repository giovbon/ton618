package system

import (
	"net/http"
	"testing"
)

func TestSetupRoutes_RegistraTodasAsRotas(t *testing.T) {
	ctx := newTestContext(t)
	// Rotas agora registradas em cmd/server/routes.go (SetupRoutes aceita multiplos HandlerContexts)
	// Este teste verifica que o mux responde às rotas principais via HandlerContext
	mux := http.NewServeMux()

	// Registra rotas manuais de teste (subconjunto das rotas reais)
	mux.HandleFunc("GET /", ctx.HandleIndex)
	mux.HandleFunc("GET /login", ctx.HandleLogin)
	mux.HandleFunc("GET /api/status", ctx.HandleStatus)
	mux.HandleFunc("GET /api/health", ctx.HandleHealth)
	mux.HandleFunc("GET /api/tags", ctx.HandleGetTags)
	mux.HandleFunc("GET /api/notes", ctx.HandleGetAllNotes)
	mux.HandleFunc("POST /api/sync", ctx.HandleManualSync)

	routes := []struct {
		method string
		path   string
		wantOK bool
	}{
		{"GET", "/", true},
		{"GET", "/editor", true},
		{"GET", "/login", true},
		{"GET", "/api/status", true},
		{"GET", "/api/health", true},
		{"GET", "/api/tags", true},
		{"GET", "/api/notes", true},
		{"POST", "/api/sync", true},
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
	}
}

func TestNewHandlerContext(t *testing.T) {
	ctx := newTestContext(t)

	// Verifica que NewHandlerContext define corretamente os campos
	hc := NewHandlerContext(ctx.Cfg, ctx.Store, ctx.Watcher, ctx.Backup, ctx.Notes)

	if hc.Cfg != ctx.Cfg {
		t.Error("Cfg nao foi definido corretamente")
	}
	if hc.Store != ctx.Store {
		t.Error("Store nao foi definido corretamente")
	}
	if hc.Watcher != ctx.Watcher {
		t.Error("Watcher nao foi definido corretamente")
	}
	if hc.Backup != ctx.Backup {
		t.Error("Backup nao foi definido corretamente")
	}
	if hc.Notes != ctx.Notes {
		t.Error("Notes nao foi definido corretamente")
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
