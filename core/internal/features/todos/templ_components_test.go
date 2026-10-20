package todos

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"ton618/core/internal/core/db"
)

func TestMarkerURL(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		marker   string
		extra    []string
		contains []string
	}{
		{
			name:     "Marker simples sem extras",
			base:     "/api/todo-markers/update",
			marker:   "TODO",
			contains: []string{"/api/todo-markers/update?marker=TODO"},
		},
		{
			name:     "Marker com espacos e caracteres especiais",
			base:     "/api/todo-markers/update",
			marker:   "FAZER AGORA & URGENTE",
			contains: []string{"marker=FAZER+AGORA+%26+URGENTE"},
		},
		{
			name:     "Marker com parametros extras",
			base:     "/api/todo-markers/update",
			marker:   "DONE",
			extra:    []string{"active=false", "count_in_badge=true"},
			contains: []string{"marker=DONE", "&active=false", "&count_in_badge=true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markerURL(tt.base, tt.marker, tt.extra...)
			for _, exp := range tt.contains {
				if !strings.Contains(got, exp) {
					t.Errorf("markerURL() = %q, esperava conter %q", got, exp)
				}
			}
		})
	}
}

func TestTodoBadgeRender(t *testing.T) {
	t.Run("Count 0 nao renderiza badge", func(t *testing.T) {
		var buf bytes.Buffer
		err := TodoBadge(0).Render(context.Background(), &buf)
		if err != nil {
			t.Fatalf("Render erro: %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("Esperava buffer vazio para count=0, got %q", buf.String())
		}
	})

	t.Run("Count > 0 renderiza numero e title", func(t *testing.T) {
		var buf bytes.Buffer
		err := TodoBadge(5).Render(context.Background(), &buf)
		if err != nil {
			t.Fatalf("Render erro: %v", err)
		}
		output := buf.String()
		if !strings.Contains(output, "5 tarefa(s)") {
			t.Errorf("Resposta nao contem o atributo title correto: %s", output)
		}
		if !strings.Contains(output, ">5</span>") {
			t.Errorf("Resposta nao contem o numero 5: %s", output)
		}
	})
}

func TestTodoBadgeOOBRender(t *testing.T) {
	var buf bytes.Buffer
	err := TodoBadgeOOB(3).Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("Render erro: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, `hx-swap-oob="innerHTML:#todos-badge"`) {
		t.Errorf("Falta swap oob desktop: %s", output)
	}
	if !strings.Contains(output, `hx-swap-oob="innerHTML:#mobile-todos-badge"`) {
		t.Errorf("Falta swap oob mobile: %s", output)
	}
	if !strings.Contains(output, "3") {
		t.Errorf("Falta o numero 3 no badge OOB: %s", output)
	}
}

func TestMarkersListRender(t *testing.T) {
	t.Run("Lista de marcadores vazia", func(t *testing.T) {
		var buf bytes.Buffer
		err := MarkersList([]db.TodoMarker{}).Render(context.Background(), &buf)
		if err != nil {
			t.Fatalf("Render erro: %v", err)
		}
		output := buf.String()
		if !strings.Contains(output, "Nenhum marcador configurado") {
			t.Errorf("Nao exibiu mensagem de lista vazia: %s", output)
		}
	})

	t.Run("Lista com marcadores ativos", func(t *testing.T) {
		markers := []db.TodoMarker{
			{Marker: "TODO", Color: "#ff0000", Active: true, CountInBadge: true, SortOrder: 1},
			{Marker: "WAITING", Color: "#00ff00", Active: false, CountInBadge: false, SortOrder: 2},
		}

		var buf bytes.Buffer
		err := MarkersList(markers).Render(context.Background(), &buf)
		if err != nil {
			t.Fatalf("Render erro: %v", err)
		}
		output := buf.String()

		if !strings.Contains(output, "TODO") || !strings.Contains(output, "WAITING") {
			t.Errorf("Resposta nao contem os marcadores esperados: %s", output)
		}
		if !strings.Contains(output, "#ff0000") || !strings.Contains(output, "#00ff00") {
			t.Errorf("Resposta nao contem as cores dos marcadores: %s", output)
		}
	})
}
