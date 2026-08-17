package notes

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestHandleUploadAttachment_StoresUTCMtime garante que o anexo é registrado
// no file_mods com mtime em UTC (sufixo "Z"), consistente com o watcher.
//
// Regressão: o handler usava time.Now().Format(time.RFC3339) (formato local,
// ex: -03:00) enquanto o watcher grava UTC ("Z"). A mistura de formatos
// quebrava a ordenação por string da sidebar — anexos novos pareciam mais
// antigos que PDFs.
func TestHandleUploadAttachment_StoresUTCMtime(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()

	// Cria um multipart com um arquivo (como o frontend faz com XMLHttpRequest)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("files", "arquivo.txt")
	if err != nil {
		t.Fatalf("criar form file: %v", err)
	}
	if _, err := fw.Write([]byte("conteudo do anexo")); err != nil {
		t.Fatalf("escrever form file: %v", err)
	}
	w.Close()

	req := httptest.NewRequest("POST", "/api/upload-attachment", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	ctx.HandleUploadAttachment(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303 (redirect), got %d (%s)", rec.Code, rec.Body.String())
	}

	// Localiza o anexo criado em docs/attachments/
	attachDir := ctx.Cfg.DocsDir + "/attachments"
	entries, err := os.ReadDir(attachDir)
	if err != nil {
		t.Fatalf("ler attachments: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("nenhum attachment criado")
	}

	filename := "attachments/" + entries[0].Name()
	mtime, err := ctx.Store.GetFileMod(filename)
	if err != nil {
		t.Fatalf("GetFileMod(%q): %v", filename, err)
	}
	if mtime == "" {
		t.Fatalf("file_mods vazio para %q", filename)
	}

	// Deve estar em UTC: termina com "Z" e é parseável como RFC3339.
	if !strings.HasSuffix(mtime, "Z") {
		t.Errorf("mtime do anexo deveria estar em UTC (sufixo Z), got %q", mtime)
	}
	if _, err := time.Parse(time.RFC3339, mtime); err != nil {
		t.Errorf("mtime %q não parseia como RFC3339: %v", mtime, err)
	}
}
