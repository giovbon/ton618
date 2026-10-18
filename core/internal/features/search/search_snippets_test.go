package search

import (
	"strings"
	"testing"

	"ton618/core/internal/core/db"
	"ton618/core/internal/search"
)

// Quando o termo casa no TÍTULO/seção (e não no corpo), o destaque deve ser
// aplicado mesmo assim — é o caso em que o snippet do corpo não pega (o FTS
// snippet() usa só a coluna texto).
func TestHighlightSpans_TermoNoTitulo(t *testing.T) {
	out := highlightSpans("Lula explica decisão de política", "lula")
	if !strings.Contains(out, "search-highlight") {
		t.Fatalf("esperava span de destaque no título, got %q", out)
	}
	if !strings.Contains(out, "Lula") {
		t.Errorf("o termo deveria permanecer no título, got %q", out)
	}
}

// Sem ocorrência do termo, o texto é devolvido intacto (sem span).
func TestHighlightSpans_SemOcorrencia(t *testing.T) {
	out := highlightSpans("Nota sobre outro assunto", "lula")
	if strings.Contains(out, "search-highlight") {
		t.Fatalf("sem ocorrência não deveria destacar, got %q", out)
	}
	if out != "Nota sobre outro assunto" {
		t.Errorf("sem ocorrência deveria devolver o texto intacto, got %q", out)
	}
}

// Regressão do bug do trecho: quando o termo aparece DEPOIS de muitas palavras
// acentuadas, o índice obtido no texto sem acento (mais curto) ficava
// DESALINHADO com a fatia do texto original — e o trecho recortado perdia o
// termo (sem destaque). Com o fix (buscar só em minúsculas, que preserva bytes),
// o trecho passa a conter a ocorrência.
func TestExtractSnippetHighlight_AposAcentosPreservaTermo(t *testing.T) {
	prefix := strings.Repeat("educação e nação em situação, ", 25)
	texto := prefix + "você passa tempo com o celular no final."

	snip := extractSnippetAroundMatch(texto, "lula", 40)
	if !strings.Contains(snip, "lula") {
		t.Fatalf("o trecho deveria conter 'lula', got %q", snip)
	}
	hl := highlightSnippetManual(snip, "lula")
	if !strings.Contains(hl, "__HL_START__") {
		t.Fatalf("'lula' deveria estar destacada no trecho, got %q", hl)
	}
}

// containsSearchTerm detecta substring insensível a acento/maiúsculas. Cobre o
// caso do LIKE (termo no meio de palavra) e do FTS (termo isolado).
func TestContainsSearchTerm(t *testing.T) {
	cases := []struct {
		texto string
		query string
		want  bool
	}{
		{"vamos fala a palavra lula aqui", "lula", true},
		{"o termo LULA em maiúscula", "lula", true},
		{"aqui não aparece", "lula", false},
		{"uma palavra acentuada caçação", "cacacao", true}, // acento-insensível
	}
	for _, c := range cases {
		if got := containsSearchTerm(c.texto, c.query); got != c.want {
			t.Errorf("containsSearchTerm(%q, %q) = %v, want %v", c.texto, c.query, got, c.want)
		}
	}
}

// ── Regressão do offset por acentos / destaque ─────────────────────────────

// buildSnippet deve gerar o span de destaque quando o termo está no corpo como
// substring no meio de uma palavra ("lula" dentro de "celular").
func TestBuildSnippet_SubstringMidPalavra_Destaca(t *testing.T) {
	hit := search.SearchHit{Doc: db.Document{Texto: "você passa muito tempo com a cara no celular e no computador."}}
	out := buildSnippet(hit, "lula")
	if !strings.Contains(out, "search-highlight") || !strings.Contains(out, "lula") {
		t.Fatalf("esperava span search-highlight envolvendo 'lula', got %q", out)
	}
	// "lula" deve estar destacada DENTRO de "celular": com o span, a palavra fica
	// quebrada em ce[span]lula[/span]r — "celular" contíguo não existe no HTML.
	if !strings.Contains(out, "ce<span") || !strings.Contains(out, "lula</span>r") {
		t.Fatalf("esperava 'lula' destacada no meio de 'celular' (ce[highlight]lula[/highlight]r), got %q", out)
	}
}

// Sem o termo no corpo, buildSnippet não gera destaque.
func TestBuildSnippet_SemTermo_NaoDestaca(t *testing.T) {
	hit := search.SearchHit{Doc: db.Document{Texto: "não há nenhuma referência aqui neste texto."}}
	out := buildSnippet(hit, "lula")
	if strings.Contains(out, "search-highlight") {
		t.Fatalf("sem o termo não deveria destacar, got %q", out)
	}
}

// O núcleo DA regressão: o termo aparece DEPOIS de uma longa sequência de
// palavras acentuadas — o antigo `removeAccents` desalinhava os bytes e o trecho
// ia para um ponteiro antes do termo. Agora o trecho precisa conter o termo.
func TestExtractSnippetAroundMatch_OffsetComMuitosAcentos(t *testing.T) {
	texto := strings.Repeat("configuração e operação em situação, ", 50) + "olha o celular aqui."
	snip := extractSnippetAroundMatch(texto, "lula", 40)
	if !strings.Contains(snip, "lula") {
		t.Fatalf("o trecho deveria conter 'lula' (offset por acentos fixado), got %q", snip)
	}
	if !strings.Contains(snip, "celular") {
		t.Fatalf("o trecho deveria conter 'celular', got %q", snip)
	}
}

// Termo no FIM do texto: o recorte não deve estourar nem perder o termo.
func TestExtractSnippetAroundMatch_TermoNoFinal(t *testing.T) {
	texto := "texto de uma página inteira sem o termo até o final, então celular."
	snip := extractSnippetAroundMatch(texto, "lula", 30)
	if !strings.Contains(snip, "lula") {
		t.Fatalf("termo no final deveria aparecer, got %q", snip)
	}
}

// Termo no INÍCIO do texto: o recorte deve incluir o termo.
func TestExtractSnippetAroundMatch_TermoNoInicio(t *testing.T) {
	texto := "celular é a palavra; o resto do texto continua sem termo."
	snip := extractSnippetAroundMatch(texto, "lula", 20)
	if !strings.Contains(snip, "lula") {
		t.Fatalf("termo no início deveria aparecer, got %q", snip)
	}
}
