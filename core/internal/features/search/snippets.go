package search

import (
	"html"
	"regexp"
	"sort"
	"strings"
	"sync"

	"ton618/core/internal/search"
)

var searchQuotedRe = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)

func cleanTermForMatching(t string) string {
	t = strings.TrimSpace(t)
	t = strings.TrimLeft(t, "+-~#")
	t = strings.TrimRight(t, "*~")
	t = strings.Trim(t, `"'`)
	return strings.TrimSpace(t)
}

func extractSearchTerms(query string) []string {
	var terms []string
	remaining := query

	quotedRe := searchQuotedRe
	for {
		m := quotedRe.FindStringSubmatch(remaining)
		if m == nil {
			break
		}
		phrase := m[1]
		if phrase == "" {
			phrase = m[2]
		}
		phrase = strings.TrimSpace(phrase)
		if len(phrase) > 1 {
			terms = append(terms, phrase)
		}
		remaining = strings.Replace(remaining, m[0], " ", 1)
	}

	rawTerms := strings.Fields(remaining)
	for _, t := range rawTerms {
		t = strings.TrimSpace(t)
		if len(t) <= 1 {
			continue
		}
		if strings.HasPrefix(t, "-") || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "+tags:") || strings.HasPrefix(t, "tags:") {
			continue
		}

		cleaned := cleanTermForMatching(t)
		if len(cleaned) <= 1 {
			continue
		}

		terms = append(terms, cleaned)
	}
	return terms
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

func removeAccents(s string) string {
	return accentReplacer.Replace(s)
}

func buildSnippet(hit search.SearchHit, query string) string {
	snippet := ""
	hasFTSMatchInText := false
	if len(hit.Highlight) > 0 {
		if ftsSnippets, ok := hit.Highlight["texto"]; ok && len(ftsSnippets) > 0 {
			rawFts := ftsSnippets[0]
			if strings.Contains(rawFts, "__HL_START__") {
				hasFTSMatchInText = true
				snippet = rawFts
			}
		}
	}

	if !hasFTSMatchInText || snippet == "" {
		snippet = extractSnippetAroundMatch(hit.Doc.Texto, query, 150)
		snippet = highlightSnippetManual(snippet, query)
	}

	snippet = strings.Join(strings.Fields(snippet), " ")

	safeSnippet := html.EscapeString(snippet)
	safeSnippet = strings.ReplaceAll(safeSnippet, "__HL_START__", `<span class="search-highlight text-sky-400 font-bold bg-sky-500/10 rounded-sm px-0.5">`)
	safeSnippet = strings.ReplaceAll(safeSnippet, "__HL_END__", "</span>")
	return safeSnippet
}

func buildPlainSnippet(content string, limit int) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "---") {
		if idx := strings.Index(text[4:], "\n---"); idx != -1 {
			text = strings.TrimSpace(text[idx+7:])
		}
	}
	text = strings.Join(strings.Fields(text), " ")
	if limit > 0 && len(text) > limit {
		text = text[:limit] + "…"
	}
	return html.EscapeString(text)
}

func findQueryLineInText(text, query string) int {
	if query == "" || text == "" {
		return 0
	}

	terms := extractSearchTerms(query)
	if len(terms) == 0 {
		return 0
	}

	var normalizedTerms []string
	for _, term := range terms {
		normalizedTerms = append(normalizedTerms, removeAccents(strings.ToLower(term)))
	}

	lines := strings.Split(text, "\n")
	startIdx := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				startIdx = i + 1
				break
			}
		}
	}

	for i := startIdx; i < len(lines); i++ {
		lineNormalized := removeAccents(strings.ToLower(lines[i]))
		allMatch := true
		for _, term := range normalizedTerms {
			if !strings.Contains(lineNormalized, term) {
				allMatch = false
				break
			}
		}
		if allMatch {
			return i + 1
		}
	}

	for i := startIdx; i < len(lines); i++ {
		lineNormalized := removeAccents(strings.ToLower(lines[i]))
		if strings.Contains(lineNormalized, normalizedTerms[0]) {
			return i + 1
		}
	}

	return startIdx + 1
}

func extractSnippetAroundMatch(text, query string, windowSize int) string {
	if text == "" {
		return ""
	}

	terms := extractSearchTerms(query)

	matchPos := -1
	textLower := removeAccents(strings.ToLower(text))
	for _, term := range terms {
		normalizedTerm := removeAccents(strings.ToLower(term))
		if idx := strings.Index(textLower, normalizedTerm); idx != -1 {
			if matchPos == -1 || idx < matchPos {
				matchPos = idx
			}
		}
	}

	if matchPos == -1 {
		if len(text) > windowSize*2 {
			return text[:windowSize*2] + "..."
		}
		return text
	}

	start := matchPos - windowSize
	if start < 0 {
		start = 0
	}
	end := matchPos + windowSize
	if end > len(text) {
		end = len(text)
	}

	for start > 0 && text[start]&0xC0 == 0x80 {
		start--
	}
	for end < len(text) && text[end]&0xC0 == 0x80 {
		end++
	}

	snippet := text[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(text) {
		snippet = snippet + "..."
	}
	return snippet
}

var accentCharMap = map[rune]string{
	'a': "[aáàâãäAÁÀÂÃÄ]", 'á': "[aáàâãäAÁÀÂÃÄ]", 'à': "[aáàâãäAÁÀÂÃÄ]", 'â': "[aáàâãäAÁÀÂÃÄ]", 'ã': "[aáàâãäAÁÀÂÃÄ]", 'ä': "[aáàâãäAÁÀÂÃÄ]",
	'e': "[eéèêëEÉÈÊË]", 'é': "[eéèêëEÉÈÊË]", 'è': "[eéèêëEÉÈÊË]", 'ê': "[eéèêëEÉÈÊË]", 'ë': "[eéèêëEÉÈÊË]",
	'i': "[iíìîïIÍÌÎÏ]", 'í': "[iíìîïIÍÌÎÏ]", 'ì': "[iíìîïIÍÌÎÏ]", 'î': "[iíìîïIÍÌÎÏ]", 'ï': "[iíìîïIÍÌÎÏ]",
	'o': "[oóòôõöOÓÒÔÕÖ]", 'ó': "[oóòôõöOÓÒÔÕÖ]", 'ò': "[oóòôõöOÓÒÔÕÖ]", 'ô': "[oóòôõöOÓÒÔÕÖ]", 'õ': "[oóòôõöOÓÒÔÕÖ]", 'ö': "[oóòôõöOÓÒÔÕÖ]",
	'u': "[uúùûüUÚÙÛÜ]", 'ú': "[uúùûüUÚÙÛÜ]", 'ù': "[uúùûüUÚÙÛÜ]", 'û': "[uúùûüUÚÙÛÜ]", 'ü': "[uúùûüUÚÙÛÜ]",
	'c': "[cçCÇ]", 'ç': "[cçCÇ]",
}

func makeAccentInsensitivePatternGo(str string) string {
	var pattern strings.Builder
	for _, ch := range strings.ToLower(str) {
		if val, ok := accentCharMap[ch]; ok {
			pattern.WriteString(val)
		} else {
			pattern.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	return pattern.String()
}

var (
	accentHighlightMu    sync.Mutex
	accentHighlightCache = map[string]*regexp.Regexp{}
)

func accentHighlightRegex(query string) *regexp.Regexp {
	accentHighlightMu.Lock()
	if re, ok := accentHighlightCache[query]; ok {
		accentHighlightMu.Unlock()
		return re
	}
	accentHighlightMu.Unlock()

	terms := extractSearchTerms(query)
	var pats []string
	for _, term := range terms {
		if len(term) > 1 {
			pats = append(pats, makeAccentInsensitivePatternGo(term))
		}
	}
	var compiled *regexp.Regexp
	if len(pats) > 0 {
		sort.Slice(pats, func(i, j int) bool { return len(pats[i]) > len(pats[j]) })
		if re, err := regexp.Compile("(?i)(" + strings.Join(pats, "|") + ")"); err == nil {
			compiled = re
		}
	}

	accentHighlightMu.Lock()
	if len(accentHighlightCache) > 256 {
		accentHighlightCache = map[string]*regexp.Regexp{}
	}
	accentHighlightCache[query] = compiled
	accentHighlightMu.Unlock()
	return compiled
}

func highlightSnippetManual(snippet, query string) string {
	re := accentHighlightRegex(query)
	if re == nil {
		return snippet
	}
	return re.ReplaceAllString(snippet, "__HL_START__${1}__HL_END__")
}
