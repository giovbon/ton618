package search

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	bleveSearch "github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"

	"etl/internal/models"
)

type cacheEntry struct {
	Result    models.SearchResults
	FileIDs   []string
	Timestamp time.Time
}

var (
	cache      = make(map[string]cacheEntry)
	keys       = make([]string, 0)
	cacheMu    sync.RWMutex
	maxEntries = 15
	ttl        = 60 * time.Second
)

var stopwords = map[string]bool{
	"de": true, "da": true, "do": true, "em": true, "no": true, "na": true,
	"um": true, "uma": true, "os": true, "as": true, "com": true, "por": true,
	"para": true, "que": true, "seu": true, "sua": true, "dos": true, "das": true,
	"pelo": true, "pela": true, "nos": true, "nas": true, "o": true, "a": true,
	"e": true, "the": true, "and": true, "or": true, "of": true, "to": true, "in": true,
}

var (
	quoteRegex         = regexp.MustCompile(`"([^"]*)"`)
	sysFilterRegex     = regexp.MustCompile(`([\+\-]?)?([a-zA-Z0-9_]+:("[^"]+"|[^\s]+))`)
	nativeHashtagRegex = regexp.MustCompile(`(?:\s|^)#([a-zA-Z0-9_À-ÿ\-]+)([?!]*)`)
	tagRegex           = regexp.MustCompile(`\+?tags:("[^"]+"|[^\s]+)`)
	genericFilterRegex = regexp.MustCompile(`[a-zA-Z0-9_]+:("[^"]+"|[^\s]+)`)
)

func buildStandardWordQuery(word string) query.Query {
	cleanWord := strings.ToLower(word)
	if cleanWord == "" {
		return nil
	}

	if len(cleanWord) <= 2 {
		return bleve.NewMatchQuery(cleanWord)
	}

	if len(cleanWord) <= 4 {
		q1 := bleve.NewMatchQuery(cleanWord)
		q2 := bleve.NewPrefixQuery(cleanWord)
		return bleve.NewDisjunctionQuery(q1, q2)
	}

	q1 := bleve.NewMatchQuery(cleanWord)
	q2 := bleve.NewFuzzyQuery(cleanWord)
	q2.SetFuzziness(1)
	q3 := bleve.NewPrefixQuery(cleanWord)

	return bleve.NewDisjunctionQuery(q1, q2, q3)
}

func removeDiacritics(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case 'á', 'à', 'â', 'ã', 'ä':
			sb.WriteRune('a')
		case 'é', 'è', 'ê', 'ë':
			sb.WriteRune('e')
		case 'í', 'ì', 'î', 'ï':
			sb.WriteRune('i')
		case 'ó', 'ò', 'ô', 'õ', 'ö':
			sb.WriteRune('o')
		case 'ú', 'ù', 'û', 'ü':
			sb.WriteRune('u')
		case 'ç':
			sb.WriteRune('c')
		case 'ñ':
			sb.WriteRune('n')
		case 'Á', 'À', 'Â', 'Ã', 'Ä':
			sb.WriteRune('a')
		case 'É', 'È', 'Ê', 'Ë':
			sb.WriteRune('e')
		case 'Í', 'Ì', 'Î', 'Ï':
			sb.WriteRune('i')
		case 'Ó', 'Ò', 'Ô', 'Õ', 'Ö':
			sb.WriteRune('o')
		case 'Ú', 'Ù', 'Û', 'Ü':
			sb.WriteRune('u')
		case 'Ç':
			sb.WriteRune('c')
		case 'Ñ':
			sb.WriteRune('n')
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func buildCompactQuery(word string) query.Query {
	cleanWord := strings.ToLower(word)
	foldedWord := removeDiacritics(cleanWord)

	var queries []query.Query

	// Wildcard em arquivo
	w1 := bleve.NewWildcardQuery("*" + cleanWord + "*")
	w1.SetField("arquivo")
	queries = append(queries, w1)

	if foldedWord != cleanWord {
		w2 := bleve.NewWildcardQuery("*" + foldedWord + "*")
		w2.SetField("arquivo")
		queries = append(queries, w2)
	}

	// Wildcard em secao
	w3 := bleve.NewWildcardQuery("*" + cleanWord + "*")
	w3.SetField("secao")
	queries = append(queries, w3)

	if foldedWord != cleanWord {
		w4 := bleve.NewWildcardQuery("*" + foldedWord + "*")
		w4.SetField("secao")
		queries = append(queries, w4)
	}

	// Match queries para análise inteligente com analisador do campo
	mq1 := bleve.NewMatchQuery(cleanWord)
	mq1.SetField("arquivo")
	queries = append(queries, mq1)

	mq2 := bleve.NewMatchQuery(cleanWord)
	mq2.SetField("secao")
	queries = append(queries, mq2)

	if foldedWord != cleanWord {
		mq3 := bleve.NewMatchQuery(foldedWord)
		mq3.SetField("arquivo")
		queries = append(queries, mq3)

		mq4 := bleve.NewMatchQuery(foldedWord)
		mq4.SetField("secao")
		queries = append(queries, mq4)
	}

	// Term em tags
	t1 := bleve.NewTermQuery(cleanWord)
	t1.SetField("tags")
	queries = append(queries, t1)

	if foldedWord != cleanWord {
		t2 := bleve.NewTermQuery(foldedWord)
		t2.SetField("tags")
		queries = append(queries, t2)
	}

	return bleve.NewDisjunctionQuery(queries...)
}


func BuildBleveQuery(raw string, isCompact bool) query.Query {
	if raw == "" || raw == "*" {
		return bleve.NewMatchAllQuery()
	}

	var mustQueries []query.Query
	var mustNotQueries []query.Query

	remaining := raw

	tags, remaining := extractTagsFromQuery(remaining)
	for _, tag := range tags {
		q := bleve.NewTermQuery(tag)
		q.SetField("tags")
		mustQueries = append(mustQueries, q)
	}

	sysFilters := sysFilterRegex.FindAllStringSubmatch(remaining, -1)
	for _, m := range sysFilters {
		prefix := m[1]
		content := m[2]
		parts := strings.SplitN(content, ":", 2)
		if len(parts) == 2 && parts[0] != "tags" {
			field := parts[0]
			value := strings.Trim(parts[1], `"`)
			q := bleve.NewMatchQuery(value)
			q.SetField(field)

			if prefix == "-" {
				mustNotQueries = append(mustNotQueries, q)
			} else {
				mustQueries = append(mustQueries, q)
			}
		}
		remaining = strings.Replace(remaining, m[0], " ", 1)
	}

	quotes := quoteRegex.FindAllStringSubmatch(remaining, -1)
	for _, match := range quotes {
		if len(match) == 2 {
			phrase := strings.ToLower(strings.TrimSpace(match[1]))
			if len(phrase) > 0 {
				q := bleve.NewMatchPhraseQuery(phrase)
				q.SetField("texto")
				mustQueries = append(mustQueries, q)
			}
			remaining = strings.Replace(remaining, match[0], " ", 1)
		}
	}

	punctRegex := regexp.MustCompile(`[?,;.:]+`)
	remaining = punctRegex.ReplaceAllString(remaining, " ")
	words := strings.Fields(remaining)

	for _, word := range words {
		clean := strings.ToLower(word)
		prefix := ""
		if strings.HasPrefix(clean, "+") {
			prefix = "+"
			clean = clean[1:]
		} else if strings.HasPrefix(clean, "-") {
			prefix = "-"
			clean = clean[1:]
		}

		if clean == "" || (stopwords[clean] && (len(words) > 1 || len(mustQueries) > 0)) {
			continue
		}

		var q query.Query
		if isCompact {
			q = buildCompactQuery(clean)
		} else {
			q = buildStandardWordQuery(clean)
		}

		if prefix == "-" {
			mustNotQueries = append(mustNotQueries, q)
		} else {
			mustQueries = append(mustQueries, q)
		}
	}

	if len(mustQueries) == 0 && len(mustNotQueries) == 0 {
		return bleve.NewMatchAllQuery()
	}

	boolQuery := bleve.NewBooleanQuery()
	if len(mustQueries) > 0 {
		boolQuery.AddMust(mustQueries...)
	}
	if len(mustNotQueries) > 0 {
		boolQuery.AddMustNot(mustNotQueries...)
	}
	return boolQuery
}

func extractTagsFromQuery(raw string) (tags []string, remaining string) {
	remaining = raw
	matches := tagRegex.FindAllStringSubmatch(remaining, -1)
	for _, m := range matches {
		if len(m) > 1 {
			tags = append(tags, strings.ToLower(strings.Trim(m[1], `"`)))
			remaining = strings.Replace(remaining, m[0], " ", 1)
		}
	}
	hashtags := nativeHashtagRegex.FindAllStringSubmatch(remaining, -1)
	for _, m := range hashtags {
		if len(m) > 1 {
			tags = append(tags, strings.ToLower(m[1]))
			remaining = strings.Replace(remaining, m[0], " ", 1)
		}
	}
	return tags, remaining
}

func GetHeuristicTerms(raw string) []string {
	tags, remaining := extractTagsFromQuery(raw)
	clean := genericFilterRegex.ReplaceAllString(remaining, " ")

	words := strings.Fields(strings.ToLower(clean))
	filtered := []string{}
	for _, w := range words {
		if !stopwords[w] && len(w) > 1 {
			filtered = append(filtered, w)
		}
	}
	filtered = append(filtered, tags...)
	return filtered
}

func GetCachedResult(body []byte) (models.SearchResults, bool) {
	key := string(body)
	cacheMu.RLock()
	entry, exists := cache[key]
	expired := exists && time.Since(entry.Timestamp) >= ttl
	cacheMu.RUnlock()

	if !exists || expired {
		return models.SearchResults{}, false
	}

	cacheMu.Lock()
	for i, k := range keys {
		if k == key {
			keys = append(keys[:i], keys[i+1:]...)
			keys = append(keys, key)
			break
		}
	}
	cacheMu.Unlock()

	return entry.Result, true
}

func SetCachedResult(body []byte, result models.SearchResults) {
	key := string(body)

	// Coleta IDs de arquivos para invalidação granular
	fileIDs := make([]string, 0)
	for _, hit := range result.Hits.Hits {
		if hit.Source.Arquivo != "" {
			fileIDs = append(fileIDs, hit.Source.Arquivo)
		}
	}

	cacheMu.Lock()
	defer cacheMu.Unlock()
	if _, exists := cache[key]; !exists {
		if len(keys) >= maxEntries {
			oldest := keys[0]
			delete(cache, oldest)
			keys = append([]string{}, keys[1:]...)
		}
		keys = append(keys, key)
	} else {
		for i, k := range keys {
			if k == key {
				keys = append(keys[:i], keys[i+1:]...)
				keys = append(keys, key)
				break
			}
		}
	}
	cache[key] = cacheEntry{
		Result:    result,
		FileIDs:   fileIDs,
		Timestamp: time.Now(),
	}
}

func ExecuteSearch(ctx context.Context, rawQuery string, isCompact bool, from, size int) (models.SearchResults, error) {
	idx := GetIndex()
	if idx == nil {
		return models.SearchResults{}, fmt.Errorf("index not initialized")
	}

	q := BuildBleveQuery(rawQuery, isCompact)
	heuristicTerms := GetHeuristicTerms(rawQuery)
	cleanedQuery := CleanQueryForMatch(rawQuery)

	log.Printf("[Search] Query: '%s' (Compact: %v)\n", rawQuery, isCompact)

	searchRequest := bleve.NewSearchRequest(q)
	searchRequest.From = from
	searchRequest.Size = size

	if isCompact {
		// No modo compacto, ordenamos por data de criação (mais recentes primeiro)
		searchRequest.SortBy([]string{"-created_at", "-_score"})
		if searchRequest.Size < 300 {
			searchRequest.Size = 300
		}
	}

	searchRequest.Highlight = bleve.NewHighlight()
	searchRequest.Fields = []string{"arquivo", "secao", "tags", "tipo", "ordem", "pagina", "@timestamp", "created_at", "texto"}

	results, err := idx.Search(searchRequest)
	if err != nil {
		return models.SearchResults{}, err
	}

	finalResult := models.SearchResults{
		SemanticSimilarities: make(map[string]float64),
	}
	finalResult.Hits.Total.Value = int(results.Total)

	for _, hit := range results.Hits {
		searchHit := models.SearchHit{ID: hit.ID, Score: hit.Score}
		doc := models.Document{ID: hit.ID}
		fillFields(&doc, &searchHit, hit)

		// Re-score com pesos customizados. Passamos 0 para popularidade e links, o PostProcess cuidará do refinamento final.
		score, details := ScoreFragment(&searchHit, heuristicTerms, rawQuery, cleanedQuery, 0, 0)
		searchHit.FinalScore = score
		searchHit.ScoreDetails = details

		finalResult.Hits.Hits = append(finalResult.Hits.Hits, searchHit)
	}

	SortHitsByScore(finalResult.Hits.Hits)
	return finalResult, nil
}

func fillFields(doc *models.Document, searchHit *models.SearchHit, hit *bleveSearch.DocumentMatch) {
	getString := func(key string) string {
		val, ok := hit.Fields[key]
		if !ok {
			return ""
		}
		if s, ok := val.(string); ok {
			return s
		}
		if slice, ok := val.([]interface{}); ok && len(slice) > 0 {
			if s, ok := slice[0].(string); ok {
				return s
			}
		}
		return ""
	}

	doc.Tipo = getString("tipo")
	doc.Arquivo = getString("arquivo")
	doc.Secao = getString("secao")
	doc.Texto = getString("texto")
	doc.Timestamp = getString("@timestamp")
	doc.Created = getString("created_at")

	if val, ok := hit.Fields["pagina"]; ok {
		if f, ok := val.(float64); ok {
			doc.Pagina = int(f)
		}
	}
	if val, ok := hit.Fields["ordem"]; ok {
		if f, ok := val.(float64); ok {
			doc.Ordem = int(f)
		}
	}

	if tags, ok := hit.Fields["tags"].([]interface{}); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok {
				doc.Tags = append(doc.Tags, s)
			}
		}
	} else if tag, ok := hit.Fields["tags"].(string); ok {
		doc.Tags = append(doc.Tags, tag)
	}

	searchHit.Source = *doc
	searchHit.Highlight = make(map[string][]string)
	for field, frags := range hit.Fragments {
		searchHit.Highlight[field] = frags
	}
}

func ClearCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cache = make(map[string]cacheEntry)
	keys = make([]string, 0)
	slog.Debug("Cache de busca limpo (Global)")
}

func InvalidateFile(filename string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	removed := 0
	newKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		entry, exists := cache[key]
		if !exists {
			continue
		}
		shouldRemove := false
		for _, fid := range entry.FileIDs {
			if fid == filename {
				shouldRemove = true
				break
			}
		}
		if shouldRemove {
			delete(cache, key)
			removed++
		} else {
			newKeys = append(newKeys, key)
		}
	}
	keys = newKeys

	if removed > 0 {
		slog.Debug("Cache invalidated by file change", "file", filename, "removed", removed)
	}
}
