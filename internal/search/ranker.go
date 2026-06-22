package search

import (
	"math"
	"strings"
	"time"
)

// scoreFragment recalcula o score de um hit com pesos relativos ao BM25.
// BM25 é convertido para positivo e usado como base. Os bônus são frações
// do BM25 ou pequenos valores aditivos.
func scoreFragment(hit *SearchHit, queryTerms []string, cleanedQuery string, synapticWeight float64, backlinkCount int) (float64, map[string]float64) {
	details := make(map[string]float64)

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

	// ── CAP: no máximo 5× o BM25 base antes do RLHF ──
	if score > baseScore*5.0 {
		score = baseScore * 5.0
		details["capped_base"] = score
	}

	// ── INTEGRAÇÃO RLHF + GRAFO (Backlinks) ──
	// Se o peso sináptico for inválido ou zero (ex: em testes antigos), assume 1.0
	if synapticWeight <= 0 {
		synapticWeight = 1.0
	}

	// Transclusão: Cada backlink que aponta para esta nota dá +0.5 no multiplicador (capado em +3.0)
	structuralBonus := math.Min(3.0, float64(backlinkCount)*0.5)
	
	// Peso Combinado = Peso Dinâmico + Bônus do Grafo
	finalWeight := synapticWeight + structuralBonus

	// Multiplica o score final (BM25 + Heurísticas textuais) pelo peso acumulado
	score = score * finalWeight

	details["rlhf_weight"] = finalWeight
	details["backlinks"] = float64(backlinkCount)

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
