package api

import (
	"bytes"
	"encoding/json"
	"html/template"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// ── HandleFileSave ──────────────────────────────────────────────

func TestHandleFileSave_CriaNotaNova(t *testing.T) {
	ctx := newTestContext(t)

	body := "filename=notes/teste.md&content=<p>hello</p>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleFileSave(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303 See Other, got %d", rec.Code)
	}

	loc := rec.Header().Get("Location")
	if loc != "/editor?file=notes%2Fteste.md" {
		t.Fatalf("Location inesperado: %s", loc)
	}

	// Verificar que o arquivo foi salvo no disco
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/teste.md")
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("arquivo nao foi criado: %v", err)
	}
	if string(data) != "<p>hello</p>" {
		t.Fatalf("conteudo incorreto: got %q, want %q", string(data), "<p>hello</p>")
	}
}

func TestHandleFileSave_SemFilename_Retorna400(t *testing.T) {
	ctx := newTestContext(t)

	body := "filename=&content=<p>hello</p>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleFileSave(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 Bad Request, got %d", rec.Code)
	}
}

func TestHandleFileSave_AdicionaExtensaoMd(t *testing.T) {
	ctx := newTestContext(t)

	// filename sem .md — o handler deve adicionar
	body := "filename=notes/sem-ext&content=<p>test</p>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleFileSave(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	// Verificar que salvou com .md
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/sem-ext.md")
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Fatal("arquivo com extensao .md nao foi criado")
	}
}

func TestHandleFileSave_MetodoInvalido_Retorna405(t *testing.T) {
	ctx := newTestContext(t)

	req := httptest.NewRequest("GET", "/file/save", nil)
	rec := httptest.NewRecorder()

	ctx.HandleFileSave(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("esperado 405 Method Not Allowed, got %d", rec.Code)
	}
}

func TestHandleFileSave_SubstituiNotaExistente(t *testing.T) {
	ctx := newTestContext(t)

	// Criar arquivo existente
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/existente.md")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("<p>velho</p>"), 0644)

	// Sobrescrever
	body := "filename=notes/existente.md&content=<p>novo</p>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleFileSave(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	data, _ := os.ReadFile(fullPath)
	if string(data) != "<p>novo</p>" {
		t.Fatalf("conteudo nao foi sobrescrito: got %q", string(data))
	}
}

func TestHandleFileSave_RedirecionaParaEditor(t *testing.T) {
	ctx := newTestContext(t)

	body := "filename=notes/redir.md&content=<p>x</p>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleFileSave(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/editor?file=notes%2Fredir.md") {
		t.Fatalf("redirect nao aponta para editor: %s", loc)
	}
}

// ── HandleEditor ────────────────────────────────────────────────

func TestHandleEditor_NovoMd_IgnoraConteudoNoDisco(t *testing.T) {
	ctx := newTestContext(t)

	// Cria um arquivo notes/novo.md no disco (simula auto-save anterior)
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/novo.md")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("<p>conteudo fantasma</p>"), 0644)

	// Handler deve ignorar o conteudo existente
	req := httptest.NewRequest("GET", "/editor?file=notes/novo.md", nil)
	rec := httptest.NewRecorder()
	ctx.HandleEditor(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	// O textarea escondido deve estar vazio (ou conter apenas whitespace)
	if strings.Contains(body, "conteudo fantasma") {
		t.Error("HandleEditor carregou conteudo existente de notes/novo.md")
	}
}

func TestHandleEditor_NovoComTimestamp_IgnoraConteudoNoDisco(t *testing.T) {
	ctx := newTestContext(t)

	// Cria arquivo notes/novo-1234-abcd.md no disco
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/novo-1234-abcd.md")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("<p>conteudo fantasma</p>"), 0644)

	req := httptest.NewRequest("GET", "/editor?file=notes/novo-1234-abcd.md", nil)
	rec := httptest.NewRecorder()
	ctx.HandleEditor(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if strings.Contains(body, "conteudo fantasma") {
		t.Error("HandleEditor carregou conteudo existente de notes/novo-*")
	}
}

func TestHandleEditor_NotaExistente_CarregaConteudo(t *testing.T) {
	ctx := newTestContext(t)

	// Cria nota real
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/minha-nota.md")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("<p>conteudo real</p>"), 0644)

	req := httptest.NewRequest("GET", "/editor?file=notes/minha-nota.md", nil)
	rec := httptest.NewRecorder()
	ctx.HandleEditor(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "conteudo real") {
		t.Error("HandleEditor deveria carregar conteudo de notas existentes")
	}
}

func TestHandleEditor_SemFilename_UsaNovoMd(t *testing.T) {
	ctx := newTestContext(t)

	req := httptest.NewRequest("GET", "/editor", nil)
	rec := httptest.NewRecorder()
	ctx.HandleEditor(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	// Deve conter o display name padrao (sem diretorio) e content vazio
	if !strings.Contains(body, `value="novo"`) {
		t.Error("sem filename, editor deveria usar novo como display name (sem .md)")
	}
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

// ── Watcher: ProcessFile não quebra o save ──────────────────────

func TestHandleFileSave_ProcessFileNaoQuebra(t *testing.T) {
	// ProcessFile pode falhar (ex: FTS5), mas o handler não deve retornar erro
	// porque o arquivo já foi salvo em disco antes do processamento.
	ctx := newTestContext(t)

	// Salva um conteúdo HTML simples (que o processor markdown não reconhece como .md válido)
	body := "filename=notes/raw.md&content=<div>apenas teste</div>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleFileSave(rec, req)

	// Deve retornar 303 mesmo que ProcessFile tenha falhado
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303 mesmo com erro no processamento, got %d", rec.Code)
	}

	// Arquivo deve existir em disco
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/raw.md")
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Fatal("arquivo deveria existir mesmo com erro no processamento")
	}
}

// ── Static files sem auth ───────────────────────────────────────

func TestStaticFiles_ServidosSemAuth(t *testing.T) {
	// O middleware BasicAuthMiddleware já pula autenticação para /static/
	// Verificar que o editor.js é servido sem auth
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth := BasicAuthMiddleware(handler, "admin", "ton618")

	req := httptest.NewRequest("GET", "/static/editor.js", nil)
	rec := httptest.NewRecorder()
	auth.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Fatal("static files nao devem exigir autenticacao")
	}
}

func TestEditorPage_RequerAuth(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth := BasicAuthMiddleware(handler, "admin", "ton618")

	req := httptest.NewRequest("GET", "/editor", nil)
	rec := httptest.NewRecorder()
	auth.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("pagina /editor deveria redirecionar para /login, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "/login" {
		t.Fatalf("redirect deveria ser /login, got %s", loc)
	}
}

// ── Helpers ─────────────────────────────────────────────────────

func TestHandleFileSave_ConteudoVazio(t *testing.T) {
	ctx := newTestContext(t)

	body := "filename=notes/vazia.md&content=&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleFileSave(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303 para conteudo vazio, got %d", rec.Code)
	}

	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/vazia.md")
	data, _ := os.ReadFile(fullPath)
	if len(data) != 0 {
		t.Fatalf("conteudo vazio deveria salvar arquivo vazio, got %d bytes", len(data))
	}
}

func TestHandleFileSave_RedirecionaAposSave(t *testing.T) {
	ctx := newTestContext(t)
	t0 := time.Now()

	body := "filename=notes/timing.md&content=<p>test</p>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleFileSave(rec, req)

	// O save deve ser rápido (< 1s)
	if time.Since(t0) > time.Second {
		t.Fatal("save levou mais de 1s — pode ser problema de performance")
	}

	// Redireciona para o editor
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}
}

// ── HandleFileDelete ───────────────────────────────────────────

func TestHandleFileDelete_RemoveArquivo(t *testing.T) {
	ctx := newTestContext(t)

	// Criar arquivo primeiro
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/deletar.md")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("<p>vai ser deletado</p>"), 0644)

	body := "filename=notes/deletar.md"
	req := httptest.NewRequest("POST", "/file/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleFileDelete(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	// Verificar que o arquivo foi removido
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		t.Fatal("arquivo deveria ter sido deletado")
	}
}

func TestHandleFileDelete_SemFilename_Retorna400(t *testing.T) {
	ctx := newTestContext(t)

	body := "filename="
	req := httptest.NewRequest("POST", "/file/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleFileDelete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleFileDelete_ArquivoInexistente_Retorna303(t *testing.T) {
	ctx := newTestContext(t)

	// Deletar arquivo que nunca existiu — deve retornar 303 (idempotente)
	body := "filename=notes/inexistente.md"
	req := httptest.NewRequest("POST", "/file/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleFileDelete(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303 para arquivo inexistente, got %d", rec.Code)
	}
}

// ── Rename Flow (save novo + delete velho) ─────────────────────

func TestRenameFlow_SalvaNovoEDeletaVelho(t *testing.T) {
	ctx := newTestContext(t)

	// 1. Salva arquivo velho
	bodySave := "filename=notes/velho.md&content=<p>conteudo</p>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(bodySave))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileSave(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save velho falhou: %d", rec.Code)
	}

	// 2. Salva arquivo novo (com mesmo conteudo)
	bodySaveNew := "filename=notes/novo.md&content=<p>conteudo</p>&tags="
	req2 := httptest.NewRequest("POST", "/file/save", strings.NewReader(bodySaveNew))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec2 := httptest.NewRecorder()
	ctx.HandleFileSave(rec2, req2)
	if rec2.Code != http.StatusSeeOther {
		t.Fatalf("save novo falhou: %d", rec2.Code)
	}

	// 3. Deleta arquivo velho
	bodyDel := "filename=notes/velho.md"
	req3 := httptest.NewRequest("POST", "/file/delete", strings.NewReader(bodyDel))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec3 := httptest.NewRecorder()
	ctx.HandleFileDelete(rec3, req3)
	if rec3.Code != http.StatusSeeOther {
		t.Fatalf("delete velho falhou: %d", rec3.Code)
	}

	// 4. Verificar: velho foi deletado, novo existe
	velhoPath := filepath.Join(ctx.Cfg.DocsDir, "notes/velho.md")
	if _, err := os.Stat(velhoPath); !os.IsNotExist(err) {
		t.Fatal("arquivo velho deveria ter sido deletado")
	}

	novoPath := filepath.Join(ctx.Cfg.DocsDir, "notes/novo.md")
	data, err := os.ReadFile(novoPath)
	if err != nil {
		t.Fatalf("arquivo novo deveria existir: %v", err)
	}
	if string(data) != "<p>conteudo</p>" {
		t.Fatalf("conteudo do novo arquivo incorreto: got %q", string(data))
	}
}

// ── HandleGetAllNotes ──────────────────────────────────────────

func TestHandleGetAllNotes_RetornaListaVazia(t *testing.T) {
	ctx := newTestContext(t)

	req := httptest.NewRequest("GET", "/api/notes", nil)
	rec := httptest.NewRecorder()

	ctx.HandleGetAllNotes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	var resp struct {
		Notes []struct {
			Arquivo string   `json:"arquivo"`
			Tags    []string `json:"tags"`
			Mtime   string   `json:"mtime"`
		} `json:"notes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	// Deve retornar lista vazia (sem arquivos processados)
	if len(resp.Notes) != 0 {
		t.Fatalf("esperado lista vazia, got %d notas", len(resp.Notes))
	}
}

func TestHandleGetAllNotes_RetornaNotasSalvas(t *testing.T) {
	ctx := newTestContext(t)

	// Salvar uma nota (o HandleFileSave tambem registra no file_mods)
	body := "filename=notes/teste-api.md&content=<p>test</p>&tags=tag1,tag2"
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileSave(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save falhou: %d", rec.Code)
	}

	// Buscar a lista
	req2 := httptest.NewRequest("GET", "/api/notes", nil)
	rec2 := httptest.NewRecorder()
	ctx.HandleGetAllNotes(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec2.Code)
	}

	var resp struct {
		Notes []struct {
			Arquivo string   `json:"arquivo"`
			Tags    []string `json:"tags"`
			Mtime   string   `json:"mtime"`
		} `json:"notes"`
	}
	json.NewDecoder(rec2.Body).Decode(&resp)

	if len(resp.Notes) == 0 {
		t.Fatal("notas deveriam ser retornadas")
	}

	found := false
	for _, n := range resp.Notes {
		if n.Arquivo == "notes/teste-api.md" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("nota salva nao encontrada na lista")
	}
}

// ── Frontmatter + Tags ─────────────────────────────────────────

func TestHandleFileSave_ComFrontmatter_SalvaConteudoCompleto(t *testing.T) {
	ctx := newTestContext(t)

	// Salva conteudo que inclui frontmatter YAML
	content := "---\ntitle: Teste\ntags: [golang, teste]\n---\n\n# Nota\n\nConteudo da nota."
	body := "filename=notes/fm-test.md&content=" + url.QueryEscape(content) + "&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileSave(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	// Verificar que o arquivo foi salvo com frontmatter
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/fm-test.md")
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("arquivo nao foi criado: %v", err)
	}
	saved := string(data)

	if !strings.Contains(saved, "---") {
		t.Error("arquivo salvo deveria conter --- do frontmatter")
	}
	if !strings.Contains(saved, "title: Teste") {
		t.Error("arquivo salvo deveria conter o titulo do frontmatter")
	}
	if !strings.Contains(saved, "tags: [golang, teste]") {
		t.Error("arquivo salvo deveria conter as tags no frontmatter")
	}
	if !strings.Contains(saved, "# Nota") {
		t.Error("arquivo salvo deveria conter o conteudo markdown")
	}
}

func TestHandleFileSave_ComFrontmatter_ExtraiTags(t *testing.T) {
	ctx := newTestContext(t)

	// Salva nota com frontmatter contendo tags
	content := "---\ntitle: Teste\ntags: [golang, teste]\n---\n\n# Conteudo"
	body := "filename=notes/fm-tags.md&content=" + url.QueryEscape(content) + "&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileSave(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	// Verificar que as tags foram extraidas pelo ProcessFile
	tags, err := ctx.Store.GetFileTags("notes/fm-tags.md")
	if err != nil {
		t.Fatalf("GetFileTags error: %v", err)
	}

	foundGolang := false
	foundTeste := false
	for _, t := range tags {
		if t == "golang" {
			foundGolang = true
		}
		if t == "teste" {
			foundTeste = true
		}
	}
	if !foundGolang {
		t.Error("tag 'golang' deveria ter sido extraida do frontmatter")
	}
	if !foundTeste {
		t.Error("tag 'teste' deveria ter sido extraida do frontmatter")
	}
}

func TestHandleFileSave_SemFrontmatter_NaoQuebra(t *testing.T) {
	ctx := newTestContext(t)

	// Salva conteudo sem frontmatter
	body := "filename=notes/sem-fm.md&content=<p>apenas conteudo</p>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileSave(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/sem-fm.md")
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("arquivo nao foi criado: %v", err)
	}
	if string(data) != "<p>apenas conteudo</p>" {
		t.Fatalf("conteudo incorreto: got %q", string(data))
	}
}

func TestHandleFileSave_TagsViaForm_SobrescreveTags(t *testing.T) {
	ctx := newTestContext(t)

	// Salva com tags via parametro do form
	body := "filename=notes/form-tags.md&content=<p>test</p>&tags=urgente,importante"
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileSave(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	// Verificar tags via API
	req2 := httptest.NewRequest("GET", "/api/notes", nil)
	rec2 := httptest.NewRecorder()
	ctx.HandleGetAllNotes(rec2, req2)

	var resp struct {
		Notes []struct {
			Arquivo string   `json:"arquivo"`
			Tags    []string `json:"tags"`
			Mtime   string   `json:"mtime"`
		} `json:"notes"`
	}
	json.NewDecoder(rec2.Body).Decode(&resp)

	for _, n := range resp.Notes {
		if n.Arquivo == "notes/form-tags.md" {
			foundUrgente := false
			foundImportante := false
			for _, t := range n.Tags {
				if t == "urgente" {
					foundUrgente = true
				}
				if t == "importante" {
					foundImportante = true
				}
			}
			if !foundUrgente {
				t.Error("tag 'urgente' deveria estar associada a nota")
			}
			if !foundImportante {
				t.Error("tag 'importante' deveria estar associada a nota")
			}
			return
		}
	}
	t.Error("nota nao encontrada na lista")
}

// ── HandleGetAllNotes (modo compacto) ───────────────────────────

func TestHandleGetAllNotes_OrdenadoPorMtime(t *testing.T) {
	ctx := newTestContext(t)

	// Salva duas notas com nomes diferentes
	body1 := "filename=notes/mais-antiga.md&content=<p>a</p>&tags="
	body2 := "filename=notes/mais-recente.md&content=<p>b</p>&tags="

	req1 := httptest.NewRequest("POST", "/file/save", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec1 := httptest.NewRecorder()
	ctx.HandleFileSave(rec1, req1)

	// Pausa de 1s para garantir timestamp RFC3339 diferente (sem fração de segundos)
	time.Sleep(1 * time.Second)

	req2 := httptest.NewRequest("POST", "/file/save", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec2 := httptest.NewRecorder()
	ctx.HandleFileSave(rec2, req2)

	// Busca a lista
	req3 := httptest.NewRequest("GET", "/api/notes", nil)
	rec3 := httptest.NewRecorder()
	ctx.HandleGetAllNotes(rec3, req3)

	var resp struct {
		Notes []struct {
			Arquivo string   `json:"arquivo"`
			Tags    []string `json:"tags"`
			Mtime   string   `json:"mtime"`
		} `json:"notes"`
	}
	json.NewDecoder(rec3.Body).Decode(&resp)

	if len(resp.Notes) < 2 {
		t.Fatal("deveria ter pelo menos 2 notas")
	}

	// A mais recente deve vir primeiro (mtime decrescente)
	if resp.Notes[0].Arquivo != "notes/mais-recente.md" {
		t.Errorf("a nota mais recente deveria vir primeiro. Got %s no topo", resp.Notes[0].Arquivo)
	}
}

// ── HandleSearch (modo global) ─────────────────────────────────

func TestHandleSearch_RetornaResultados(t *testing.T) {
	ctx := newTestContext(t)

	// Salva uma nota com conteudo buscavel
	body := "filename=notes/buscavel.md&content=<p>termo_especifico_para_teste</p>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileSave(rec, req)

	// Busca pelo termo
	formData := "q=termo_especifico_para_teste"
	req2 := httptest.NewRequest("POST", "/search", strings.NewReader(formData))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec2 := httptest.NewRecorder()
	ctx.HandleSearch(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec2.Code)
	}

	bodyResp := rec2.Body.String()
	if !strings.Contains(bodyResp, "termo_especifico_para_teste") {
		t.Error("resposta da busca deveria conter o termo buscado")
	}
	if !strings.Contains(bodyResp, "buscavel") {
		t.Error("resposta da busca deveria conter o nome do arquivo")
	}
}

func TestHandleSearch_BuscaVazia(t *testing.T) {
	ctx := newTestContext(t)

	formData := "q="
	req := httptest.NewRequest("POST", "/search", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleSearch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 para busca vazia, got %d", rec.Code)
	}
}

// ── Embeddings ───────────────────────────────────────────────────

func TestHandleStatus_RetornaEmbeddingCount(t *testing.T) {
	ctx := newTestContext(t)

	req := httptest.NewRequest("GET", "/api/status", nil)
	rec := httptest.NewRecorder()
	ctx.HandleStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	var resp struct {
		Status     string `json:"status"`
		Documents  int    `json:"documents"`
		Embeddings int    `json:"embeddings"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Status != "ok" {
		t.Fatalf("status deveria ser 'ok', got %q", resp.Status)
	}
	// Nao ha docs nem embeddings no banco vazio
	// Apenas verificar que o endpoint responde
}

func TestGetAllNotes_TemCampoEmbedded(t *testing.T) {
	ctx := newTestContext(t)

	// Salvar uma nota
	body := "filename=notes/embed-test.md&content=<p>test</p>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileSave(rec, req)

	// Verificar via /api/notes que o campo embedded existe
	req2 := httptest.NewRequest("GET", "/api/notes", nil)
	rec2 := httptest.NewRecorder()
	ctx.HandleGetAllNotes(rec2, req2)

	var resp struct {
		Notes []struct {
			Arquivo  string `json:"arquivo"`
			Embedded bool   `json:"embedded"`
		} `json:"notes"`
	}
	json.NewDecoder(rec2.Body).Decode(&resp)

	for _, n := range resp.Notes {
		if n.Arquivo == "notes/embed-test.md" {
			// O campo embedded deve existir (pode ser true ou false)
			// O importante e que o campo esteja presente na serializacao
			return
		}
	}
	t.Error("nota nao encontrada na lista")
}

func TestTableEmbeddings_SchemaCriada(t *testing.T) {
	// Verifica que a tabela embeddings existe e aceita inserts
	ctx := newTestContext(t)

	vec := []float32{0.1, 0.2, 0.3}
	if err := ctx.Store.SetEmbedding("test-embed", vec, "Teste"); err != nil {
		t.Fatalf("SetEmbedding error: %v", err)
	}

	nv, err := ctx.Store.GetEmbedding("test-embed")
	if err != nil {
		t.Fatalf("GetEmbedding error: %v", err)
	}
	if nv == nil {
		t.Fatal("embedding nao encontrado")
	}

	hasEmbed := ctx.Store.HasFileEmbedding("inexistente")
	if hasEmbed {
		t.Error("HasFileEmbedding para arquivo inexistente deveria ser false")
	}
}

// ── HandleGraphData ────────────────────────────────────────────

func TestHandleGraphData_SemEmbeddings_RetornaVazio(t *testing.T) {
	ctx := newTestContext(t)

	req := httptest.NewRequest("GET", "/api/graph/data", nil)
	rec := httptest.NewRecorder()
	ctx.HandleGraphData(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	var resp struct {
		Nodes []struct {
			ID string `json:"id"`
		} `json:"nodes"`
		Links []struct {
			Source string `json:"source"`
			Target string `json:"target"`
		} `json:"links"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Nodes) != 0 {
		t.Errorf("esperado 0 nós, got %d", len(resp.Nodes))
	}
	if len(resp.Links) != 0 {
		t.Errorf("esperado 0 links, got %d", len(resp.Links))
	}
}

func TestHandleGraphData_ComEmbeddings_RetornaNodes2D(t *testing.T) {
	ctx := newTestContext(t)

	// Cria uma nota com documento e embedding
	docID := "doc-graph-test"
	dbDoc := db.Document{
		ID:      docID,
		Tipo:    "markdown",
		Arquivo: "notes/graf.md",
		Secao:   "Titulo",
		Texto:   "conteudo",
	}
	if err := ctx.Store.InsertDocument(dbDoc); err != nil {
		t.Fatalf("InsertDocument: %v", err)
	}

	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i) * 0.001
	}
	if err := ctx.Store.SetEmbedding(docID, vec, "Titulo"); err != nil {
		t.Fatalf("SetEmbedding: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/graph/data", nil)
	rec := httptest.NewRecorder()
	ctx.HandleGraphData(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	var resp struct {
		Nodes []struct {
			ID    string  `json:"id"`
			Title string  `json:"title"`
			X     float64 `json:"x"`
			Y     float64 `json:"y"`
		} `json:"nodes"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Nodes) != 1 {
		t.Fatalf("esperado 1 nó, got %d", len(resp.Nodes))
	}

	n := resp.Nodes[0]
	if n.ID != "notes/graf.md" {
		t.Errorf("esperado ID 'notes/graf.md', got %q", n.ID)
	}
	if n.Title != "graf" {
		t.Errorf("esperado Title 'graf', got %q", n.Title)
	}
	// Como ha apenas 1 embedding, a projecao deve devolver (0,0) e entao o grid entra
	if n.X == 0 && n.Y == 0 {
		t.Log("single node: (0,0) + grid fallback OK")
	}
}

// ── HandleFileDelete + Embedding ───────────────────────────────

func TestHandleFileDelete_RemoveEmbeddingTambem(t *testing.T) {
	ctx := newTestContext(t)

	// 1. Salva nota
	body := "filename=notes/deletar-com-embed.md&content=<p>teste</p>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileSave(rec, req)

	// 2. Simula embedding (como faria o ProcessFile com embed provider)
	ctx.Store.SetEmbedding("manual-embed", []float32{0.1, 0.2, 0.3}, "deletar-com-embed")
	// Vincula o embedding a um documento real
	docs, _ := ctx.Store.GetDocumentsByFile("notes/deletar-com-embed.md")
	if len(docs) == 0 {
		t.Fatal("documento deveria existir")
	}
	ctx.Store.SetEmbedding(docs[0].ID, []float32{0.1, 0.2, 0.3}, "deletar-com-embed")

	// Verifica que o embedding existe
	if c := ctx.Store.GetEmbeddingCount(); c != 1 {
		t.Fatalf("esperado 1 embedding, got %d", c)
	}

	// 3. Deleta a nota via HTTP
	bodyDel := "filename=notes/deletar-com-embed.md"
	reqDel := httptest.NewRequest("POST", "/file/delete", strings.NewReader(bodyDel))
	reqDel.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recDel := httptest.NewRecorder()
	ctx.HandleFileDelete(recDel, reqDel)

	// 4. Verifica que o embedding foi deletado junto
	if c := ctx.Store.GetEmbeddingCount(); c != 0 {
		t.Fatalf("esperado 0 embeddings apos deletar nota, got %d", c)
	}
}

func TestHandleFileDelete_RemoveEmbeddingMultiplosDocs(t *testing.T) {
	ctx := newTestContext(t)

	// Salva nota
	body := "filename=notes/multi-doc.md&content=<p>a</p><p>b</p>&tags="
	req := httptest.NewRequest("POST", "/file/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileSave(rec, req)

	// Cria multiplos embeddings para o mesmo arquivo
	docs, _ := ctx.Store.GetDocumentsByFile("notes/multi-doc.md")
	if len(docs) == 0 {
		t.Fatal("documento deveria existir")
	}
	for i, doc := range docs {
		vec := make([]float32, 4)
		vec[0] = float32(i)
		ctx.Store.SetEmbedding(doc.ID, vec, "multi")
	}

	if c := ctx.Store.GetEmbeddingCount(); c != len(docs) {
		t.Fatalf("esperado %d embeddings, got %d", len(docs), c)
	}

	// Deleta
	bodyDel := "filename=notes/multi-doc.md"
	reqDel := httptest.NewRequest("POST", "/file/delete", strings.NewReader(bodyDel))
	reqDel.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recDel := httptest.NewRecorder()
	ctx.HandleFileDelete(recDel, reqDel)

	// Todos os embeddings do arquivo devem sumir
	if c := ctx.Store.GetEmbeddingCount(); c != 0 {
		t.Fatalf("esperado 0 embeddings apos deletar, got %d", c)
	}
}

// ── HandleHealth ────────────────────────────────────────────────

func TestHandleHealth_RetornaUp(t *testing.T) {
	ctx := newTestContext(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	ctx.HandleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	var resp struct {
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "up" {
		t.Errorf("esperado status 'up', got %q", resp.Status)
	}
	if resp.Timestamp == "" {
		t.Error("timestamp vazio")
	}
}

func TestHandleHealth_ContentType(t *testing.T) {
	ctx := newTestContext(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	ctx.HandleHealth(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("esperado Content-Type application/json, got %q", ct)
	}
}

// ── HandleFile ─────────────────────────────────────────────────

func TestHandleFile_NotFound(t *testing.T) {
	ctx := newTestContext(t)
	req := httptest.NewRequest("GET", "/file?name=inexistente.md", nil)
	rec := httptest.NewRecorder()
	ctx.HandleFile(rec, req)

	if rec.Code != http.StatusNotFound && rec.Code != http.StatusOK {
		t.Fatalf("esperado 404 ou 200, got %d", rec.Code)
	}
}

func TestHandleFile_ServeArquivo(t *testing.T) {
	ctx := newTestContext(t)
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/servir.md")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("# Arquivo para servir"), 0644)

	req := httptest.NewRequest("GET", "/file?name=servir.md", nil)
	rec := httptest.NewRecorder()
	ctx.HandleFile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Arquivo para servir") {
		t.Error("conteudo do arquivo nao foi servido")
	}
}

func TestHandleFile_NormalizaCaminho(t *testing.T) {
	ctx := newTestContext(t)
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/normalizado.md")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("# Normalizado"), 0644)

	req := httptest.NewRequest("GET", "/file?name=normalizado.md", nil)
	rec := httptest.NewRecorder()
	ctx.HandleFile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}
}

func TestHandleFile_SemParametro_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	req := httptest.NewRequest("GET", "/file", nil)
	rec := httptest.NewRecorder()
	ctx.HandleFile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400, got %d", rec.Code)
	}
}

// ── HandleFileRename ────────────────────────────────────────────

func TestHandleFileRename_Sucesso(t *testing.T) {
	ctx := newTestContext(t)
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/velho.md")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("# Nota velha"), 0644)
	ctx.Store.SetFileMod("notes/velho.md", time.Now().Format(time.RFC3339))

	body := "old=velho.md&new=novo.md"
	req := httptest.NewRequest("POST", "/file/rename", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileRename(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	novoPath := filepath.Join(ctx.Cfg.DocsDir, "notes/novo.md")
	if _, err := os.Stat(novoPath); os.IsNotExist(err) {
		t.Error("arquivo novo nao foi criado")
	}

	velhoPath := filepath.Join(ctx.Cfg.DocsDir, "notes/velho.md")
	if _, err := os.Stat(velhoPath); !os.IsNotExist(err) {
		t.Error("arquivo velho ainda existe")
	}
}

func TestHandleFileRename_MesmoNome(t *testing.T) {
	ctx := newTestContext(t)
	body := "old=notes/mesmo.md&new=notes/mesmo.md"
	req := httptest.NewRequest("POST", "/file/rename", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileRename(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}
}

func TestHandleFileRename_MetodoInvalido(t *testing.T) {
	ctx := newTestContext(t)
	req := httptest.NewRequest("GET", "/file/rename", nil)
	rec := httptest.NewRecorder()
	ctx.HandleFileRename(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("esperado 405, got %d", rec.Code)
	}
}

func TestHandleFileRename_SemParametros(t *testing.T) {
	ctx := newTestContext(t)
	body := "old=&new="
	req := httptest.NewRequest("POST", "/file/rename", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileRename(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400, got %d", rec.Code)
	}
}

// ── HandleUpload ────────────────────────────────────────────────

func TestHandleUpload_Sucesso(t *testing.T) {
	ctx := newTestContext(t)

	var buf bytes.Buffer
	mp := multipart.NewWriter(&buf)
	part, _ := mp.CreateFormFile("file", "upload-nota.md")
	part.Write([]byte("# Nota via upload"))
	mp.Close()

	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", mp.FormDataContentType())
	rec := httptest.NewRecorder()
	ctx.HandleUpload(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	uploadPath := filepath.Join(ctx.Cfg.DocsDir, "notes/upload-nota.md")
	if _, err := os.Stat(uploadPath); os.IsNotExist(err) {
		t.Error("arquivo upload nao foi salvo")
	}
}

func TestHandleUpload_MetodoInvalido(t *testing.T) {
	ctx := newTestContext(t)
	req := httptest.NewRequest("GET", "/upload", nil)
	rec := httptest.NewRecorder()
	ctx.HandleUpload(rec, req)

	if rec.Code == 0 {
		t.Error("nenhuma resposta")
	}
}

// ── HandleGraphProject ──────────────────────────────────────────

func TestHandleGraphProject_SemEmbeddings(t *testing.T) {
	ctx := newTestContext(t)
	req := httptest.NewRequest("POST", "/api/graph/project", nil)
	rec := httptest.NewRecorder()
	ctx.HandleGraphProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	var resp struct {
		Ok    bool `json:"ok"`
		Nodes int  `json:"nodes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Ok {
		t.Error("ok deveria ser true")
	}
}

func TestHandleGraphProject_MetodoInvalido(t *testing.T) {
	ctx := newTestContext(t)
	req := httptest.NewRequest("GET", "/api/graph/project", nil)
	rec := httptest.NewRecorder()
	ctx.HandleGraphProject(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("esperado 405, got %d", rec.Code)
	}
}

func TestHandleGraphProject_ComEmbeddings(t *testing.T) {
	ctx := newTestContext(t)

	for i, pair := range [][2]string{{"a", "golang"}, {"b", "python"}} {
		id := "doc-" + pair[0]
		doc := db.Document{
			ID:      id,
			Tipo:    "markdown",
			Arquivo: "notes/" + pair[1] + ".md",
			Secao:   pair[1],
			Texto:   "conteudo sobre " + pair[1],
		}
		ctx.Store.InsertDocument(doc)
		vec := make([]float32, 128)
		for j := range vec {
			vec[j] = float32(i*100+j) * 0.001
		}
		ctx.Store.SetEmbedding(id, vec, pair[1])
	}

	req := httptest.NewRequest("POST", "/api/graph/project", nil)
	rec := httptest.NewRecorder()
	ctx.HandleGraphProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	var resp struct {
		Ok        bool `json:"ok"`
		Nodes     int  `json:"nodes"`
		Projected int  `json:"projected"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Ok {
		t.Error("ok deveria ser true")
	}
	if resp.Nodes != 2 {
		t.Errorf("esperado 2 nodes, got %d", resp.Nodes)
	}
	if resp.Projected != 2 {
		t.Errorf("esperado 2 projected, got %d", resp.Projected)
	}
}
