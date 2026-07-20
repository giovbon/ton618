package todos

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"path/filepath"

	"ton618/core/internal/core/config"
	"ton618/core/internal/core/db"
)

// setupTest envia um HandlerContext mockado
func setupTest(t *testing.T) *HandlerContext {
	t.Helper()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test.db")
	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	cfg := &config.AppConfig{DocsDir: t.TempDir()}
	return NewHandlerContext(cfg, store)
}

func TestHandleGetTodoMarkers(t *testing.T) {
	ctx := setupTest(t)

	req, err := http.NewRequest("GET", "/todos/markers", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	
	ctx.HandleGetTodoMarkers(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("status incorreto: got %v want %v", status, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "TODO") {
		t.Errorf("HTML de resposta nao contem o default marker TODO. Body: %s", body)
	}
}

func TestHandleAddTodoMarker(t *testing.T) {
	ctx := setupTest(t)

	form := url.Values{}
	form.Add("marker", "URGENTE")
	form.Add("color", "#ff0000")

	req, err := http.NewRequest("POST", "/todos/markers/add", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	ctx.HandleAddTodoMarker(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("status incorreto: got %v want %v", status, http.StatusOK)
	}

	// Verifica banco
	markers, _ := ctx.Store.GetTodoMarkers()
	found := false
	for _, m := range markers {
		if m.Marker == "URGENTE" {
			found = true
			if m.Color != "#ff0000" {
				t.Errorf("cor errada: got %v", m.Color)
			}
		}
	}
	if !found {
		t.Error("URGENTE nao foi salvo no DB")
	}

	body := rr.Body.String()
	if !strings.Contains(body, "URGENTE") {
		t.Error("Resposta HTMX nao contem o novo marker")
	}
}

func TestHandleUpdateTodoMarker(t *testing.T) {
	ctx := setupTest(t)

	// Garantir q o TODO default existe
	markers, _ := ctx.Store.GetTodoMarkers()
	if len(markers) == 0 {
		t.Fatal("Sem markers defaults")
	}

	form := url.Values{}
	form.Add("color", "#111111")

	req, err := http.NewRequest("POST", "/todos/markers/update?marker=TODO&active=false", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	ctx.HandleUpdateTodoMarker(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("status incorreto: got %v want %v", status, http.StatusOK)
	}

	updated, _ := ctx.Store.GetTodoMarkers()
	for _, m := range updated {
		if m.Marker == "TODO" {
			if m.Color != "#111111" {
				t.Errorf("cor nao foi atualizada: %s", m.Color)
			}
			if m.Active {
				t.Error("ativo deveria estar falso")
			}
		}
	}
}

func TestHandleRemoveTodoMarker(t *testing.T) {
	ctx := setupTest(t)

	req, err := http.NewRequest("POST", "/todos/markers/remove?marker=TODO", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()

	ctx.HandleRemoveTodoMarker(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("status incorreto: got %v want %v", status, http.StatusOK)
	}

	updated, _ := ctx.Store.GetTodoMarkers()
	for _, m := range updated {
		if m.Marker == "TODO" {
			t.Error("Marker TODO ainda existe no DB")
		}
	}
}
