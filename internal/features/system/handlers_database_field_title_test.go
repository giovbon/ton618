package system

import (
	"testing"
	"unicode/utf8"
)

// fieldTitle capitaliza apenas a primeira LETRA (runa), não o primeiro byte.
// O corte por byte corrompia campos acentuados ("área" → "�rea") no cabeçalho
// dinâmico do Acervo e na aba Colunas.
func TestFieldTitle_Multibyte(t *testing.T) {
	cases := map[string]string{
		"":        "",
		"area":    "Area",
		"área":    "Área",
		"ação":    "Ação",
		"émotion": "Émotion",
		"1campo":  "1campo",
		"über":    "Über",
	}

	for in, want := range cases {
		got := fieldTitle(in)
		if got != want {
			t.Errorf("fieldTitle(%q) = %q, want %q", in, got, want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("fieldTitle(%q) gerou UTF-8 inválido: %q", in, got)
		}
	}
}
