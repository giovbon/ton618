package system

import (
	"net/http"
	"testing"
	"time"
)

// ── Hierarquia com ARQUIVOS como filhas (DECISIONS §6.24) ──
//
// Arquivos (PDF/EPUB/ZIP/...) não têm frontmatter: o `pai` vive na tabela
// file_metadata. Eles podem ser FILHAS, nunca PAI.

// saveTestFile registra um arquivo (não-nota) em file_mods, para que ele
// apareça no Tabulator.
func saveTestFile(t *testing.T, ctx *HandlerContext, filename string) {
	t.Helper()
	if err := ctx.Store.SetFileMod(filename, time.Now().Format(time.RFC3339)); err != nil {
		t.Fatalf("SetFileMod(%s): %v", filename, err)
	}
}

func TestBuildNoteTree_ArquivoComoFilha(t *testing.T) {
	rows := []map[string]interface{}{
		treeRow("notes/projeto.md", "", ""),
		treeRow("pdfs/manual.pdf", "projeto", ""),
	}
	roots, hasTree := buildNoteTree(rows)
	if !hasTree {
		t.Fatal("arquivo filho deveria produzir árvore")
	}
	if len(roots) != 1 {
		t.Fatalf("esperava 1 raiz, got %d", len(roots))
	}
	kids := childFiles(t, roots[0])
	if len(kids) != 1 || kids[0] != "pdfs/manual.pdf" {
		t.Fatalf("esperava [pdfs/manual.pdf], got %v", kids)
	}
}

func TestBuildNoteTree_ArquivoNuncaEPai(t *testing.T) {
	rows := []map[string]interface{}{
		treeRow("pdfs/manual.pdf", "", ""),
		treeRow("notes/filha.md", "manual", ""), // tenta usar o PDF como pai
	}
	roots, hasTree := buildNoteTree(rows)
	if hasTree {
		t.Error("arquivo não pode ser pai: não deveria haver árvore")
	}
	if len(roots) != 2 {
		t.Fatalf("as duas linhas deveriam ficar na raiz, got %d", len(roots))
	}
	if findNodeDeep(roots, "pdfs/manual.pdf") == nil || findNodeDeep(roots, "notes/filha.md") == nil {
		t.Errorf("linhas perdidas na montagem: %v", roots)
	}
}

// O handler injeta o `pai` de file_metadata na linha do arquivo, e a árvore
// aninha o arquivo sob a nota.
func TestHandleGetDatabaseData_FileAsChild(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/projeto.md", "# Projeto", "")
	saveTestFile(t, ctx, "pdfs/manual.pdf")
	if err := ctx.Store.SetFileMetadata("pdfs/manual.pdf", parentKey, "projeto"); err != nil {
		t.Fatalf("SetFileMetadata: %v", err)
	}

	resp := fetchDatabase(t, ctx)
	if !resp.Meta.HasTree {
		t.Fatal("arquivo filho deveria gerar árvore no payload")
	}
	projeto := findRow(resp.Data, "notes/projeto.md")
	if projeto == nil {
		t.Fatal("nota-mãe não encontrada")
	}
	if kids := childFiles(t, projeto); len(kids) != 1 || kids[0] != "pdfs/manual.pdf" {
		t.Fatalf("esperava o PDF como filha, got %v", kids)
	}
	file := findNodeDeep(resp.Data, "pdfs/manual.pdf")
	if file == nil {
		t.Fatal("arquivo não encontrado na árvore")
	}
	if file[parentKey] != "projeto" {
		t.Errorf("a coluna Pai do arquivo deveria valer \"projeto\", got %v", file[parentKey])
	}
}

func TestHandleGetDatabaseData_FileNeverParent(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/filha.md", "# Filha", "")
	saveTestFile(t, ctx, "pdfs/manual.pdf")

	// A nota declara o nome-base do PDF como pai — mas arquivos não são pais.
	rr := postProperty(t, ctx, "notes/filha.md", parentKey, "manual")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rr.Code, rr.Body.String())
	}

	resp := fetchDatabase(t, ctx)
	if resp.Meta.HasTree {
		t.Error("nenhum vínculo válido: arquivo não pode ser pai")
	}
	if len(resp.Data) != 2 {
		t.Fatalf("as duas linhas deveriam ficar na raiz, got %d", len(resp.Data))
	}
}

func TestHandleUpdateNoteProperty_FileParent(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/projeto.md", "# Projeto", "")
	saveTestFile(t, ctx, "attachments/dados.zip")

	// Grava (normalizando caixa e wikilink).
	rr := postProperty(t, ctx, "attachments/dados.zip", parentKey, "[[PROJETO]]")
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rr.Code, rr.Body.String())
	}
	meta, err := ctx.Store.GetFileMetadata("attachments/dados.zip")
	if err != nil {
		t.Fatalf("GetFileMetadata: %v", err)
	}
	if meta[parentKey] != "projeto" {
		t.Fatalf("pai do arquivo = %q, esperava \"projeto\"", meta[parentKey])
	}

	resp := fetchDatabase(t, ctx)
	if kids := childFiles(t, findRow(resp.Data, "notes/projeto.md")); len(kids) != 1 || kids[0] != "attachments/dados.zip" {
		t.Fatalf("esperava o zip como filha, got %v", kids)
	}

	// Remover devolve o arquivo à raiz (valor vazio apaga a chave).
	rr = postProperty(t, ctx, "attachments/dados.zip", parentKey, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("remoção: status %d (%s)", rr.Code, rr.Body.String())
	}
	meta, _ = ctx.Store.GetFileMetadata("attachments/dados.zip")
	if len(meta) != 0 {
		t.Fatalf("o metadado pai deveria ter sido removido, got %v", meta)
	}
	resp = fetchDatabase(t, ctx)
	if resp.Meta.HasTree {
		t.Error("sem vínculo não deveria haver árvore")
	}
}

// Múltiplos pais também são recusados para arquivos (a árvore é de pai único).
func TestHandleUpdateNoteProperty_FileParentRejectsMultipleParents(t *testing.T) {
	ctx := newTestContext(t)
	saveTestFile(t, ctx, "pdfs/x.pdf")

	rr := postProperty(t, ctx, "pdfs/x.pdf", parentKey, "a, b")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("múltiplos pais deveriam ser recusados, got %d", rr.Code)
	}
	meta, _ := ctx.Store.GetFileMetadata("pdfs/x.pdf")
	if len(meta) != 0 {
		t.Errorf("nada deveria ser gravado, got %v", meta)
	}
}

// Regressão: a linha de um arquivo fica no cache do Tabulator (chave = mtime,
// que NÃO muda ao renomear a nota-mãe). Se a injeção dos metadados só
// preenchesse chaves ausentes, o `pai` antigo ficaria preso na linha.
func TestHandleGetDatabaseData_FileParentUpdatesAfterNoteRename(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/projeto.md", "# Projeto", "")
	saveTestFile(t, ctx, "pdfs/manual.pdf")
	if err := ctx.Store.SetFileMetadata("pdfs/manual.pdf", parentKey, "projeto"); err != nil {
		t.Fatalf("SetFileMetadata: %v", err)
	}

	// Primeira leitura: popula o cache com pai=projeto.
	resp := fetchDatabase(t, ctx)
	pdf := findNodeDeep(resp.Data, "pdfs/manual.pdf")
	if pdf == nil || pdf[parentKey] != "projeto" {
		t.Fatalf("primeira leitura: pai = %v", pdf)
	}

	// Renomeia a nota-mãe via endpoint (propaga o metadado do arquivo; o cache
	// da LINHA do arquivo não é invalidado).
	rr := postProperty(t, ctx, "notes/projeto.md", "titulo", "projeto-novo")
	if rr.Code != http.StatusOK {
		t.Fatalf("rename: %d (%s)", rr.Code, rr.Body.String())
	}

	resp = fetchDatabase(t, ctx)
	pdf = findNodeDeep(resp.Data, "pdfs/manual.pdf")
	if pdf == nil {
		t.Fatal("PDF sumiu após o rename")
	}
	if pdf[parentKey] != "projeto-novo" {
		t.Errorf("a linha cacheada manteve o pai antigo: %v", pdf[parentKey])
	}
	if novo := findRow(resp.Data, "notes/projeto-novo.md"); novo == nil || len(childFiles(t, novo)) != 1 {
		t.Errorf("PDF não ficou aninhado sob a nota renomeada")
	}
}
