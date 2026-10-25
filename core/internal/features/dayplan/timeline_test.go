package dayplan

import (
	"strings"
	"testing"

	"ton618/core/internal/core/domain"
)

func TestGantt_Empty(t *testing.T) {
	if src := Gantt("2026-10-25", nil); src != "" {
		t.Fatalf("sem atividades deveria devolver vazio, veio %q", src)
	}
}

func TestGantt_OrdenaEColore(t *testing.T) {
	src := Gantt("2026-10-25", []domain.DayActivity{
		{ID: "b", StartTime: "10:00", EndTime: "11:00", Description: "Reunião"},
		{ID: "a", StartTime: "08:00", EndTime: "09:00", Description: "Academia"},
	})

	for _, want := range []string{
		"gantt\n",
		"dateFormat YYYY-MM-DD HH:mm",
		"section Rotina",
		"Academia :c0ea5e9-1, 2026-10-25 08:00, 2026-10-25 09:00",
		"Reunião :c0ea5e9-2, 2026-10-25 10:00, 2026-10-25 11:00",
		"edge :edge0, 2026-10-25 00:00, 2026-10-25 23:59",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("fonte não contém %q:\n%s", want, src)
		}
	}

	// A ordem textual importa: Academia (08:00) antes de Reunião (10:00).
	if strings.Index(src, "Academia") > strings.Index(src, "Reunião") {
		t.Errorf("atividades fora de ordem:\n%s", src)
	}
}

func TestSanitizeMermaid(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Reunião: 10h, sala 2", "Reunião 10h sala 2"},
		{"  linhas\nquebradas  ", "linhas quebradas"},
		{"aspas \"duplas\" e 'simples'", "aspas duplas e 'simples'"},
		{"", "Atividade"},
	}
	for _, c := range cases {
		if got := sanitizeMermaid(c.in); got != c.want {
			t.Errorf("sanitizeMermaid(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	long := strings.Repeat("a", 120)
	if got := sanitizeMermaid(long); len([]rune(got)) != 80 {
		t.Errorf("truncamento: %d runas, want 80", len([]rune(got)))
	}
}
