package todos

import (
	"path/filepath"
	"testing"

	"ton618/core/internal/core/config"
	"ton618/core/internal/core/db"
	"ton618/core/internal/processor"
)

func newTestContext(t *testing.T) *HandlerContext {
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

func TestTodoMarkers_GetDefaultMarkers(t *testing.T) {
	ctx := newTestContext(t)

	markers, err := ctx.Store.GetTodoMarkers()
	if err != nil {
		t.Fatalf("GetTodoMarkers: %v", err)
	}
	if len(markers) == 0 {
		t.Fatal("esperava markers default, got 0")
	}

	// Verifica que os markers essenciais existem
	expected := map[string]bool{"TODO": false, "DOING": false, "DONE": false}
	for _, m := range markers {
		if _, ok := expected[m.Marker]; ok {
			expected[m.Marker] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("marker %s nao encontrado nos defaults", name)
		}
	}
}

func TestTodoMarkers_GetActiveMarkers(t *testing.T) {
	ctx := newTestContext(t)

	active, err := ctx.Store.GetActiveTodoMarkers()
	if err != nil {
		t.Fatalf("GetActiveTodoMarkers: %v", err)
	}
	if len(active) == 0 {
		t.Fatal("esperava pelo menos 1 marker ativo")
	}
	for _, m := range active {
		if !m.Active {
			t.Errorf("marker %s deveria estar ativo", m.Marker)
		}
	}
}

func TestTodoMarkers_SaveAndRetrieve(t *testing.T) {
	ctx := newTestContext(t)

	original, _ := ctx.Store.GetTodoMarkers()

	// Altera a cor do primeiro marker
	if len(original) > 0 {
		original[0].Color = "#ff0000"
		original[0].Active = false
		if err := ctx.Store.SaveTodoMarkers(original); err != nil {
			t.Fatalf("SaveTodoMarkers: %v", err)
		}

		saved, _ := ctx.Store.GetTodoMarkers()
		if saved[0].Color != "#ff0000" {
			t.Errorf("cor nao foi persistida: esperado #ff0000, got %s", saved[0].Color)
		}
		if saved[0].Active {
			t.Error("Active=false nao foi persistido")
		}
	}
}

func TestTodoMarkers_AddNewMarker(t *testing.T) {
	ctx := newTestContext(t)

	markers, _ := ctx.Store.GetTodoMarkers()
	newMarker := db.TodoMarker{Marker: "URGENTE", Color: "#ff0000", Active: true}
	markers = append(markers, newMarker)

	if err := ctx.Store.SaveTodoMarkers(markers); err != nil {
		t.Fatalf("SaveTodoMarkers: %v", err)
	}

	saved, _ := ctx.Store.GetTodoMarkers()
	found := false
	for _, m := range saved {
		if m.Marker == "URGENTE" {
			found = true
			if m.Color != "#ff0000" {
				t.Errorf("cor esperada #ff0000, got %s", m.Color)
			}
			break
		}
	}
	if !found {
		t.Fatal("novo marker URGENTE nao foi salvo")
	}
}

func TestTodoMarkers_RemoveMarker(t *testing.T) {
	ctx := newTestContext(t)

	markers, _ := ctx.Store.GetTodoMarkers()
	before := len(markers)

	// Remove o ultimo
	markers = markers[:len(markers)-1]
	if err := ctx.Store.SaveTodoMarkers(markers); err != nil {
		t.Fatalf("SaveTodoMarkers: %v", err)
	}

	saved, _ := ctx.Store.GetTodoMarkers()
	if len(saved) != before-1 {
		t.Errorf("esperado %d markers apos remocao, got %d", before-1, len(saved))
	}
}

// TestTodoMarkers_Reset verifica que reset restaura os markers originais.
// Usa seedDefaultMarkers do banco para comparar.
func TestTodoMarkers_Reset(t *testing.T) {
	ctx := newTestContext(t)

	// Altera drasticamente
	markers, _ := ctx.Store.GetTodoMarkers()
	markers = markers[:1] // mantem so 1
	markers[0].Color = "#000000"
	if err := ctx.Store.SaveTodoMarkers(markers); err != nil {
		t.Fatalf("SaveTodoMarkers: %v", err)
	}

	// Simula reset: deleta e deixa o seedDefaultMarkers recriar
	if err := ctx.resetAndSeed(); err != nil {
		t.Fatalf("reset: %v", err)
	}

	reset, _ := ctx.Store.GetTodoMarkers()
	if len(reset) != 3 {
		t.Errorf("apos reset esperado 3 markers, got %d", len(reset))
	}
}

func (ctx *HandlerContext) resetAndSeed() error {
	if _, err := ctx.Store.DB.Exec("DELETE FROM todo_markers"); err != nil {
		return err
	}
	var markers []db.TodoMarker
	for _, m := range processor.DefaultTodoMarkers {
		markers = append(markers, db.TodoMarker{
			Marker: m.Marker,
			Color:  m.Color,
			Active: m.Active,
		})
	}
	return ctx.Store.SaveTodoMarkers(markers)
}

// ── Testes de integração com TODOs extraídos de notas ──

func TestTodoMarkers_SaveFileTodosAndFilter(t *testing.T) {
	ctx := newTestContext(t)
	store := ctx.Store

	todos := []processor.TodoItem{
		{ID: "1", File: "notes/teste.md", Section: "Geral", Type: "TODO", Status: "pending", Text: "Fazer algo", Line: 3},
		{ID: "2", File: "notes/teste.md", Section: "Geral", Type: "FIXME", Status: "pending", Text: "Corrigir bug", Line: 5},
		{ID: "3", File: "notes/outro.md", Section: "Introducao", Type: "TODO", Status: "completed", Text: "Tarefa feita", Line: 2},
	}

	if err := store.SaveFileTodos("notes/teste.md", todos[:2]); err != nil {
		t.Fatalf("SaveFileTodos: %v", err)
	}
	if err := store.SaveFileTodos("notes/outro.md", todos[2:3]); err != nil {
		t.Fatalf("SaveFileTodos: %v", err)
	}

	// Filtro por tipo TODO
	filtered, err := store.GetTodosFiltered(map[string]bool{"TODO": true}, "")
	if err != nil {
		t.Fatalf("GetTodosFiltered: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("esperado 2 TODOs, got %d", len(filtered))
	}

	// Filtro por status completed
	completed, err := store.GetTodosFiltered(nil, "completed")
	if err != nil {
		t.Fatalf("GetTodosFiltered(completed): %v", err)
	}
	if len(completed) != 1 {
		t.Errorf("esperado 1 completed, got %d", len(completed))
	}

	// Deleta por arquivo
	if err := store.DeleteTodosByFile("notes/teste.md"); err != nil {
		t.Fatalf("DeleteTodosByFile: %v", err)
	}
	filtered2, _ := store.GetTodosFiltered(nil, "")
	if len(filtered2) != 1 {
		t.Errorf("esperado 1 TODO apos delete, got %d", len(filtered2))
	}
}
