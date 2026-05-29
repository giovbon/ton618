package search

import (
	"strings"
	"time"
)

// scoreFragment recalcula o score de um hit com pesos relativos ao BM25.
// BM25 é convertido para positivo e usado como base. Os bônus são frações
// do BM25 ou pequenos valores aditivos. Tudo é capado em 5× o BM25 base.
func scoreFragment(hit *SearchHit, queryTerms []string, cleanedQuery string, popularity int, linkCount int) (float64, map[string]float64) {
	details := make(map[string]float64)
	_ = popularity // removido do re-ranker (já refletido no FTS5 via popularidade de acesso)
	_ = linkCount  // removido do re-ranker (sinal global, não relacionado à consulta)

	// BM25 base: converte rank negativo em positivo
	baseScore := -hit.Score
	if baseScore < 0.1 {
		baseScore = 0.1
	}
	score := baseScore
	details["bm25"] = baseScore

	textoBaixo := strings.ToLower(hit.Doc.Texto)

	// ── TÍTULO: % do BM25 ──
	if b := scoreTitle(hit.Doc.Secao, queryTerms); b > 0 {
		val := baseScore * b * 0.5 // +50% exato, +20% parcial
		score += val
		details["titulo"] = val
	}

	// ── FRASE EXATA: % do BM25 base (não acumulado) ──
	if len(strings.Fields(cleanedQuery)) > 1 && strings.Contains(textoBaixo, cleanedQuery) {
		val := baseScore * 0.5 // +50% do BM25 base
		score += val
		details["frase_exata"] = val
	}

	// ── CAMINHO: aditivo pequeno ──
	if b := scorePath(hit.Doc.Arquivo, queryTerms); b > 0 {
		score += b
		details["caminho"] = b
	}

	// ── RECÊNCIA: aditivo pequeno ──
	if b := scoreFreshness(hit.Doc.Timestamp); b > 0 {
		score += b
		details["recencia"] = b
	}

	// ── CAP: no máximo 5× o BM25 base ──
	if score > baseScore*5.0 {
		score = baseScore * 5.0
		details["capped"] = score
	}

	return score, details
}

// scoreTitle retorna 1.0 para match exato, 0.4 para parcial.
// O caller multiplica por 0.5 para obter a fração do BM25.
func scoreTitle(secao string, terms []string) float64 {
	parts := strings.Split(secao, " › ")
	last := strings.ToLower(parts[len(parts)-1])
	for _, term := range terms {
		t := strings.ToLower(term)
		if stopwords[t] || len(t) < 3 {
			continue
		}
		if last == t {
			return 1.0 // exato → +50% do BM25
		}
		if strings.Contains(last, t) {
			return 0.4 // parcial → +20% do BM25
		}
	}
	return 0
}

// scorePath retorna +0.5 por termo encontrado no nome do arquivo.
func scorePath(arquivo string, terms []string) float64 {
	base := strings.ToLower(arquivo)
	boost := 0.0
	for _, term := range terms {
		t := strings.ToLower(term)
		if stopwords[t] || len(t) < 3 {
			continue
		}
		if strings.Contains(base, t) {
			boost += 0.5
		}
	}
	return boost
}

// scoreFreshness retorna bônus de recência: 0.5 (hoje), 0.25 (7 dias), 0.1 (30 dias), 0 (mais velho).
func scoreFreshness(timestamp string) float64 {
	if timestamp == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return 0
	}
	daysOld := time.Since(t).Hours() / 24

	const maxBonus = 0.5
	if daysOld < 1 {
		return maxBonus
	}
	if daysOld < 7 {
		return maxBonus * 0.5
	}
	if daysOld < 30 {
		return maxBonus * 0.2
	}
	return 0
}
