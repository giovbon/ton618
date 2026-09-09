package embeddings

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newFakeModelServer simula o HuggingFace para os testes (sem rede externa).
func newFakeModelServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake:" + r.URL.Path))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// reqPath monta o caminho de URL de um arquivo do modelo (como viria após o
// StripPrefix("/models/")): "<repo>/<arquivo>".
func reqPath(file string) string {
	lm := NewLocalModel("")
	return lm.Repo() + "/" + file
}

func TestLocalModel_EnsureBaixaEServe(t *testing.T) {
	srv := newFakeModelServer(t)
	dir := t.TempDir()

	m := NewLocalModel(dir)
	m.baseURL = srv.URL

	if err := m.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure falhou: %v", err)
	}

	ready, readyN, total, lastErr := m.Status()
	if !ready {
		t.Fatalf("esperado ready=true após download (readyN=%d/%d, err=%v)", readyN, total, lastErr)
	}

	// Todos os arquivos devem existir no disco
	for _, f := range modelFiles {
		full := m.localFile(f)
		info, err := os.Stat(full)
		if err != nil {
			t.Fatalf("arquivo %q não existe após Ensure: %v", f, err)
		}
		if info.IsDir() || info.Size() == 0 {
			t.Fatalf("arquivo %q inesperado (dir=%v size=%d)", f, info.IsDir(), info.Size())
		}
	}

	// Serve cada arquivo via GET. O handler é montado com http.StripPrefix("/models/")
	// no main.go, então o path aqui já chega sem o prefixo: /<repo>/<arquivo>.
	for _, f := range modelFiles {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://ton618.local/"+reqPath(f), nil)
		m.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s: esperado 200, got %d", f, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "fake:") {
			t.Fatalf("GET %s: corpo inesperado %q", f, rr.Body.String())
		}
	}

	// HEAD também deve responder 200 (usado por clientes para checar existência)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "http://ton618.local/"+reqPath(modelFiles[0]), nil)
	m.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("HEAD: esperado 200, got %d", rr.Code)
	}
}

func TestLocalModel_EnsurePreservaArquivoExistente(t *testing.T) {
	srv := newFakeModelServer(t)
	dir := t.TempDir()

	m := NewLocalModel(dir)
	m.baseURL = srv.URL

	// Simula download interrompido: 1 arquivo já baixado
	existing := modelFiles[0]
	os.MkdirAll(filepath.Dir(m.localFile(existing)), 0o755)
	if err := os.WriteFile(m.localFile(existing), []byte("ja-baixado"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := m.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure falhou: %v", err)
	}

	// O arquivo existente não deve ser re-baixado/sobrescrito
	got, _ := os.ReadFile(m.localFile(existing))
	if string(got) != "ja-baixado" {
		t.Fatalf("arquivo existente foi sobrescrito: %q", got)
	}
}

func TestLocalModel_ServeHTTP_404ParaInexistenteEDiretorio(t *testing.T) {
	dir := t.TempDir()
	m := NewLocalModel(dir)

	// Arquivo inexistente → 404
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://ton618.local/"+reqPath("nao-existe.onnx"), nil)
	m.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("arquivo inexistente: esperado 404, got %d", rr.Code)
	}

	// Diretório (raiz do repo) → 404, nunca listagem
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "http://ton618.local/"+m.Repo(), nil)
	m.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("diretório: esperado 404 (sem listagem), got %d", rr.Code)
	}
}

func TestLocalModel_ServeHTTP_RejeitaPathTraversal(t *testing.T) {
	dir := t.TempDir()
	// Cria um arquivo "segredo" fora do ModelDir para garantir que nunca é exposto
	secret := filepath.Join(dir, "..", "segredo.txt")
	if err := os.WriteFile(secret, []byte("top-secret"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	m := NewLocalModel(dir)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://ton618.local/models/x", nil)
	req.URL.Path = "/../segredo.txt"
	m.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("path traversal: esperado 404, got %d", rr.Code)
	}
}
