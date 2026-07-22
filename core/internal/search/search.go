package search

import (
	"context"
	"log"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"ton618/core/internal/core/db"
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
	quoteRegex       = regexp.MustCompile(`"([^"]+)"`)
	singleQuoteRegex = regexp.MustCompile(`'([^']+)'`)
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

	// Extrai tags e a query restante
	queryTags, remainingQuery := extractTags(rawQuery)
	remainingQuery = strings.TrimSpace(remainingQuery)

	// 1. Try FTS5 first
	ftsQuery := buildFTSQuery(rawQuery)
	// Se a busca textual é vazia mas temos tags, buscamos mais resultados para filtrar em Go
	limitSize := size * 3
	if remainingQuery == "" && len(queryTags) > 0 {
		limitSize = 99999
	}
	results, total, err := store.SearchFTS(ftsQuery, from, limitSize)
	if err != nil {
		log.Printf("[Search] FTS5 error: %v, falling back to LIKE\n", err)
		results, total, _ = store.SearchFTSLike(remainingQuery, 0, limitSize)
	}

	// 2. If few results, expand with LIKE (fuzzy fallback)
	if total < 3 && remainingQuery != "" {
		likeResults, likeTotal, _ := store.SearchFTSLike(remainingQuery, 0, size*3)
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

	// ── Batch-load timestamps para evitar N+1 ──
	docIDs := make([]string, len(results))
	for i, r := range results {
		docIDs[i] = r.DocID
	}
	timestampMap, _ := store.BatchGetTimestamps(docIDs)

	// ── Cache para getSynapticWeight e getBacklinkCount ──
	weightCache := make(map[string]float64, len(results))
	backlinkCache := make(map[string]int, len(results))

	var hits []SearchHit
	for _, r := range results {
		// Converte FTSResult → Document diretamente, sem chamar GetDocument
		ts := timestampMap[r.DocID]
		doc := db.Document{
			ID:        r.DocID,
			Tipo:      r.Tipo,
			Arquivo:   r.Arquivo,
			Secao:     r.Secao,
			Texto:     r.Texto,
			Tags:      r.Tags,
			Timestamp: ts,
		}

		// Filtro de tags como segunda linha de defesa (garante exatidão)
		if len(queryTags) > 0 {
			docTags := db.TagsToSlice(doc.Tags)
			docTagMap := make(map[string]bool)
			for _, dt := range docTags {
				docTagMap[strings.ToLower(dt)] = true
			}
			hasAllTags := true
			for _, qt := range queryTags {
				if !docTagMap[strings.ToLower(qt)] {
					hasAllTags = false
					break
				}
			}
			if !hasAllTags {
				continue
			}
		}

		hit := SearchHit{
			ID:    r.DocID,
			Score: r.Rank,
			Doc:   doc,
		}

		// Cache para peso sináptico
		weight, ok := weightCache[doc.Arquivo]
		if !ok {
			weight = getSynapticWeight(doc.Arquivo)
			weightCache[doc.Arquivo] = weight
		}

		// Cache para contagem de backlinks
		backlinkCount, ok := backlinkCache[doc.Arquivo]
		if !ok {
			backlinkCount = getBacklinkCount(strings.ToLower(doc.Arquivo))
			backlinkCache[doc.Arquivo] = backlinkCount
		}

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

	// Extrai tags e a query restante
	tags, remaining := extractTags(raw)
	remaining = strings.TrimSpace(remaining)

	var parts []string

	// Adiciona restrições de tags no formato FTS5 tags:"nome_da_tag"
	for _, tag := range tags {
		tag = sanitizeFTS5Term(tag)
		// Filtra apenas caracteres válidos para tag
		tag = strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
				return r
			}
			return -1
		}, tag)
		if tag != "" {
			parts = append(parts, `tags:"`+tag+`"`)
		}
	}

	if remaining != "" {
		// Extract exact quoted phrases — aplica a todas as colunas
		quotes := quoteRegex.FindAllStringSubmatch(remaining, -1)
		for _, q := range quotes {
			if len(q) == 2 && strings.TrimSpace(q[1]) != "" {
				phClean := sanitizeFTS5Term(strings.TrimSpace(q[1]))
				if phClean != "" {
					ph := `"` + phClean + `"`
					parts = append(parts, `(tags:`+ph+` OR arquivo:`+ph+` OR secao:`+ph+` OR texto:`+ph+`)`)
				}
			}
			remaining = strings.Replace(remaining, q[0], " ", 1)
		}

		// Extract proximity phrases in single quotes
		proximity := singleQuoteRegex.FindAllStringSubmatch(remaining, -1)
		for _, q := range proximity {
			if len(q) == 2 && strings.TrimSpace(q[1]) != "" {
				ph := buildProximityExpression(strings.TrimSpace(q[1]))
				if ph != "" {
					parts = append(parts, ph)
				}
			}
			remaining = strings.Replace(remaining, q[0], " ", 1)
		}

		// Substitui hífens e outros caracteres especiais por espaços na busca por palavras soltas,
		// permitindo que palavras como "e-mail" ou "go-lang" sejam divididas em termos FTS5 válidos.
		remainingCleaned := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return r
			}
			return ' '
		}, remaining)

		// Process remaining words
		words := strings.Fields(remainingCleaned)
		for _, w := range words {
			w = sanitizeFTS5Term(w)
			if w == "" || stopwords[strings.ToLower(w)] {
				continue
			}
			wLower := strings.ToLower(w)
			
			if len(wLower) > 2 {
				wLower += "*"
			}
			
			parts = append(parts, `(tags:`+wLower+` OR arquivo:`+wLower+` OR secao:`+wLower+` OR texto:`+wLower+`)`)
		}
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

	quotedRe := regexp.MustCompile(`"([^"]+)"|'([^']+)'`)
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

// removeAccents remove acentos e diacríticos de uma string.
func removeAccents(s string) string {
	r := strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c",
		"Á", "a", "À", "a", "Â", "a", "Ã", "a", "Ä", "a",
		"É", "e", "È", "e", "Ê", "e", "Ë", "e",
		"Í", "i", "Ì", "i", "Î", "i", "Ï", "i",
		"Ó", "o", "Ò", "o", "Ô", "o", "õ", "o", "Ö", "o",
		"Ú", "u", "Ù", "u", "Û", "u", "Ü", "u",
		"Ç", "c",
	)
	return r.Replace(s)
}





