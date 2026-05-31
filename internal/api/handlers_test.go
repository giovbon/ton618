package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
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

// saveTestNote cria uma nota de teste no disco, no banco (notes table) e registra metadados.
func saveTestNote(t *testing.T, ctx *HandlerContext, filename, content, tags string) {
	t.Helper()
	// Write to disk (for backwards compat with download handlers, etc)
	fullPath := filepath.Join(ctx.Cfg.DocsDir, filename)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
	// Save to notes table
	ctx.Store.SaveNote(filename, content, time.Now().Format(time.RFC3339))
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
