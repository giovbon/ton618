package db

import (
	"database/sql"
	"errors"
	"testing"

	"ton618/core/internal/core/domain"
)

func TestDayActivities_CRUD(t *testing.T) {
	s := newTestStore(t)

	a := domain.DayActivity{
		ID:          "a1",
		Day:         "2026-10-25",
		StartTime:   "09:00",
		EndTime:     "10:00",
		Description: "Academia",
	}
	if err := s.CreateDayActivity(a); err != nil {
		t.Fatalf("CreateDayActivity: %v", err)
	}
	if err := s.CreateDayActivity(domain.DayActivity{
		ID: "a2", Day: "2026-10-25", StartTime: "08:00", EndTime: "08:30", Description: "Café",
	}); err != nil {
		t.Fatalf("CreateDayActivity 2: %v", err)
	}
	// Outro dia não pode aparecer na listagem.
	if err := s.CreateDayActivity(domain.DayActivity{
		ID: "a3", Day: "2026-10-26", StartTime: "07:00", EndTime: "08:00", Description: "Outro dia",
	}); err != nil {
		t.Fatalf("CreateDayActivity 3: %v", err)
	}

	got, err := s.GetDayActivities("2026-10-25")
	if err != nil {
		t.Fatalf("GetDayActivities: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("esperava 2 atividades, veio %d", len(got))
	}
	if got[0].ID != "a2" || got[1].ID != "a1" {
		t.Fatalf("ordem por início errada: %+v", got)
	}
	if got[0].CreatedAt == "" || got[0].UpdatedAt == "" {
		t.Fatal("timestamps não foram preenchidos")
	}

	a.Description = "Academia atualizada"
	a.StartTime = "09:15"
	if err := s.UpdateDayActivity(a); err != nil {
		t.Fatalf("UpdateDayActivity: %v", err)
	}
	got, _ = s.GetDayActivities("2026-10-25")
	if got[1].Description != "Academia atualizada" || got[1].StartTime != "09:15" {
		t.Fatalf("update não aplicado: %+v", got[1])
	}

	if err := s.DeleteDayActivity("a1"); err != nil {
		t.Fatalf("DeleteDayActivity: %v", err)
	}
	got, _ = s.GetDayActivities("2026-10-25")
	if len(got) != 1 || got[0].ID != "a2" {
		t.Fatalf("delete não aplicado: %+v", got)
	}
}

func TestDayActivities_NotFound(t *testing.T) {
	s := newTestStore(t)

	if err := s.UpdateDayActivity(domain.DayActivity{ID: "nao-existe", Day: "2026-10-25"}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateDayActivity de id inexistente: esperava sql.ErrNoRows, veio %v", err)
	}
	if err := s.DeleteDayActivity("nao-existe"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteDayActivity de id inexistente: esperava sql.ErrNoRows, veio %v", err)
	}
}

// TestDayActivities_SchemaSemDono garante o requisito de uso pessoal: a tabela
// não pode ter coluna de usuário/dono. A única credencial é o dono do cofre.
func TestDayActivities_SchemaSemDono(t *testing.T) {
	s := newTestStore(t)

	rows, err := s.DB.Query(`SELECT name FROM pragma_table_info('day_activities')`)
	if err != nil {
		t.Fatalf("pragma_table_info: %v", err)
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(cols) == 0 {
		t.Fatal("tabela day_activities não existe no schema")
	}

	for _, forbidden := range []string{"user", "user_id", "owner", "owner_id", "username", "account_id", "person_id"} {
		if cols[forbidden] {
			t.Errorf("day_activities tem coluna %q: o TON-618 é de uso pessoal (sem dono por linha)", forbidden)
		}
	}

	for _, required := range []string{"id", "day", "start_time", "end_time", "description"} {
		if !cols[required] {
			t.Errorf("day_activities sem a coluna obrigatória %q", required)
		}
	}
}
