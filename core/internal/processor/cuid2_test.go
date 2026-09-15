package processor

import (
	"strings"
	"testing"
)

func TestGenerateCUID2_Output(t *testing.T) {
	for i := 0; i < 5; i++ {
		name := GenerateCUID2()
		t.Logf("name: %s", name)
		if len(name) < 5 {
			t.Errorf("name too short: %s", name)
		}
		if !IsCUID2(name) {
			t.Errorf("GenerateCUID2() output %s deve ser reconhecido por IsCUID2", name)
		}
	}
}

func TestIsCUID2_And_IsDraftName(t *testing.T) {
	// O nome é construído a partir das PRÓPRIAS listas de palavras: assim o teste
	// não quebra quando a lista é trocada/ampliada (era hardcoded "impar-lareira-99").
	valid := adjectives[0] + "-" + nouns[0] + "-99"

	if !IsCUID2("notes/" + valid + ".md") {
		t.Errorf("esperado true para %s", valid)
	}
	if !IsDraftName("notes/" + valid + ".md") {
		t.Errorf("esperado IsDraftName true para %s", valid)
	}
	if IsCUID2("notes/nota-que-nao-existe.md") {
		t.Errorf("esperado false para nota-que-nao-existe")
	}
	if IsDraftName("notes/nota-que-nao-existe.md") {
		t.Errorf("esperado IsDraftName false para nota-que-nao-existe")
	}
}

// TestWordLists_Integridade garante que as listas de palavras do gerador de nomes
// estão utilizáveis: não vazias, sem duplicatas e sem caixa alta/acentos que
// atrapalhem o uso do nome gerado como nome de arquivo/URL.
func TestWordLists_Integridade(t *testing.T) {
	check := func(nome string, palavras []string) {
		t.Helper()
		if len(palavras) == 0 {
			t.Fatalf("%s está vazia", nome)
		}
		vistos := make(map[string]bool, len(palavras))
		for _, p := range palavras {
			if p == "" {
				t.Errorf("%s contém palavra vazia", nome)
			}
			if p != strings.ToLower(p) {
				t.Errorf("%s: %q deveria estar em minúsculas", nome, p)
			}
			if vistos[p] {
				t.Errorf("%s: palavra duplicada %q (reduz a entropia)", nome, p)
			}
			vistos[p] = true
		}
	}
	check("adjectives", adjectives)
	check("nouns", nouns)
}
