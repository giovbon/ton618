package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── buildContextSnippet ─────────────────────────────────────────

func TestBuildContextSnippet_EmptyText(t *testing.T) {
	result := buildContextSnippet("test", "")
	if result != "..." {
		t.Errorf("esperado '...', got %q", result)
	}
}

func TestBuildContextSnippet_NoMatch(t *testing.T) {
	text := "This is a long text about something else entirely."
	result := buildContextSnippet("nonexistent", text)
	if !strings.HasPrefix(result, "This is") {
		t.Errorf("esperado prefixo do texto original, got %q", result)
	}
}

func TestBuildContextSnippet_BasicMatch(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog."
	result := buildContextSnippet("fox", text)
	if !strings.Contains(result, "fox") {
		t.Errorf("snippet deve conter o termo buscado, got %q", result)
	}
}

func TestBuildContextSnippet_MultipleTerms(t *testing.T) {
	text := "Go is a statically typed compiled programming language designed at Google."
	result := buildContextSnippet("Go Google", text)
	if !strings.Contains(result, "Go") || !strings.Contains(result, "Google") {
		t.Errorf("snippet deve conter ambos os termos, got %q", result)
	}
}

func TestBuildContextSnippet_ExactPhrase(t *testing.T) {
	text := "The Go programming language is known for its simplicity and concurrency support."
	result := buildContextSnippet(`"programming language"`, text)
	if !strings.Contains(result, "programming language") {
		t.Errorf("snippet deve conter a frase exata, got %q", result)
	}
}

func TestBuildContextSnippet_LongTextTruncation(t *testing.T) {
	text := strings.Repeat("word ", 200)
	result := buildContextSnippet("nothing", text)
	if len(result) > 260 {
		t.Errorf("texto longo sem match deve ser truncado para ~250 chars, got %d: %q", len(result), result)
	}
}

func TestBuildContextSnippet_IgnoresTagsAndOperators(t *testing.T) {
	text := "Some text with a tag and operator filter."
	result := buildContextSnippet("-exclude #tag +tags:something", text)
	if !strings.Contains(result, "text") {
		t.Errorf("deve ignorar operadores e mostrar o texto, got %q", result)
	}
}

func TestBuildContextSnippet_FarApartTerms(t *testing.T) {
	text := "The first part of a very long document that discusses various topics. " +
		strings.Repeat("padding ", 50) +
		"The second part talks about Go and concurrency."
	result := buildContextSnippet("first Go", text)
	if !strings.Contains(result, "first") || !strings.Contains(result, "Go") {
		t.Errorf("snippet deve conter ambos os termos separados, got %q", result)
	}
}

// ── HandleBulkArchive ───────────────────────────────────────────

func TestHandleBulkArchive_NoFiles(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/bulk-archive", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleBulkArchive(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleBulkArchive_Success(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/archive-me-1.md", "# Nota 1", "teste")
	saveTestNote(t, ctx, "notes/archive-me-2.md", "# Nota 2", "teste")

	rec := httptest.NewRecorder()
	body := strings.NewReader("files=notes/archive-me-1.md&files=notes/archive-me-2.md")
	req := httptest.NewRequest("POST", "/api/bulk-archive", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleBulkArchive(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro: %v", err)
	}
	ok, _ := resp["ok"].(bool)
	if !ok {
		t.Errorf("esperado ok=true, got %v", resp)
	}
	archived, _ := resp["archived"].(float64)
	if archived != 2 {
		t.Errorf("esperado 2 arquivos arquivados, got %v", archived)
	}

	// Arquivos originais removidos
	for _, f := range []string{"notes/archive-me-1.md", "notes/archive-me-2.md"} {
		fullPath := filepath.Join(ctx.Cfg.DocsDir, f)
		if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
			t.Errorf("arquivo %s deveria ter sido removido", f)
		}
	}
}

// ── HandleBulkDelete ────────────────────────────────────────────

func TestHandleBulkDelete_NoFilter(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/bulk-delete", nil)

	ctx.HandleBulkDelete(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleBulkDelete_ByTag(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/delete-tag-1.md", "# Nota 1", "cleanup")
	saveTestNote(t, ctx, "notes/delete-tag-2.md", "# Nota 2", "cleanup")
	saveTestNote(t, ctx, "notes/keep-me.md", "# Nota 3", "keep")

	rec := httptest.NewRecorder()
	body := strings.NewReader("by_tag=true&tag_name=cleanup")
	req := httptest.NewRequest("POST", "/api/bulk-delete", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleBulkDelete(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro: %v", err)
	}
	deleted, _ := resp["deleted"].(float64)
	if deleted != 2 {
		t.Errorf("esperado 2 notas deletadas, got %v", deleted)
	}

	// Nota com tag "keep" deve permanecer
	keepPath := filepath.Join(ctx.Cfg.DocsDir, "notes/keep-me.md")
	if _, err := os.Stat(keepPath); os.IsNotExist(err) {
		t.Error("nota 'keep-me' nao deveria ter sido deletada")
	}
}

func TestHandleBulkDelete_ByTagPreview(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/preview-test.md", "# Preview", "preview-tag")

	rec := httptest.NewRecorder()
	body := strings.NewReader("by_tag=true&tag_name=preview-tag&preview=true")
	req := httptest.NewRequest("POST", "/api/bulk-delete", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleBulkDelete(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro: %v", err)
	}
	total, _ := resp["total"].(float64)
	if total != 1 {
		t.Errorf("esperado 1 no preview, got %v", total)
	}

	// Arquivo nao deve ser deletado (preview)
	previewPath := filepath.Join(ctx.Cfg.DocsDir, "notes/preview-test.md")
	if _, err := os.Stat(previewPath); os.IsNotExist(err) {
		t.Error("preview nao deveria deletar o arquivo")
	}
}

func TestHandleBulkDelete_ExplicitFiles(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/exp-1.md", "# Explicit 1", "")
	saveTestNote(t, ctx, "notes/exp-2.md", "# Explicit 2", "")

	rec := httptest.NewRecorder()
	body := strings.NewReader("files=notes/exp-1.md&files=notes/exp-2.md")
	req := httptest.NewRequest("POST", "/api/bulk-delete", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleBulkDelete(rec, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro: %v", err)
	}
	deleted, _ := resp["deleted"].(float64)
	if deleted != 2 {
		t.Errorf("esperado 2, got %v", deleted)
	}
}

// ── HandleListArchives ──────────────────────────────────────────

func TestHandleListArchives_Empty(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/archives", nil)

	ctx.HandleListArchives(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro: %v", err)
	}
	archives, _ := resp["archives"].([]interface{})
	if len(archives) != 0 {
		t.Errorf("esperado lista vazia, got %d archives", len(archives))
	}
}

func TestHandleListArchives_WithArchives(t *testing.T) {
	ctx := newTestContext(t)

	// Cria um archive manualmente
	archiveDir := filepath.Join(ctx.Cfg.DocsDir, "archives")
	os.MkdirAll(archiveDir, 0755)

	// Cria um ZIP de teste
	zipPath := filepath.Join(archiveDir, "test-archive.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	zw := zip.NewWriter(zf)
	f1, _ := zw.Create("notes/file.md")
	f1.Write([]byte("# content"))
	zw.Close()
	zf.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/archives", nil)

	ctx.HandleListArchives(rec, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro: %v", err)
	}
	archives, _ := resp["archives"].([]interface{})
	if len(archives) != 1 {
		t.Errorf("esperado 1 archive, got %d", len(archives))
	}
}

// ── HandleRestoreArchive ────────────────────────────────────────

func TestHandleRestoreArchive_MissingName(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/archive/restore", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleRestoreArchive(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleRestoreArchive_PathTraversal(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader("archive=../../etc/passwd")
	req := httptest.NewRequest("POST", "/api/archive/restore", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleRestoreArchive(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleRestoreArchive_Success(t *testing.T) {
	ctx := newTestContext(t)

	// Cria notes para arquivar
	saveTestNote(t, ctx, "notes/restore-test.md", "# Restore Me", "test")

	// Arquiva
	rec1 := httptest.NewRecorder()
	body1 := strings.NewReader("files=notes/restore-test.md")
	req1 := httptest.NewRequest("POST", "/api/bulk-archive", body1)
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx.HandleBulkArchive(rec1, req1)

	var archiveResp map[string]interface{}
	json.NewDecoder(rec1.Body).Decode(&archiveResp)
	archiveName, _ := archiveResp["archive"].(string)
	if archiveName == "" {
		t.Fatal("archive name nao pode ser vazio")
	}

	// Arquivo original removido
	if _, err := os.Stat(filepath.Join(ctx.Cfg.DocsDir, "notes/restore-test.md")); !os.IsNotExist(err) {
		t.Error("arquivo original deveria ter sido removido")
	}

	// Restaura
	rec2 := httptest.NewRecorder()
	body2 := strings.NewReader(fmt.Sprintf("archive=%s", archiveName))
	req2 := httptest.NewRequest("POST", "/api/archive/restore", body2)
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx.HandleRestoreArchive(rec2, req2)

	if rec2.Code != 200 {
		t.Errorf("esperado 200, got %d", rec2.Code)
	}

	var restoreResp map[string]interface{}
	if err := json.NewDecoder(rec2.Body).Decode(&restoreResp); err != nil {
		t.Fatalf("erro: %v", err)
	}
	ok, _ := restoreResp["ok"].(bool)
	if !ok {
		t.Errorf("esperado ok=true, got %v", restoreResp)
	}
	restored, _ := restoreResp["restored"].(float64)
	if restored != 1 {
		t.Errorf("esperado 1 arquivo restaurado, got %v", restored)
	}

	// Arquivo restaurado
	restoredPath := filepath.Join(ctx.Cfg.DocsDir, "notes/restore-test.md")
	if _, err := os.Stat(restoredPath); os.IsNotExist(err) {
		t.Error("arquivo deveria ter sido restaurado")
	}
}

// ── HandleMergeNotes ────────────────────────────────────────────

func TestHandleMergeNotes_LessThanTwoFiles(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader("files=notes/single.md")
	req := httptest.NewRequest("POST", "/api/merge-notes", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleMergeNotes(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleMergeNotes_Success(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/merge-a.md", "# Nota A\n\nConteudo da nota A", "merge")
	saveTestNote(t, ctx, "notes/merge-b.md", "# Nota B\n\nConteudo da nota B", "merge")

	rec := httptest.NewRecorder()
	body := strings.NewReader("files=notes/merge-a.md&files=notes/merge-b.md")
	req := httptest.NewRequest("POST", "/api/merge-notes", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleMergeNotes(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro: %v", err)
	}
	ok, _ := resp["ok"].(bool)
	if !ok {
		t.Errorf("esperado ok=true, got %v", resp)
	}
	deleted, _ := resp["deleted"].(float64)
	if deleted != 2 {
		t.Errorf("esperado 2 notas deletadas, got %v", deleted)
	}

	filename, _ := resp["filename"].(string)
	if filename == "" {
		t.Error("esperado filename na resposta")
	}
	if !strings.HasPrefix(filename, "notes/mesclado-") {
		t.Errorf("esperado prefixo 'notes/mesclado-', got %q", filename)
	}

	// Notas originais deletadas
	for _, f := range []string{"notes/merge-a.md", "notes/merge-b.md"} {
		fullPath := filepath.Join(ctx.Cfg.DocsDir, f)
		if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
			t.Errorf("arquivo %s deveria ter sido deletado", f)
		}
	}

	// Nota mesclada deve existir e conter ambos os conteudos
	mergedPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	data, err := os.ReadFile(mergedPath)
	if err != nil {
		t.Fatalf("erro ao ler nota mesclada: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Nota A") || !strings.Contains(content, "Nota B") {
		t.Errorf("nota mesclada deve conter ambas as notas, got %q", content)
	}
}
