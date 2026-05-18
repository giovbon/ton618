package search

import (
	"etl/internal/models"
	"math"
	"testing"
	"time"
)

func TestRankingHeuristics(t *testing.T) {
	// 1. Testar Popularidade (Bônus Logarítmico)
	h1 := &models.SearchHit{Score: 10.0}
	s1, _ := ScoreFragment(h1, []string{"teste"}, "teste", "", 0, 0)
	s2, _ := ScoreFragment(h1, []string{"teste"}, "teste", "", 10, 0) // 10 acessos

	if s2 <= s1 {
		t.Errorf("Popularidade deveria aumentar o score: %f <= %f", s2, s1)
	}

	// 2. Testar Autoridade por Links (Backlinks)
	hL := &models.SearchHit{Score: 10.0}
	sL0, _ := ScoreFragment(hL, []string{"teste"}, "teste", "", 0, 0)
	sL5, _ := ScoreFragment(hL, []string{"teste"}, "teste", "", 0, 5) // 5 links

	if sL5 <= sL0 {
		t.Errorf("Autoridade de links deveria aumentar o score: %f <= %f", sL5, sL0)
	}

	// 3. Testar Recência (Decaimento Temporal)
	agora := time.Now().Format(time.RFC3339)
	mesPassado := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)

	hRecente := &models.SearchHit{Score: 10.0, Source: models.Document{Timestamp: agora}}
	hAntigo := &models.SearchHit{Score: 10.0, Source: models.Document{Timestamp: mesPassado}}

	sRec, _ := ScoreFragment(hRecente, []string{"x"}, "x", "", 0, 0)
	sAnt, _ := ScoreFragment(hAntigo, []string{"x"}, "x", "", 0, 0)

	if sRec <= sAnt {
		t.Errorf("Nota recente deveria pontuar mais que nota de 30 dias atrás: %f <= %f", sRec, sAnt)
	}

	// 4. Testar Estrutura (Completude)
	hSimples := &models.SearchHit{Score: 10.0, Source: models.Document{Texto: "texto simples", Tipo: "markdown"}}
	hRico := &models.SearchHit{Score: 10.0, Source: models.Document{Texto: "Tabela:\n|--|\n|--|\n\n```go\ncode\n```", Tipo: "markdown"}}

	sSimp, _ := ScoreFragment(hSimples, []string{"x"}, "x", "", 0, 0)
	sRico, _ := ScoreFragment(hRico, []string{"x"}, "x", "", 0, 0)

	if sRico <= sSimp {
		t.Errorf("Nota estruturada deveria pontuar mais: %f <= %f", sRico, sSimp)
	}

	// 5. Testar Frase Exata (Boost Crítico)
	hFrase := &models.SearchHit{Score: 10.0, Source: models.Document{Texto: "aprendendo react hooks hoje"}}
	query := []string{"react", "hooks"}

	sFrase, _ := ScoreFragment(hFrase, query, "react hooks", "react hooks", 0, 0)
	if sFrase < 20.0 { // 10 base + 10 boost frase
		t.Errorf("Frase exata deveria dar bônus massivo, obteve %f", sFrase)
	}
}

func TestRichnessHeuristic(t *testing.T) {
	textRico := "Implementação de arquitetura escalável e performática utilizando goroutines assíncronas."
	textPobre := "fazer isso e aquilo com um teste de um dia"

	hRico := &models.SearchHit{Score: 10.0, Source: models.Document{Texto: textRico}}
	hPobre := &models.SearchHit{Score: 10.0, Source: models.Document{Texto: textPobre}}

	longRico := ""
	for i := 0; i < 5; i++ {
		longRico += textRico + " "
	}
	hRico.Source.Texto = longRico

	longPobre := ""
	for i := 0; i < 5; i++ {
		longPobre += textPobre + " "
	}
	hPobre.Source.Texto = longPobre

	sRicoNew, _ := ScoreFragment(hRico, []string{"x"}, "x", "", 0, 0)
	sPobreNew, _ := ScoreFragment(hPobre, []string{"x"}, "x", "", 0, 0)

	if sRicoNew <= sPobreNew {
		t.Errorf("Texto técnico com palavras longas deveria pontuar mais em riqueza: %f <= %f", sRicoNew, sPobreNew)
	}
}

func TestMaxPopularityLog(t *testing.T) {
	h := &models.SearchHit{Score: 10.0}

	// Testar se o crescimento é logarítmico (não explode)
	s10, _ := ScoreFragment(h, []string{"x"}, "x", "", 10, 0)
	s1000, _ := ScoreFragment(h, []string{"x"}, "x", "", 1000, 0)

	diff := s1000 - s10
	if diff > 10 {
		t.Errorf("O bônus de popularidade cresceu demais: diff %f", diff)
	}
}

func TestIsStopword(t *testing.T) {
	tests := []struct {
		word     string
		expected bool
	}{
		{"de", true},
		{"DA", true},
		{"computador", false},
		{"para", true},
		{"pelo", true},
		{"algoritmo", false},
		{"que", true},
		{"um", true},
	}
	for _, tt := range tests {
		if got := isStopword(tt.word); got != tt.expected {
			t.Errorf("isStopword(%q) = %v, want %v", tt.word, got, tt.expected)
		}
	}
}

func TestScoreFragmentStopwords(t *testing.T) {
	hit := &models.SearchHit{
		Score: 10.0,
		Source: models.Document{
			Secao: "Manual de Uso",
			Texto: "o manual de uso explica como usar de forma correta",
		},
	}

	_, details1 := ScoreFragment(hit, []string{"manual"}, "manual", "", 0, 0)
	if details1["titulo"] == 0 {
		t.Error("Palavra forte 'manual' deveria ter ganho bônus de título")
	}

	score2, details2 := ScoreFragment(hit, []string{"de"}, "de", "", 0, 0)
	if details2["titulo"] != 0 {
		t.Error("Stopword 'de' NÃO deveria ganhar bônus de título")
	}
	if math.Abs(score2-10.0) > 0.01 {
		t.Errorf("Stopword 'de' deveria manter o score base 10.0, mas obteve %f", score2)
	}
}

func TestUnicodeRanking(t *testing.T) {
	hit := &models.SearchHit{
		Score: 10.0,
		Source: models.Document{
			Secao: "Configuração de Situação",
			Texto: "Conteúdo sobre situação",
		},
	}

	_, details := ScoreFragment(hit, []string{"situação"}, "situação", "", 0, 0)
	if details["titulo"] == 0 {
		t.Error("Deveria ter ganho bônus de título para 'situação' (com acento)")
	}
}

func TestSortHitsByScoreTimestampTieBreak(t *testing.T) {
	agora := "2026-04-16T02:00:00Z"
	depois := "2026-04-16T02:05:00Z"

	hits := []models.SearchHit{
		{
			FinalScore: 10.0,
			Source:     models.Document{Arquivo: "antiga.md", Timestamp: agora},
		},
		{
			FinalScore: 10.0,
			Source:     models.Document{Arquivo: "nova.md", Timestamp: depois},
		},
		{
			FinalScore: 15.0,
			Source:     models.Document{Arquivo: "relevante.md", Timestamp: agora},
		},
	}

	SortHitsByScore(hits)

	if hits[0].Source.Arquivo != "relevante.md" {
		t.Errorf("Esperava 'relevante.md' (maior score) no topo, obteve %s", hits[0].Source.Arquivo)
	}
	if hits[1].Source.Arquivo != "nova.md" {
		t.Errorf("Esperava 'nova.md' (empate de score, mais recente) em segundo, obteve %s", hits[1].Source.Arquivo)
	}
	if hits[2].Source.Arquivo != "antiga.md" {
		t.Errorf("Esperava 'antiga.md' (empate de score, mais antiga) em último, obteve %s", hits[2].Source.Arquivo)
	}
}

func TestTagMatchBoost(t *testing.T) {
	hA := &models.SearchHit{
		Score: 10.0,
		Source: models.Document{
			Texto: "o curso de golang é muito bom",
			Tags:  []string{"curso"},
		},
	}

	hB := &models.SearchHit{
		Score: 10.0,
		Source: models.Document{
			Texto: "introdução básica",
			Tags:  []string{"golang", "intro"},
		},
	}

	query := []string{"golang"}

	sA, _ := ScoreFragment(hA, query, "golang", "", 0, 0)
	sB, _ := ScoreFragment(hB, query, "golang", "", 0, 0)

	if sB <= sA {
		t.Errorf("Documento com tag exata deveria pontuar mais que documento com apenas o termo no texto: %f <= %f", sB, sA)
	}

	if (sB - sA) < 9.0 { // Ajustado para refletir a lógica aditiva atual (base 10 vs tag 20?)
		t.Errorf("Bônus de tag insuficiente detectado: diff %f", sB-sA)
	}
}
