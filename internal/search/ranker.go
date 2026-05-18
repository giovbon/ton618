package search

import (
	"math"
	"strings"
	"time"

	"ton618/internal/db"
)

// scoreFragment recalcula o score de um hit com pesos customizados.
func scoreFragment(hit *SearchHit, queryTerms []string, cleanedQuery string, popularity int, linkCount int) (float64, map[string]float64) {
	details := make(map[string]float64)

	textoBaixo := strings.ToLower(hit.Doc.Texto)

	// 1. Base score
	score := hit.Score * weights.BaseMultiplier
	details["base"] = score

	// 2. Title boost
	titleBoost := scoreTitle(hit.Doc.Secao, queryTerms)

	// 3. Tag boost
	tagBonus := 0.0
	docTags := db.TagsToSlice(hit.Doc.Tags)
	for _, term := range queryTerms {
		t := strings.ToLower(term)
		if stopwords[t] || len(t) < 3 {
			continue
		}
		for _, tag := range docTags {
			if strings.ToLower(tag) == t {
				tagBonus += weights.BoostTag
				break
			}
		}
	}

	// 4. Phrase exact match
	hasPhrase := len(strings.Fields(cleanedQuery)) > 1 && strings.Contains(textoBaixo, cleanedQuery)

	// 5. Path boost
	pathBoost := scorePath(hit.Doc.Arquivo, queryTerms)

	// 6. Freshness
	freshness := scoreFreshness(hit.Doc.Timestamp)

	if titleBoost > 0 {
		val := 10.0 * titleBoost
		score += val
		details["titulo"] = val
	}

	if tagBonus > 0 {
		score += tagBonus
		details["tags"] = tagBonus
	}

	if hasPhrase {
		val := score * weights.BoostPhrase
		score += val
		details["frase_exata"] = val
	}

	if pathBoost > 0 {
		score += pathBoost
		details["caminho"] = pathBoost
	}

	if freshness > 0 {
		score += freshness
		details["recencia"] = freshness
	}

	if popularity > 0 {
		val := math.Log2(float64(popularity+1)) * 0.5
		score += val
		details["popularidade"] = val
	}

	if linkCount > 0 {
		val := math.Log2(float64(linkCount+1)) * weights.BoostLinkAuthority
		score += val
		details["autoridade_links"] = val
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
		return weights.BoostFreshness * 2
	}
	if daysOld < 7 {
		return weights.BoostFreshness
	}
	if daysOld < 30 {
		return weights.BoostFreshness * 0.3
	}
	return 0
}
