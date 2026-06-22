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
	quoteRegex       = regexp.MustCompile(`"([^"]*)"`)
	singleQuoteRegex = regexp.MustCompile(`'([^']*)'`)
	tagFilterRegex   = regexp.MustCompile(`\+?tags:("[^"]+"|[^\s]+)`)
	cleanQueryRe     = regexp.MustCompile(`[\+\*"]`)
	spacesRe         = regexp.MustCompile(`\s+`)
	nativeHashtag    = regexp.MustCompile(`(?:\s|^)#([a-zA-Z0-9_À-ÿ\-]+)([?!]*)`)
	ftsUnsafeRe      = regexp.MustCompile(`[\^~()]`) // chars que interferem com sintaxe FTS5
)

// sanitizeFTS5Term removes characters that interfere with FTS5 query syntax.
func sanitizeFTS5Term(term string) string {
	term = ftsUnsafeRe.ReplaceAllString(term, "")
	if len(term) > 100 {
		term = term[:100]
	}
	return term
}

var stopwords = map[string]bool{
	"de": true, "da": true, "do": true, "em": true, "no": true, "na": true,
	"um": true, "uma": true, "os": true, "as": true, "com": true, "por": true,
	"para": true, "que": true, "seu": true, "sua": true, "dos": true, "das": true,
	"pelo": true, "pela": true, "nos": true, "nas": true, "o": true, "a": true,
	"e": true, "the": true, "and": true, "or": true, "of": true, "to": true, "in": true,
}

func Search(ctx context.Context, store *db.Store, rawQuery string, from, size int, getBacklinkCount func(string) int, getSynapticWeight func(string) float64) (*SearchResults, error) {
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

		weight := getSynapticWeight(doc.Arquivo)
		backlinkCount := getBacklinkCount(strings.ToLower(doc.Arquivo))

		score, details := scoreFragment(&hit, heuristicTerms, cleanedQuery, weight, backlinkCount)
		hit.FinalScore = score
		hit.ScoreDetails = details
		if r.Snippet != "" {
			hit.Highlight = map[string][]string{"texto": {r.Snippet}}
		}

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
	docs, total, err := store.GetDocumentsPaginated(from, size)
	if err != nil {
		return nil, err
	}

	var hits []SearchHit
	for _, doc := range docs {
		hits = append(hits, SearchHit{
			ID:         doc.ID,
			Doc:        doc,
			Score:      1.0,
			FinalScore: 1.0,
		})
	}
	return &SearchResults{Hits: hits, Total: total}, nil
}

// buildFTSQuery converte query do usuário para sintaxe FTS5 com pesos por coluna.
// Pesos: tags 50x, arquivo 20x, secao 10x, texto 1x
func buildFTSQuery(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var parts []string

	// Extract exact quoted phrases — aplica a todas as colunas
	quotes := quoteRegex.FindAllStringSubmatch(raw, -1)
	for _, q := range quotes {
		if len(q) == 2 && strings.TrimSpace(q[1]) != "" {
			ph := `"` + strings.TrimSpace(q[1]) + `"`
			parts = append(parts, `(tags:`+ph+` OR arquivo:`+ph+` OR secao:`+ph+` OR texto:`+ph+`)`)
		}
		raw = strings.Replace(raw, q[0], " ", 1)
	}

	// Extract proximity phrases in single quotes
	proximity := singleQuoteRegex.FindAllStringSubmatch(raw, -1)
	for _, q := range proximity {
		if len(q) == 2 && strings.TrimSpace(q[1]) != "" {
			ph := buildProximityExpression(strings.TrimSpace(q[1]))
			if ph != "" {
				parts = append(parts, ph)
			}
		}
		raw = strings.Replace(raw, q[0], " ", 1)
	}

	// Remove tag filters
	raw = tagFilterRegex.ReplaceAllString(raw, " ")

	// Process remaining words
	words := strings.Fields(raw)
	for _, w := range words {
		w = strings.Trim(w, "?,;.:!+-")
		w = sanitizeFTS5Term(w)
		if w == "" || stopwords[strings.ToLower(w)] {
			continue
		}
		wLower := strings.ToLower(w)
		if len(wLower) > 2 {
			wLower += "*"
		}
		// Column weights: tags > arquivo > secao > texto
		parts = append(parts, `(tags:`+wLower+` OR arquivo:`+wLower+` OR secao:`+wLower+` OR texto:`+wLower+`)`)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " AND ")
}

func buildProximityExpression(phrase string) string {
	words := strings.Fields(phrase)
	if len(words) == 0 {
		return ""
	}
	if len(words) == 1 {
		w := strings.Trim(words[0], "?,;.:!+-")
		if w == "" || stopwords[strings.ToLower(w)] {
			return ""
		}
		return `(tags:` + w + ` OR arquivo:` + w + ` OR secao:` + w + ` OR texto:` + w + `)`
	}

	// FTS5 nao suporta NEAR. Usa AND simples para que o LIKE fallback
	// (no search.go) possa fazer a correspondencia de proximidade.
	var colQueries []string
	for _, col := range []string{"texto", "secao"} {
		var colTerms []string
		for _, w := range words {
			w = strings.Trim(w, "?,;.:!+-")
			if w == "" || stopwords[strings.ToLower(w)] {
				continue
			}
			colTerms = append(colTerms, col+":\""+w+"\"")
		}
		if len(colTerms) >= 2 {
			colQueries = append(colQueries, "("+strings.Join(colTerms, " AND ")+")")
		}
	}
	// Fallback: se apos filtrar stopwords sobrou 1 termo, retorna como termo unico
	if len(colQueries) == 0 {
		var single string
		for _, w := range words {
			w = strings.Trim(w, "?,;.:!+-")
			if w == "" || stopwords[strings.ToLower(w)] {
				continue
			}
			single = w
			break
		}
		if single != "" {
			return `(tags:` + single + ` OR arquivo:` + single + ` OR secao:` + single + ` OR texto:` + single + `)`
		}
	}
	if len(colQueries) > 0 {
		return strings.Join(colQueries, " OR ")
	}
	return ""
}

// extractTerms extrai termos relevantes da query (para re-ranking).
// Preserva frases entre aspas duplas ou simples como termo unico.
func extractTerms(raw string) []string {
	var terms []string
	remaining := raw

	quotedRe := regexp.MustCompile(`"([^"]*)"|'([^']*)'`)
	for {
		m := quotedRe.FindStringSubmatch(remaining)
		if m == nil {
			break
		}
		phrase := strings.TrimSpace(m[1])
		if phrase == "" {
			phrase = strings.TrimSpace(m[2])
		}
		if len(phrase) > 1 && !stopwords[phrase] {
			terms = append(terms, phrase)
			// Adiciona palavras individuais como fallback
			for _, pw := range strings.Fields(phrase) {
				pl := strings.ToLower(strings.Trim(pw, "?,;.:!+-"))
				if pl != "" && !stopwords[pl] && len(pl) > 1 {
					terms = append(terms, pl)
				}
			}
		}
		remaining = strings.Replace(remaining, m[0], " ", 1)
	}

	clean := tagFilterRegex.ReplaceAllString(remaining, " ")
	clean = cleanQueryRe.ReplaceAllString(clean, " ")
	words := strings.Fields(strings.ToLower(clean))

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


