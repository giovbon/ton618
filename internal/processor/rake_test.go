package processor

import (
	"strings"
	"testing"
)

func TestExtractKeywords_Empty(t *testing.T) {
	if got := ExtractKeywords("", 3); got != nil {
		t.Errorf("empty text should return nil, got %v", got)
	}
	if got := ExtractKeywords("   \n  ", 3); got != nil {
		t.Errorf("whitespace-only text should return nil, got %v", got)
	}
}

func TestExtractKeywords_OnlyStopwords(t *testing.T) {
	text := "de e em com o a os as um uma"
	if got := ExtractKeywords(text, 3); got != nil {
		t.Errorf("only stopwords should return nil, got %v", got)
	}
}

func TestExtractKeywords_BasicPortuguese(t *testing.T) {
	text := `O algoritmo de aprendizado profundo utiliza redes neurais convolucionais
	para classificação de imagens médicas. Os resultados mostraram alta precisão
	no diagnóstico de doenças cardíacas.`

	kw := ExtractKeywords(text, 3)
	if len(kw) == 0 {
		t.Fatal("expected keywords, got nil")
	}
	t.Logf("Keywords: %v", kw)

	// Deve conter termos relevantes do domínio
	expectedTerms := []string{"aprendizado", "redes", "neurais", "convolucionais",
		"classificação", "imagens", "diagnóstico", "doenças", "cardíacas"}
	found := false
	for _, k := range kw {
		for _, et := range expectedTerms {
			if strings.Contains(k, et) {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Errorf("keywords %v should contain at least one domain term from %v", kw, expectedTerms)
	}
}

func TestExtractKeywords_ShortText(t *testing.T) {
	text := "Inteligência artificial é muito importante"
	kw := ExtractKeywords(text, 3)
	if len(kw) == 0 {
		t.Fatal("expected at least 1 keyword")
	}
	t.Logf("Keywords: %v", kw)
}

func TestExtractKeywords_TopN(t *testing.T) {
	text := `Python é uma linguagem de programação muito utilizada para ciência de dados.
	JavaScript é usado para desenvolvimento web moderno.
	Go é uma linguagem eficiente para sistemas de alta performance.
	Rust oferece segurança de memória sem garbage collector.`

	kw3 := ExtractKeywords(text, 3)
	kw5 := ExtractKeywords(text, 5)

	if len(kw3) > 3 {
		t.Errorf("expected max 3 keywords with topN=3, got %d: %v", len(kw3), kw3)
	}
	if len(kw5) > 5 {
		t.Errorf("expected max 5 keywords with topN=5, got %d: %v", len(kw5), kw5)
	}
	if len(kw5) < len(kw3) {
		t.Errorf("topN=5 should return at least as many as topN=3: %d vs %d", len(kw5), len(kw3))
	}
	t.Logf("top3: %v", kw3)
	t.Logf("top5: %v", kw5)
}

func TestExtractKeywords_RelevantTerms(t *testing.T) {
	text := `O transformer é uma arquitetura de rede neural baseada em atenção
	que revolucionou o processamento de linguagem natural. Modelos como BERT
	e GPT utilizam esta arquitetura para tarefas de classificação de texto,
	tradução automática e sumarização.`

	kw := ExtractKeywords(text, 3)
	t.Logf("Keywords: %v", kw)

	// Verifica se palavras relevantes aparecem (não necessariamente todas)
	relevantTerms := []string{"transformer", "atencao", "bert", "gpt",
		"linguagem", "classificacao", "traducao", "sumarizacao"}
	hasRelevant := false
	for _, k := range kw {
		for _, rt := range relevantTerms {
			if strings.Contains(k, rt) {
				hasRelevant = true
				break
			}
		}
		if hasRelevant {
			break
		}
	}
	if !hasRelevant {
		t.Errorf("keywords %v should contain at least one relevant term", kw)
	}
}

func TestExtractKeywords_MarkdownContent(t *testing.T) {
	// Simula conteúdo de nota markdown real
	text := `## Instalação do PostgreSQL

Para instalar o PostgreSQL no Ubuntu, use os seguintes comandos:

sudo apt update
sudo apt install postgresql postgresql-contrib

## Configuração Inicial

Após a instalação, é necessário configurar o usuário postgres e criar um banco de dados.
O serviço pode ser gerenciado com systemctl.`

	kw := ExtractKeywords(text, 3)
	t.Logf("Keywords de nota markdown: %v", kw)

	if len(kw) == 0 {
		t.Fatal("expected keywords from markdown content")
	}
}

func TestExtractKeywords_Deduplication(t *testing.T) {
	// Frases repetidas não devem gerar duplicatas
	text := `Python é uma linguagem versátil. Python é uma linguagem versátil e poderosa.
	Python é uma linguagem muito versátil.`

	kw := ExtractKeywords(text, 5)
	t.Logf("Keywords (dedup): %v", kw)

	// Verifica duplicação
	seen := make(map[string]bool)
	for _, k := range kw {
		if seen[k] {
			t.Errorf("duplicate keyword: %s", k)
		}
		seen[k] = true
	}
}

func TestExtractKeywords_PortugueseAccents(t *testing.T) {
	text := `Avaliação de desempenho de sistemas de informação gerenciais
	na administração pública brasileira. Utilização de métricas de qualidade
	para medição de resultados organizacionais.`

	kw := ExtractKeywords(text, 3)
	t.Logf("Keywords (acentos): %v", kw)

	if len(kw) == 0 {
		t.Fatal("expected keywords from text with accents")
	}
}

func TestExtractKeywords_FiltersNumbers(t *testing.T) {
	text := `Em 2026 o projeto foi atualizado para a versão 3.0.
	A reunião aconteceu em 2025 e o prazo final é 2027.
	O sistema foi migrado para Python 3.12.`
	kw := ExtractKeywords(text, 5)
	t.Logf("Keywords (sem números): %v", kw)
	for _, k := range kw {
		if k == "2026" || k == "2025" || k == "2027" || k == "3" || k == "12" || k == "0" {
			t.Errorf("keyword '%s' é um número puro e deveria ter sido filtrado", k)
		}
	}
	// Deve conter termos textuais relevantes
	if len(kw) == 0 {
		t.Error("expected textual keywords even with numbers present")
	}
}
