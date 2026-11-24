package search

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/ui/icons"
)

// titleAnchorTag extrai a tag <a> do título (é o único link de cada item).
func titleAnchorTag(t *testing.T, html string) string {
	t.Helper()
	i := strings.Index(html, "<a ")
	if i < 0 {
		t.Fatalf("nenhum <a> no HTML renderizado:\n%s", html)
	}
	j := strings.Index(html[i:], ">")
	if j < 0 {
		t.Fatalf("tag <a> sem fechamento:\n%s", html)
	}
	return html[i : i+j+1]
}

// O NOME da nota nos resultados da busca global deve usar a MESMA cor do tipo
// (fonte única em icons.Config) — antes a cor era fixa por ramo do `if` e tinha
// divergido do Config (desenho/markdown saíam em azul, tabulator em lime-400).
//
// O valor esperado é derivado de icons.GetColor, então o teste não copia a
// paleta: ele garante o VÍNCULO entre o título e a cor do tipo. Se alguém voltar
// a fixar `text-lime-400`/`text-sky-400` na mão, o teste quebra.
func TestSearchResultItem_CorDoTituloSegueTipo(t *testing.T) {
	cases := []struct {
		tipo    string
		arquivo string
		href    string
	}{
		{"tabulator", "notes/planilha.md", "/tabulator?file=notes/planilha.md"},
		{"markdown", "notes/nota.md", "/editor?file=notes/nota.md"},
		{"markmap", "notes/mapa.md", "/mindmap?file=notes/mapa.md"},
		{"desenho", "notes/desenho.md", "/drawing?file=notes/desenho.md"},
		{"pdf", "pdfs/livro.pdf", "/file?name=pdfs/livro.pdf"},
	}

	seen := map[string]string{} // tipo -> cor aplicada (para provar que varia)

	for _, tc := range cases {
		t.Run(tc.tipo, func(t *testing.T) {
			data := domain.SearchResultsData{
				Query: "x",
				Total: 1,
				Results: []domain.SearchResultItem{
					{Tipo: tc.tipo, Arquivo: tc.arquivo, Secao: "secao", Snippet: "trecho"},
				},
			}
			var buf bytes.Buffer
			if err := SearchResults(data, 10).Render(context.Background(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			tag := titleAnchorTag(t, buf.String())

			colorClass, colorStyle := icons.ParseClassAndStyle(icons.GetColor(tc.tipo))
			want := colorStyle // cores raw viram style inline (ex: color:#10C809)
			if want == "" {
				want = colorClass // classes Tailwind ficam no atributo class
			}
			if want == "" {
				t.Fatalf("icons.GetColor(%q) sem cor aplicável", tc.tipo)
			}
			if !strings.Contains(tag, want) {
				t.Errorf("título não usa a cor do tipo %q (%q):\n%s", tc.tipo, want, tag)
			}
			if !strings.Contains(tag, tc.href) {
				t.Errorf("href do título: esperava %q em:\n%s", tc.href, tag)
			}

			// PDF abre em nova aba; os demais seguem na mesma aba.
			if tc.tipo == "pdf" {
				if !strings.Contains(tag, `target="_blank"`) {
					t.Errorf("pdf deveria abrir em nova aba:\n%s", tag)
				}
			} else if strings.Contains(tag, "target=") {
				t.Errorf("só o pdf deveria ter target:\n%s", tag)
			}

			seen[tc.tipo] = want
		})
	}

	// Padronização visual: tipos com cores distintas NÃO podem compartilhar a
	// mesma cor aplicada (era o sintoma reclamado: tudo verde).
	if seen["tabulator"] == seen["markdown"] || seen["markmap"] == seen["desenho"] {
		t.Errorf("cores de tipos distintos ficaram iguais: %v", seen)
	}
}
