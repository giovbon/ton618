package search

import (
	"context"
	"log"
	"math"
	"regexp"
	"sort"
	"strings"

	"ton618/internal/db"
)

type SearchHit struct {
	ID           string
	Score        float64
	Doc          db.Document
	FinalScore   float64
	ScoreDetails map[string]float64
	Highlight    map[string][]string
}

type SearchResults struct {
	Hits  []SearchHit
	Total int
}

var (
	quoteRegex     = regexp.MustCompile(`"([^"]*)"`)
	tagFilterRegex = regexp.MustCompile(`\+?tags:("[^"]+"|[^\s]+)`)
	cleanQueryRe   = regexp.MustCompile(`[\+\*"]`)
	spacesRe       = regexp.MustCompile(`\s+`)
	nativeHashtag  = regexp.MustCompile(`(?:\s|^)#([a-zA-Z0-9_À-ÿ\-]+)([?!]*)`)
)

var stopwords = map[string]bool{
	"de": true, "da": true, "do": true, "em": true, "no": true, "na": true,
	"um": true, "uma": true, "os": true, "as": true, "com": true, "por": true,
	"para": true, "que": true, "seu": true, "sua": true, "dos": true, "das": true,
	"pelo": true, "pela": true, "nos": true, "nas": true, "o": true, "a": true,
	"e": true, "the": true, "and": true, "or": true, "of": true, "to": true, "in": true,
}

type RankWeights struct {
	BaseMultiplier     float64
	BoostTitleExact    float64
	BoostTitlePartial  float64
	BoostPhrase        float64
	BoostFreshness     float64
	BoostLinkAuthority float64
	BoostTag           float64
}

var weights = RankWeights{
	BaseMultiplier:     1.0,
	BoostTitleExact:    2.0,
	BoostTitlePartial:  0.8,
	BoostPhrase:        1.5,
	BoostFreshness:     0.5,
	BoostLinkAuthority: 1.5,
	BoostTag:           3.0,
}

func Search(ctx context.Context, store *db.Store, rawQuery string, from, size int, getLinkCount func(string) int, getPopularity func(string) int) (*SearchResults, error) {
	if rawQuery == "" || rawQuery == "*" {
		return listAll(store, from, size)
	}

	// 1. Try FTS5 first
	ftsQuery := buildFTSQuery(rawQuery)
	results, total, err := store.SearchFTS(ftsQuery, from, size*3) // Get more for re-ranking
	if err != nil {
		log.Printf("[Search] FTS5 error: %v, falling back to LIKE\n", err)
		results, total, _ = store.SearchFTSLike(rawQuery, 0, size*3)
	}

	// 2. If few results, expand with LIKE (fuzzy fallback)
	if total < 3 {
		likeResults, likeTotal, _ := store.SearchFTSLike(rawQuery, 0, size*3)
		// Merge, deduplicating
		seen := make(map[string]bool)
		for _, r := range results {
			seen[r.DocID] = true
		}
		for _, r := range likeResults {
			if !seen[r.DocID] {
				results = append(results, r)
				total += 1
				seen[r.DocID] = true
			}
		}
		_ = likeTotal
	}

	// 3. Convert to SearchHits and re-rank
	heuristicTerms := extractTerms(rawQuery)
	cleanedQuery := cleanQuery(rawQuery)

	var hits []SearchHit
	for _, r := range results {
		// Fetch full doc
		doc, _ := store.GetDocument(r.DocID)
		if doc == nil {
			continue
		}

		hit := SearchHit{
			ID:    r.DocID,
			Score: r.Rank,
			Doc:   *doc,
		}

		pop := getPopularity(doc.Arquivo)
		linkCount := getLinkCount(strings.ToLower(doc.Arquivo))

		score, details := scoreFragment(&hit, heuristicTerms, cleanedQuery, pop, linkCount)
		hit.FinalScore = score
		hit.ScoreDetails = details
		hit.Highlight = buildHighlight(doc.Texto, heuristicTerms)

		hits = append(hits, hit)
	}

	// 4. Sort by final score
	sort.Slice(hits, func(i, j int) bool {
		if math.Abs(hits[i].FinalScore-hits[j].FinalScore) < 0.0001 {
			return hits[i].Doc.Timestamp > hits[j].Doc.Timestamp
		}
		return hits[i].FinalScore > hits[j].FinalScore
	})

	// 5. Trim to requested size
	if len(hits) > size {
		hits = hits[:size]
	}

	return &SearchResults{Hits: hits, Total: total}, nil
}

func listAll(store *db.Store, from, size int) (*SearchResults, error) {
	results, total, err := store.SearchFTS("", from, size)
	if err != nil {
		return nil, err
	}

	var hits []SearchHit
	for _, r := range results {
		doc, _ := store.GetDocument(r.DocID)
		if doc == nil {
			continue
		}
		hits = append(hits, SearchHit{
			ID:         r.DocID,
			Score:      1.0,
			Doc:        *doc,
			FinalScore: 1.0,
		})
	}
	return &SearchResults{Hits: hits, Total: total}, nil
}

// buildFTSQuery converte query do usuário para sintaxe FTS5.
func buildFTSQuery(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var parts []string

	// Extract quoted phrases
	quotes := quoteRegex.FindAllStringSubmatch(raw, -1)
	for _, q := range quotes {
		if len(q) == 2 && strings.TrimSpace(q[1]) != "" {
			parts = append(parts, `"`+strings.TrimSpace(q[1])+`"`)
		}
		raw = strings.Replace(raw, q[0], " ", 1)
	}

	// Remove tag filters
	raw = tagFilterRegex.ReplaceAllString(raw, " ")

	// Process remaining words
	words := strings.Fields(raw)
	for _, w := range words {
		w = strings.Trim(w, "?,;.:!+-")
		if w == "" || stopwords[strings.ToLower(w)] {
			continue
		}
		// Add prefix wildcard (FTS5 suporta prefixo com *)
		if len(w) > 2 {
			parts = append(parts, strings.ToLower(w)+"*")
		} else {
			parts = append(parts, strings.ToLower(w))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// extractTerms extrai termos relevantes da query (para re-ranking).
func extractTerms(raw string) []string {
	clean := tagFilterRegex.ReplaceAllString(raw, " ")
	clean = cleanQueryRe.ReplaceAllString(clean, " ")
	words := strings.Fields(strings.ToLower(clean))

	var terms []string
	for _, w := range words {
		w = strings.Trim(w, "?,;.:!+-")
		if w != "" && !stopwords[w] && len(w) > 1 {
			terms = append(terms, w)
		}
	}
	return terms
}

func cleanQuery(raw string) string {
	clean := strings.ToLower(cleanQueryRe.ReplaceAllString(raw, " "))
	return strings.TrimSpace(spacesRe.ReplaceAllString(clean, " "))
}

// extractTags extrai tags formatadas (tags:xxx ou #hashtag) da query bruta.
func extractTags(raw string) (tags []string, remaining string) {
	remaining = raw
	matches := tagFilterRegex.FindAllStringSubmatch(remaining, -1)
	for _, m := range matches {
		if len(m) > 1 {
			tags = append(tags, strings.ToLower(strings.Trim(m[1], `"`)))
			remaining = strings.Replace(remaining, m[0], " ", 1)
		}
	}
	hashtags := nativeHashtag.FindAllStringSubmatch(remaining, -1)
	for _, m := range hashtags {
		if len(m) > 1 {
			tags = append(tags, strings.ToLower(m[1]))
			remaining = strings.Replace(remaining, m[0], " ", 1)
		}
	}
	return tags, remaining
}

// buildHighlight gera fragmentos de destaque (simplificado).
func buildHighlight(text string, terms []string) map[string][]string {
	if len(terms) == 0 {
		return nil
	}
	lower := strings.ToLower(text)
	var fragments []string
	for _, term := range terms {
		idx := strings.Index(lower, term)
		if idx >= 0 {
			start := idx - 40
			if start < 0 {
				start = 0
			}
			end := idx + len(term) + 40
			if end > len(text) {
				end = len(text)
			}
			if start > 0 {
				fragments = append(fragments, "..."+text[start:end]+"...")
			} else {
				fragments = append(fragments, text[start:end])
			}
		}
	}
	if len(fragments) > 0 {
		return map[string][]string{"texto": fragments}
	}
	return nil
}
