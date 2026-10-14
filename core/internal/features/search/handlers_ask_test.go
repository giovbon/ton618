package search

import (
	"strings"
	"testing"
)

func TestExtractQuestionKeywords(t *testing.T) {
	q := "qual comando do uv cria ambiente virtual do python"
	keywords := extractQuestionKeywords(q)

	expected := map[string]bool{
		"comando":  true,
		"uv":       true,
		"ambiente": true,
		"virtual":  true,
		"python":   true,
	}

	for _, kw := range keywords {
		if !expected[kw] {
			t.Errorf("palavra inesperada nos keywords: %s", kw)
		}
	}

	// Verifica que palavras como "qual", "do", "cria" foram devidamente filtradas
	for _, kw := range keywords {
		if kw == "qual" || kw == "do" || kw == "cria" {
			t.Errorf("stopword não deveria estar nos keywords: %s", kw)
		}
	}
}

func TestBuildFTSFlexibleQuery(t *testing.T) {
	keywords := []string{"uv", "ambiente", "python"}
	flex := buildFTSFlexibleQuery(keywords)

	if !strings.Contains(flex, "tags:uv") || !strings.Contains(flex, "texto:python*") {
		t.Errorf("query FTS flexível incorreta: %s", flex)
	}
	if !strings.Contains(flex, " OR ") {
		t.Errorf("query flexível deve unir termos por OR: %s", flex)
	}
}

func TestExtractBestExcerpt_MarkmapUv(t *testing.T) {
	note := `---
type: markmap
markmap:
  maxWidth: 400
---
# uv

- Como instalar uv
  - linx ` + "`curl -LsSf https://astral.sh/uv/install.sh | sh`" + `
  - win ` + "`powershell -ExecutionPolicy ByPass -c \"irm https://astral.sh/uv/install.ps1 | iex\"`" + `
  - universal ` + "`pip install uv`" + `
- Comandos
  - Ambiente python
    - ` + "`uv venv`" + `
    - Ativação
      - linx ` + "`source .venv/bin/activate`" + ` ou win ` + "`.venv\\Scripts\\Activate.ps1`" + `
  - Instalação de pacotes
    - ` + "`uv pip install nome-do-pacote`" + `
`

	q := "qual comando do uv cria ambiente virtual do python"
	excerpt, score, distinct := extractBestExcerpt(note, q, 400)

	if score <= 0 || distinct < 2 {
		t.Errorf("esperava score positivo e termos distintos >= 2, got score=%d, distinct=%d", score, distinct)
	}
	if !strings.Contains(excerpt, "uv venv") {
		t.Errorf("esperava que o trecho contivesse 'uv venv', got: %s", excerpt)
	}
	if !strings.Contains(excerpt, "Ambiente python") {
		t.Errorf("esperava que o trecho contivesse 'Ambiente python', got: %s", excerpt)
	}
}

func TestExtractBestExcerpt_RevolucaoFrancaRejected(t *testing.T) {
	note := `# Revolução Francesa
A Primeira República Francesa foi proclamada em setembro de 1792.
O rei Luís XVI foi executado no ano seguinte.
Houve guerras revolucionárias francesas e disputas políticas.`

	q := "qual comando do uv cria ambiente virtual do python"
	excerpt, score, _ := extractBestExcerpt(note, q, 400)

	if excerpt != "" || score > 0 {
		t.Errorf("esperava que nota não relacionada da revolução francesa fosse rejeitada, got excerpt: %s", excerpt)
	}
}
