package notes

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"ton618/internal/core/config"
	"ton618/internal/core/db"
)

func TestHandleUpload_InvalidExtension(t *testing.T) {
	cfg := &config.AppConfig{
		DocsDir: t.TempDir(),
	}
	
	ctx := &HandlerContext{
		Cfg:   cfg,
		Store: &db.Store{},
	}

	// Create multipart form with an .exe file
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "malicious.exe")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("malicious content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()

	ctx.HandleUpload(rr, req)

	// Expect 403 Forbidden
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden, got %d", rr.Code)
	}

	expectedError := "apenas arquivos PDF, EPUB ou imagens (.png, .jpg, .jpeg) são permitidos\n"
	if rr.Body.String() != expectedError {
		t.Errorf("expected body %q, got %q", expectedError, rr.Body.String())
	}
}
