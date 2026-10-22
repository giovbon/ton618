package search

import (
	"strings"
	"testing"
)

// Garante paridade entre caixa alta e baixa na normalização de acentos.
// RemoveAccents apenas troca diacríticos (não faz case-fold), então os
// callers combinam com strings.ToLower. A tabela duplicada em features/search
// já divergiu (faltava mapear "Õ").
func TestRemoveAccents(t *testing.T) {
	cases := map[string]string{
		"reunião":    "reuniao",
		"REUNIÃO":    "reuniao",
		"ação":       "acao",
		"AÇÃO":       "acao",
		"õ":          "o",
		"Õ":          "o",
		"café":       "cafe",
		"CAFÉ":       "cafe",
		"sem acento": "sem acento",
	}
	for in, want := range cases {
		if got := strings.ToLower(RemoveAccents(in)); got != want {
			t.Errorf("RemoveAccents(%q) = %q; want %q", in, got, want)
		}
	}
}
