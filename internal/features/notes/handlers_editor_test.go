package notes

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// ── HandleEditor ─────────────────────────────────────────────────────

func TestHandleEditor_SemArquivo_GeraNovoFilename(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/editor", nil)

	ctx.HandleEditor(rec, req)

	// Sem ?file= deve render 200 (novo rascunho)
	if rec.Code != 200 {
		t.Errorf("esperado 200 para novo rascunho, got %d", rec.Code)
	}
}

func TestHandleEditor_ArquivoExistente(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/minha-nota.md", "# Minha Nota\n\nConteudo aqui.", "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/editor?file=notes/minha-nota.md", nil)

	ctx.HandleEditor(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200 para nota existente, got %d", rec.Code)
	}
}

func TestHandleEditor_ZipRedireciona(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/editor?file=attachments/arquivo.zip", nil)

	ctx.HandleEditor(rec, req)

	// ZIP deve redirecionar para /file/download
	if rec.Code != 302 {
		t.Errorf("esperado 302 para ZIP, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/file/download") {
		t.Errorf("esperado redirect para /file/download, got %q", loc)
	}
}

func TestHandleEditor_FilenameNormalizadoRedireciona(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	// Sem prefixo notes/ e sem .md → deve normalizar e redirecionar
	req := httptest.NewRequest("GET", "/editor?file=minha-nota", nil)

	ctx.HandleEditor(rec, req)

	if rec.Code != 302 {
		t.Errorf("esperado 302 para filename sem prefixo, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "notes/") || !strings.Contains(loc, ".md") {
		t.Errorf("esperado redirect para filename normalizado, got %q", loc)
	}
}

func TestHandleEditor_NotaDoTipoDrawingRedireciona(t *testing.T) {
	ctx := newTestContext(t)
	// Nota com tag de drawing deve redirecionar para /drawing
	saveTestNote(t, ctx, "notes/meu-desenho.md", "---\ntype: drawing\n---\n", "type-drawing")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/editor?file=notes/meu-desenho.md", nil)

	ctx.HandleEditor(rec, req)

	if rec.Code != 302 {
		t.Errorf("esperado 302 para nota do tipo drawing, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/drawing") {
		t.Errorf("esperado redirect para /drawing, got %q", loc)
	}
}

// ── HandleDrawing ─────────────────────────────────────────────────────

func TestHandleDrawing_NovaNotaRenderiza(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/drawing?file=notes/novo-desenho.md", nil)

	ctx.HandleDrawing(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200 para novo desenho, got %d", rec.Code)
	}
}

func TestHandleDrawing_SemFilenameGeraNovoRascunho(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/drawing", nil)

	ctx.HandleDrawing(rec, req)

	// ensureNoteFilename sem ?file= gera um novo filename CUID2 já normalizado
	// (notes/xxxxxxx.md), portanto não redireciona — renderiza o editor normalmente (200)
	if rec.Code != 200 {
		t.Errorf("esperado 200 ao acessar /drawing sem file= (novo rascunho), got %d", rec.Code)
	}
}

func TestHandleDrawing_NotaNormalRedireciona(t *testing.T) {
	ctx := newTestContext(t)
	// Nota markdown comum não deve ser editada como drawing
	saveTestNote(t, ctx, "notes/nota-comum.md", "# Nota comum\n\nTexto normal.", "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/drawing?file=notes/nota-comum.md", nil)

	ctx.HandleDrawing(rec, req)

	// Nota comum tem EditorRoute=/editor, então deve redirecionar de /drawing para /editor
	if rec.Code != 302 {
		t.Errorf("esperado 302 ao tentar abrir nota comum no drawing, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/editor") {
		t.Errorf("esperado redirect para /editor, got %q", loc)
	}
}

// ── HandleMindmap ─────────────────────────────────────────────────────

func TestHandleMindmap_NovaNotaRenderiza(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/mindmap?file=notes/meu-mapa.md", nil)

	ctx.HandleMindmap(rec, req)

	if rec.Code != 200 {
		t.Errorf("esperado 200 para novo mindmap, got %d", rec.Code)
	}
}

func TestHandleMindmap_ConteudoDefaultParaNovaNota(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/mindmap?file=notes/mapa-novo.md", nil)

	ctx.HandleMindmap(rec, req)

	body := rec.Body.String()
	// O conteúdo default deve incluir o marcador de markmap
	if !strings.Contains(body, "markmap") {
		t.Errorf("esperado conteudo default com 'markmap', got body com %d chars", len(body))
	}
}

func TestHandleMindmap_NotaNormalRedireciona(t *testing.T) {
	ctx := newTestContext(t)
	saveTestNote(t, ctx, "notes/nota-comum.md", "# Nota comum\n\nTexto normal.", "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/mindmap?file=notes/nota-comum.md", nil)

	ctx.HandleMindmap(rec, req)

	// nota comum tem EditorRoute=/editor, deve redirecionar de /mindmap
	if rec.Code != 302 {
		t.Errorf("esperado 302 ao tentar abrir nota comum no mindmap, got %d", rec.Code)
	}
}

func TestHandleMindmap_CacheControlSemCache(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/mindmap?file=notes/mapa.md", nil)

	ctx.HandleMindmap(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if !strings.Contains(cc, "no-cache") {
		t.Errorf("esperado Cache-Control com no-cache, got %q", cc)
	}
}
