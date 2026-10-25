package system

import "testing"

// O slug é o que aparece no href (#...) e no id dos títulos. Precisa ser
// estável, sem acentos e sem pontuação — senão o link do índice não fecha.
func TestHelpSlug(t *testing.T) {
	casos := []struct{ entrada, esperado string }{
		{"4.5 Nota semanal", "4-5-nota-semanal"},
		{"8.8 Expressões (`onde:`) — CEL", "8-8-expressoes-onde-cel"},
		{"Conexões & Ajustes", "conexoes-ajustes"},
		{"O que é isto?", "o-que-e-isto"},
		{"   ", "secao"},
		{"—", "secao"},
	}
	for _, c := range casos {
		if got := helpSlug(c.entrada); got != c.esperado {
			t.Errorf("helpSlug(%q) = %q, esperado %q", c.entrada, got, c.esperado)
		}
	}
}

// Dois títulos iguais não podem compartilhar âncora: o segundo ganha sufixo.
func TestHelpUniqueID(t *testing.T) {
	usados := map[string]int{}
	got := []string{
		helpUniqueID("notas", usados),
		helpUniqueID("notas", usados),
		helpUniqueID("notas", usados),
		helpUniqueID("tarefas", usados),
	}
	esperado := []string{"notas", "notas-2", "notas-3", "tarefas"}
	for i := range esperado {
		if got[i] != esperado[i] {
			t.Errorf("id %d = %q, esperado %q", i, got[i], esperado[i])
		}
	}
}

func TestHelpTOCLabel(t *testing.T) {
	casos := []struct {
		entries []HelpTOCEntry
		want    string
	}{
		{nil, "0 seções"},
		{[]HelpTOCEntry{{ID: "a"}}, "1 seção"},
		{[]HelpTOCEntry{{ID: "a"}, {ID: "b", Children: []HelpTOCEntry{{ID: "b1"}}}}, "2 seções · 1 subseção"},
		{[]HelpTOCEntry{{ID: "a", Children: []HelpTOCEntry{{ID: "a1"}, {ID: "a2"}}}}, "1 seção · 2 subseções"},
	}
	for _, c := range casos {
		if got := helpTOCLabel(c.entries); got != c.want {
			t.Errorf("helpTOCLabel = %q, esperado %q", got, c.want)
		}
	}
}
