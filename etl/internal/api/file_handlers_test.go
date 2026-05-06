package api

import (
	"bytes"
	"encoding/json"
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/search"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleGetTags(t *testing.T) {
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	appState.SetFileTags("file1.md", []string{"tag1", "tag2"})
	appState.RebuildKnownTagsCache()

	ctx := &HandlerContext{State: appState}

	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()

	ctx.HandleGetTags(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Esperado 200 OK, obteve %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	tags := resp["tags"].([]interface{})
	if len(tags) != 2 {
		t.Errorf("Esperado 2 tags, obteve %d", len(tags))
	}
}

func TestHandleGetNotes(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "notes"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "notes", "test-note.md"), []byte("content"), 0644)

	ctx := &HandlerContext{
		Cfg: &config.AppConfig{DocsDir: tmpDir},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	w := httptest.NewRecorder()

	ctx.HandleGetNotes(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Esperado 200 OK, obteve %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	notes := resp["notes"].([]interface{})
	if len(notes) != 1 {
		t.Errorf("Esperado 1 nota, obteve %d", len(notes))
	}
	if notes[0].(string) != "test-note" {
		t.Errorf("Esperado nome 'test-note', obteve %s", notes[0])
	}
}

func TestHandleFile(t *testing.T) {
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "file.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: tmpDir},
		State: appState,
	}

	noteRelPath := "notes/test.md"
	notePath := filepath.Join(tmpDir, noteRelPath)
	os.MkdirAll(filepath.Dir(notePath), 0755)
	os.WriteFile(notePath, []byte("# Hello"), 0644)

	t.Run("GET_Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/file?name="+noteRelPath, nil)
		w := httptest.NewRecorder()
		ctx.HandleFile(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("GET: esperado 200, obteve %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "# Hello") {
			t.Error("Corpo do arquivo não encontrado no GET")
		}
	})

	t.Run("GET_Fallback", func(t *testing.T) {
		// Criar arquivo em subdiretório 'notes' mas pedir sem o prefixo
		os.MkdirAll(filepath.Join(tmpDir, "notes"), 0755)
		os.WriteFile(filepath.Join(tmpDir, "notes", "fallback.md"), []byte("fallback content"), 0644)

		req := httptest.NewRequest(http.MethodGet, "/api/file?name=fallback.md", nil)
		w := httptest.NewRecorder()
		ctx.HandleFile(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Fallback: esperado 200, obteve %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "fallback content") {
			t.Error("Conteúdo do fallback não encontrado")
		}
	})

	t.Run("GET_NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/file?name=ghost.md", nil)
		w := httptest.NewRecorder()
		ctx.HandleFile(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("GET inexistente: esperado 404, obteve %d", w.Code)
		}
	})

	t.Run("POST_Save", func(t *testing.T) {
		payload := `{"content": "# New Content"}`
		req := httptest.NewRequest(http.MethodPost, "/api/file?name="+noteRelPath, strings.NewReader(payload))
		w := httptest.NewRecorder()
		ctx.HandleFile(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("POST: esperado 200, obteve %d", w.Code)
		}

		// Validar disco
		content, _ := os.ReadFile(notePath)
		if !strings.Contains(string(content), "# New Content") {
			t.Error("Conteúdo não foi salvo no disco")
		}
	})

	t.Run("DELETE_Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/file?name="+noteRelPath, nil)
		w := httptest.NewRecorder()
		ctx.HandleFile(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("DELETE: esperado 200, obteve %d", w.Code)
		}

		// Validar disco
		if _, err := os.Stat(notePath); !os.IsNotExist(err) {
			t.Error("Arquivo ainda existe no disco após DELETE")
		}
	})

	t.Run("ForbiddenTraversal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/file?name=../../etc/passwd", nil)
		w := httptest.NewRecorder()
		ctx.HandleFile(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("Traversal: esperado 403, obteve %d", w.Code)
		}
	})

	t.Run("POST_InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/file?name=test.md", strings.NewReader(`{bad json`))
		w := httptest.NewRecorder()
		ctx.HandleFile(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("POST JSON inválido: esperado 400, obteve %d", w.Code)
		}
	})

	t.Run("DELETE_Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/file?name=secret.sh", nil)
		w := httptest.NewRecorder()
		ctx.HandleFile(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("DELETE proibido: esperado 403, obteve %d", w.Code)
		}
	})
}

func TestHandleRename(t *testing.T) {
	tmpDir := t.TempDir()
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: tmpDir},
		State: appState,
	}

	from := "old.md"
	to := "new.md"
	os.WriteFile(filepath.Join(tmpDir, from), []byte("content"), 0644)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rename?from=%s&to=%s", from, to), nil)
	w := httptest.NewRecorder()

	ctx.HandleRename(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Rename: esperado 200, obteve %d", w.Code)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, to)); err != nil {
		t.Error("Arquivo novo não existe")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, from)); !os.IsNotExist(err) {
		t.Error("Arquivo antigo ainda existe")
	}
}

func TestHandleRename_SamePath(t *testing.T) {
	tmpDir := t.TempDir()
	from := "same.md"
	os.WriteFile(filepath.Join(tmpDir, from), []byte("content"), 0644)
	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: tmpDir},
		State: ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()}),
	}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rename?from=%s&to=%s", from, from), nil)
	w := httptest.NewRecorder()
	ctx.HandleRename(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Rename mesmo caminho: esperado 200, obteve %d", w.Code)
	}
}

func TestHandleRename_Traversal(t *testing.T) {
	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: "/tmp"},
		State: ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()}),
	}
	req := httptest.NewRequest(http.MethodPut, "/api/rename?from=old.md&to=../../etc/passwd", nil)
	w := httptest.NewRecorder()
	ctx.HandleRename(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("Rename traversal: esperado 403, obteve %d", w.Code)
	}
}

func TestHandleRename_Forbidden(t *testing.T) {
	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: "/tmp"},
		State: ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()}),
	}
	req := httptest.NewRequest(http.MethodPut, "/api/rename?from=old.md&to=new.sh", nil)
	w := httptest.NewRecorder()
	ctx.HandleRename(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("Rename proibido: esperado 403, obteve %d", w.Code)
	}
}

func TestHandleUpload(t *testing.T) {
	tmpDir := t.TempDir()
	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	ctx := &HandlerContext{
		Cfg:   &config.AppConfig{DocsDir: tmpDir},
		State: appState,
	}

	t.Run("UploadPDF", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.pdf")
		part.Write([]byte("%PDF-1.4 dummy content"))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		ctx.HandleUpload(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Upload PDF: esperado 200, obteve %d", w.Code)
		}
		if _, err := os.Stat(filepath.Join(tmpDir, "pdfs", "test.pdf")); err != nil {
			t.Error("Arquivo PDF não foi salvo")
		}
	})

	t.Run("UploadImage", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.jpg")
		part.Write([]byte("fake image content"))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		ctx.HandleUpload(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Upload Image: esperado 200, obteve %d", w.Code)
		}
		if _, err := os.Stat(filepath.Join(tmpDir, "attachments", "test.jpg")); err != nil {
			t.Error("Arquivo de imagem não foi salvo em attachments")
		}
	})

	t.Run("UploadForbidden", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "script.sh")
		part.Write([]byte("echo hi"))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		ctx.HandleUpload(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Upload .sh: esperado 403, obteve %d", w.Code)
		}
	})
}
