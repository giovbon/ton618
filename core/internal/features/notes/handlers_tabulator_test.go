package notes

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"ton618/core/internal/core/config"
)

// newTabulatorUploadReq monta uma requisição multipart com o campo `field`.
func newTabulatorUploadReq(t *testing.T, field, filename string, content []byte) *http.Request {
	t.Helper()

	body := new(bytes.Buffer)
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/tabulator/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

type tabulatorUploadResponse struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func TestHandleTabulatorUpload_GravaEmDatasets(t *testing.T) {
	docs := t.TempDir()
	ctx := &HandlerContext{Cfg: &config.AppConfig{DocsDir: docs}}

	content := []byte("%PDF-1.4 conteudo de teste")
	rr := httptest.NewRecorder()
	ctx.HandleTabulatorUpload(rr, newTabulatorUploadReq(t, "file", "relatório final.pdf", content))

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	var res tabulatorUploadResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("resposta não é JSON: %v", err)
	}
	if res.Path != "datasets/relatório final.pdf" {
		t.Errorf("path = %q", res.Path)
	}
	if res.Size != int64(len(content)) {
		t.Errorf("size = %d, want %d", res.Size, len(content))
	}

	onDisk := filepath.Join(docs, "datasets", "relatório final.pdf")
	got, err := os.ReadFile(onDisk)
	if err != nil {
		t.Fatalf("arquivo não gravado em datasets/: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("conteúdo gravado difere do enviado")
	}

	// Segundo envio com o mesmo nome NÃO sobrescreve o primeiro.
	rr2 := httptest.NewRecorder()
	ctx.HandleTabulatorUpload(rr2, newTabulatorUploadReq(t, "file", "relatório final.pdf", []byte("outro")))
	var res2 tabulatorUploadResponse
	if err := json.Unmarshal(rr2.Body.Bytes(), &res2); err != nil {
		t.Fatalf("resposta não é JSON: %v", err)
	}
	if res2.Path != "datasets/relatório final-2.pdf" {
		t.Errorf("colisão de nome deveria virar -2: %q", res2.Path)
	}
	if b, _ := os.ReadFile(onDisk); !bytes.Equal(b, content) {
		t.Errorf("o arquivo original foi sobrescrito")
	}
}

func TestHandleTabulatorUpload_SanitizaNomeDoArquivo(t *testing.T) {
	docs := t.TempDir()
	ctx := &HandlerContext{Cfg: &config.AppConfig{DocsDir: docs}}

	// Nome vindo do browser com "diretório" (não pode escapar de datasets/).
	rr := httptest.NewRecorder()
	ctx.HandleTabulatorUpload(rr, newTabulatorUploadReq(t, "file", "../../etc/passwd", []byte("x")))

	if rr.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var res tabulatorUploadResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Path != "datasets/passwd" {
		t.Errorf("path = %q, esperado datasets/passwd", res.Path)
	}
	if _, err := os.Stat(filepath.Join(docs, "datasets", "passwd")); err != nil {
		t.Errorf("arquivo deveria estar em datasets/passwd: %v", err)
	}
}

func TestHandleTabulatorUpload_SemCampoFile(t *testing.T) {
	docs := t.TempDir()
	ctx := &HandlerContext{Cfg: &config.AppConfig{DocsDir: docs}}

	rr := httptest.NewRecorder()
	ctx.HandleTabulatorUpload(rr, newTabulatorUploadReq(t, "outro", "x.txt", []byte("x")))

	if rr.Code != http.StatusBadRequest {
		t.Errorf("esperado 400 sem o campo file, got %d", rr.Code)
	}
}
