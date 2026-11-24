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
	"ton618/core/internal/core/domain"
)

type SearchHit struct {
	ID           string
	Score        float64
	Doc          db.Document
	FinalScore   float64
	ScoreDetails map[string]float64
	Highlight    map[string][]string
	// HasOriginalTerm indica se o documento contém um termo ORIGINAL da query.
	// Governa a ordenação: docs com o termo exato sempre vêm antes dos que
	// casaram apenas pelo fallback fuzzy (LIKE).
	HasOriginalTerm bool
}

type SearchResults struct {
	Hits  []SearchHit
	Total int
}

var (
	quoteRegex       = regexp.MustCompile(`"([^"]+)"`)
	singleQuoteRegex = regexp.MustCompile(`'([^']+)'`)
	tagFilterRegex   = regexp.MustCompile(`\+?tags:("[^"]+"|[^\s]+)`)
	// tipoFilterRegex casa os filtros de tipo de nota (`tipo:pdf`,
	// `type:"mapa mental"`, case-insensitive) — alias `type:` aceito.
	// Vocabulário canônico PT snake_case: domain.NormalizeNoteTipo
	// (ver note_tipo.go).
	tipoFilterRegex = regexp.MustCompile(`(?i)\+?(?:tipo|type):("[^"]+"|[^\s]+)`)
	cleanQueryRe    = regexp.MustCompile(`[\+\*"]`)
	spacesRe        = regexp.MustCompile(`\s+`)
	nativeHashtag   = regexp.MustCompile(`(?:\s|^)#([a-zA-Z0-9_À-ÿ\-]+)([?!]*)`)
	ftsUnsafeRe     = regexp.MustCompile(`[\^~()]`) // chars que interferem com sintaxe FTS5
	// quotedTermsRegex é compilado uma vez (antes era recompilado a cada
	// chamada de extractTerms, que roda em loop no gate da busca híbrida).
	quotedTermsRegex = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)
)

// maxTagOnlyCandidates limita o pool de candidatos de uma busca "só por tag".
// O FTS5 já restringe o match pela tag, então não é preciso materializar o
// corpus inteiro: antes o LIMIT era 99999 e chegava a alocar ~180MB por busca
// em vaults grandes. O `total` continua exato (vem do COUNT), só reduzimos
// quantos documentos são carregados para re-ranking.
const maxTagOnlyCandidates = 500

// searchFragmentsPerNote é quantos FRAGMENTOS (docs) da mesma nota a busca
// carrega por página de RESULTADOS. A página é de notas (cards): o handler
// agrupa os hits por arquivo e mostra até maxSnippetsPerNote trechos de cada
// nota (ex.: várias linhas da planilha Tabulator com o termo) — o pool de
// candidatos e o trim final seguem essa proporção pra não matar os fragmentos
// extras antes do agrupamento.
const searchFragmentsPerNote = 5

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

	// Decompõe a query UMA vez: filtro de tipo, tags e texto restante.
	pq := parseQuery(rawQuery)
	noteTipos, queryTags, remainingQuery := pq.noteTipos, pq.tags, pq.remaining

	// 1. Try FTS5 first
	ftsQuery := buildFTSQueryWithParsed(pq)
	// Se a busca textual é vazia mas temos tags ou filtro de tipo, o FTS já
	// filtra por eles; ampliamos o pool de candidatos para re-ranking, mas de
	// forma limitada (evita materializar o corpus inteiro — ver
	// maxTagOnlyCandidates).
	limitSize := size * searchFragmentsPerNote
	if remainingQuery == "" && (len(queryTags) > 0 || len(noteTipos) > 0) {
		limitSize = size * 10
		if limitSize > maxTagOnlyCandidates {
			limitSize = maxTagOnlyCandidates
		}
	}
	results, total, err := store.SearchFTSWithContext(ctx, ftsQuery, from, limitSize, noteTipos)
	ftsFailed := false
	if err != nil {
		log.Printf("[Search] FTS5 error: %v, falling back to LIKE\n", err)
		ftsFailed = true
		results, total, _ = store.SearchFTSLikeWithContext(ctx, remainingQuery, 0, limitSize, noteTipos)
	}

	// 2. Fallback fuzzy (LIKE) SOMENTE quando o FTS não achou nada. Antes
	// disparava com `total < 3`, forçando um full scan de `documents` (~80ms)
	// mesmo quando o FTS já tinha 1-2 resultados legítimos.
	if total == 0 && remainingQuery != "" && !ftsFailed {
		likeResults, likeTotal, _ := store.SearchFTSLikeWithContext(ctx, remainingQuery, 0, size*3, noteTipos)
		// total==0 ⇒ a página do FTS está vazia; o conjunto é todo do LIKE.
		results = likeResults
		total = likeTotal
	}

	// 3. Convert to SearchHits and re-rank
	heuristicTerms := extractTerms(rawQuery)
	cleanedQuery := cleanQuery(rawQuery)
	// Os termos de evidência são normalizados UMA vez; HasContentEvidence
	// reparseava a query e renormalizava cada termo a cada hit.
	evidence := newEvidenceMatcherFromTerms(heuristicTerms)

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
				docTagMap[normalizeTagKey(dt)] = true
			}
			hasAllTags := true
			for _, qt := range queryTags {
				if !docTagMap[normalizeTagKey(qt)] {
					hasAllTags = false
					break
				}
			}
			if !hasAllTags {
				continue
			}
		}

		hit := SearchHit{
			ID:              r.DocID,
			Score:           r.Rank,
			Doc:             doc,
			HasOriginalTerm: evidence.matches(doc),
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

	// 4. Sort by original-term presence first, then by final score.
	// Docs que contêm o termo ORIGINAL da busca vêm sempre antes dos que casam
	// apenas pelo fallback fuzzy, independentemente do BM25. Dentro de cada
	// grupo, a ordem segue o FinalScore e, no empate, a recência.
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].HasOriginalTerm != hits[j].HasOriginalTerm {
			return hits[i].HasOriginalTerm
		}
		if math.Abs(hits[i].FinalScore-hits[j].FinalScore) < 0.0001 {
			return hits[i].Doc.Timestamp > hits[j].Doc.Timestamp
		}
		return hits[i].FinalScore > hits[j].FinalScore
	})

	// 5. Trim em FRAGMENTOS (não notas): o agrupamento por nota acontece no
	// handler, que precisa ver todos os fragmentos candidatos da página.
	if len(hits) > size*searchFragmentsPerNote {
		hits = hits[:size*searchFragmentsPerNote]
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
// Pesos: tags 50x, arquivo 20x, secao 10x, texto 1x.
func buildFTSQuery(raw string) string {
	return buildFTSQueryWithParsed(parseQuery(raw))
}

// columnClause monta o OR entre as colunas indexadas para um termo FTS5 já
// sanitizado (ex.: "foo*" ou "\"frase exata\""). O conjunto de colunas é o
// mesmo em toda a construção da query; centralizar evita cópias divergentes.
func columnClause(term string) string {
	return `(tags:` + term + ` OR arquivo:` + term + ` OR secao:` + term + ` OR texto:` + term + `)`
}

// parsedQuery é a query decomposta uma única vez por busca: filtros de tipo,
// tags e o texto restante. Antes cada etapa (FTS, fallback LIKE e evidência)
// repetia o mesmo parsing sobre a query crua.
type parsedQuery struct {
	raw       string
	noteTipos []string
	tags      []string
	remaining string
}

func parseQuery(raw string) parsedQuery {
	// Nessa ordem: o `tipo:` sai primeiro para não virar termo de conteúdo.
	tipos, withoutTipos := extractNoteTipo(raw)
	tags, remaining := extractTags(withoutTipos)
	return parsedQuery{
		raw:       raw,
		noteTipos: tipos,
		tags:      tags,
		remaining: strings.TrimSpace(remaining),
	}
}

func buildFTSQueryWithParsed(p parsedQuery) string {
	if strings.TrimSpace(p.raw) == "" {
		return ""
	}

	tipos, tags, remaining := p.noteTipos, p.tags, p.remaining

	var parts []string

	// Filtro por tipo de nota (`tipo:mapa_mental`): predicado na coluna
	// note_type (migração v24). Múltiplos filtros viram OR.
	if pred := noteTypeFTSPredicate(tipos); pred != "" {
		parts = append(parts, pred)
	}

	// Adiciona restrições de tags no formato FTS5 tags:"nome_da_tag"
	for _, tag := range tags {
		tag = sanitizeFTS5Term(normalizeTagKey(tag))
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
					parts = append(parts, columnClause(ph))
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

		// Process remaining words. Cada palavra vira um prefixo de busca unido
		// por AND entre as colunas indexadas.
		for _, w := range words {
			w = sanitizeFTS5Term(w)
			if w == "" || stopwords[strings.ToLower(w)] {
				continue
			}
			wLower := strings.ToLower(w)
			wStar := wLower
			if len(wStar) > 2 {
				wStar += "*"
			}
			parts = append(parts, columnClause(wStar))
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
		return columnClause(w)
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
			return columnClause(single)
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

	for {
		m := quotedTermsRegex.FindStringSubmatch(remaining)
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
	clean = tipoFilterRegex.ReplaceAllString(clean, " ")
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
	raw = tipoFilterRegex.ReplaceAllString(raw, " ")
	clean := strings.ToLower(cleanQueryRe.ReplaceAllString(raw, " "))
	return strings.TrimSpace(spacesRe.ReplaceAllString(clean, " "))
}

// normalizeTagKey é a chave de comparação de tags: minúscula e sem acentos.
// O FTS5 unicode61 já remove diacríticos; sem isso, a segunda linha de defesa
// em Go descartava hits que o FTS havia casado (ex.: tags:saude × tag "saúde").
func normalizeTagKey(tag string) string {
	return RemoveAccents(strings.ToLower(strings.TrimSpace(tag)))
}

// extractTags extrai tags formatadas (tags:xxx ou #hashtag) da query bruta.
// As tags saem na forma canônica de comparação (minúsculas, sem acentos).
func extractTags(raw string) (tags []string, remaining string) {
	remaining = raw
	matches := tagFilterRegex.FindAllStringSubmatch(remaining, -1)
	for _, m := range matches {
		if len(m) > 1 {
			tags = append(tags, normalizeTagKey(strings.Trim(m[1], `"`)))
			remaining = strings.Replace(remaining, m[0], " ", 1)
		}
	}
	hashtags := nativeHashtag.FindAllStringSubmatch(remaining, -1)
	for _, m := range hashtags {
		if len(m) > 1 {
			tags = append(tags, normalizeTagKey(m[1]))
			remaining = strings.Replace(remaining, m[0], " ", 1)
		}
	}
	return tags, remaining
}

// extractNoteTipo extrai os filtros de tipo de nota da query (`tipo:pdf`,
// `type:"mapa mental"`) e devolve os valores normalizados para o vocabulário
// PT snake_case (domain.NormalizeNoteTipo) junto da query sem eles. Múltiplos
// filtros são unidos por OR no predicado.
func extractNoteTipo(raw string) (tipos []string, remaining string) {
	remaining = raw
	for _, m := range tipoFilterRegex.FindAllStringSubmatch(remaining, -1) {
		if len(m) < 2 {
			continue
		}
		if v := domain.NormalizeNoteTipo(strings.Trim(m[1], `"`)); v != "" {
			tipos = append(tipos, v)
		}
		remaining = strings.Replace(remaining, m[0], " ", 1)
	}
	return tipos, remaining
}

// noteTypeFTSPredicate monta o predicado FTS5 da coluna note_type. O valor vai
// entre aspas (frase): "mapa_mental" tokeniza como mapa+mental adjacentes e casa
// com o texto indexado "mapa_mental". Caracteres fora de letras/dígitos/_ são
// removidos antes (mesma limpeza das tags).
func noteTypeFTSPredicate(tipos []string) string {
	var or []string
	for _, t := range tipos {
		t = strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				return r
			}
			return -1
		}, t)
		if t != "" {
			or = append(or, `note_type:"`+t+`"`)
		}
	}
	switch len(or) {
	case 0:
		return ""
	case 1:
		return or[0]
	default:
		return "(" + strings.Join(or, " OR ") + ")"
	}
}

// HasExplicitTagFilter indica se a query contém um filtro explícito de tag
// (`tags:xxx` ou `#hashtag`). Nesses casos o match por tag é intencional e não
// deve passar pelo gate de evidência da busca híbrida.
func HasExplicitTagFilter(rawQuery string) bool {
	tags, _ := extractTags(rawQuery)
	return len(tags) > 0
}

// HasContentEvidence indica se algum termo da query aparece no conteúdo da nota
// (texto/secao/arquivo), desconsiderando ocorrências apenas como hashtag
// (#termo é rótulo, não menção textual). Usado pela busca híbrida como gate de
// evidência: docs que casam no FTS só por tag/hashtag precisam de aval semântico.
// Retorna true quando a query não tem termos verificáveis (ex: só stopwords),
// para não filtrar indevidamente a busca vazia/lista completa.
// evidenceMatcher pré-computa os termos da query (normalizados) uma única vez.
// `hasTerms` preserva a semântica original: query sem termos ⇒ tudo é evidência.
type evidenceMatcher struct {
	terms    []string
	hasTerms bool
}

func newEvidenceMatcherFromTerms(terms []string) evidenceMatcher {
	normalized := make([]string, 0, len(terms))
	for _, term := range terms {
		t := removeAccents(strings.ToLower(term))
		if stopwords[t] || len(t) < 2 {
			continue
		}
		normalized = append(normalized, t)
	}
	return evidenceMatcher{terms: normalized, hasTerms: len(terms) > 0}
}

func (m evidenceMatcher) matches(doc db.Document) bool {
	if !m.hasTerms {
		return true
	}
	// Remove hashtags do texto antes da verificação (rótulo ≠ menção real).
	texto := nativeHashtag.ReplaceAllString(removeAccents(strings.ToLower(doc.Texto)), " ")
	secao := removeAccents(strings.ToLower(doc.Secao))
	arquivo := removeAccents(strings.ToLower(doc.Arquivo))
	for _, t := range m.terms {
		if strings.Contains(texto, t) || strings.Contains(secao, t) || strings.Contains(arquivo, t) {
			return true
		}
	}
	return false
}

func HasContentEvidence(doc db.Document, rawQuery string) bool {
	return newEvidenceMatcherFromTerms(extractTerms(rawQuery)).matches(doc)
}

var accentReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c",
	"Á", "a", "À", "a", "Â", "a", "Ã", "a", "Ä", "a",
	"É", "e", "È", "e", "Ê", "e", "Ë", "e",
	"Í", "i", "Ì", "i", "Î", "i", "Ï", "i",
	"Ó", "o", "Ò", "o", "Ô", "o", "Õ", "o", "Ö", "o",
	"Ú", "u", "Ù", "u", "Û", "u", "Ü", "u",
	"Ç", "c",
)

// RemoveAccents remove acentos e diacríticos de uma string. É o SSOT dessa
// normalização — o pacote features/search reutiliza em vez de manter a tabela
// duplicada (que já divergia: faltava mapear "Õ").
func RemoveAccents(s string) string {
	return accentReplacer.Replace(s)
}

func removeAccents(s string) string {
	return RemoveAccents(s)
}
