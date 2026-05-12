package search

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"etl/internal/models"
)

var (
	reCleanQuery = regexp.MustCompile(`[\+\*"]`)
	reSpaces     = regexp.MustCompile(`\s+`)
)

// CleanQueryForMatch prepara a query bruta para comparações de frase exata, evitando processamento repetido.
func CleanQueryForMatch(raw string) string {
	clean := strings.ToLower(reCleanQuery.ReplaceAllString(raw, " "))
	return strings.TrimSpace(reSpaces.ReplaceAllString(clean, " "))
}

func ScoreFragment(hit *models.SearchHit, queryTerms []string, rawQuery string, cleanedQuery string, popularity int, linkCount int) (float64, map[string]float64) {
	details := make(map[string]float64)
	w := GetWeights()

	textoBaixo := strings.ToLower(hit.Source.Texto)
	if cleanedQuery == "" {
		cleanedQuery = CleanQueryForMatch(rawQuery)
	}

	// --- 1. CÁLCULO DA BASE DE RELEVÂNCIA ---
	score := hit.Score * w.BaseMultiplier
	details["zinc_base"] = score

	// --- 2. BÔNUS DE TEXTO E ESTRUTURA (LÉXICO/HEURÍSTICO) ---

	// Título
	titleBoost := scoreTitle(hit.Source.Secao, queryTerms, w)

	// Tags e Radicais
	keywordBonus := 0.0
	for _, term := range queryTerms {
		if isStopword(term) || len(term) < 3 {
			continue
		}
		t := strings.ToLower(term)

		// Match em Tag (Mais forte)
		isTag := false
		for _, tag := range hit.Source.Tags {
			if strings.ToLower(tag) == t {
				isTag = true
				break
			}
		}

		if isTag {
			keywordBonus += 20.0 // Bônus alto para tag exata
		} else if strings.Contains(textoBaixo, t) {
			keywordBonus += 1.0
		} else {
			stem := t
			if len(t) > 4 {
				stem = t[:4]
			}
			if strings.Contains(textoBaixo, stem) {
				keywordBonus += 0.5
			}
		}
	}

	// Frase Exata
	hasPhrase := len(strings.Fields(cleanedQuery)) > 1 && strings.Contains(textoBaixo, cleanedQuery)

	// Riqueza Estrutural (Palavras longas e técnicas)
	richnessBonus := 0.0
	wordLen := 0
	longWords := 0
	totalWords := 0

	for _, r := range hit.Source.Texto {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			if wordLen > 0 {
				totalWords++
				if wordLen > 8 {
					longWords++
				}
				wordLen = 0
			}
		} else {
			wordLen++
		}
	}
	// processar a última palavra
	if wordLen > 0 {
		totalWords++
		if wordLen > 8 {
			longWords++
		}
	}

	if totalWords > 20 && longWords > 5 {
		richnessBonus += 1.0
	}
	if strings.Contains(hit.Source.Texto, "|--|") {
		richnessBonus += w.BoostTechnical
	}
	if strings.Contains(hit.Source.Texto, "```") {
		richnessBonus += w.BoostTechnical
	}

	// Bônus adicional pelo tamanho do texto (0.5 a cada 500 palavras, máx 2.0)
	lengthBonus := (float64(totalWords) / 500.0) * 0.5
	if lengthBonus > 2.0 {
		lengthBonus = 2.0
	}
	richnessBonus += lengthBonus

	// --- 3. APLICAÇÃO DOS PESOS (MODO ESTRITO) ---

	// 1. Título
	if titleBoost > 0 {
		val := 10.0 * titleBoost
		score += val
		details["titulo"] = val
	}

	// 2. Frase Exata (Multiplicador sobre Score + Título)
	if hasPhrase {
		val := score * w.BoostPhrase
		score += val
		details["proximidade_exata"] = val
	}

	// 3. Caminho, Recência e Riqueza
	if pathBoost := scorePath(hit.Source.Arquivo, queryTerms, w); pathBoost > 0 {
		score += pathBoost
		details["caminho"] = pathBoost
	}
	if fBoost := scoreFreshness(hit.Source.Timestamp, w); fBoost > 0 {
		score += fBoost
		details["recencia"] = fBoost
	}
	if richnessBonus > 0 {
		score += richnessBonus
		details["riqueza_estrutural"] = richnessBonus
	}

	// 4. Keywords (Radicais/Tags)
	if keywordBonus > 0 {
		score += keywordBonus
		details["relevancia_textual"] = keywordBonus
	}

	// 5. Popularidade e Links
	if popularity > 0 {
		val := math.Log2(float64(popularity+1)) * 1.0
		score += val
		details["popularidade"] = val
	}
	if linkCount > 0 {
		val := math.Log2(float64(linkCount+1)) * w.BoostLinkAuthority
		score += val
		details["autoridade_links"] = val
	}

	return score, details
}

func scoreTitle(secao string, terms []string, w RankingWeights) float64 {
	parts := strings.Split(secao, " > ")
	last := strings.ToLower(parts[len(parts)-1])
	boost := 0.0
	for _, term := range terms {
		t := strings.ToLower(term)
		if isStopword(t) || len(t) < 3 {
			continue
		}
		if last == t {
			boost += w.BoostTitleExact
		} else if strings.Contains(last, t) {
			boost += w.BoostTitlePartial
		}
	}
	return boost
}

func scoreFreshness(timestamp string, w RankingWeights) float64 {
	if timestamp == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return 0
	}
	daysOld := time.Since(t).Hours() / 24

	if daysOld < 1 {
		return w.BoostFreshnessMax
	}
	if daysOld < 7 {
		return w.BoostFreshnessMax * 0.5
	}
	if daysOld < 30 {
		return w.BoostFreshnessMax * 0.2
	}
	return 0
}

func scorePath(arquivo string, terms []string, w RankingWeights) float64 {
	base := strings.ToLower(arquivo)
	boost := 0.0
	for _, term := range terms {
		t := strings.ToLower(term)
		if isStopword(t) || len(t) < 3 {
			continue
		}
		if strings.Contains(base, t) {
			boost += w.BoostPathContext
		}
	}
	return boost
}

func isStopword(t string) bool {
	return stopwords[strings.ToLower(t)]
}

func SortHitsByScore(hits []models.SearchHit) {
	sort.Slice(hits, func(i, j int) bool {
		if math.Abs(hits[i].FinalScore-hits[j].FinalScore) < 0.0001 {
			return hits[i].Source.Timestamp > hits[j].Source.Timestamp
		}
		return hits[i].FinalScore > hits[j].FinalScore
	})
}
