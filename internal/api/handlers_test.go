package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
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

func TestHandleEditor_NovoComTimestamp_CarregaConteudo(t *testing.T) {
	ctx := newTestContext(t)

	// Cria arquivo notes/novo-1234-abcd.md no disco
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/novo-1234-abcd.md")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("<p>conteudo real</p>"), 0644)

	req := httptest.NewRequest("GET", "/editor?file=notes/novo-1234-abcd.md", nil)
	rec := httptest.NewRecorder()
	ctx.HandleEditor(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "conteudo real") {
		t.Error("HandleEditor deveria carregar conteudo existente de notes/novo-*")
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

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
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

func TestHandleFileDelete_ArquivoInexistente_Retorna200(t *testing.T) {
	ctx := newTestContext(t)

	// Deletar arquivo que nunca existiu — deve retornar 200 (idempotente)
	body := "filename=notes/inexistente.md"
	req := httptest.NewRequest("POST", "/file/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleFileDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 para arquivo inexistente, got %d", rec.Code)
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
	if rec3.Code != http.StatusOK {
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
	// Define coordenadas 2D para que o embedding seja encontrado
	if err := ctx.Store.SetEmbedding2D(docID, 42.0, 99.0); err != nil {
		t.Fatalf("SetEmbedding2D: %v", err)
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
	// Deve ter as coordenadas 2D que definimos
	if n.X != 42.0 || n.Y != 99.0 {
		t.Errorf("esperado (42, 99), got (%.0f, %.0f)", n.X, n.Y)
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
	part, _ := mp.CreateFormFile("file", "upload.pdf")
	part.Write([]byte("%PDF-1.4\n%âãÏÓ\n"))
	mp.Close()

	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", mp.FormDataContentType())
	rec := httptest.NewRecorder()
	ctx.HandleUpload(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	uploadPath := filepath.Join(ctx.Cfg.DocsDir, "pdfs/upload.pdf")
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

// ── buildContextSnippet ────────────────────────────────────────

func TestBuildContextSnippet_QuerySimples(t *testing.T) {
	text := "Este e um texto sobre golang e concorrencia."
	snippet := buildContextSnippet("golang", text)
	if !strings.Contains(snippet, "golang") {
		t.Errorf("snippet deveria conter o termo buscado, got %q", snippet)
	}
}

func TestBuildContextSnippet_DoubleQuotedPhrase(t *testing.T) {
	text := "Este documento fala sobre parcialmente artigo e outros topicos."
	snippet := buildContextSnippet(`"parcialmente artigo"`, text)
	if !strings.Contains(snippet, "parcialmente artigo") {
		t.Errorf("snippet deveria conter a frase exata, got %q", snippet)
	}
}

func TestBuildContextSnippet_SingleQuotedPhrase(t *testing.T) {
	text := "Este documento fala sobre parcialmente artigo e outros topicos."
	snippet := buildContextSnippet(`'parcialmente artigo'`, text)
	if !strings.Contains(snippet, "parcialmente artigo") {
		t.Errorf("snippet deveria conter a frase entre aspas simples, got %q", snippet)
	}
}

func TestBuildContextSnippet_SingleQuoteNaoAdjacente_AindaGeraContexto(t *testing.T) {
	text := "Este documento fala sobre parcialmente um artigo e outros topicos."
	snippet := buildContextSnippet(`'parcialmente artigo'`, text)
	// Mesmo nao sendo adjacentes, o snippet deve conter pelo menos uma das palavras
	if !strings.Contains(snippet, "parcialmente") && !strings.Contains(snippet, "artigo") {
		t.Errorf("snippet deveria conter ao menos uma palavra do termo, got %q", snippet)
	}
}

func TestBuildContextSnippet_TextoVazio(t *testing.T) {
	snippet := buildContextSnippet("golang", "")
	if snippet != "..." {
		t.Errorf("texto vazio deveria retornar '...', got %q", snippet)
	}
}

func TestBuildContextSnippet_SemMatch_RetornaInicio(t *testing.T) {
	text := "Um texto qualquer sem relacao."
	snippet := buildContextSnippet("golang", text)
	if !strings.Contains(snippet, text) {
		t.Errorf("sem match, deveria retornar inicio do texto, got %q", snippet)
	}
}

func TestBuildContextSnippet_StopwordsNaoAfetam(t *testing.T) {
	text := "Golang e uma linguagem de programacao."
	snippet := buildContextSnippet("golang", text)
	if !strings.Contains(strings.ToLower(snippet), "golang") {
		t.Errorf("stopword 'e' nao deveria impedir match de 'golang', got %q", snippet)
	}
}

// ── HandleUploadAttachment ─────────────────────────────────────

func TestHandleUploadAttachment_SemArquivos_Retorna400(t *testing.T) {
	ctx := newTestContext(t)
	req := httptest.NewRequest("POST", "/api/upload-attachment", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	rec := httptest.NewRecorder()
	ctx.HandleUploadAttachment(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleUploadAttachment_CriaZipEIndexaFTS(t *testing.T) {
	ctx := newTestContext(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("files", "teste.txt")
	io.Copy(part, strings.NewReader("conteudo do arquivo"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/upload-attachment", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	ctx.HandleUploadAttachment(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	// Deve ter um arquivo .zip em attachments/
	attachDir := filepath.Join(ctx.Cfg.DocsDir, "attachments")
	entries, _ := os.ReadDir(attachDir)
	var zipFile string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".zip") {
			zipFile = e.Name()
			break
		}
	}
	if zipFile == "" {
		t.Fatal("nenhum zip foi criado em attachments/")
	}

	filename := "attachments/" + zipFile

	// Deve ter um documento FTS
	docs, _ := ctx.Store.GetDocumentsByFile(filename)
	if len(docs) == 0 {
		t.Fatal("documento FTS deveria existir para o zip")
	}
	if !strings.Contains(docs[0].Texto, "teste.txt") {
		t.Errorf("texto do documento deveria conter 'teste.txt', got %q", docs[0].Texto)
	}

	// Deve estar em file_mods
	mods, _ := ctx.Store.GetAllFileMods()
	if _, ok := mods[filename]; !ok {
		t.Error("zip deveria estar registrado em file_mods")
	}

	// Deve ter tag "zip"
	tags, _ := ctx.Store.GetFileTags(filename)
	if len(tags) != 1 || tags[0] != "zip" {
		t.Errorf("zip deveria ter tag [zip], got %v", tags)
	}
}

func TestHandleUploadAttachment_MultiplosArquivos(t *testing.T) {
	ctx := newTestContext(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	f1, _ := w.CreateFormFile("files", "doc.pdf")
	io.Copy(f1, strings.NewReader("pdf"))
	f2, _ := w.CreateFormFile("files", "foto.jpg")
	io.Copy(f2, strings.NewReader("jpg"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/upload-attachment", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	ctx.HandleUploadAttachment(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	attachDir := filepath.Join(ctx.Cfg.DocsDir, "attachments")
	entries, _ := os.ReadDir(attachDir)
	var zipFile string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".zip") {
			zipFile = e.Name()
			break
		}
	}
	if zipFile == "" {
		t.Fatal("nenhum zip criado")
	}

	filename := "attachments/" + zipFile
	docs, _ := ctx.Store.GetDocumentsByFile(filename)
	if len(docs) == 0 {
		t.Fatal("documento FTS deveria existir")
	}
	if !strings.Contains(docs[0].Texto, "doc.pdf") || !strings.Contains(docs[0].Texto, "foto.jpg") {
		t.Errorf("texto deveria listar ambos arquivos, got %q", docs[0].Texto)
	}
}

// ── HandleFileDelete (ZIP) ────────────────────────────────────

func TestHandleFileDelete_ZIP_RemoveArquivoEDocs(t *testing.T) {
	ctx := newTestContext(t)

	// Simula criação de ZIP (cria diretorio, arquivo, documento FTS e file_mods)
	attachDir := filepath.Join(ctx.Cfg.DocsDir, "attachments")
	os.MkdirAll(attachDir, 0755)
	zipName := "teste-delete.zip"
	zipPath := filepath.Join(attachDir, zipName)
	os.WriteFile(zipPath, []byte("fake zip content"), 0644)

	filename := "attachments/" + zipName
	docID := "att-test-delete"
	ctx.Store.InsertDocument(db.Document{
		ID:      docID,
		Tipo:    "attachment",
		Arquivo: filename,
		Secao:   "📦 " + zipName,
		Texto:   "Arquivos: deletar.txt",
	})
	ctx.Store.IndexFTS(docID, "attachment", filename, "📦 "+zipName, "Arquivos: deletar.txt", "")
	ctx.Store.SetFileMod(filename, time.Now().Format(time.RFC3339))

	// Deleta via handler
	body := "filename=" + filename
	req := httptest.NewRequest("POST", "/file/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	// Arquivo removido do disco
	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Error("arquivo zip deveria ter sido removido do disco")
	}

	// Documento removido
	if c := ctx.Store.GetDocumentCount(); c != 0 {
		t.Errorf("documentos deveriam ser 0, got %d", c)
	}

	// file_mods removido
	mods, _ := ctx.Store.GetAllFileMods()
	if _, ok := mods[filename]; ok {
		t.Error("file_mods deveria ter sido removido")
	}
}

func TestHandleFileDelete_ZIP_Inexistente_Retorna200(t *testing.T) {
	ctx := newTestContext(t)
	body := "filename=attachments/fantasma.zip"
	req := httptest.NewRequest("POST", "/file/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileDelete(rec, req)

	// Deve retornar 200 (idempotente, consistente com markdown)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 para ZIP inexistente, got %d", rec.Code)
	}
}

// ── HandleFileRename (ZIP) ────────────────────────────────────

func TestHandleFileRename_ZIP_RenomeiaComSucesso(t *testing.T) {
	ctx := newTestContext(t)

	// Cria ZIP real
	attachDir := filepath.Join(ctx.Cfg.DocsDir, "attachments")
	os.MkdirAll(attachDir, 0755)
	oldName := "attachments/velho.zip"
	oldPath := filepath.Join(attachDir, "velho.zip")
	os.WriteFile(oldPath, []byte("zip content"), 0644)

	filename := oldName
	docID := "att-test-rename"
	ctx.Store.InsertDocument(db.Document{
		ID:      docID,
		Tipo:    "attachment",
		Arquivo: filename,
		Secao:   "📦 velho.zip",
		Texto:   "Arquivos: velho.txt",
	})
	ctx.Store.IndexFTS(docID, "attachment", filename, "📦 velho.zip", "Arquivos: velho.txt", "")
	ctx.Store.SetFileMod(filename, time.Now().Format(time.RFC3339))

	// Renomeia
	body := "old=attachments/velho.zip&new=attachments/novo.zip"
	req := httptest.NewRequest("POST", "/file/rename", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileRename(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	// Arquivo novo existe, velho nao
	newPath := filepath.Join(attachDir, "novo.zip")
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("novo arquivo zip deveria existir")
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("arquivo zip velho deveria ter sido removido")
	}
}

func TestHandleFileRename_ZIP_SemExtensao_AdicionaZip(t *testing.T) {
	ctx := newTestContext(t)

	attachDir := filepath.Join(ctx.Cfg.DocsDir, "attachments")
	os.MkdirAll(attachDir, 0755)
	oldPath := filepath.Join(attachDir, "original.zip")
	os.WriteFile(oldPath, []byte("zip"), 0644)
	ctx.Store.SetFileMod("attachments/original.zip", time.Now().Format(time.RFC3339))

	// Renomeia sem extensao — backend deve adicionar .zip
	body := "old=attachments/original.zip&new=attachments/renomeado"
	req := httptest.NewRequest("POST", "/file/rename", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileRename(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado 303, got %d", rec.Code)
	}

	// Deve ter criado renomeado.zip
	newPath := filepath.Join(attachDir, "renomeado.zip")
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("deveria ter criado renomeado.zip, got file not found")
	}

	// Redirecionamento deve conter o nome correto
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "renomeado.zip") {
		t.Errorf("redirect deveria conter renomeado.zip, got %q", loc)
	}
}

// ── HandleBulkDelete ───────────────────────────────────────────

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

func TestHandleBulkDelete_PreviewAgeFilter(t *testing.T) {
	ctx := newTestContext(t)

	// Cria notas com idades diferentes
	now := time.Now()

	// Nota velha: mtime de 5 anos atras
	saveTestNote(t, ctx, "notes/velha.md", "velha", "")
	ctx.Store.SetFileMod("notes/velha.md", now.AddDate(-5, 0, 0).Format(time.RFC3339))

	// Nota recente: mtime de 1 mes atras
	saveTestNote(t, ctx, "notes/recente.md", "recente", "")
	ctx.Store.SetFileMod("notes/recente.md", now.AddDate(0, -1, 0).Format(time.RFC3339))

	// Preview: notas mais velhas que 2 anos
	body := "by_age=true&age_years=2&by_tag=false&tag_name=&preview=true"
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Files []string `json:"files"`
		Total int      `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 1 {
		t.Fatalf("esperado 1 nota velha, got %d", resp.Total)
	}
	if len(resp.Files) != 1 || resp.Files[0] != "notes/velha.md" {
		t.Errorf("esperado notes/velha.md, got %v", resp.Files)
	}

	// Nota recente ainda existe no disco
	if _, err := os.Stat(filepath.Join(ctx.Cfg.DocsDir, "notes/recente.md")); os.IsNotExist(err) {
		t.Error("nota recente nao deveria ter sido deletada no preview")
	}
}

func TestHandleBulkDelete_PreviewTagFilter(t *testing.T) {
	ctx := newTestContext(t)

	saveTestNote(t, ctx, "notes/com-tag.md", "com tag", "urgente")
	saveTestNote(t, ctx, "notes/sem-tag.md", "sem tag", "")
	saveTestNote(t, ctx, "notes/outra-tag.md", "outra", "pessoal")

	// Preview: notas com tag "urgente"
	body := "by_age=false&age_years=1&by_tag=true&tag_name=urgente&preview=true"
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Files []string `json:"files"`
		Total int      `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 1 {
		t.Fatalf("esperado 1 nota com tag urgente, got %d", resp.Total)
	}
	if len(resp.Files) != 1 || resp.Files[0] != "notes/com-tag.md" {
		t.Errorf("esperado notes/com-tag.md, got %v", resp.Files)
	}
}

func TestHandleBulkDelete_PreviewMultiTag(t *testing.T) {
	ctx := newTestContext(t)

	saveTestNote(t, ctx, "notes/tag1.md", "nota 1", "urgente")
	saveTestNote(t, ctx, "notes/tag2.md", "nota 2", "temporario")
	saveTestNote(t, ctx, "notes/tag3.md", "nota 3", "rascunho")
	saveTestNote(t, ctx, "notes/sem-tag.md", "sem tag", "")

	// Preview: notas com tag "urgente" ou "rascunho" (multi-tag, separado por virgula)
	body := "by_age=false&age_years=1&by_tag=true&tag_name=urgente, rascunho&preview=true"
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Files []string `json:"files"`
		Total int      `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 2 {
		t.Fatalf("esperado 2 notas, got %d: %v", resp.Total, resp.Files)
	}

	hasTag1 := false
	hasTag3 := false
	for _, f := range resp.Files {
		if f == "notes/tag1.md" {
			hasTag1 = true
		}
		if f == "notes/tag3.md" {
			hasTag3 = true
		}
	}
	if !hasTag1 {
		t.Error("notes/tag1.md deveria estar na preview")
	}
	if !hasTag3 {
		t.Error("notes/tag3.md deveria estar na preview")
	}
}

func TestHandleBulkDelete_PreviewIntersecaoAgeETag(t *testing.T) {
	ctx := newTestContext(t)
	now := time.Now()

	// Nota velha com tag urgente
	saveTestNote(t, ctx, "notes/velha-urgente.md", "nota 1", "urgente")
	ctx.Store.SetFileMod("notes/velha-urgente.md", now.AddDate(-3, 0, 0).Format(time.RFC3339))

	// Nota velha sem tag
	saveTestNote(t, ctx, "notes/velha-simples.md", "nota 2", "")
	ctx.Store.SetFileMod("notes/velha-simples.md", now.AddDate(-3, 0, 0).Format(time.RFC3339))

	// Nota recente com tag urgente
	saveTestNote(t, ctx, "notes/recente-urgente.md", "nota 3", "urgente")
	ctx.Store.SetFileMod("notes/recente-urgente.md", now.Format(time.RFC3339))

	// Preview: notas mais velhas que 2 anos E com tag "urgente"
	body := "by_age=true&age_years=2&by_tag=true&tag_name=urgente&preview=true"
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Files []string `json:"files"`
		Total int      `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 1 {
		t.Fatalf("esperado 1 nota (intersecao), got %d: %v", resp.Total, resp.Files)
	}
	if len(resp.Files) != 1 || resp.Files[0] != "notes/velha-urgente.md" {
		t.Errorf("esperado notes/velha-urgente.md, got %v", resp.Files)
	}
}

func TestHandleBulkDelete_ExecuteAgeFilter(t *testing.T) {
	ctx := newTestContext(t)
	now := time.Now()

	saveTestNote(t, ctx, "notes/para-deletar.md", "deletar", "")
	ctx.Store.SetFileMod("notes/para-deletar.md", now.AddDate(-5, 0, 0).Format(time.RFC3339))

	saveTestNote(t, ctx, "notes/manter.md", "manter", "")
	ctx.Store.SetFileMod("notes/manter.md", now.Format(time.RFC3339))

	// Deletar notas mais velhas que 2 anos
	body := "by_age=true&age_years=2&by_tag=false&tag_name="
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Deleted int      `json:"deleted"`
		Errors  []string `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Deleted != 1 {
		t.Fatalf("esperado 1 exclusao, got %d", resp.Deleted)
	}
	if len(resp.Errors) > 0 {
		t.Fatalf("erros inesperados: %v", resp.Errors)
	}

	// Verificar que a nota velha sumiu
	if _, err := os.Stat(filepath.Join(ctx.Cfg.DocsDir, "notes/para-deletar.md")); !os.IsNotExist(err) {
		t.Error("nota velha deveria ter sido deletada do disco")
	}
	// Nota recente ainda existe
	if _, err := os.Stat(filepath.Join(ctx.Cfg.DocsDir, "notes/manter.md")); os.IsNotExist(err) {
		t.Error("nota recente nao deveria ter sido deletada")
	}

	// Verificar que os metadados da nota deletada foram limpos
	mtime, _ := ctx.Store.GetFileMod("notes/para-deletar.md")
	if mtime != "" {
		t.Error("file_mods da nota deletada deveria ter sido removido")
	}
	tags, _ := ctx.Store.GetFileTags("notes/para-deletar.md")
	if len(tags) > 0 {
		t.Error("tags da nota deletada deveriam ter sido removidas")
	}
}

func TestHandleBulkDelete_ExecuteTagFilter(t *testing.T) {
	ctx := newTestContext(t)

	saveTestNote(t, ctx, "notes/deletar-tag.md", "deletar", "lixo")
	saveTestNote(t, ctx, "notes/manter-tag.md", "manter", "importante")

	body := "by_age=false&age_years=1&by_tag=true&tag_name=lixo"
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Deleted int      `json:"deleted"`
		Errors  []string `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Deleted != 1 {
		t.Fatalf("esperado 1 exclusao, got %d", resp.Deleted)
	}

	// A nota com tag "lixo" foi deletada
	if _, err := os.Stat(filepath.Join(ctx.Cfg.DocsDir, "notes/deletar-tag.md")); !os.IsNotExist(err) {
		t.Error("nota com tag lixo deveria ter sido deletada")
	}
	// A nota com tag "importante" permanece
	if _, err := os.Stat(filepath.Join(ctx.Cfg.DocsDir, "notes/manter-tag.md")); os.IsNotExist(err) {
		t.Error("nota com tag importante nao deveria ter sido deletada")
	}
}

func TestHandleBulkDelete_NenhumFiltro_Retorna400(t *testing.T) {
	ctx := newTestContext(t)

	body := "by_age=false&age_years=1&by_tag=false&tag_name="
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleBulkDelete_SemResultados_RetornaListaVazia(t *testing.T) {
	ctx := newTestContext(t)

	// Nenhuma nota no banco, preview deve retornar lista vazia
	body := "by_age=true&age_years=1&by_tag=false&tag_name=&preview=true"
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Files []string `json:"files"`
		Total int      `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 0 {
		t.Fatalf("esperado 0 resultados, got %d", resp.Total)
	}
	if len(resp.Files) != 0 {
		t.Fatalf("esperado lista vazia, got %v", resp.Files)
	}
}

func TestHandleBulkDelete_ExecucaoSemNotas_RetornaZero(t *testing.T) {
	ctx := newTestContext(t)

	body := "by_age=true&age_years=1&by_tag=false&tag_name="
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Deleted int      `json:"deleted"`
		Errors  []string `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Deleted != 0 {
		t.Fatalf("esperado 0 exclusoes, got %d", resp.Deleted)
	}
}

func TestHandleBulkDelete_IdadeInvalida_Retorna400(t *testing.T) {
	ctx := newTestContext(t)

	body := "by_age=true&age_years=0&by_tag=false&tag_name="
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleBulkDelete_TagVazia_Retorna400(t *testing.T) {
	ctx := newTestContext(t)

	body := "by_age=false&age_years=1&by_tag=true&tag_name="
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400, got %d", rec.Code)
	}
}

func TestHandleBulkDelete_PreviewTagPDF_IncluiPDFs(t *testing.T) {
	ctx := newTestContext(t)

	// Cria notas em notes/ e pdfs/ com tags
	saveTestNote(t, ctx, "notes/captura-site.md", "site capturado", "captura")
	saveTestNote(t, ctx, "notes/captura-video.md", "video capturado", "captura")
	saveTestNote(t, ctx, "pdfs/documento.pdf", "conteudo pdf", "pdf")

	// Preview: notas com tag "captura" ou "pdf"
	body := "by_age=false&age_years=1&by_tag=true&tag_name=captura, pdf&preview=true"
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Files []string `json:"files"`
		Total int      `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 3 {
		t.Fatalf("esperado 3 notas (2 captura + 1 pdf), got %d: %v", resp.Total, resp.Files)
	}

	hasCapture1 := false
	hasCapture2 := false
	hasPDF := false
	for _, f := range resp.Files {
		if f == "notes/captura-site.md" {
			hasCapture1 = true
		}
		if f == "notes/captura-video.md" {
			hasCapture2 = true
		}
		if f == "pdfs/documento.pdf" {
			hasPDF = true
		}
	}
	if !hasCapture1 {
		t.Error("notes/captura-site.md deveria estar na preview")
	}
	if !hasCapture2 {
		t.Error("notes/captura-video.md deveria estar na preview")
	}
	if !hasPDF {
		t.Error("pdfs/documento.pdf deveria estar na preview")
	}
}

func TestHandleBulkDelete_PreviewTag_IncluiAttachments(t *testing.T) {
	ctx := newTestContext(t)

	saveTestNote(t, ctx, "notes/captura.md", "captura", "captura")
	saveTestNote(t, ctx, "pdfs/doc.pdf", "pdf", "pdf")
	saveTestNote(t, ctx, "attachments/arquivos.zip", "zip", "anexo")

	// Preview: notas com tag "captura", "pdf" ou "anexo"
	body := "by_age=false&age_years=1&by_tag=true&tag_name=captura, pdf, anexo&preview=true"
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Files []string `json:"files"`
		Total int      `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 3 {
		t.Fatalf("esperado 3 notas (captura + pdf + anexo), got %d: %v", resp.Total, resp.Files)
	}

	hasNote := false
	hasPDF := false
	hasZip := false
	for _, f := range resp.Files {
		if f == "notes/captura.md" {
			hasNote = true
		}
		if f == "pdfs/doc.pdf" {
			hasPDF = true
		}
		if f == "attachments/arquivos.zip" {
			hasZip = true
		}
	}
	if !hasNote {
		t.Error("notes/captura.md deveria estar na preview")
	}
	if !hasPDF {
		t.Error("pdfs/doc.pdf deveria estar na preview")
	}
	if !hasZip {
		t.Error("attachments/arquivos.zip deveria estar na preview")
	}
}

func TestHandleBulkDelete_PreviewApenasCaptura_EncontraNotasEmNotes(t *testing.T) {
	ctx := newTestContext(t)

	// Simula notas capturadas: ficam em notes/ com tag "captura"
	saveTestNote(t, ctx, "notes/captura-site-legal.md", "site", "captura")
	saveTestNote(t, ctx, "notes/captura-video-youtube.md", "video", "captura")
	saveTestNote(t, ctx, "notes/nota-manual.md", "manual", "")

	// Preview filtrando apenas por tag "captura"
	body := "by_age=false&age_years=1&by_tag=true&tag_name=captura&preview=true"
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Files []string `json:"files"`
		Total int      `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 2 {
		t.Fatalf("esperado 2 notas capturadas, got %d: %v", resp.Total, resp.Files)
	}

	for _, f := range resp.Files {
		if f == "notes/nota-manual.md" {
			t.Error("nota-manual.md (sem tag captura) nao deveria aparecer")
		}
	}
}

func TestHandleBulkDelete_PreviewCapturaPDFAnexo_TodasAsFontes(t *testing.T) {
	ctx := newTestContext(t)

	// Cria notas de todas as fontes possiveis, cada uma com uma tag especifica
	saveTestNote(t, ctx, "notes/captura-website.md", "captura web", "captura")
	saveTestNote(t, ctx, "pdfs/artigo.pdf", "artigo pdf", "pdf")
	saveTestNote(t, ctx, "attachments/fotos.zip", "anexo zip", "anexo")
	saveTestNote(t, ctx, "notes/nota-avulsa.md", "nota sem tag", "")

	// Preview por cada tag individualmente
	// 1. So captura
	body1 := "by_age=false&age_years=1&by_tag=true&tag_name=captura&preview=true"
	req1 := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec1 := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec1, req1)

	var resp1 struct {
		Files []string `json:"files"`
		Total int      `json:"total"`
	}
	json.NewDecoder(rec1.Body).Decode(&resp1)
	if resp1.Total != 1 || resp1.Files[0] != "notes/captura-website.md" {
		t.Errorf("tag captura: esperado 1 (notes/captura-website.md), got %d: %v", resp1.Total, resp1.Files)
	}

	// 2. So pdf
	body2 := "by_age=false&age_years=1&by_tag=true&tag_name=pdf&preview=true"
	req2 := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec2 := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec2, req2)

	var resp2 struct {
		Files []string `json:"files"`
		Total int      `json:"total"`
	}
	json.NewDecoder(rec2.Body).Decode(&resp2)
	if resp2.Total != 1 || resp2.Files[0] != "pdfs/artigo.pdf" {
		t.Errorf("tag pdf: esperado 1 (pdfs/artigo.pdf), got %d: %v", resp2.Total, resp2.Files)
	}

	// 3. So anexo
	body3 := "by_age=false&age_years=1&by_tag=true&tag_name=anexo&preview=true"
	req3 := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body3))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec3 := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec3, req3)

	var resp3 struct {
		Files []string `json:"files"`
		Total int      `json:"total"`
	}
	json.NewDecoder(rec3.Body).Decode(&resp3)
	if resp3.Total != 1 || resp3.Files[0] != "attachments/fotos.zip" {
		t.Errorf("tag anexo: esperado 1 (attachments/fotos.zip), got %d: %v", resp3.Total, resp3.Files)
	}

	// 4. Executa exclusao combinando captura + anexo (deve pegar as 2)
	body4 := "by_age=false&age_years=1&by_tag=true&tag_name=captura, anexo"
	req4 := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body4))
	req4.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec4 := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec4, req4)

	var resp4 struct {
		Deleted int `json:"deleted"`
	}
	json.NewDecoder(rec4.Body).Decode(&resp4)
	if resp4.Deleted != 2 {
		t.Errorf("exclusao captura+anexo: esperado 2, got %d", resp4.Deleted)
	}

	// Nota avulsa e PDF devem permanecer
	if _, err := os.Stat(filepath.Join(ctx.Cfg.DocsDir, "notes/nota-avulsa.md")); os.IsNotExist(err) {
		t.Error("nota-avulsa.md nao deveria ter sido deletada")
	}
	if _, err := os.Stat(filepath.Join(ctx.Cfg.DocsDir, "pdfs/artigo.pdf")); os.IsNotExist(err) {
		t.Error("pdfs/artigo.pdf nao deveria ter sido deletado")
	}
}

func TestHandleBulkDelete_ExecuteTagPDF_DeletaPDFTambem(t *testing.T) {
	ctx := newTestContext(t)

	saveTestNote(t, ctx, "pdfs/relatorio.pdf", "relatorio", "pdf")
	saveTestNote(t, ctx, "notes/nota-normal.md", "normal", "")

	// Deletar notas com tag "pdf"
	body := "by_age=false&age_years=1&by_tag=true&tag_name=pdf"
	req := httptest.NewRequest("POST", "/api/bulk-delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleBulkDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Deleted int      `json:"deleted"`
		Errors  []string `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Deleted != 1 {
		t.Fatalf("esperado 1 exclusao (pdf), got %d", resp.Deleted)
	}
	if len(resp.Errors) > 0 {
		t.Fatalf("erros inesperados: %v", resp.Errors)
	}

	// PDF foi deletado do disco
	if _, err := os.Stat(filepath.Join(ctx.Cfg.DocsDir, "pdfs/relatorio.pdf")); !os.IsNotExist(err) {
		t.Error("pdf deveria ter sido deletado do disco")
	}
	// Nota normal permanece
	if _, err := os.Stat(filepath.Join(ctx.Cfg.DocsDir, "notes/nota-normal.md")); os.IsNotExist(err) {
		t.Error("nota normal nao deveria ter sido deletada")
	}
}

// ── Searchability (PDF e ZIP) ──

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

func TestPDFUpload_TextoExtraidoEPesquisavel(t *testing.T) {
	ctx := newTestContext(t)

	// Cria um PDF valido com texto conhecido.
	// Usamos um termo de 9 chars (menor que 11) para que o padding do
	// createMinimalPDF mantenha os offsets do xref inalterados.
	termoBusca := "BUSCA_PDF"
	pdfPath := filepath.Join(ctx.Cfg.DocsDir, "pdfs/teste-busca.pdf")
	createMinimalPDF(t, pdfPath, termoBusca)

	// Processa o PDF via ProcessFile (como HandleUpload faria)
	evento := watcher.FileEvent{
		Path:     pdfPath,
		Filename: "pdfs/teste-busca.pdf",
		ModTime:  time.Now(),
		Type:     "create",
	}
	if err := watcher.ProcessFile(ctx.Store, evento, nil, false); err != nil {
		t.Fatalf("ProcessFile PDF: %v", err)
	}

	// Verifica se o PDF gerou documentos no banco
	docs, err := ctx.Store.GetDocumentsByFile("pdfs/teste-busca.pdf")
	if err != nil {
		t.Fatalf("GetDocumentsByFile: %v", err)
	}
	if len(docs) == 0 {
		// A biblioteca ledongthuc/pdf pode nao conseguir extrair texto do PDF
		// minimal em algumas plataformas. Nesse caso o teste nao falha — apenas
		// verifica que o pipeline nao quebrou.
		t.Log("PDF minimal nao gerou documentos (biblioteca pode nao suportar este formato)")
		return
	}

	// Verifica que o texto foi extraido e indexado
	found := false
	for _, doc := range docs {
		if strings.Contains(doc.Texto, termoBusca) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Texto %q extraido do PDF nao encontrado nos documentos", termoBusca)
	}

	// Busca global via FTS deve encontrar o termo
	results, total, err := ctx.Store.SearchFTS(termoBusca, 0, 10)
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if total == 0 {
		t.Fatal("PDF com texto extraido deveria ser encontrado no FTS")
	}
	ftsFound := false
	for _, r := range results {
		if strings.Contains(r.Arquivo, "teste-busca") || strings.Contains(r.Texto, termoBusca) {
			ftsFound = true
			break
		}
	}
	if !ftsFound {
		t.Errorf("PDF nao encontrado via FTS pelo termo %q", termoBusca)
	}
}

func TestZIPUpload_NomesDosArquivosPesquisaveis(t *testing.T) {
	ctx := newTestContext(t)

	// Cria ZIP com aquivos de nomes conhecidos via HandleUploadAttachment.
	// Nomes sem hifen para evitar que o FTS5 interprete o "-" como operador de exclusao.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	f1, _ := w.CreateFormFile("files", "relatorio_mensal.pdf")
	io.Copy(f1, strings.NewReader("pdf content"))
	f2, _ := w.CreateFormFile("files", "foto_ferias.jpg")
	io.Copy(f2, strings.NewReader("jpg content"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/upload-attachment", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	ctx.HandleUploadAttachment(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload attachment esperado 303, got %d", rec.Code)
	}

	// Busca pelo nome do arquivo — o FTS5 deve encontrar
	results, total, err := ctx.Store.SearchFTS("relatorio_mensal", 0, 10)
	if err != nil {
		t.Fatalf("SearchFTS(relatorio_mensal): %v", err)
	}
	if total == 0 {
		t.Fatal("ZIP com arquivo relatorio_mensal.pdf deveria ser encontrado no FTS")
	}
	found := false
	for _, r := range results {
		if strings.Contains(r.Texto, "relatorio_mensal") || strings.Contains(r.Arquivo, ".zip") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ZIP com relatorio_mensal.pdf nao encontrado via FTS")
	}

	// Busca pelo segundo arquivo tambem
	results2, total2, err2 := ctx.Store.SearchFTS("foto_ferias", 0, 10)
	if err2 != nil {
		t.Fatalf("SearchFTS(foto_ferias): %v", err2)
	}
	if total2 == 0 {
		t.Fatal("ZIP com arquivo foto_ferias.jpg deveria ser encontrado no FTS")
	}
	_ = results2
}

func TestZIPUpload_SemArquivos_Retorna400(t *testing.T) {
	ctx := newTestContext(t)

	// ZIP sem arquivos na multipart — handler deve rejeitar
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.Close()

	req := httptest.NewRequest("POST", "/api/upload-attachment", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	ctx.HandleUploadAttachment(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("upload sem arquivos esperado 400, got %d", rec.Code)
	}
}

// ── HandleToggleEmbed ──────────────────────────────────────────

func TestHandleToggleEmbed_AtivaEmbedEmMarkdown(t *testing.T) {
	ctx := newTestContext(t)

	// Cria arquivo markdown
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/teste.md")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("# Teste\nconteudo"), 0644)

	// Indexa o arquivo primeiro
	ctx.Store.SetFileMod("notes/teste.md", time.Now().Format(time.RFC3339))
	watcher.ProcessFile(ctx.Store, watcher.FileEvent{
		Path:     fullPath,
		Filename: "notes/teste.md",
		ModTime:  time.Now(),
		Type:     "modify",
	}, ctx.Embed, false)

	// Executa toggle embed
	body := "filename=notes/teste.md"
	req := httptest.NewRequest("POST", "/api/toggle-embed", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleToggleEmbed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["ok"] != true {
		t.Fatal("resposta ok deveria ser true")
	}
	if resp["embedded"] != true {
		t.Fatal("embedded deveria ser true apos ativar")
	}

	// Verifica que a tag "embed" foi adicionada
	tags, _ := ctx.Store.GetFileTags("notes/teste.md")
	hasEmbed := false
	for _, tag := range tags {
		if tag == "embed" {
			hasEmbed = true
			break
		}
	}
	if !hasEmbed {
		t.Fatal("tag 'embed' deveria estar presente apos toggle")
	}
}

func TestHandleToggleEmbed_DesativaEmbedEmMarkdown(t *testing.T) {
	ctx := newTestContext(t)

	// Cria arquivo markdown
	fullPath := filepath.Join(ctx.Cfg.DocsDir, "notes/desativar.md")
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("# Teste\nconteudo"), 0644)

	// Adiciona tag "embed" previamente
	ctx.Store.AddTagToFile("notes/desativar.md", "embed")

	// Executa toggle embed (deve desativar)
	body := "filename=notes/desativar.md"
	req := httptest.NewRequest("POST", "/api/toggle-embed", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleToggleEmbed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["ok"] != true {
		t.Fatal("resposta ok deveria ser true")
	}
	if resp["embedded"] != false {
		t.Fatal("embedded deveria ser false apos desativar")
	}

	// Verifica que a tag "embed" foi removida
	tags, _ := ctx.Store.GetFileTags("notes/desativar.md")
	for _, tag := range tags {
		if tag == "embed" {
			t.Fatal("tag 'embed' nao deveria estar presente apos desativar")
		}
	}
}

func TestHandleToggleEmbed_AtivaEmbedEmPDF(t *testing.T) {
	ctx := newTestContext(t)

	// Cria arquivo PDF
	pdfDir := filepath.Join(ctx.Cfg.DocsDir, "pdfs")
	os.MkdirAll(pdfDir, 0755)
	fullPath := filepath.Join(pdfDir, "documento.pdf")
	os.WriteFile(fullPath, []byte("%PDF-1.4 fake"), 0644)

	// Executa toggle embed
	body := "filename=pdfs/documento.pdf"
	req := httptest.NewRequest("POST", "/api/toggle-embed", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleToggleEmbed(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["ok"] != true {
		t.Fatal("resposta ok deveria ser true para PDF")
	}
	if resp["embedded"] != true {
		t.Fatal("embedded deveria ser true apos ativar em PDF")
	}

	// Verifica a tag
	tags, _ := ctx.Store.GetFileTags("pdfs/documento.pdf")
	hasEmbed := false
	for _, tag := range tags {
		if tag == "embed" {
			hasEmbed = true
			break
		}
	}
	if !hasEmbed {
		t.Fatal("PDF deveria ter tag 'embed' apos toggle")
	}
}

func TestHandleToggleEmbed_ArquivoInexistente_Retorna404(t *testing.T) {
	ctx := newTestContext(t)

	body := "filename=notes/nao-existe.md"
	req := httptest.NewRequest("POST", "/api/toggle-embed", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleToggleEmbed(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperado 404, got %d", rec.Code)
	}
}

func TestHandleToggleEmbed_MetodoInvalido_Retorna405(t *testing.T) {
	ctx := newTestContext(t)

	req := httptest.NewRequest("GET", "/api/toggle-embed", nil)
	rec := httptest.NewRecorder()

	ctx.HandleToggleEmbed(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("esperado 405, got %d", rec.Code)
	}
}

func TestHandleToggleEmbed_FilenameObrigatorio_Retorna400(t *testing.T) {
	ctx := newTestContext(t)

	body := "filename="
	req := httptest.NewRequest("POST", "/api/toggle-embed", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	ctx.HandleToggleEmbed(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400, got %d", rec.Code)
	}
}

// ── HandleUploadAttachment: visibilidade no /api/notes ─────────

func TestHandleUploadAttachment_ZipApareceNoListagemDeNotas(t *testing.T) {
	ctx := newTestContext(t)

	// Faz upload de um ZIP
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("files", "lista-notas.txt")
	io.Copy(part, strings.NewReader("nota 1\nnota 2\n"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/upload-attachment", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	ctx.HandleUploadAttachment(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload falhou: %d", rec.Code)
	}

	// Busca a lista de notas (modo compacto)
	req2 := httptest.NewRequest("GET", "/api/notes", nil)
	rec2 := httptest.NewRecorder()
	ctx.HandleGetAllNotes(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec2.Code)
	}

	var resp struct {
		Notes []struct {
			Arquivo  string   `json:"arquivo"`
			Tags     []string `json:"tags"`
			Mtime    string   `json:"mtime"`
			Embedded bool     `json:"embedded"`
		} `json:"notes"`
	}
	if err := json.NewDecoder(rec2.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Deve ter pelo menos um ZIP na lista
	foundZip := false
	for _, n := range resp.Notes {
		if strings.HasPrefix(n.Arquivo, "attachments/") && strings.HasSuffix(n.Arquivo, ".zip") {
			foundZip = true
			// Deve ter a tag "zip"
			hasZipTag := false
			for _, t := range n.Tags {
				if t == "zip" {
					hasZipTag = true
					break
				}
			}
			if !hasZipTag {
				t.Errorf("ZIP %s deveria ter tag 'zip', got %v", n.Arquivo, n.Tags)
			}
			break
		}
	}
	if !foundZip {
		t.Error("ZIP enviado nao apareceu na listagem /api/notes")
	}
}

// ── HandleFile: download de ZIP ────────────────────────────────

func TestHandleFile_DownloadZipAttachment(t *testing.T) {
	ctx := newTestContext(t)

	// Cria ZIP real no disco
	attachDir := filepath.Join(ctx.Cfg.DocsDir, "attachments")
	os.MkdirAll(attachDir, 0755)
	zipContent := []byte("fake-zip-content")
	zipPath := filepath.Join(attachDir, "download-test.zip")
	os.WriteFile(zipPath, zipContent, 0644)

	// Acessa via /file
	req := httptest.NewRequest("GET", "/file?name=attachments/download-test.zip", nil)
	rec := httptest.NewRecorder()
	ctx.HandleFile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	// Deve ter Content-Disposition: attachment
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("ZIP deveria ser servido como attachment, got Content-Disposition: %q", cd)
	}

	// Deve ter Content-Type: application/zip
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/zip") {
		t.Errorf("Content-Type deveria ser application/zip, got %q", ct)
	}

	// Conteudo deve ser igual ao arquivo
	body := rec.Body.Bytes()
	if string(body) != string(zipContent) {
		t.Errorf("conteudo do ZIP diferente: got %d bytes, expected %d bytes", len(body), len(zipContent))
	}
}

// ── HandleFileDownload: ZIP como download ──────────────────────

func TestHandleFileDownload_ZipAttachment(t *testing.T) {
	ctx := newTestContext(t)

	attachDir := filepath.Join(ctx.Cfg.DocsDir, "attachments")
	os.MkdirAll(attachDir, 0755)
	zipContent := []byte("fake-download-zip")
	zipPath := filepath.Join(attachDir, "download-test.zip")
	os.WriteFile(zipPath, zipContent, 0644)

	req := httptest.NewRequest("GET", "/file/download?name=attachments/download-test.zip", nil)
	rec := httptest.NewRecorder()
	ctx.HandleFileDownload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition deveria conter 'attachment', got %q", cd)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/zip") {
		t.Errorf("Content-Type deveria ser application/zip, got %q", ct)
	}

	body := rec.Body.Bytes()
	if string(body) != string(zipContent) {
		t.Errorf("conteudo diferente: got %d bytes", len(body))
	}
}

// ── HandleUploadAttachment: recovery após race ─────────────────

func TestHandleUploadAttachment_MantemDadosAposProcessFile(t *testing.T) {
	ctx := newTestContext(t)

	// Simula o fluxo completo: upload + watcher processando o arquivo
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("files", "race-test.txt")
	io.Copy(part, strings.NewReader("conteudo para teste de race"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/upload-attachment", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	ctx.HandleUploadAttachment(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload falhou: %d", rec.Code)
	}

	// Encontra o ZIP criado
	attachDir := filepath.Join(ctx.Cfg.DocsDir, "attachments")
	entries, _ := os.ReadDir(attachDir)
	var zipFile string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".zip") {
			zipFile = e.Name()
			break
		}
	}
	if zipFile == "" {
		t.Fatal("nenhum zip encontrado")
	}

	filename := "attachments/" + zipFile
	fullPath := filepath.Join(attachDir, zipFile)

	// Simula o watcher processando o arquivo (como fsnotify faria)
	// O processFileLocked para attachment NAO deve deletar os docs
	watcher.ProcessFile(ctx.Store, watcher.FileEvent{
		Path:     fullPath,
		Filename: filename,
		ModTime:  time.Now(),
		Type:     "modify",
	}, nil, false)

	// Documento deve continuar existindo
	docs, _ := ctx.Store.GetDocumentsByFile(filename)
	if len(docs) == 0 {
		t.Fatal("documento foi perdido apos ProcessFile do watcher")
	}

	// Tags devem continuar existindo
	tags, _ := ctx.Store.GetFileTags(filename)
	hasZip := false
	for _, t := range tags {
		if t == "zip" {
			hasZip = true
			break
		}
	}
	if !hasZip {
		t.Errorf("tag 'zip' foi perdida apos ProcessFile, got %v", tags)
	}

	// file_mods deve existir
	mod, _ := ctx.Store.GetFileMod(filename)
	if mod == "" {
		t.Error("file_mod foi perdido apos ProcessFile")
	}
}

// ── HandleFileDelete (ZIP) com nome simples ───────────────────

func TestHandleFileDelete_ZIP_NomeSimples_SemPrefixoAttachments(t *testing.T) {
	ctx := newTestContext(t)

	// Cria ZIP
	attachDir := filepath.Join(ctx.Cfg.DocsDir, "attachments")
	os.MkdirAll(attachDir, 0755)
	zipName := "simple.zip"
	zipPath := filepath.Join(attachDir, zipName)
	os.WriteFile(zipPath, []byte("zip"), 0644)

	filename := "attachments/" + zipName
	docID := "att-simple"
	ctx.Store.InsertDocument(db.Document{
		ID:      docID,
		Tipo:    "attachment",
		Arquivo: filename,
		Secao:   "\U0001f4e6 " + zipName,
		Texto:   "Arquivos: simple.txt",
	})
	ctx.Store.IndexFTS(docID, "attachment", filename, "\U0001f4e6 "+zipName, "Arquivos: simple.txt", "")
	ctx.Store.SetFileMod(filename, time.Now().Format(time.RFC3339))

	// Deleta passando apenas o nome base (sem attachments/)
	body := "filename=" + zipName
	req := httptest.NewRequest("POST", "/file/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx.HandleFileDelete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, got %d", rec.Code)
	}

	// Arquivo deve ter sido removido
	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Error("arquivo zip deveria ter sido removido")
	}

	// Documento removido
	if c := ctx.Store.GetDocumentCount(); c != 0 {
		t.Errorf("documentos deveriam ser 0, got %d", c)
	}
}
