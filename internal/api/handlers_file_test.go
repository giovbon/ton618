package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"ton618/internal/db"
)

// ── isNoteOrPdf ─────────────────────────────────────────────────

func TestIsNoteOrPdf_Notes(t *testing.T) {
	if !isNoteOrPdf("notes/foo.md") {
		t.Error("notes/ prefix deve retornar true")
	}
}

func TestIsNoteOrPdf_Pdfs(t *testing.T) {
	if !isNoteOrPdf("pdfs/doc.pdf") {
		t.Error("pdfs/ prefix deve retornar true")
	}
}

func TestIsNoteOrPdf_Attachments(t *testing.T) {
	if !isNoteOrPdf("attachments/file.zip") {
		t.Error("attachments/ prefix deve retornar true")
	}
}

func TestIsNoteOrPdf_Other(t *testing.T) {
	if isNoteOrPdf("other/file.txt") {
		t.Error("outros prefixos deve retornar false")
	}
}

func TestIsNoteOrPdf_Empty(t *testing.T) {
	if isNoteOrPdf("") {
		t.Error("vazio deve retornar false")
	}
}

// ── HandleFileDownload ──────────────────────────────────────────

func TestHandleFileDownload_FileNotFound(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/file/download?name=notes/inexistente.md", nil)

	ctx.HandleFileDownload(rec, req)

	if rec.Code != 404 {
		t.Errorf("esperado 404, got %d", rec.Code)
	}
}

func TestHandleFileDownload_NameRequired(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/file/download", nil)

	ctx.HandleFileDownload(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleFileDownload_InvalidPath(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/file/download?name=../etc/passwd", nil)

	ctx.HandleFileDownload(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleFileDownload_Success(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/test-note.md", "Hello World", "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/file/download?name=notes/test-note.md", nil)

	ctx.HandleFileDownload(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}
}

func TestHandleFileDownload_PDFInline(t *testing.T) {
	ctx := newTestContext(t)
	createMinimalPDF(t, filepath.Join(ctx.Cfg.DocsDir, "pdfs/doc.pdf"), "Test PDF")
	ctx.Store.SetFileMod("pdfs/doc.pdf", "2025-01-01T00:00:00Z")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/file/download?name=pdfs/doc.pdf", nil)

	ctx.HandleFileDownload(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/pdf" {
		t.Errorf("esperado Content-Type application/pdf, got %q", ct)
	}
	disp := rec.Header().Get("Content-Disposition")
	if !strings.HasPrefix(disp, "inline") {
		t.Errorf("esperado inline disposition, got %q", disp)
	}
}

// ── HandleFileDelete ────────────────────────────────────────────

func TestHandleFileDelete_MethodNotAllowed(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/file/delete", nil)

	ctx.HandleFileDelete(rec, req)

	if rec.Code != 405 {
		t.Errorf("esperado 405, got %d", rec.Code)
	}
}

func TestHandleFileDelete_NoFilename(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/file/delete", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleFileDelete(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleFileDelete_Success(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/delete-me.md", "Conteudo para deletar", "test")

	rec := httptest.NewRecorder()
	body := strings.NewReader("filename=delete-me.md")
	req := httptest.NewRequest("POST", "/file/delete", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleFileDelete(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}

	// Verifica que a nota foi removida do banco
	if ctx.Store.NoteExists("notes/delete-me.md") {
		t.Error("nota deveria ter sido removida do banco")
	}

	// Verifica resposta JSON
	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro ao decodificar JSON: %v", err)
	}
	if !resp["ok"] {
		t.Error("esperado ok=true na resposta")
	}
}

// ── HandleFileRename ────────────────────────────────────────────

func TestHandleFileRename_MethodNotAllowed(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/file/rename", nil)

	ctx.HandleFileRename(rec, req)

	if rec.Code != 405 {
		t.Errorf("esperado 405, got %d", rec.Code)
	}
}

func TestHandleFileRename_MissingParams(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/file/rename", strings.NewReader("old=teste.md"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleFileRename(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleFileRename_Success(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/old-name.md", "Conteudo original", "test")

	rec := httptest.NewRecorder()
	body := strings.NewReader("old=old-name.md&new=new-name.md")
	req := httptest.NewRequest("POST", "/file/rename", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleFileRename(rec, req)

	// Rename redireciona (303 See Other)
	if rec.Code != 303 {
		t.Errorf("esperado 303, got %d", rec.Code)
	}

	// Nota antiga nao deve existir no banco
	if ctx.Store.NoteExists("notes/old-name.md") {
		t.Error("nota antiga deveria ter sido renomeada no banco")
	}

	// Nota nova deve existir no banco
	if !ctx.Store.NoteExists("notes/new-name.md") {
		t.Error("nota nova deveria existir no banco")
	}
}

// ── HandleUploadAttachment ──────────────────────────────────────

func TestHandleUploadAttachment_MethodNotAllowed(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/upload-attachment", nil)

	ctx.HandleUploadAttachment(rec, req)

	if rec.Code != 405 {
		t.Errorf("esperado 405, got %d", rec.Code)
	}
}

func TestHandleUploadAttachment_Success(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()

	// Create multipart form
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("files", "test.txt")
	if err != nil {
		t.Fatalf("erro ao criar form file: %v", err)
	}
	fw.Write([]byte("conteudo do arquivo"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/upload-attachment", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	ctx.HandleUploadAttachment(rec, req)

	if rec.Code != 303 {
		t.Errorf("esperado 303 (redirect), got %d", rec.Code)
	}

	// Verifica que o ZIP foi criado
	attachDir := filepath.Join(ctx.Cfg.DocsDir, "attachments")
	entries, err := os.ReadDir(attachDir)
	if err != nil {
		t.Fatalf("erro ao ler attachDir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("nenhum attachment foi criado")
	}
}

// ── HandleUpload ────────────────────────────────────────────────

func TestHandleUpload_MethodNotAllowed(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/upload", nil)

	ctx.HandleUpload(rec, req)

	if rec.Code != 405 {
		t.Errorf("esperado 405, got %d", rec.Code)
	}
}

func TestHandleUpload_PDF(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "test.pdf")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	fw.Write([]byte("%PDF-1.4 fake pdf content"))
	w.Close()

	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	ctx.HandleUpload(rec, req)

	if rec.Code != 303 {
		t.Errorf("esperado 303 (redirect), got %d", rec.Code)
	}

	// Verifica que o PDF foi salvo em pdfs/
	pdfPath := filepath.Join(ctx.Cfg.DocsDir, "pdfs/test.pdf")
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Error("PDF deveria ter sido salvo em pdfs/")
	}
}

// ── HandleUploadImage ───────────────────────────────────────────

func TestHandleUploadImage_MethodNotAllowed(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/upload-image", nil)

	ctx.HandleUploadImage(rec, req)

	if rec.Code != 405 {
		t.Errorf("esperado 405, got %d", rec.Code)
	}
}

func TestHandleUploadImage_Success(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "photo.png")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	fw.Write([]byte("fake png content"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/upload-image", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	ctx.HandleUploadImage(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro ao decodificar JSON: %v", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		t.Errorf("esperado ok=true, got %v", resp)
	}
	if resp["filename"] == nil {
		t.Error("esperado filename na resposta")
	}
}

func TestHandleUploadImage_InvalidExtension(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "document.pdf")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	fw.Write([]byte("fake pdf"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/upload-image", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	ctx.HandleUploadImage(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro: %v", err)
	}
	if ok, _ := resp["ok"].(bool); ok {
		t.Errorf("esperado ok=false para extensao invalida, got %v", resp)
	}
}

// ── HandleCleanupImages ─────────────────────────────────────────

func TestHandleCleanupImages_MethodNotAllowed(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/cleanup-images", nil)

	ctx.HandleCleanupImages(rec, req)

	if rec.Code != 405 {
		t.Errorf("esperado 405, got %d", rec.Code)
	}
}

func TestHandleCleanupImages_RemovesOrphan(t *testing.T) {
	ctx := newTestContext(t)

	// Cria uma imagem orfa (nao referenciada em nenhum documento)
	imgName := "notes/img_123456_test.png"
	imgPath := filepath.Join(ctx.Cfg.DocsDir, imgName)
	os.MkdirAll(filepath.Dir(imgPath), 0755)
	if err := os.WriteFile(imgPath, []byte("fake png"), 0644); err != nil {
		t.Fatalf("erro ao criar imagem: %v", err)
	}
	ctx.Store.SetFileMod(imgName, "2025-01-01T00:00:00Z")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/cleanup-images", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleCleanupImages(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro: %v", err)
	}
	count, _ := resp["count"].(float64)
	if count != 1 {
		t.Errorf("esperado 1 imagem removida, got %v", resp["count"])
	}

	// Arquivo deve ter sido deletado
	if _, err := os.Stat(imgPath); !os.IsNotExist(err) {
		t.Error("imagem orfa deveria ter sido removida")
	}
}

func TestHandleCleanupImages_SkipsReferencedImage(t *testing.T) {
	ctx := newTestContext(t)

	// Cria uma imagem
	imgName := "notes/img_999999_referenced.png"
	imgPath := filepath.Join(ctx.Cfg.DocsDir, imgName)
	os.MkdirAll(filepath.Dir(imgPath), 0755)
	if err := os.WriteFile(imgPath, []byte("fake png"), 0644); err != nil {
		t.Fatalf("erro ao criar imagem: %v", err)
	}
	ctx.Store.SetFileMod(imgName, "2025-01-01T00:00:00Z")

	// Cria um documento que referencia a imagem
	ctx.Store.InsertDocument(db.Document{
		ID:        "test-doc-1",
		Tipo:      "note",
		Arquivo:   "notes/test.md",
		Secao:     "Test",
		Texto:     "This image is referenced: notes/img_999999_referenced.png",
		Timestamp: "2025-01-01T00:00:00Z",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/cleanup-images", nil)

	ctx.HandleCleanupImages(rec, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro: %v", err)
	}
	count, _ := resp["count"].(float64)
	if count != 0 {
		t.Errorf("esperado 0 imagens removidas (referenciada), got %v", count)
	}

	// Arquivo deve permanecer
	if _, err := os.Stat(imgPath); os.IsNotExist(err) {
		t.Error("imagem referenciada nao deveria ter sido removida")
	}
}

// ── Test zip file helpers ───────────────────────────────────────

func createTestZip(t *testing.T, dir string) string {
	t.Helper()
	zipPath := filepath.Join(dir, "test.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("erro ao criar zip: %v", err)
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	f1, _ := zw.Create("notes/file1.md")
	f1.Write([]byte("# File 1"))
	f2, _ := zw.Create("notes/file2.md")
	f2.Write([]byte("# File 2"))
	zw.Close()
	return zipPath
}

func TestCountFilesInZip(t *testing.T) {
	dir := t.TempDir()
	zipPath := createTestZip(t, dir)

	count := countFilesInZip(zipPath)
	if count != 2 {
		t.Errorf("esperado 2 arquivos no zip, got %d", count)
	}
}

func TestCountFilesInZip_InvalidPath(t *testing.T) {
	count := countFilesInZip("/tmp/nao-existe.zip")
	if count != 0 {
		t.Errorf("esperado 0 para zip invalido, got %d", count)
	}
}

// Helper to read archive dir
func readArchiveDir(t *testing.T, ctx *HandlerContext) []os.DirEntry {
	t.Helper()
	archiveDir := filepath.Join(ctx.Cfg.DocsDir, "archives")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("erro ao ler archives: %v", err)
	}
	return entries
}
