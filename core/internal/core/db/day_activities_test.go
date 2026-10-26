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

// TestEnsureDayCopied cobre a cópia automática da Rotina: o dia de destino
// recebe as atividades do mesmo dia da semana anterior, com ids/timestamps
// novos, uma única vez.
func TestEnsureDayCopied(t *testing.T) {
	s := newTestStore(t)

	source := "2026-10-18"
	target := "2026-10-25"
	for _, a := range []domain.DayActivity{
		{ID: "src-1", Day: source, StartTime: "09:00", EndTime: "10:00", Description: "Academia"},
		{ID: "src-2", Day: source, StartTime: "08:00", EndTime: "08:30", Description: "Café"},
	} {
		if err := s.CreateDayActivity(a); err != nil {
			t.Fatalf("CreateDayActivity %s: %v", a.ID, err)
		}
	}

	n, err := s.EnsureDayCopied(target, source)
	if err != nil {
		t.Fatalf("EnsureDayCopied: %v", err)
	}
	if n != 2 {
		t.Fatalf("copiou %d atividades, want 2", n)
	}

	got, err := s.GetDayActivities(target)
	if err != nil {
		t.Fatalf("GetDayActivities: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("destino tem %d atividades, want 2", len(got))
	}
	if got[0].Description != "Café" || got[1].Description != "Academia" {
		t.Fatalf("ordem/conteúdo copiado errado: %+v", got)
	}
	if got[0].StartTime != "08:00" || got[0].EndTime != "08:30" {
		t.Fatalf("horários copiados errados: %+v", got[0])
	}
	if got[0].ID == "src-2" || got[1].ID == "src-1" || got[0].ID == got[1].ID {
		t.Fatalf("a cópia deve gerar ids novos: %+v", got)
	}
	if got[0].CreatedAt == "" || got[0].UpdatedAt == "" {
		t.Fatal("timestamps da cópia não foram preenchidos")
	}

	// A origem continua intacta.
	if src, _ := s.GetDayActivities(source); len(src) != 2 {
		t.Fatalf("origem foi alterada: %d atividades", len(src))
	}

	// Segunda chamada não duplica.
	n, err = s.EnsureDayCopied(target, source)
	if err != nil || n != 0 {
		t.Fatalf("segunda cópia = (%d, %v), want (0, nil)", n, err)
	}
	if got, _ = s.GetDayActivities(target); len(got) != 2 {
		t.Fatalf("destino duplicou: %d atividades", len(got))
	}

	// Mesmo que a origem ganhe atividades depois, o dia já processado não muda.
	if err := s.CreateDayActivity(domain.DayActivity{
		ID: "src-3", Day: source, StartTime: "11:00", EndTime: "12:00", Description: "Almoço",
	}); err != nil {
		t.Fatalf("CreateDayActivity src-3: %v", err)
	}
	if n, err = s.EnsureDayCopied(target, source); err != nil || n != 0 {
		t.Fatalf("cópia retroativa = (%d, %v), want (0, nil)", n, err)
	}
	if got, _ = s.GetDayActivities(target); len(got) != 2 {
		t.Fatalf("cópia retroativa alterou o destino: %d atividades", len(got))
	}
}

// TestEnsureDayCopied_OrigemVazia garante o requisito "se tiver coisas copia,
// se não, não copia": a origem vazia não cria nada, mas marca o dia como
// processado (evitando consultas repetidas e cópia retroativa).
func TestEnsureDayCopied_OrigemVazia(t *testing.T) {
	s := newTestStore(t)

	source := "2026-10-18"
	target := "2026-10-25"

	n, err := s.EnsureDayCopied(target, source)
	if err != nil || n != 0 {
		t.Fatalf("cópia de origem vazia = (%d, %v), want (0, nil)", n, err)
	}
	if got, _ := s.GetDayActivities(target); len(got) != 0 {
		t.Fatalf("origem vazia criou %d atividades", len(got))
	}

	var copied int
	if err := s.DB.QueryRow(`SELECT copied FROM day_activity_seeds WHERE day = ?`, target).Scan(&copied); err != nil {
		t.Fatalf("marcador não foi gravado: %v", err)
	}
	if copied != 0 {
		t.Fatalf("copied = %d, want 0", copied)
	}

	// A origem ganhar atividades depois não dispara cópia retroativa.
	if err := s.CreateDayActivity(domain.DayActivity{
		ID: "src-1", Day: source, StartTime: "09:00", EndTime: "10:00", Description: "Academia",
	}); err != nil {
		t.Fatalf("CreateDayActivity: %v", err)
	}
	if n, err = s.EnsureDayCopied(target, source); err != nil || n != 0 {
		t.Fatalf("cópia retroativa = (%d, %v), want (0, nil)", n, err)
	}
	if got, _ := s.GetDayActivities(target); len(got) != 0 {
		t.Fatalf("cópia retroativa criou %d atividades", len(got))
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
