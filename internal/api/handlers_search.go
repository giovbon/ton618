package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ton618/internal/db"
	"ton618/internal/index"
)

// buildContextSnippet gera um trecho do texto com contexto ao redor de termos encontrados.
// Suporta "frases exatas" entre aspas como termo único.
func buildContextSnippet(query, text string) string {
	const before = 80
	const after = 120

	if text == "" {
		return "..."
	}

	// Extrai frases exatas e termos individuais
	var terms []string
	remaining := query

	// Primeiro: extrai frases entre aspas
	quotedRe := regexp.MustCompile(`"([^"]+)"`)
	for {
		m := quotedRe.FindStringSubmatch(remaining)
		if m == nil {
			break
		}
		phrase := strings.TrimSpace(m[1])
		if phrase == "" {
			phrase = strings.TrimSpace(m[2])
		}
		if len(phrase) > 1 {
			terms = append(terms, phrase)
			// Adiciona palavras individuais como fallback
			for _, pw := range strings.Fields(phrase) {
				if len(pw) > 1 {
					terms = append(terms, pw)
				}
			}
		}
		remaining = strings.Replace(remaining, m[0], " ", 1)
	}

	// Depois: extrai termos individuais do restante
	rawTerms := strings.Fields(remaining)
	for _, t := range rawTerms {
		t = strings.TrimSpace(t)
		if len(t) <= 1 {
			continue
		}
		if t[0] == '-' || t[0] == '#' || strings.HasPrefix(t, "+tags:") {
			continue
		}
		t = strings.Trim(t, `"`)
		if len(t) <= 1 {
			continue
		}
		cleaned := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return r
			}
			return -1
		}, t)
		if len(cleaned) > 1 {
			terms = append(terms, cleaned)
		}
	}

	if len(terms) == 0 {
		if len(text) > 250 {
			return text[:250] + "..."
		}
		return text
	}

	textLower := strings.ToLower(text)

	// Find first occurrence of each term
	type match struct {
		pos  int
		term string
	}
	var matches []match
	seen := make(map[string]bool)
	for _, term := range terms {
		termLower := strings.ToLower(term)
		if seen[termLower] {
			continue
		}
		seen[termLower] = true
		if pos := strings.Index(textLower, termLower); pos >= 0 {
			matches = append(matches, match{pos: pos, term: termLower})
		}
	}

	if len(matches) == 0 {
		if len(text) > 250 {
			return text[:250] + "..."
		}
		return text
	}

	// Sort by position
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].pos < matches[j].pos
	})

	// Build context windows, merging close ones
	const gapThreshold = 150
	type window struct {
		start, end int
	}
	var windows []window

	for _, m := range matches {
		start := m.pos - before
		if start < 0 {
			start = 0
		}
		end := m.pos + len(m.term) + after
		if end > len(text) {
			end = len(text)
		}

		if len(windows) > 0 {
			last := &windows[len(windows)-1]
			// If this window overlaps or is close enough, merge
			if start <= last.end+gapThreshold {
				if end > last.end {
					last.end = end
				}
				continue
			}
		}
		windows = append(windows, window{start: start, end: end})
	}

	// Build final snippet with ellipsis
	var parts []string
	for i, w := range windows {
		part := text[w.start:w.end]
		// Trim to sentence boundaries at edges when possible
		if w.start > 0 {
			part = "... " + part
		}
		if w.end < len(text) {
			part = part + " ..."
		}

		// If this is not the first window and previous window is far, add separator
		if i > 0 {
			parts = append(parts, "...")
		}
		parts = append(parts, part)
	}

	return strings.Join(parts, " ")
}

// ── Search (HTMX partial) ──

func (ctx *HandlerContext) HandleSearch(w http.ResponseWriter, r *http.Request) {
	// Set request timeout
	rCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	query := r.FormValue("q")
	if query == "" && r.Method == "POST" {
		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			query = string(body)
			// parse form-encoded or simple string
			if strings.HasPrefix(query, "q=") {
				query = strings.TrimPrefix(query, "q=")
			}
		}
	}

	from, _ := strconv.Atoi(r.FormValue("from"))
	size, _ := strconv.Atoi(r.FormValue("size"))
	if size <= 0 {
		size = 20
	}

	results, err := index.Search(rCtx, ctx.Store, query, from, size,
		ctx.Store.GetLinkCount, ctx.Store.GetPopularity)
	if err != nil {
		slog.Error("search error", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build template data
	type resultItem struct {
		ID         string
		Arquivo    string
		Secao      string
		Tags       []string
		Snippet    string
		Score      float64
		Tipo       string
		Timestamp  string
		IsIndexing bool
	}

	var items []resultItem
	for _, hit := range results.Hits {
		// Clean snippet: strip HTML, show context around query
		snippet := hit.Doc.Texto
		// Strip basic HTML tags
		snippet = strings.ReplaceAll(snippet, "<p>", "")
		snippet = strings.ReplaceAll(snippet, "</p>", " ")
		snippet = strings.ReplaceAll(snippet, "<br>", " ")
		snippet = strings.ReplaceAll(snippet, "<br/>", " ")
		snippet = strings.ReplaceAll(snippet, "<strong>", "")
		snippet = strings.ReplaceAll(snippet, "</strong>", "")
		snippet = strings.ReplaceAll(snippet, "<em>", "")
		snippet = strings.ReplaceAll(snippet, "</em>", "")
		snippet = strings.ReplaceAll(snippet, "<code>", "")
		snippet = strings.ReplaceAll(snippet, "</code>", "")
		snippet = strings.ReplaceAll(snippet, "<pre>", "")
		snippet = strings.ReplaceAll(snippet, "</pre>", "")
		snippet = strings.ReplaceAll(snippet, "<h1>", "")
		snippet = strings.ReplaceAll(snippet, "</h1>", " - ")
		snippet = strings.ReplaceAll(snippet, "<h2>", "")
		snippet = strings.ReplaceAll(snippet, "</h2>", " - ")
		snippet = strings.ReplaceAll(snippet, "<h3>", "")
		snippet = strings.ReplaceAll(snippet, "</h3>", " - ")
		snippet = strings.ReplaceAll(snippet, "<ul>", "")
		snippet = strings.ReplaceAll(snippet, "</ul>", "")
		snippet = strings.ReplaceAll(snippet, "<li>", "  ")
		snippet = strings.ReplaceAll(snippet, "</li>", "")
		snippet = strings.ReplaceAll(snippet, "<a[^>]*>", "")
		snippet = strings.ReplaceAll(snippet, "</a>", "")
		// Normalize whitespace
		snippet = strings.Join(strings.Fields(snippet), " ")

		// Extract multi-term context windows with ellipsis between distant terms
		snippet = buildContextSnippet(query, snippet)
		tags := db.TagsToSlice(hit.Doc.Tags)
		items = append(items, resultItem{
			ID:         hit.Doc.ID,
			Arquivo:    hit.Doc.Arquivo,
			Secao:      hit.Doc.Secao,
			Tags:       tags,
			Snippet:    snippet,
			Score:      hit.FinalScore,
			Tipo:       hit.Doc.Tipo,
			Timestamp:  hit.Doc.Timestamp,
			IsIndexing: hit.Doc.IsIndexing,
		})
	}

	data := map[string]interface{}{
		"Query":   query,
		"Results": items,
		"Total":   results.Total,
	}

	// HTMX: return only the results partial
	w.Header().Set("Content-Type", "text/html")
	ctx.renderPartial(w, "search_results.html", data)
}

// ── Bulk Delete (Config → Exclusão) ──

func (ctx *HandlerContext) HandleBulkDelete(w http.ResponseWriter, r *http.Request) {
	byAge := r.FormValue("by_age") == "true"
	byTag := r.FormValue("by_tag") == "true"
	ageYears, _ := strconv.Atoi(r.FormValue("age_years"))
	tagNamesRaw := strings.TrimSpace(r.FormValue("tag_name"))
	isPreview := r.FormValue("preview") == "true"

	if !byAge && !byTag {
		http.Error(w, "pelo menos um filtro deve estar ativo", http.StatusBadRequest)
		return
	}

	filesToDelete := make(map[string]bool)
	firstFilter := true

	// Filter 1: by age
	if byAge {
		if ageYears < 1 || ageYears > 10 {
			http.Error(w, "age_years invalido (1-10)", http.StatusBadRequest)
			return
		}
		cutoff := time.Now().AddDate(-ageYears, 0, 0)
		allMods, err := ctx.Store.GetAllFileMods()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for arquivo, mtimeStr := range allMods {
			if !isNoteOrPdf(arquivo) {
				continue
			}
			mtime, err := time.Parse(time.RFC3339, mtimeStr)
			if err != nil {
				continue
			}
			if mtime.Before(cutoff) {
				filesToDelete[arquivo] = true
			}
		}
		firstFilter = false
	}

	// Filter 2: by tag(s) — múltiplas tags separadas por vírgula
	if byTag {
		if tagNamesRaw == "" {
			http.Error(w, "tag_name obrigatorio", http.StatusBadRequest)
			return
		}
		tagNames := strings.Split(tagNamesRaw, ",")
		tagSet := make(map[string]bool)
		for _, tn := range tagNames {
			tn = strings.TrimSpace(tn)
			if tn == "" {
				continue
			}
			tagFiles, err := ctx.Store.GetFilesByTag(tn)
			if err != nil {
				continue
			}
			for _, f := range tagFiles {
				if isNoteOrPdf(f) {
					tagSet[f] = true
				}
			}
		}

		if firstFilter {
			filesToDelete = tagSet
		} else {
			for f := range filesToDelete {
				if !tagSet[f] {
					delete(filesToDelete, f)
				}
			}
		}
		firstFilter = false
	}

	// Preview mode: return list of files without deleting
	if isPreview {
		fileList := make([]string, 0, len(filesToDelete))
		for f := range filesToDelete {
			fileList = append(fileList, f)
		}
		sort.Strings(fileList)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": fileList,
			"total": len(fileList),
		})
		return
	}

	if len(filesToDelete) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"deleted": 0,
			"message": "nenhuma nota encontrada com os filtros selecionados",
		})
		return
	}

	deleted := 0
	var errors []string
	for arquivo := range filesToDelete {
		fullPath := filepath.Join(ctx.Cfg.DocsDir, arquivo)

		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			errors = append(errors, arquivo+": "+err.Error())
			continue
		}

		ctx.Store.DeleteEmbeddingsByFile(arquivo)
		ctx.Store.DeleteDocumentsByFile(arquivo)
		ctx.Store.DeleteFTSByFile(arquivo)
		ctx.Store.DeleteFileMod(arquivo)
		ctx.Store.ResetPopularity(arquivo)
		ctx.Store.SetFileTags(arquivo, nil)

		deleted++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted": deleted,
		"errors":  errors,
	})
}
