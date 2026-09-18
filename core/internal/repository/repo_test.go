package repository

import (
	"testing"
)

func TestParseMtime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantZero bool
		wantYear int
	}{
		{
			name:     "RFC3339 valido UTC",
			input:    "2026-09-18T14:30:00Z",
			wantZero: false,
			wantYear: 2026,
		},
		{
			name:     "RFC3339 valido com fuso",
			input:    "2026-09-18T11:30:00-03:00",
			wantZero: false,
			wantYear: 2026,
		},
		{
			name:     "String vazia retorna time zero",
			input:    "",
			wantZero: true,
		},
		{
			name:     "String em formato invalido retorna time zero",
			input:    "18/09/2026 14:30:00",
			wantZero: true,
		},
		{
			name:     "Texto qualquer retorna time zero",
			input:    "invalid-date",
			wantZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMtime(tt.input)
			if tt.wantZero {
				if !got.IsZero() {
					t.Errorf("ParseMtime(%q) = %v, esperava time.Time zero", tt.input, got)
				}
			} else {
				if got.IsZero() {
					t.Errorf("ParseMtime(%q) retornou time zero inesperadamente", tt.input)
				}
				if got.Year() != tt.wantYear {
					t.Errorf("ParseMtime(%q).Year() = %d, want %d", tt.input, got.Year(), tt.wantYear)
				}
			}
		})
	}
}
