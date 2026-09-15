package db

import (
	"testing"
	"time"

	"ton618/core/internal/processor"
)

// saveTodo insere uma tarefa de teste no banco.
// Atenção: SaveFileTodos SUBSTITUI as tarefas do arquivo, então os itens são
// agrupados por arquivo e enviados numa única chamada.
func saveTodo(t *testing.T, s *Store, id, file, tipo, status, text string) {
	t.Helper()
	items := []processor.TodoItem{{
		ID:      id,
		File:    file,
		Section: "Geral",
		Type:    tipo,
		Status:  status,
		Text:    text,
		Line:    1,
		Created: time.Now().UTC(),
	}}
	if err := s.SaveFileTodos(file, items); err != nil {
		t.Fatalf("SaveFileTodos: %v", err)
	}
}

// saveTodosPorArquivo agrupa itens por arquivo (ver observação em saveTodo).
func saveTodosPorArquivo(t *testing.T, s *Store, byFile map[string][]processor.TodoItem) {
	t.Helper()
	for file, items := range byFile {
		for i := range items {
			items[i].File = file
			if items[i].Section == "" {
				items[i].Section = "Geral"
			}
			if items[i].Created.IsZero() {
				items[i].Created = time.Now().UTC()
			}
			if items[i].Status == "" {
				items[i].Status = "pending"
			}
		}
		if err := s.SaveFileTodos(file, items); err != nil {
			t.Fatalf("SaveFileTodos(%s): %v", file, err)
		}
	}
}

// TestCountTodosByMarkers garante que a contagem do badge soma apenas os tipos
// informados — é o que permite excluir marcadores da contagem (ex: DONE) e
// deixar de fora os checkboxes comuns (tipo "TASK").
func TestCountTodosByMarkers(t *testing.T) {
	s := newTestStore(t)

	saveTodosPorArquivo(t, s, map[string][]processor.TodoItem{
		"notes/a.md": {
			{ID: "t1", Type: "TODO", Text: "fazer A", Line: 1},
			{ID: "t2", Type: "TODO", Text: "fazer B", Line: 2},
		},
		"notes/b.md": {
			{ID: "d1", Type: "DOING", Text: "fazendo C", Line: 1},
			{ID: "x1", Type: "DONE", Text: "feito D", Line: 2},
		},
		// Checkbox comum de markdown (`- [ ]`) — nunca deve contar.
		"notes/c.md": {
			{ID: "k1", Type: "TASK", Text: "checkbox solto", Line: 1},
		},
	})

	tests := []struct {
		name    string
		markers []string
		want    int
	}{
		{"Todos os marcadores padrão", []string{"TODO", "DOING", "DONE"}, 4},
		{"Sem o DONE (fora da contagem)", []string{"TODO", "DOING"}, 3},
		{"Só TODO", []string{"TODO"}, 2},
		{"Marcador inexistente", []string{"URGENTE"}, 0},
		{"Lista vazia", nil, 0},
		{"Minúsculas são normalizadas", []string{"todo"}, 2},
		{"Espaços são ignorados", []string{"  TODO  "}, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.CountTodosByMarkers(tc.markers)
			if err != nil {
				t.Fatalf("CountTodosByMarkers: %v", err)
			}
			if got != tc.want {
				t.Errorf("CountTodosByMarkers(%v) = %d, want %d", tc.markers, got, tc.want)
			}
		})
	}

	// O tipo TASK continua no banco (só não é listado/contado).
	if got, _ := s.CountTodosByMarkers([]string{"TASK"}); got != 1 {
		t.Errorf("contagem explícita de TASK = %d, want 1 (a linha existe)", got)
	}
}

// TestTodoMarkers_CountInBadgeRoundTrip garante que a preferência de contagem
// por marcador é persistida e lida de volta.
func TestTodoMarkers_CountInBadgeRoundTrip(t *testing.T) {
	s := newTestStore(t)

	markers, err := s.GetTodoMarkers()
	if err != nil {
		t.Fatalf("GetTodoMarkers: %v", err)
	}
	if len(markers) == 0 {
		t.Fatal("esperava marcadores default")
	}

	// Default: todos contam (comportamento anterior preservado).
	for _, m := range markers {
		if !m.CountInBadge {
			t.Errorf("marcador %s deveria contar por padrão", m.Marker)
		}
	}

	// Desmarca o primeiro marcador.
	markers[0].CountInBadge = false
	markers[0].Color = "#123456"
	if err := s.SaveTodoMarkers(markers); err != nil {
		t.Fatalf("SaveTodoMarkers: %v", err)
	}

	saved, err := s.GetTodoMarkers()
	if err != nil {
		t.Fatalf("GetTodoMarkers após save: %v", err)
	}
	if len(saved) != len(markers) {
		t.Fatalf("esperava %d marcadores, got %d", len(markers), len(saved))
	}

	found := false
	for _, m := range saved {
		if m.Marker == markers[0].Marker {
			found = true
			if m.CountInBadge {
				t.Errorf("marcador %s deveria ter CountInBadge=false", m.Marker)
			}
		} else if !m.CountInBadge {
			t.Errorf("marcador %s não deveria ter sido afetado", m.Marker)
		}
	}
	if !found {
		t.Errorf("marcador %s sumiu após o save", markers[0].Marker)
	}

	// GetActiveTodoMarkers também precisa trazer a coluna.
	active, err := s.GetActiveTodoMarkers()
	if err != nil {
		t.Fatalf("GetActiveTodoMarkers: %v", err)
	}
	for _, m := range active {
		if m.Marker == markers[0].Marker && m.CountInBadge {
			t.Errorf("GetActiveTodoMarkers não trouxe CountInBadge=false")
		}
	}
}

// TestTodoMarkers_CountInBadgeColumnExiste garante que a migração v12 criou a coluna.
func TestTodoMarkers_CountInBadgeColumnExiste(t *testing.T) {
	s := newTestStore(t)
	if !s.hasMarkerColumn("count_in_badge") {
		t.Fatal("coluna count_in_badge não existe em todo_markers")
	}
	if !s.hasMarkerColumn("sort_order") {
		t.Fatal("coluna sort_order não existe em todo_markers")
	}
}
