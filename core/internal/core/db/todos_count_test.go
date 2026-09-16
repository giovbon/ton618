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

// ── Update/Add/Remove pontuais ──
//
// O painel de marcadores dispara uma requisição por checkbox. O fluxo antigo
// (GetTodoMarkers → altera em memória → SaveTodoMarkers) reescrevia a tabela
// inteira a cada toggle: duas requisições concorrentes perdiam uma das
// alterações e um erro de leitura apagava a configuração toda. Estes métodos
// fazem UPDATE/INSERT/DELETE pontuais.

func TestUpdateTodoMarker_SoAlteraOCampoInformado(t *testing.T) {
	s := newTestStore(t)

	antes, err := s.GetTodoMarkers()
	if err != nil {
		t.Fatalf("GetTodoMarkers: %v", err)
	}
	if len(antes) < 2 {
		t.Fatalf("esperava ao menos 2 marcadores, got %d", len(antes))
	}

	alvo := antes[0]
	outro := antes[1]

	// Só o CountInBadge do alvo muda.
	novoCount := !alvo.CountInBadge
	ok, err := s.UpdateTodoMarker(alvo.Marker, TodoMarkerUpdate{CountInBadge: &novoCount})
	if err != nil {
		t.Fatalf("UpdateTodoMarker: %v", err)
	}
	if !ok {
		t.Fatal("UpdateTodoMarker deveria informar que atualizou")
	}

	depois, err := s.GetTodoMarkers()
	if err != nil {
		t.Fatalf("GetTodoMarkers após update: %v", err)
	}
	if len(depois) != len(antes) {
		t.Fatalf("quantidade mudou: %d → %d", len(antes), len(depois))
	}

	for _, m := range depois {
		switch m.Marker {
		case alvo.Marker:
			if m.CountInBadge != novoCount {
				t.Errorf("CountInBadge do alvo = %v, want %v", m.CountInBadge, novoCount)
			}
			// Campos não informados ficam como estavam.
			if m.Color != alvo.Color || m.Active != alvo.Active || m.SortOrder != alvo.SortOrder {
				t.Errorf("campos não informados mudaram: antes=%+v depois=%+v", alvo, m)
			}
		case outro.Marker:
			if m.CountInBadge != outro.CountInBadge {
				t.Errorf("marcador %s não deveria ter sido afetado", m.Marker)
			}
		}
	}
}

func TestUpdateTodoMarker_OutrosCampos(t *testing.T) {
	s := newTestStore(t)

	ativo := false
	color := "#101010"
	ordem := 3

	if ok, err := s.UpdateTodoMarker("DOING", TodoMarkerUpdate{
		Active:       &ativo,
		Color:        &color,
		SortOrder:    &ordem,
		CountInBadge: &ativo,
	}); err != nil || !ok {
		t.Fatalf("UpdateTodoMarker: ok=%v err=%v", ok, err)
	}

	markers, err := s.GetTodoMarkers()
	if err != nil {
		t.Fatalf("GetTodoMarkers: %v", err)
	}
	for _, m := range markers {
		if m.Marker != "DOING" {
			continue
		}
		if m.Active || m.CountInBadge {
			t.Errorf("flags deveriam estar false: %+v", m)
		}
		if m.Color != color {
			t.Errorf("cor = %q, want %q", m.Color, color)
		}
		if m.SortOrder != ordem {
			t.Errorf("sort_order = %d, want %d", m.SortOrder, ordem)
		}
		return
	}
	t.Fatal("marcador DOING sumiu")
}

func TestUpdateTodoMarker_MarcadorInexistente(t *testing.T) {
	s := newTestStore(t)

	v := false
	ok, err := s.UpdateTodoMarker("NAOEXISTE", TodoMarkerUpdate{CountInBadge: &v})
	if err != nil {
		t.Fatalf("UpdateTodoMarker: %v", err)
	}
	if ok {
		t.Error("não deveria informar update para marcador inexistente")
	}
}

func TestAddTodoMarker_IdempotenteEPreservaCor(t *testing.T) {
	s := newTestStore(t)

	criado, err := s.AddTodoMarker(TodoMarker{Marker: "NOVO", Color: "#111111", Active: true, CountInBadge: true})
	if err != nil || !criado {
		t.Fatalf("AddTodoMarker: criado=%v err=%v", criado, err)
	}

	// Segunda chamada não cria de novo nem sobrescreve a cor.
	criado, err = s.AddTodoMarker(TodoMarker{Marker: "NOVO", Color: "#222222", Active: true, CountInBadge: false})
	if err != nil {
		t.Fatalf("AddTodoMarker (2ª): %v", err)
	}
	if criado {
		t.Error("segundo AddTodoMarker deveria retornar false (já existia)")
	}

	markers, err := s.GetTodoMarkers()
	if err != nil {
		t.Fatalf("GetTodoMarkers: %v", err)
	}
	ocorrencias := 0
	for _, m := range markers {
		if m.Marker == "NOVO" {
			ocorrencias++
			if m.Color != "#111111" {
				t.Errorf("cor = %q, want #111111 (preservada)", m.Color)
			}
			if !m.CountInBadge {
				t.Error("CountInBadge original deveria ter sido preservado")
			}
		}
	}
	if ocorrencias != 1 {
		t.Errorf("NOVO deveria existir 1x, got %d", ocorrencias)
	}
}

func TestRemoveTodoMarker(t *testing.T) {
	s := newTestStore(t)

	antes, _ := s.GetTodoMarkers()

	removido, err := s.RemoveTodoMarker("DONE")
	if err != nil || !removido {
		t.Fatalf("RemoveTodoMarker: removido=%v err=%v", removido, err)
	}

	// Idempotência: remover de novo retorna false, sem erro.
	removido, err = s.RemoveTodoMarker("DONE")
	if err != nil {
		t.Fatalf("RemoveTodoMarker (2ª): %v", err)
	}
	if removido {
		t.Error("segunda remoção deveria retornar false")
	}

	depois, _ := s.GetTodoMarkers()
	if len(depois) != len(antes)-1 {
		t.Fatalf("esperava %d marcadores, got %d", len(antes)-1, len(depois))
	}
	for _, m := range depois {
		if m.Marker == "DONE" {
			t.Error("DONE deveria ter sido removido")
		}
	}
}

// TestUpdateTodoMarker_SemCampos garante erro em vez de UPDATE vazio (SQL inválido).
func TestUpdateTodoMarker_SemCampos(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.UpdateTodoMarker("TODO", TodoMarkerUpdate{}); err == nil {
		t.Error("esperava erro quando nenhum campo é informado")
	}
}

// TestHasMarkerColumn_Memoizado garante que o PRAGMA é feito uma única vez por
// Store (o badge do cabeçalho chama GetActiveTodoMarkers a cada carga de página).
func TestHasMarkerColumn_Memoizado(t *testing.T) {
	s := newTestStore(t)

	if !s.hasMarkerColumn("count_in_badge") {
		t.Fatal("coluna count_in_badge deveria existir")
	}
	if s.markerCols == nil {
		t.Fatal("esperava cache de colunas preenchido")
	}

	// Se o memoize estivesse errado (relendo o banco), marcar o cache como cheio
	// e removido forçaria uma releitura; aqui basta garantir que o mapa é estável.
	s.markerCols["coluna_fake"] = true
	if !s.hasMarkerColumn("coluna_fake") {
		t.Error("hasMarkerColumn deveria consultar o cache memoizado")
	}
}
