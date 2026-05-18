package search

import (
	"math"
	"strings"
	"time"

	"ton618/internal/db"
)

// scoreFragment recalcula o score de um hit com pesos heurísticos (estilo LEGADO).
func scoreFragment(hit *SearchHit, queryTerms []string, cleanedQuery string, popularity int, linkCount int) (float64, map[string]float64) {
	details := make(map[string]float64)

	textoBaixo := strings.ToLower(hit.Doc.Texto)

	// ── 1. BASE ──
	score := hit.Score * weights.BaseMultiplier
	details["bm25_base"] = score

	// ── 2. TÍTULO (seção) ──
	titleBoost := scoreTitle(hit.Doc.Secao, queryTerms)
	if titleBoost > 0 {
		val := 10.0 * titleBoost
		score += val
		details["titulo"] = val
	}

	// ── 3. FRASE EXATA ──
	if len(strings.Fields(cleanedQuery)) > 1 && strings.Contains(textoBaixo, cleanedQuery) {
		val := score * weights.BoostPhrase // +120% do score atual
		score += val
		details["frase_exata"] = val
	}

	// ── 4. CAMINHO (nome do arquivo) ──
	pathBoost := scorePath(hit.Doc.Arquivo, queryTerms)
	if pathBoost > 0 {
		score += pathBoost
		details["caminho"] = pathBoost
	}

	// ── 5. TAGS E KEYWORDS ──
	keywordBonus := scoreKeywords(hit.Doc.Tags, textoBaixo, queryTerms)
	if keywordBonus > 0 {
		score += keywordBonus
		details["keywords"] = keywordBonus
	}

	// ── 6. RECÊNCIA ──
	freshness := scoreFreshness(hit.Doc.Timestamp)
	if freshness > 0 {
		score += freshness
		details["recencia"] = freshness
	}

	// ── 7. RIQUEZA ESTRUTURAL ──
	richness := scoreRichness(hit.Doc.Texto)
	if richness > 0 {
		score += richness
		details["riqueza"] = richness
	}

	// ── 8. POPULARIDADE + LINKS ──
	if popularity > 0 {
		val := math.Log2(float64(popularity+1)) * 1.0
		score += val
		details["popularidade"] = val
	}
	if linkCount > 0 {
		val := math.Log2(float64(linkCount+1)) * weights.BoostLinkAuthority
		score += val
		details["autoridade"] = val
	}

	return score, details
}

func scoreTitle(secao string, terms []string) float64 {
	parts := strings.Split(secao, " › ")
	last := strings.ToLower(parts[len(parts)-1])
	boost := 0.0
	for _, term := range terms {
		t := strings.ToLower(term)
		if stopwords[t] || len(t) < 3 {
			continue
		}
		if last == t {
			boost += weights.BoostTitleExact
		} else if strings.Contains(last, t) {
			boost += weights.BoostTitlePartial
		}
	}
	return boost
}

func scoreKeywords(tagsData string, textoBaixo string, terms []string) float64 {
	docTags := db.TagsToSlice(tagsData)
	bonus := 0.0
	for _, term := range terms {
		t := strings.ToLower(term)
		if stopwords[t] || len(t) < 3 {
			continue
		}
		// Match exato em tag → boost massivo
		for _, tag := range docTags {
			if strings.ToLower(tag) == t {
				bonus += 3.0
				goto next
			}
		}
		// Match no texto
		if strings.Contains(textoBaixo, t) {
			bonus += 1.0
		} else {
			// Match por radical (4 primeiras letras)
			stem := t
			if len(t) > 4 {
				stem = t[:4]
			}
			if strings.Contains(textoBaixo, stem) {
				bonus += 0.5
			}
		}
	next:
	}
	return bonus
}

func scorePath(arquivo string, terms []string) float64 {
	base := strings.ToLower(arquivo)
	boost := 0.0
	for _, term := range terms {
		t := strings.ToLower(term)
		if stopwords[t] || len(t) < 3 {
			continue
		}
		if strings.Contains(base, t) {
			boost += weights.BoostPathContext
		}
	}
	return boost
}

func scoreFreshness(timestamp string) float64 {
	if timestamp == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return 0
	}
	daysOld := time.Since(t).Hours() / 24

	if daysOld < 1 {
		return weights.BoostFreshness // 0.5
	}
	if daysOld < 7 {
		return weights.BoostFreshness * 0.5
	}
	if daysOld < 30 {
		return weights.BoostFreshness * 0.2
	}
	return 0
}

// scoreRichness avalia a "riqueza estrutural" do documento (tabelas, código, etc.)
func scoreRichness(texto string) float64 {
	bonus := 0.0
	totalWords := 0
	longWords := 0
	wordLen := 0

	for _, r := range texto {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' || r == '<' || r == '>' {
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
	if wordLen > 0 {
		totalWords++
		if wordLen > 8 {
			longWords++
		}
	}

	if totalWords > 20 && longWords > 5 {
		bonus += 1.0
	}
	if strings.Contains(texto, "|--|") || strings.Contains(texto, "<table") {
		bonus += weights.BoostTechnical
	}
	if strings.Contains(texto, "```") || strings.Contains(texto, "<pre>") {
		bonus += weights.BoostTechnical
	}

	// Bônus por tamanho (0.5 a cada 500 palavras, máx 2.0)
	lengthBonus := (float64(totalWords) / 500.0) * 0.5
	if lengthBonus > 2.0 {
		lengthBonus = 2.0
	}
	bonus += lengthBonus

	return bonus
}
