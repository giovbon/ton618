package system

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fetchDatabaseRows chama /api/database/data e devolve as linhas (já achatadas,
// sem árvore — estas notas não declaram `pai`).
func fetchDatabaseRows(t *testing.T, ctx *HandlerContext) []map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/database/data", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetDatabaseData(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status code: %d", rr.Code)
	}
	var response struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return response.Data
}

func findRowByArquivo(rows []map[string]interface{}, arquivo string) map[string]interface{} {
	for _, row := range rows {
		if row["arquivo"] == arquivo {
			return row
		}
	}
	return nil
}

// Nome de arquivo só de pontuação (ex: "notes/....md" → "...") não é um título
// útil: a coluna Título deve cair para o primeiro heading da nota.
func TestHandleGetDatabaseData_TituloFallbackPrimeiroHeading(t *testing.T) {
	ctx := newTestContext(t)

	if err := ctx.Notes.Save("notes/....md", "# Título Real da Nota\n\nConteúdo.", nil); err != nil {
		t.Fatalf("Notes.Save: %v", err)
	}

	rows := fetchDatabaseRows(t, ctx)
	row := findRowByArquivo(rows, "notes/....md")
	if row == nil {
		t.Fatal("linha notes/....md não encontrada no Tabulator")
	}
	if got := row["titulo"]; got != "Título Real da Nota" {
		t.Errorf("titulo = %q, esperado %q", got, "Título Real da Nota")
	}
}

// Sem heading, o fallback seguinte é o `titulo:` do frontmatter.
func TestHandleGetDatabaseData_TituloFallbackFrontmatter(t *testing.T) {
	ctx := newTestContext(t)

	content := "---\ntitulo: Título do Frontmatter\n---\nConteúdo sem heading."
	if err := ctx.Notes.Save("notes/....md", content, nil); err != nil {
		t.Fatalf("Notes.Save: %v", err)
	}

	rows := fetchDatabaseRows(t, ctx)
	row := findRowByArquivo(rows, "notes/....md")
	if row == nil {
		t.Fatal("linha notes/....md não encontrada no Tabulator")
	}
	if got := row["titulo"]; got != "Título do Frontmatter" {
		t.Errorf("titulo = %q, esperado %q", got, "Título do Frontmatter")
	}
}

// Um nome normal continua sendo exibido como está (o fallback não pode mudar o
// comportamento padrão).
func TestHandleGetDatabaseData_TituloNormalNaoUsaFallback(t *testing.T) {
	ctx := newTestContext(t)

	if err := ctx.Notes.Save("notes/nome-normal.md", "# Heading Diferente\n\nConteúdo.", nil); err != nil {
		t.Fatalf("Notes.Save: %v", err)
	}

	rows := fetchDatabaseRows(t, ctx)
	row := findRowByArquivo(rows, "notes/nome-normal.md")
	if row == nil {
		t.Fatal("linha notes/nome-normal.md não encontrada no Tabulator")
	}
	if got := row["titulo"]; got != "nome-normal" {
		t.Errorf("titulo = %q, esperado %q (nome do arquivo)", got, "nome-normal")
	}
}

// Renomear para um título só de pontos é recusado — era assim que nascia a nota
// com "..." no título.
func TestHandleUpdateNoteProperty_TituloSoPontosRejeitado(t *testing.T) {
	ctx := newTestContext(t)

	if err := ctx.Notes.Save("notes/renomear.md", "# Renomear\n\nConteúdo.", nil); err != nil {
		t.Fatalf("Notes.Save: %v", err)
	}

	body, _ := json.Marshal(UpdatePropertyRequest{File: "notes/renomear.md", Key: "titulo", Value: "..."})
	req := httptest.NewRequest("POST", "/api/notes/update-property", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	ctx.HandleUpdateNoteProperty(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 ao renomear para '...', got %d (%s)", rr.Code, rr.Body.String())
	}
	if !ctx.Store.NoteExists("notes/renomear.md") {
		t.Error("a nota original foi alterada/removida mesmo com o título inválido")
	}
}
