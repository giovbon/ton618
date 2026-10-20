package notes

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── HandleFileSave ────────────────────────────────────────────────────

func TestHandleFileSave_SemFilename_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleFileSave(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400 sem filename, got %d", rec.Code)
	}
}

func TestHandleFileSave_SalvaERedireciona(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader("filename=minha-nota.md&content=# Conteudo Salvo&tags=teste")
	req := httptest.NewRequest("POST", "/file/save", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleFileSave(rec, req)

	// Deve redirecionar para o editor após salvar
	if rec.Code != 303 {
		t.Errorf("esperado 303 SeeOther após salvar, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/editor") {
		t.Errorf("esperado redirect para /editor, got %q", loc)
	}
}

func TestHandleFileSave_NormalizaFilename(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	// Filename sem prefixo notes/ → deve normalizar
	body := strings.NewReader("filename=nota-sem-prefixo&content=Conteudo")
	req := httptest.NewRequest("POST", "/file/save", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleFileSave(rec, req)

	if rec.Code != 303 {
		t.Errorf("esperado 303 após salvar com filename normalizado, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "notes%2F") && !strings.Contains(loc, "notes/") {
		t.Errorf("esperado redirect para notes/, got %q", loc)
	}
}

func TestHandleFileSave_ConteudoPersistidoNoBanco(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader("filename=notes/nota-persistida.md&content=Conteudo persistido no banco")
	req := httptest.NewRequest("POST", "/file/save", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleFileSave(rec, req)

	if rec.Code != 303 {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	// Confirma que o conteúdo foi salvo no banco
	content, err := ctx.Store.GetNote("notes/nota-persistida.md")
	if err != nil {
		t.Fatalf("erro ao buscar nota no banco: %v", err)
	}
	if content != "Conteudo persistido no banco" {
		t.Errorf("conteudo incorreto no banco: %q", content)
	}
}

// ── HandleNoteSaveJSON ────────────────────────────────────────────────

func TestHandleNoteSaveJSON_SemFilename_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/note/save", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleNoteSaveJSON(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400 sem filename, got %d", rec.Code)
	}
}

func TestHandleNoteSaveJSON_SalvaComSucesso(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader("filename=notes/nota-json.md&content=Conteudo JSON")
	req := httptest.NewRequest("POST", "/api/note/save", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleNoteSaveJSON(rec, req)

	// Novo conteúdo → deve retornar 200 com HX-Redirect
	if rec.Code != 200 {
		t.Errorf("esperado 200 ao salvar nova nota via JSON, got %d", rec.Code)
	}
	hxRedirect := rec.Header().Get("HX-Redirect")
	if !strings.Contains(hxRedirect, "/editor") {
		t.Errorf("esperado HX-Redirect para /editor, got %q", hxRedirect)
	}
}

func TestHandleNoteSaveJSON_ConteudoIgual_Retorna204(t *testing.T) {
	ctx := newTestContext(t)
	content := "Conteudo identico"
	saveTestNote(t, ctx, "notes/nota-sem-mudanca.md", content, "")

	rec := httptest.NewRecorder()
	body := strings.NewReader("filename=notes/nota-sem-mudanca.md&content=" + content)
	req := httptest.NewRequest("POST", "/api/note/save", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleNoteSaveJSON(rec, req)

	// Conteúdo idêntico → unchanged = true → 204 No Content
	if rec.Code != 204 {
		t.Errorf("esperado 204 quando conteudo eh identico, got %d", rec.Code)
	}
}

func TestHandleNoteSaveJSON_SilentMode_Retorna204(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader("filename=notes/nota-silent.md&content=Novo conteudo&silent=true")
	req := httptest.NewRequest("POST", "/api/note/save", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleNoteSaveJSON(rec, req)

	// silent=true → sempre 204 mesmo que seja novo conteúdo
	if rec.Code != 204 {
		t.Errorf("esperado 204 no modo silent, got %d", rec.Code)
	}
}

func TestHandleNoteSaveJSON_ConteudoPersistidoNoBanco(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader("filename=notes/nota-persistida-json.md&content=Conteudo via JSON&silent=true")
	req := httptest.NewRequest("POST", "/api/note/save", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleNoteSaveJSON(rec, req)

	if rec.Code != 204 {
		t.Fatalf("esperado 204, got %d", rec.Code)
	}

	content, err := ctx.Store.GetNote("notes/nota-persistida-json.md")
	if err != nil {
		t.Fatalf("erro ao buscar nota no banco: %v", err)
	}
	if content != "Conteudo via JSON" {
		t.Errorf("conteudo incorreto no banco: %q", content)
	}
}

// ── HandleBackup ──────────────────────────────────────────────────────

func TestHandleBackup_RetornaZip(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/nota-backup.md", "# Nota para backup\n\nConteudo.", "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/backup", nil)

	ctx.HandleBackup(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200 no backup, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/zip" {
		t.Errorf("esperado Content-Type application/zip, got %q", ct)
	}

	disp := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "attachment") || !strings.Contains(disp, ".zip") {
		t.Errorf("esperado Content-Disposition com attachment e .zip, got %q", disp)
	}
}

func TestHandleBackup_NomeContémBackupNotas(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/backup", nil)

	ctx.HandleBackup(rec, req)

	disp := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "ton618-backup-notas-") {
		t.Errorf("esperado filename com 'ton618-backup-notas-', got %q", disp)
	}
}

func TestHandleBackup_FullBackupMudaNome(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/backup?full=true", nil)

	ctx.HandleBackup(rec, req)

	disp := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "ton618-backup-completo-") {
		t.Errorf("esperado filename com 'ton618-backup-completo-', got %q", disp)
	}
}

func TestHandleBackup_XContentTypeOptionsNosniff(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/backup", nil)

	ctx.HandleBackup(rec, req)

	xct := rec.Header().Get("X-Content-Type-Options")
	if xct != "nosniff" {
		t.Errorf("esperado X-Content-Type-Options: nosniff, got %q", xct)
	}
}

// ── HandleDuplicateNote ───────────────────────────────────────────────

func TestHandleDuplicateNote_SemFilename_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/file/duplicate", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleDuplicateNote(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400 sem filename, got %d", rec.Code)
	}
}

func TestHandleDuplicateNote_NotaInexistente_Retorna404(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	body := strings.NewReader("filename=notes/inexistente.md")
	req := httptest.NewRequest("POST", "/file/duplicate", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleDuplicateNote(rec, req)

	if rec.Code != 404 {
		t.Errorf("esperado 404 para nota inexistente, got %d", rec.Code)
	}
}

func TestHandleDuplicateNote_Sucesso_CriaNovaNota(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/nota-original.md", "# Original\n\nConteudo.", "tag1")

	rec := httptest.NewRecorder()
	body := strings.NewReader("filename=notes/nota-original.md")
	req := httptest.NewRequest("POST", "/file/duplicate", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleDuplicateNote(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200 ao duplicar nota, got %d — body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro ao decodificar resposta JSON: %v", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		t.Errorf("esperado ok=true na resposta, got %v", resp)
	}
	newFilename, _ := resp["new_filename"].(string)
	if newFilename == "" {
		t.Error("esperado new_filename na resposta")
	}
	if !strings.Contains(newFilename, "copia-") {
		t.Errorf("esperado new_filename contendo 'copia-', got %q", newFilename)
	}
}

func TestHandleDuplicateNote_NotaComFrontmatterTitle_PrefixaTitle(t *testing.T) {
	ctx := newTestContext(t)
	content := "---\ntitle: Minha Nota\n---\n\n# Conteudo"
	saveTestNote(t, ctx, "notes/nota-com-title.md", content, "")

	rec := httptest.NewRecorder()
	body := strings.NewReader("filename=notes/nota-com-title.md")
	req := httptest.NewRequest("POST", "/file/duplicate", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleDuplicateNote(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperado 200 ao duplicar, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	newFilename, _ := resp["new_filename"].(string)

	// Verifica que a cópia existe e tem o title prefixado
	copiedContent, err := ctx.Store.GetNote(newFilename)
	if err != nil {
		t.Fatalf("erro ao buscar nota copiada: %v", err)
	}
	if !strings.Contains(copiedContent, "copia-Minha Nota") {
		t.Errorf("esperado title 'copia-Minha Nota' no frontmatter, got nota: %q", copiedContent)
	}
}

// TestHandleDuplicateNote_PreservaTagsDoBanco cobre a gravação ÚNICA do
// HandleDuplicateNote: a tabela `tags` é reconstruída a partir do frontmatter na
// reindexação, então uma tag que só existe no banco (sem frontmatter) precisa ser
// mesclada no conteúdo da cópia — antes o SetFileTags era sobrescrito.
func TestHandleDuplicateNote_PreservaTagsDoBanco(t *testing.T) {
	ctx := newTestContext(t)
	// Conteúdo SEM frontmatter: a tag fica só na tabela tags.
	saveTestNote(t, ctx, "notes/nota-tag.md", "# Nota\n\nConteudo.", "tag-do-banco")

	rec := httptest.NewRecorder()
	body := strings.NewReader("filename=notes/nota-tag.md")
	req := httptest.NewRequest("POST", "/file/duplicate", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleDuplicateNote(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperado 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	newFilename, _ := resp["new_filename"].(string)

	tags, err := ctx.Store.GetFileTags(newFilename)
	if err != nil {
		t.Fatalf("GetFileTags: %v", err)
	}
	found := false
	for _, tag := range tags {
		if tag == "tag-do-banco" {
			found = true
		}
	}
	if !found {
		t.Errorf("tag 'tag-do-banco' perdida na cópia; tags da cópia: %v", tags)
	}
}

// TestHandleFileRename_ReportaEFilhasRepontadas cobre o item "aviso no rename":
// o backend propaga o `pai:` das filhas (elas não ficam órfãs) e informa quantas
// foram atualizadas no header X-Children-Updated, que o editor exibe ao usuário.
func TestHandleFileRename_ReportaEFilhasRepontadas(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/pai.md", "---\ntitle: Pai\n---\n\nConteudo do pai.\n", "")
	saveTestNote(t, ctx, "notes/filha.md", "---\npai: pai\n---\n\nConteudo da filha.\n", "")
	saveTestNote(t, ctx, "notes/outra.md", "---\ntitle: Outra\n---\n\nSem hierarquia.\n", "")

	rec := httptest.NewRecorder()
	body := strings.NewReader("old=notes/pai.md&new=notes/pai-renomeado.md")
	req := httptest.NewRequest("POST", "/file/rename", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleFileRename(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperado 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Children-Updated"); got != "1" {
		t.Errorf("esperado X-Children-Updated=1, got %q", got)
	}

	// A filha foi repontada para o novo nome (não virou órfã).
	childContent, err := ctx.Store.GetNote("notes/filha.md")
	if err != nil {
		t.Fatalf("GetNote(filha): %v", err)
	}
	if !strings.Contains(childContent, "pai: pai-renomeado") {
		t.Errorf("esperado `pai: pai-renomeado` no frontmatter da filha, got:\n%s", childContent)
	}

	// A nota sem hierarquia ficou intacta.
	otherContent, err := ctx.Store.GetNote("notes/outra.md")
	if err != nil {
		t.Fatalf("GetNote(outra): %v", err)
	}
	if strings.Contains(otherContent, "pai:") {
		t.Errorf("nota sem hierarquia foi modificada:\n%s", otherContent)
	}
}

// TestHandleFileRename_SemFilhasNaoEnviaHeader garante que o header só aparece
// quando há filhas (o editor usa a ausência para não exibir aviso).
func TestHandleFileRename_SemFilhasNaoEnviaHeader(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/sozinha.md", "---\ntitle: Sozinha\n---\n\nConteudo.\n", "")

	rec := httptest.NewRecorder()
	body := strings.NewReader("old=notes/sozinha.md&new=notes/sozinha-2.md")
	req := httptest.NewRequest("POST", "/file/rename", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx.HandleFileRename(rec, req)

	if rec.Code != 200 {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("X-Children-Updated"); got != "" {
		t.Errorf("header não deveria ser enviado sem filhas, got %q", got)
	}
}

// ── HandleCapture ─────────────────────────────────────────────────────

func TestHandleCapture_MetodoInvalido_Retorna405(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/capture", nil)

	ctx.HandleCapture(rec, req)

	if rec.Code != 405 {
		t.Errorf("esperado 405 para GET, got %d", rec.Code)
	}
}

func TestHandleCapture_JSONInvalido_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/capture", strings.NewReader("nao e json"))
	req.Header.Set("Content-Type", "application/json")

	ctx.HandleCapture(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400 para JSON invalido, got %d", rec.Code)
	}
}

func TestHandleCapture_URLVazia_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/capture", strings.NewReader(`{"url":""}`))
	req.Header.Set("Content-Type", "application/json")

	ctx.HandleCapture(rec, req)

	if rec.Code != 400 {
		t.Errorf("esperado 400 para URL vazia, got %d", rec.Code)
	}
}
