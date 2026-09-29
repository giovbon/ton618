package embeddings

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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

// ── Retomada do download (HTTP Range) e progresso ──

// newRangeModelServer simula o HuggingFace com suporte a HTTP Range: responde 206
// + Content-Range para requisições com Range e 200 para as demais.
func newRangeModelServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		total := int64(len(body))
		rangeHdr := r.Header.Get("Range")
		if rangeHdr == "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
			return
		}

		var start int64
		if _, err := fmt.Sscanf(rangeHdr, "bytes=%d-", &start); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if start >= total {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, total-1, total))
		w.Header().Set("Content-Length", strconv.FormatInt(total-start, 10))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(body[start:]))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLocalModel_DownloadRetomaDoPartViaRange(t *testing.T) {
	const conteudo = "0123456789ABCDEFGHIJ"
	srv := newRangeModelServer(t, conteudo)
	dir := t.TempDir()

	m := NewLocalModel(dir)
	m.baseURL = srv.URL

	// Simula um download interrompido: 8 dos 20 bytes já no .part.
	dest := m.localFile(modelFiles[0])
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(dest+".part", []byte(conteudo[:8]), 0o644); err != nil {
		t.Fatalf("setup .part: %v", err)
	}

	if err := m.download(context.Background(), modelFiles[0], dest); err != nil {
		t.Fatalf("download: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ler destino: %v", err)
	}
	if string(got) != conteudo {
		t.Errorf("conteúdo retomado incorreto:\nesperado %q\ngot      %q", conteudo, got)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Error(".part deveria ter sido promovido (removido após o rename)")
	}
}

func TestLocalModel_DownloadComRangeRecusadoReinicia(t *testing.T) {
	const conteudo = "conteudo-completo-do-arquivo"
	srv := newRangeModelServer(t, conteudo)
	dir := t.TempDir()

	m := NewLocalModel(dir)
	m.baseURL = srv.URL

	dest := m.localFile(modelFiles[0])
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// .part MAIOR que o arquivo remoto → o servidor responde 416 e o .part é descartado.
	if err := os.WriteFile(dest+".part", []byte(strings.Repeat("x", len(conteudo)+10)), 0o644); err != nil {
		t.Fatalf("setup .part: %v", err)
	}

	if err := m.download(context.Background(), modelFiles[0], dest); err != nil {
		t.Fatalf("download: %v", err)
	}

	got, _ := os.ReadFile(dest)
	if string(got) != conteudo {
		t.Errorf("deveria recomeçar do zero: esperado %q, got %q", conteudo, got)
	}
}

func TestLocalModel_ServidorSemRangeReinicia(t *testing.T) {
	// O servidor fake original responde 200 sempre (ignora Range).
	srv := newFakeModelServer(t)
	dir := t.TempDir()

	m := NewLocalModel(dir)
	m.baseURL = srv.URL

	dest := m.localFile(modelFiles[0])
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(dest+".part", []byte("lixo-antigo"), 0o644); err != nil {
		t.Fatalf("setup .part: %v", err)
	}

	if err := m.download(context.Background(), modelFiles[0], dest); err != nil {
		t.Fatalf("download: %v", err)
	}

	got, _ := os.ReadFile(dest)
	// O servidor fake devolve "fake:" + o path da requisição (que inclui
	// /resolve/main/), provando que o corpo antigo do .part foi descartado.
	want := "fake:/" + m.Repo() + "/resolve/main/" + modelFiles[0]
	if string(got) != want {
		t.Errorf("Range ignorado deveria truncar e rebaixar do zero:\nesperado %q\ngot      %q", want, got)
	}
}

func TestLocalModel_DownloadTruncadoNaoPromoveArquivo(t *testing.T) {
	// Servidor declara 100 bytes mas envia 10 e fecha a conexão.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("0123456789"))
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	m := NewLocalModel(dir)
	m.baseURL = srv.URL

	dest := m.localFile(modelFiles[0])
	if err := m.download(context.Background(), modelFiles[0], dest); err == nil {
		t.Fatal("esperado erro em download truncado")
	}

	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("arquivo definitivo não deve existir após download truncado")
	}
	part, err := os.Stat(dest + ".part")
	if err != nil {
		t.Fatalf(".part deveria ser preservado para retomada: %v", err)
	}
	if part.Size() != 10 {
		t.Errorf("esperado 10 bytes preservados no .part, got %d", part.Size())
	}
}

func TestLocalModel_Progress(t *testing.T) {
	srv := newFakeModelServer(t)
	dir := t.TempDir()

	m := NewLocalModel(dir)
	m.baseURL = srv.URL

	// Antes de qualquer download: nada pronto e nenhum arquivo ativo.
	p := m.Progress()
	if p.Ready {
		t.Error("ready deveria ser false antes do download")
	}
	if p.FilesTotal != len(modelFiles) {
		t.Errorf("FilesTotal: esperado %d, got %d", len(modelFiles), p.FilesTotal)
	}
	if p.File != "" {
		t.Errorf("nenhum download ativo deveria ter File vazio, got %q", p.File)
	}

	if err := m.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	p = m.Progress()
	if !p.Ready {
		t.Error("ready deveria ser true após Ensure")
	}
	if p.FilesDone != len(modelFiles) {
		t.Errorf("FilesDone: esperado %d, got %d", len(modelFiles), p.FilesDone)
	}
	if p.File != "" || p.Bytes != 0 {
		t.Errorf("após concluir, o progresso deve estar zerado: %+v", p)
	}
}

func TestLocalModel_EnsureRemovePartOrfao(t *testing.T) {
	srv := newFakeModelServer(t)
	dir := t.TempDir()

	m := NewLocalModel(dir)
	m.baseURL = srv.URL

	// Arquivo definitivo já presente + .part órfão de uma tentativa anterior.
	dest := m.localFile(modelFiles[0])
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(dest, []byte("ja-baixado"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(dest+".part", []byte("parcial"), 0o644); err != nil {
		t.Fatalf("setup .part: %v", err)
	}

	if err := m.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Error(".part órfão deveria ser removido quando o arquivo definitivo existe")
	}
}

func TestContentRangeStart(t *testing.T) {
	cases := []struct {
		in     string
		want   int64
		wantOK bool
	}{
		{"bytes 100-499/1234", 100, true},
		{"bytes 0-9/10", 0, true},
		{"bytes */1234", 0, false},
		{"", 0, false},
		{"100-499/1234", 0, false},
	}
	for _, tc := range cases {
		got, ok := contentRangeStart(tc.in)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("contentRangeStart(%q) = (%d, %v), want (%d, %v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestHandleModelStatus_SemModeloLocal(t *testing.T) {
	ctx := newTestHandlerContext(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/embeddings/model-status", nil)
	ctx.HandleModelStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"ready":true`) {
		t.Errorf("sem modelo local o status deve ser ready=true, got %s", rr.Body.String())
	}
}

func TestHandleModelStatus_ComModelo(t *testing.T) {
	m := NewLocalModel(t.TempDir())
	ctx := newTestHandlerContext(t).WithModel(m)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/embeddings/model-status", nil)
	ctx.HandleModelStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"files_total":`+strconv.Itoa(len(modelFiles))) {
		t.Errorf("payload sem files_total esperado: %s", body)
	}
	if !strings.Contains(body, `"ready":false`) {
		t.Errorf("modelo não baixado deveria reportar ready=false: %s", body)
	}
}
