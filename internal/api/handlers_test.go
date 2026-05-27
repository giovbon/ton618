package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ton618/internal/config"
	"ton618/internal/db"
	internalTpl "ton618/internal/template"
	"ton618/internal/watcher"
)

// newTestContext cria um HandlerContext isolado para testes.
// O diretório de docs e o banco são criados em tempdir (limpos automaticamente).
func newTestContext(t *testing.T) *HandlerContext {
	t.Helper()
	docsDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test.db")

	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := &config.AppConfig{
		DocsDir: docsDir,
	}

	w := watcher.NewWatcher(cfg, store)

	tpl := template.New("layout.html").Funcs(template.FuncMap{
		"hasPrefix": strings.HasPrefix,
		"baseName": func(s string) string {
			if s == "" {
				return ""
			}
			parts := strings.Split(s, "/")
			return parts[len(parts)-1]
		},
		"join": strings.Join,
		"noteIcon": func(arquivo string, tags []string) string {
			isPdf := strings.HasPrefix(arquivo, "pdfs/")
			hasTag := func(tag string) bool {
				for _, t := range tags {
					if t == tag {
						return true
					}
				}
				return false
			}
			if isPdf {
				return "📕"
			} else if hasTag("youtube") {
				return "🎬"
			} else if hasTag("artigo") {
				return "📰"
			} else if hasTag("captura") {
				return "📋"
			}
			return "📝"
		},
	})
	tpl, _ = tpl.ParseFS(internalTpl.TemplatesFS, "*.html")

	ctx := &HandlerContext{
		Cfg:     cfg,
		Store:   store,
		Watcher: w,
	}
	ctx.SetTemplates(tpl)

	return ctx
}

// ── Template: elementos do auto-save no editor ──────────────────

func TestEditorTemplate_TemElementosAutoSave(t *testing.T) {
	// Verifica que o template editor.html contém os elementos JS de auto-save
	// Necessário para evitar regressão: se alguém remover esses elementos,
	// o auto-save quebra.

	tplPath := "../template/editor.html"
	data, err := os.ReadFile(tplPath)
	if err != nil {
		t.Fatalf("ler template: %v", err)
	}
	html := string(data)

	checks := []struct {
		name string
		find string
	}{
		// Auto-save
		{"doSave", "async function doSave"},
		{"setStatus", "function setStatus"},
		{"ctrl+s", "e.key ==="},
		{"fetch file/save", "fetch(\"/file/save\""},
		{"editor-status", "editor-status"},
		{"saveNow", "function saveNow"},
		{"scheduleSave timeout", "setTimeout(doSave, 2000)"},
		// Bubble menu
		{"bubble-btn bold", `data-cmd="bold"`},
		{"bubble-btn italic", `data-cmd="italic"`},
		{"bubble-btn underline", `data-cmd="underline"`},
		{"bubble-btn highlight", `data-cmd="highlight"`},
		{"bubble-btn strike", `data-cmd="strike"`},
		{"bubble-menu event", "bubbleEl.addEventListener"},
		// Slash commands
		{"SLASH_COMMANDS array", "SLASH_COMMANDS"},
		{"slash-items", "slash-items"},
		{"makeSlashAction", "makeSlashAction"},
		{"slash deleteRange", "deleteRange({ from: slashPos"},
		{"table slash manual", "hideSlashMenu();"},
		// Wikilink
		{"wikilink-menu", "wikilink-menu"},
		{"wikilink-items", "wikilink-items"},
		{"loadWikiNotes", "loadWikiNotes"},
		{"selectWiki", "selectWiki"},
		{"wikiNotes array", "wikiNotes = []"},
		{"[[ detection", "event.key === \"[\""},
		{"wikish-link CSS", "wikish-link"},
		// Rename
		{"file-name input", "file-name"},
		{"doRename function", "async function doRename"},
		{"delete old file", "/file/delete"},
		// Markdown save
		{"getMarkdown", "getMarkdown"},
		// Save button
		{"save-btn listener", "getElementById(\"save-btn\").addEventListener"},
		{"toggle-fm-btn element", "toggle-fm-btn"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(html, c.find) {
				t.Errorf("template nao contem %q — auto-save pode estar quebrado", c.find)
			}
		})
	}
}

// ── Template index.html: elementos de busca ─────────────────────

func TestIndexTemplate_TemElementosDeBusca(t *testing.T) {
	tplPath := "../template/index.html"
	data, err := os.ReadFile(tplPath)
	if err != nil {
		t.Fatalf("ler template: %v", err)
	}
	html := string(data)

	checks := []struct {
		name string
		find string
	}{
		// Modo compacto
		{"buildNoteRows", "buildNoteRows"},
		{"renderCompactNotes", "renderCompactNotes"},
		{"compact-container", "compact-container"},
		{"global-container", "global-container"},
		{"contentArea element", "contentArea"},
		// Highlight
		{"search-highlight CSS", "search-highlight"},
		// Busca global
		{"doGlobalSearch", "doGlobalSearch"},
		{"fetch /search POST", "fetch(\"/search\""},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(html, c.find) {
				t.Errorf("template nao contem %q — modo de busca pode estar quebrado", c.find)
			}
		})
	}
}

// ── Helpers ─────────────────────────────────────────────────────

// saveTestNote cria uma nota de teste no disco e registra metadados no banco.
func saveTestNote(t *testing.T, ctx *HandlerContext, filename, content, tags string) {
	t.Helper()
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
	if tags != "" {
		tagList := strings.Split(tags, ",")
		ctx.Store.SetFileTags(filename, tagList)
	}
	ctx.Store.SetFileMod(filename, time.Now().Format(time.RFC3339))
}

// createMinimalPDF escreve um PDF valido com o texto informado.
func createMinimalPDF(t *testing.T, path, text string) {
	t.Helper()
	// Usa padding para manter o texto com pelo menos 11 chars, igual ao "Hello PDF"
	// usado nos offsets fixos do xref.
	paddedText := text
	if len(paddedText) < 11 {
		paddedText = paddedText + strings.Repeat(" ", 11-len(paddedText))
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	content := fmt.Sprintf("%%PDF-1.4\n"+
		"1 0 obj\n"+
		"<< /Type /Catalog /Pages 2 0 R >>\n"+
		"endobj\n"+
		"2 0 obj\n"+
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n"+
		"endobj\n"+
		"3 0 obj\n"+
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]\n"+
		"   /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\n"+
		"endobj\n"+
		"4 0 obj\n"+
		"<< /Length 44 >>\n"+
		"stream\n"+
		"BT /F1 12 Tf 100 700 Td (%s) Tj ET\n"+
		"endstream\n"+
		"endobj\n"+
		"5 0 obj\n"+
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\n"+
		"endobj\n"+
		"xref\n"+
		"0 6\n"+
		"0000000000 65535 f \n"+
		"0000000009 00000 n \n"+
		"0000000058 00000 n \n"+
		"0000000115 00000 n \n"+
		"0000000266 00000 n \n"+
		"0000000363 00000 n \n"+
		"trailer\n"+
		"<< /Size 6 /Root 1 0 R >>\n"+
		"startxref\n"+
		"442\n"+
		"%%EOF", paddedText)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("escrever PDF: %v", err)
	}
}

// ── Testes de estabilidade do mapa semântico ──

// insertTestEmbedding insere um documento com embedding (com ou sem coordenadas 2D).
func insertTestEmbedding(t *testing.T, ctx *HandlerContext, filename, contentStr string, withCoords bool, x, y float64) string {
	t.Helper()
	docID := fmt.Sprintf("%x", []byte(filename))
	ctx.Store.InsertDocument(db.Document{
		ID:      docID,
		Tipo:    "md",
		Arquivo: filename,
		Secao:   filename,
		Texto:   contentStr,
	})
	vec := []float32{float32(len(contentStr)) * 0.01, float32(len(filename)) * 0.01, 0.5, -0.3}
	ctx.Store.SetEmbedding(docID, vec, filename)
	if withCoords {
		ctx.Store.SetEmbedding2D(docID, x, y)
	}
	return docID
}

func TestHandleGraphData_Estabilidade(t *testing.T) {
	ctx := newTestContext(t)

	// Insere 3 notas com coordenadas 2D fixas
	insertTestEmbedding(t, ctx, "prog/go.md", "Goroutines e concorrencia em Go", true, 100, 200)
	insertTestEmbedding(t, ctx, "prog/python.md", "Decorators e list comprehension", true, -100, 50)
	insertTestEmbedding(t, ctx, "culin/massa.md", "Massa fresca italiana", true, 300, -150)

	// Chama HandleGraphData 2x
	var nodes1, nodes2 []map[string]interface{}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/graph/data", nil)
		rec := httptest.NewRecorder()
		ctx.HandleGraphData(rec, req)

		if rec.Code != 200 {
			t.Fatalf("HandleGraphData retornou %d", rec.Code)
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json decode: %v", err)
		}
		nodes, _ := resp["nodes"].([]interface{})
		if i == 0 {
			nodes1 = make([]map[string]interface{}, len(nodes))
			for j, n := range nodes {
				nodes1[j] = n.(map[string]interface{})
			}
		} else {
			nodes2 = make([]map[string]interface{}, len(nodes))
			for j, n := range nodes {
				nodes2[j] = n.(map[string]interface{})
			}
		}
	}

	if len(nodes1) != len(nodes2) {
		t.Fatalf("qtd de nos mudou: %d vs %d", len(nodes1), len(nodes2))
	}

	for i := range nodes1 {
		id1 := nodes1[i]["id"].(string)
		id2 := nodes2[i]["id"].(string)
		if id1 != id2 {
			t.Fatalf("ordem dos nos mudou: %s vs %s", id1, id2)
		}
		x1 := nodes1[i]["x"].(float64)
		y1 := nodes1[i]["y"].(float64)
		x2 := nodes2[i]["x"].(float64)
		y2 := nodes2[i]["y"].(float64)

		if x1 != x2 || y1 != y2 {
			t.Errorf("coordenada de %s mudou: (%f,%f) vs (%f,%f)", id1, x1, y1, x2, y2)
		}
	}
}

func TestHandleGraphData_NovasNotasNaoAlteramExistentes(t *testing.T) {
	ctx := newTestContext(t)

	// 3 notas com coordenadas
	insertTestEmbedding(t, ctx, "a.md", "Conteudo A", true, 100, 200)
	insertTestEmbedding(t, ctx, "b.md", "Conteudo B", true, -50, 75)
	insertTestEmbedding(t, ctx, "c.md", "Conteudo C", true, 30, -100)

	// Carga 1: coordenadas de referencia
	req := httptest.NewRequest("GET", "/api/graph/data", nil)
	rec := httptest.NewRecorder()
	ctx.HandleGraphData(rec, req)
	var resp1 map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp1)

	refCoords := make(map[string]map[string]float64)
	for _, n := range resp1["nodes"].([]interface{}) {
		node := n.(map[string]interface{})
		refCoords[node["id"].(string)] = map[string]float64{
			"x": node["x"].(float64),
			"y": node["y"].(float64),
		}
	}

	// Nota NOVA (sem coordenadas)
	insertTestEmbedding(t, ctx, "d.md", "Conteudo D novo", false, 0, 0)

	// Carga 2
	req2 := httptest.NewRequest("GET", "/api/graph/data", nil)
	rec2 := httptest.NewRecorder()
	ctx.HandleGraphData(rec2, req2)
	var resp2 map[string]interface{}
	json.Unmarshal(rec2.Body.Bytes(), &resp2)

	nodes2 := resp2["nodes"].([]interface{})
	if len(nodes2) != 4 {
		t.Fatalf("esperado 4 nos, got %d", len(nodes2))
	}

	for _, n := range nodes2 {
		node := n.(map[string]interface{})
		id := node["id"].(string)
		if ref, ok := refCoords[id]; ok {
			x := node["x"].(float64)
			y := node["y"].(float64)
			if x != ref["x"] || y != ref["y"] {
				t.Errorf("nota %s mudou: (%.1f,%.1f) -> (%.1f,%.1f)",
					id, ref["x"], ref["y"], x, y)
			}
		} else {
			x := node["x"].(float64)
			y := node["y"].(float64)
			if x == 0 && y == 0 {
				t.Errorf("nova nota %s ficou em (0,0) — nearest-neighbor falhou", id)
			}
		}
	}
}

func TestHandleGraphData_PrimeiraProjecaoGeraCoordenadas(t *testing.T) {
	ctx := newTestContext(t)

	// 4 notas SEM coordenadas 2D
	insertTestEmbedding(t, ctx, "n1.md", "Primeira nota", false, 0, 0)
	insertTestEmbedding(t, ctx, "n2.md", "Segunda nota", false, 0, 0)
	insertTestEmbedding(t, ctx, "n3.md", "Terceira nota", false, 0, 0)
	insertTestEmbedding(t, ctx, "n4.md", "Quarta nota", false, 0, 0)

	req := httptest.NewRequest("GET", "/api/graph/data", nil)
	rec := httptest.NewRecorder()
	ctx.HandleGraphData(rec, req)

	if rec.Code != 200 {
		t.Fatalf("HandleGraphData retornou %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	nodes, _ := resp["nodes"].([]interface{})

	if len(nodes) != 4 {
		t.Fatalf("esperado 4 nos, got %d", len(nodes))
	}

	for _, n := range nodes {
		node := n.(map[string]interface{})
		x := node["x"].(float64)
		y := node["y"].(float64)
		if x == 0 && y == 0 {
			t.Errorf("nota %s ficou em (0,0) — projecao falhou", node["id"])
		}
	}

	// Verifica que as coordenadas foram persistidas no banco
	for i, id := range []string{"n1.md", "n2.md", "n3.md", "n4.md"} {
		docID := fmt.Sprintf("%x", []byte(id))
		nv, err := ctx.Store.GetEmbedding(docID)
		if err != nil || nv == nil {
			t.Errorf("embedding %s (%s) nao encontrado apos projecao", id, docID)
			continue
		}
		if nv.X == 0 && nv.Y == 0 {
			t.Errorf("%s nao persistiu coordenadas (x=%.1f,y=%.1f)", id, nv.X, nv.Y)
		}
		_ = i
	}
}

func TestHandleGraphData_SemEmbeddings_RetornaVazio(t *testing.T) {
	ctx := newTestContext(t)
	// Nao insere nada

	req := httptest.NewRequest("GET", "/api/graph/data", nil)
	rec := httptest.NewRecorder()
	ctx.HandleGraphData(rec, req)

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	nodes := resp["nodes"]
	if nodes == nil {
		return // nil eh aceitavel
	}
	arr, ok := nodes.([]interface{})
	if !ok {
		t.Fatal("nodes nao eh array")
	}
	if len(arr) != 0 {
		t.Errorf("sem embeddings, esperado 0 nos, got %d", len(arr))
	}
}

func TestHandleGraphData_OrfaosNaoCausamReprojecao(t *testing.T) {
	ctx := newTestContext(t)

	// 1 nota com coordenadas validas
	insertTestEmbedding(t, ctx, "existente.md", "Nota existente", true, 50, 100)

	// 1 embedding ORFAO (doc_id que nao existe em documents) com coordenadas
	ctx.Store.SetEmbedding("orfao-com-coord", []float32{0.1, 0.2}, "Orfao")
	ctx.Store.SetEmbedding2D("orfao-com-coord", 999, 999)

	// 1 embedding ORFAO sem coordenadas (simula sobra de chunk)
	ctx.Store.SetEmbedding("orfao-sem-coord", []float32{0.3, 0.4}, "Orfao2")

	// HandleGraphData nao deve reprojetar nada — so retornar a nota valida
	req := httptest.NewRequest("GET", "/api/graph/data", nil)
	rec := httptest.NewRecorder()
	ctx.HandleGraphData(rec, req)

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	nodes, _ := resp["nodes"].([]interface{})

	if len(nodes) != 1 {
		t.Fatalf("esperado 1 no (nota valida), got %d — orfaos causaram reprojecao", len(nodes))
	}

	// A nota existente deve manter sua coordenada original
	node := nodes[0].(map[string]interface{})
	x := node["x"].(float64)
	y := node["y"].(float64)
	if x != 50 || y != 100 {
		t.Errorf("nota existente mudou de (50,100) para (%.1f,%.1f) — orfaos causaram reprojecao", x, y)
	}
}

func TestHandleGraphData_PollAllNaoReprocessaSemMudanca(t *testing.T) {
	ctx := newTestContext(t)

	// Cria diretorio monitorado e arquivo
	docsNotes := filepath.Join(ctx.Cfg.DocsDir, "notes")
	os.MkdirAll(docsNotes, 0755)
	testFile := filepath.Join(docsNotes, "teste.md")
	os.WriteFile(testFile, []byte("# Teste\nConteudo"), 0644)

	// Simula primeira indexacao via watcher
	mtime := time.Now()
	ctx.Store.SetFileMod("notes/teste.md", mtime.Format(time.RFC3339))

	// Insere documento e embedding manualmente
	ctx.Store.InsertDocument(db.Document{
		ID:      "teste-doc",
		Tipo:    "md",
		Arquivo: "notes/teste.md",
		Texto:   "# Teste\nConteudo",
	})
	ctx.Store.SetEmbedding("teste-doc", []float32{0.5, 0.3}, "Teste")
	ctx.Store.SetEmbedding2D("teste-doc", 42, 99)

	// pollAll deve IGNORAR o arquivo (mtime igual)
	ctx.Watcher.PollAll()

	// Verifica que o embedding ainda existe e tem as coordenadas originais
	nv, _ := ctx.Store.GetEmbedding("teste-doc")
	if nv == nil {
		t.Fatal("embedding foi deletado pelo pollAll")
	}
	if nv.X != 42 || nv.Y != 99 {
		t.Errorf("pollAll alterou coordenadas: (42,99) -> (%.1f,%.1f)", nv.X, nv.Y)
	}
}
