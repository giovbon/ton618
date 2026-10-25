package mcp_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"ton618/core/internal/core/db"
	"ton618/core/internal/features/notes"
	tonmcp "ton618/core/internal/mcp"
)

// newHarness cria um acervo temporário com duas notas e conecta um cliente MCP
// em memória — o mesmo caminho de código da produção, sem stdio.
func newHarness(t *testing.T) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()

	dir := t.TempDir()
	store, err := db.NewStore(filepath.Join(dir, "ton618.db"))
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	svc := notes.NewNoteService(store, store, store, store, store, store, filepath.Join(dir, "docs"))
	save := func(name, content string, tags ...string) {
		t.Helper()
		if err := svc.Save(name, content, tags); err != nil {
			t.Fatalf("salvar %q: %v", name, err)
		}
	}
	save("alpha", "---\ntags:\n  - animais\n---\n\n# Alfa\n\nOs gatos dormem no sofá.\n")
	save("beta", "---\ntags:\n  - animais\n  - resumo\n---\n\n# Beta\n\nNota que cita [[alpha]] e também fala de gatos.\n")

	srv := tonmcp.NewServer(store, filepath.Join(dir, "docs"))
	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	serverSession, err := srv.SDK().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	return clientSession
}

func call(t *testing.T, cs *sdk.ClientSession, name string, args map[string]any) *sdk.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}

func resultText(t *testing.T, res *sdk.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatalf("resultado sem conteúdo")
	}
	tc, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("conteúdo não é texto: %T", res.Content[0])
	}
	return tc.Text
}

func TestSearchNotes(t *testing.T) {
	cs := newHarness(t)
	res := call(t, cs, "search_notes", map[string]any{"query": "gatos"})
	if res.IsError {
		t.Fatalf("erro: %s", resultText(t, res))
	}

	var out struct {
		TotalHits  int `json:"total_hits"`
		NotesFound int `json:"notes_found"`
		Notes      []struct {
			File     string   `json:"file"`
			Title    string   `json:"title"`
			Type     string   `json:"type"`
			Tags     []string `json:"tags"`
			Sections []string `json:"sections"`
			Excerpt  string   `json:"excerpt"`
		} `json:"notes"`
	}
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out.NotesFound != 2 {
		t.Fatalf("esperava 2 notas, veio %d: %s", out.NotesFound, resultText(t, res))
	}
	files := map[string]bool{}
	for _, n := range out.Notes {
		files[n.File] = true
		if n.Excerpt == "" {
			t.Errorf("nota %s sem trecho", n.File)
		}
		if n.Type != "nota" {
			t.Errorf("tipo de %s = %q, esperava nota", n.File, n.Type)
		}
	}
	if !files["notes/alpha.md"] || !files["notes/beta.md"] {
		t.Errorf("arquivos inesperados: %v", files)
	}
}

func TestSearchNotesFilterByTag(t *testing.T) {
	cs := newHarness(t)
	res := call(t, cs, "search_notes", map[string]any{"query": "tags:resumo"})
	if res.IsError {
		t.Fatalf("erro: %s", resultText(t, res))
	}
	text := resultText(t, res)
	if !strings.Contains(text, "notes/beta.md") || strings.Contains(text, "notes/alpha.md") {
		t.Fatalf("filtro de tag não respeitado: %s", text)
	}
}

func TestGetNote(t *testing.T) {
	cs := newHarness(t)
	res := call(t, cs, "get_note", map[string]any{"file": "alpha"})
	if res.IsError {
		t.Fatalf("erro: %s", resultText(t, res))
	}
	text := resultText(t, res)

	for _, want := range []string{
		"Nota: notes/alpha.md",
		"Título: alpha",
		"Tipo: nota",
		"Tags: animais",
		"gatos dormem",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("saída sem %q:\n%s", want, text)
		}
	}
}

func TestGetNotePagination(t *testing.T) {
	cs := newHarness(t)
	res := call(t, cs, "get_note", map[string]any{"file": "alpha", "max_chars": 10})
	if res.IsError {
		t.Fatalf("erro: %s", resultText(t, res))
	}
	text := resultText(t, res)
	if !strings.Contains(text, "Trecho: 0–10") {
		t.Errorf("paginação ausente:\n%s", text)
	}
}

func TestGetNoteNotFound(t *testing.T) {
	cs := newHarness(t)
	res := call(t, cs, "get_note", map[string]any{"file": "nao-existe"})
	if !res.IsError {
		t.Fatalf("esperava erro para nota inexistente")
	}
	if !strings.Contains(resultText(t, res), "não encontrada") {
		t.Errorf("mensagem inesperada: %s", resultText(t, res))
	}
}

func TestListNotes(t *testing.T) {
	cs := newHarness(t)

	res := call(t, cs, "list_notes", map[string]any{"tipo": "nota"})
	if res.IsError {
		t.Fatalf("erro: %s", resultText(t, res))
	}
	var out struct {
		Total int `json:"total"`
		Notes []struct {
			File string `json:"file"`
			Type string `json:"type"`
		} `json:"notes"`
	}
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out.Total != 2 {
		t.Fatalf("esperava 2 notas, veio %d: %s", out.Total, resultText(t, res))
	}
	for _, n := range out.Notes {
		if n.Type != "nota" {
			t.Errorf("tipo %q, esperava nota", n.Type)
		}
	}

	res = call(t, cs, "list_notes", map[string]any{"tag": "resumo"})
	if res.IsError {
		t.Fatalf("erro: %s", resultText(t, res))
	}
	text := resultText(t, res)
	if !strings.Contains(text, "notes/beta.md") || strings.Contains(text, "notes/alpha.md") {
		t.Fatalf("filtro por tag falhou: %s", text)
	}
}

func TestQueryNotes(t *testing.T) {
	cs := newHarness(t)
	res := call(t, cs, "query_notes", map[string]any{
		"consulta": "tipo: notas\ntexto: gatos\nlimite: 10",
	})
	if res.IsError {
		t.Fatalf("erro: %s", resultText(t, res))
	}
	var out struct {
		Total    int              `json:"total"`
		Returned int              `json:"returned"`
		Rows     []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out.Total != 2 || out.Returned != 2 {
		t.Fatalf("esperava 2 linhas: %s", resultText(t, res))
	}

	// Filtro por tag via YAML.
	res = call(t, cs, "query_notes", map[string]any{
		"consulta": "tipo: notas\ntags: [resumo]",
	})
	if res.IsError {
		t.Fatalf("erro: %s", resultText(t, res))
	}
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out.Total != 1 || len(out.Rows) != 1 || out.Rows[0]["arquivo"] != "notes/beta.md" {
		t.Fatalf("filtro por tag na consulta falhou: %s", resultText(t, res))
	}
}

func TestQueryNotesInvalid(t *testing.T) {
	cs := newHarness(t)
	res := call(t, cs, "query_notes", map[string]any{"consulta": "tipo: inexistente"})
	if !res.IsError {
		t.Fatalf("esperava erro de consulta inválida")
	}
	if !strings.Contains(resultText(t, res), "inválido") {
		t.Errorf("mensagem inesperada: %s", resultText(t, res))
	}
}

func TestBacklinks(t *testing.T) {
	cs := newHarness(t)
	res := call(t, cs, "backlinks", map[string]any{"file": "alpha"})
	if res.IsError {
		t.Fatalf("erro: %s", resultText(t, res))
	}
	var out struct {
		File      string   `json:"file"`
		Backlinks []string `json:"backlinks"`
		Links     []string `json:"links"`
	}
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out.File != "notes/alpha.md" {
		t.Errorf("arquivo = %q", out.File)
	}
	if len(out.Backlinks) != 1 || out.Backlinks[0] != "notes/beta.md" {
		t.Errorf("backlinks = %v", out.Backlinks)
	}
}

func TestToolsAreReadOnly(t *testing.T) {
	cs := newHarness(t)

	// Nenhuma tool de escrita deve existir.
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	allowed := map[string]bool{
		"search_notes": true,
		"get_note":     true,
		"list_notes":   true,
		"query_notes":  true,
		"backlinks":    true,
	}
	for _, tool := range list.Tools {
		if !allowed[tool.Name] {
			t.Errorf("tool inesperada (escrita?) exposta: %s", tool.Name)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %s não declara readOnlyHint", tool.Name)
		}
	}
}
