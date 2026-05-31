package api

import (
	"net/http/httptest"
	"testing"
)

// ── render error paths ──────────────────────────────────────────

func TestRender_NilTemplates(t *testing.T) {
	// Cria um HandlerContext SEM templates
	ctx := &HandlerContext{}
	rec := httptest.NewRecorder()

	ctx.render(rec, "nonexistent.html", nil)

	if rec.Code != 500 {
		t.Errorf("esperado 500 para templates nil, got %d", rec.Code)
	}
	if rec.Body.String() != "templates not loaded\n" {
		t.Errorf("esperado 'templates not loaded', got %q", rec.Body.String())
	}
}

func TestRenderPartial_NilTemplates(t *testing.T) {
	ctx := &HandlerContext{}
	rec := httptest.NewRecorder()

	ctx.renderPartial(rec, "nonexistent.html", nil)

	if rec.Code != 500 {
		t.Errorf("esperado 500 para templates nil, got %d", rec.Code)
	}
}

func TestRenderLogin_NilTemplates(t *testing.T) {
	ctx := &HandlerContext{}
	rec := httptest.NewRecorder()

	ctx.renderLogin(rec, "nonexistent.html", nil)

	if rec.Code != 500 {
		t.Errorf("esperado 500 para templates nil, got %d", rec.Code)
	}
}
