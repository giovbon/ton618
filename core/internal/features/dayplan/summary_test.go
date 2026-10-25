package dayplan

import (
	"testing"

	"ton618/core/internal/core/domain"
)

func activity(id, start, end string) domain.DayActivity {
	return domain.DayActivity{ID: id, Day: "2026-10-25", StartTime: start, EndTime: end, Description: id}
}

func TestSummarize_Empty(t *testing.T) {
	s := Summarize(nil)
	if s.OccupiedMinutes != 0 || s.FreeMinutes != 1440 {
		t.Fatalf("vazio: ocupado=%d livre=%d", s.OccupiedMinutes, s.FreeMinutes)
	}
	if s.OccupiedPct() != 0 || s.ActivityCountLabel() != "nenhuma atividade" {
		t.Fatalf("vazio: pct=%d label=%q", s.OccupiedPct(), s.ActivityCountLabel())
	}
}

func TestSummarize_UniaoSemContarDuasVezes(t *testing.T) {
	s := Summarize([]domain.DayActivity{
		activity("a", "09:00", "10:30"),
		activity("b", "10:00", "11:00"),
	})
	if s.OccupiedMinutes != 120 {
		t.Fatalf("ocupado = %d, want 120", s.OccupiedMinutes)
	}
	if s.FreeMinutes != 1320 {
		t.Fatalf("livre = %d, want 1320", s.FreeMinutes)
	}
	if s.OccupiedPct() != 8 {
		t.Fatalf("pct = %d, want 8", s.OccupiedPct())
	}
	if s.Overlapping != 2 {
		t.Fatalf("sobrepostas = %d, want 2", s.Overlapping)
	}
	if s.OverlapNote() == "" {
		t.Fatal("esperava aviso de sobreposição")
	}
	if s.ActivityCountLabel() != "2 atividades" {
		t.Fatalf("label = %q", s.ActivityCountLabel())
	}
}

func TestSummarize_IntervalosEncostadosNaoSobrepoem(t *testing.T) {
	s := Summarize([]domain.DayActivity{
		activity("a", "09:00", "10:00"),
		activity("b", "10:00", "11:00"),
	})
	if s.OccupiedMinutes != 120 {
		t.Fatalf("ocupado = %d, want 120", s.OccupiedMinutes)
	}
	if s.Overlapping != 0 {
		t.Fatalf("sobrepostas = %d, want 0", s.Overlapping)
	}
	if s.OverlapNote() != "" {
		t.Fatalf("não esperava aviso, veio %q", s.OverlapNote())
	}
}

func TestSummarize_IgnoraIntervaloInvalido(t *testing.T) {
	s := Summarize([]domain.DayActivity{
		activity("a", "10:00", "09:00"),
		activity("b", "25:00", "26:00"),
		activity("c", "08:00", "09:00"),
	})
	if s.OccupiedMinutes != 60 {
		t.Fatalf("ocupado = %d, want 60", s.OccupiedMinutes)
	}
	if s.ActivityCount != 3 {
		t.Fatalf("contagem = %d, want 3 (o rótulo conta o cadastro)", s.ActivityCount)
	}
}

func TestFormatMinutes(t *testing.T) {
	cases := map[int]string{
		0:    "0min",
		30:   "30min",
		60:   "1h",
		90:   "1h 30min",
		1440: "24h",
	}
	for in, want := range cases {
		if got := formatMinutes(in); got != want {
			t.Errorf("formatMinutes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestDateLabels(t *testing.T) {
	if got := DateLabel("2026-10-25"); got != "25/10/2026" {
		t.Fatalf("DateLabel = %q", got)
	}
	if got := DateLabel("invalida"); got != "invalida" {
		t.Fatalf("DateLabel inválida = %q", got)
	}
	if got := WeekdayLabel("2026-10-25"); got != "dom" {
		t.Fatalf("WeekdayLabel = %q, want dom", got)
	}
}
