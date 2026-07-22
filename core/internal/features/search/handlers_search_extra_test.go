package search

import (
	"archive/zip"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/db"
)

func TestExtractSearchTerms(t *testing.T) {
	tests := []struct {
		query    string
		expected []string
	}{
		{"C++", []string{"C++"}},
		{"C#", []string{"C#"}},
		{".NET", []string{".NET"}},
		{"\"programming language\" C++", []string{"programming language", "C++"}},
		{"-exclude #tag +tags:something C++", []string{"C++"}},
		{"a", nil}, // single character terms should be ignored
	}

	for _, tc := range tests {
		result := extractSearchTerms(tc.query)
		if len(result) != len(tc.expected) {
			t.Errorf("para %q, esperado %v, got %v", tc.query, tc.expected, result)
			continue
		}
		for i, v := range result {
			if v != tc.expected[i] {
				t.Errorf("para %q, no índice %d esperado %q, got %q", tc.query, i, tc.expected[i], v)
			}
		}
	}
}

func TestFindQueryLineInText_SpecialCharactersAndMultipleTerms(t *testing.T) {
	noteContent := `---
title: Test Note
---
This is a test note.
Let's learn C++ coding on line 5.
We also write C# on line 6.
And web apps with .NET on line 7.`

	// 1. Single term with special character
	line := findQueryLineInText(noteContent, "C++")
	if line != 5 {
		t.Errorf("esperado linha 5 para 'C++', got %d", line)
	}

	lineCsharp := findQueryLineInText(noteContent, "C#")
	if lineCsharp != 6 {
		t.Errorf("esperado linha 6 para 'C#', got %d", lineCsharp)
	}

	lineDotnet := findQueryLineInText(noteContent, ".NET")
	if lineDotnet != 7 {
		t.Errorf("esperado linha 7 para '.NET', got %d", lineDotnet)
	}

	// 2. Multiple terms (some not on same line, should fallback to first term line)
	lineMulti := findQueryLineInText(noteContent, "C++ Java")
	if lineMulti != 5 {
		t.Errorf("esperado linha 5 para 'C++ Java' (fallback para primeiro termo), got %d", lineMulti)
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

	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "2 notas arquivadas com sucesso") {
		t.Errorf("esperado mensagem de sucesso para 2 notas arquivadas, got %q", bodyStr)
	}

	// Notas removidas do banco
	for _, f := range []string{"notes/archive-me-1.md", "notes/archive-me-2.md"} {
		if ctx.Store.NoteExists(f) {
			t.Errorf("nota %s deveria ter sido removida do banco", f)
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

	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "2 notas excluídas permanentemente") {
		t.Errorf("esperado mensagem de sucesso para 2 notas excluídas, got %q", bodyStr)
	}

	// Nota com tag "keep" deve permanecer no banco
	if !ctx.Store.NoteExists("notes/keep-me.md") {
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

	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "Notas encontradas (1)") {
		t.Errorf("esperado preview com 1 nota encontrada, got %q", bodyStr)
	}

	// Nota nao deve ser deletada (preview) - verifica no banco
	if !ctx.Store.NoteExists("notes/preview-test.md") {
		t.Error("preview nao deveria deletar a nota do banco")
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

	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "2 notas excluídas permanentemente") {
		t.Errorf("esperado mensagem de sucesso para 2 notas excluídas, got %q", bodyStr)
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

	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "Nenhum arquivo morto encontrado") {
		t.Errorf("esperado mensagem de nenhum arquivo morto, got %q", bodyStr)
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

	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "test-archive.zip") {
		t.Errorf("esperado conter test-archive.zip, got %q", bodyStr)
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

	bodyStr1 := rec1.Body.String()
	// Encontra o nome do zip na string do body, formato usual: "(id_cuid2.zip)"
	startIdx := strings.Index(bodyStr1, "(")
	endIdx := strings.Index(bodyStr1, ")")
	var archiveName string
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		archiveName = bodyStr1[startIdx+1 : endIdx]
	}
	if archiveName == "" {
		t.Fatal("archive name nao pode ser vazio, body: " + bodyStr1)
	}

	// Nota removida do banco apos arquivar
	if ctx.Store.NoteExists("notes/restore-test.md") {
		t.Error("nota original deveria ter sido removida do banco")
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

	bodyStr2 := rec2.Body.String()
	if !strings.Contains(bodyStr2, "1 notas restauradas com sucesso") {
		t.Errorf("esperado mensagem contendo '1 notas restauradas com sucesso', got %q", bodyStr2)
	}

	// Arquivo restaurado no banco
	if !ctx.Store.NoteExists("notes/restore-test.md") {
		t.Error("nota deveria ter sido restaurada no banco")
	}
}

func TestHandleSearch_IntegrationSpecialCharacters(t *testing.T) {
	ctx := newTestContext(t)
	
	// 1. Create and save note
	noteContent := "Let's learn and code in C++ today. It is a powerful language."
	saveTestNote(t, ctx, "notes/test-cpp.md", noteContent, "estudos")
	
	// Index in FTS manually for the search subsystem to find it
	now := time.Now().Format(time.RFC3339)
	ctx.Store.InsertDocument(db.Document{
		ID:        "doc-cpp",
		Tipo:      "markdown",
		Arquivo:   "notes/test-cpp.md",
		Secao:     "Geral",
		Texto:     noteContent,
		Tags:      "estudos",
		Timestamp: now,
	})
	ctx.Store.IndexFTS("doc-cpp", "markdown", "notes/test-cpp.md", "Geral", noteContent, "estudos")
	
	// 2. Perform HTTP search request
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/search?q=C%2B%2B", nil) // Q is "C++" encoded
	
	ctx.HandleSearch(rec, req)
	
	if rec.Code != 200 {
		t.Errorf("esperado status 200, got %d", rec.Code)
	}
	
	bodyStr := rec.Body.String()
	
	// 3. Verify snippet and file name are in the HTML output
	if !strings.Contains(bodyStr, "test-cpp.md") {
		t.Errorf("esperado encontrar o arquivo 'test-cpp.md' no output HTML, got %q", bodyStr)
	}
	
	// The snippet must contain the context containing "C++"
	if !strings.Contains(bodyStr, "code in C++ today") {
		t.Errorf("esperado encontrar o snippet contendo 'code in C++ today' no output HTML, got %q", bodyStr)
	}
}
