package search

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
	"ton618/core/internal/search"
)

// ── Search (HTMX partial) ──

func (ctx *HandlerContext) HandleSearch(w http.ResponseWriter, r *http.Request) {
	rCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	query := r.FormValue("q")
	if query == "" && r.Method == "POST" {
		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			query = string(body)
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
	if from < 0 {
		from = 0
	}
	if size > 100 {
		size = 100
	}

	results, err := search.Search(rCtx, ctx.Store, query, from, size,
		ctx.Store.GetBacklinkCount, ctx.Store.GetSynapticWeight)
	if err != nil {
		slog.Error("search error", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	seenFiles := make(map[string]bool)
	weightCache := make(map[string]float64)
	var items []domain.SearchResultItem
	for _, hit := range results.Hits {
		if seenFiles[hit.Doc.Arquivo] {
			continue
		}
		seenFiles[hit.Doc.Arquivo] = true

		snippet := buildSnippet(hit, query)

		tags := db.TagsToSlice(hit.Doc.Tags)
		var userTags []string
		for _, t := range tags {
			lowerT := strings.ToLower(t)
			if lowerT != "drawing" {
				userTags = append(userTags, t)
			}
		}

		weight, ok := weightCache[hit.Doc.Arquivo]
		if !ok {
			weight = ctx.Store.GetSynapticWeight(hit.Doc.Arquivo)
			weightCache[hit.Doc.Arquivo] = weight
		}
		if weight <= 0.105 {
			userTags = append(userTags, "esquecida")
		} else if weight <= 0.25 {
			userTags = append(userTags, "fria")
		}

		noteType := string(domain.DetectNoteType(tags, hit.Doc.Arquivo))

		line := findQueryLineInText(hit.Doc.Texto, query)

		displayTime := hit.Doc.Timestamp
		if t, err := time.Parse(time.RFC3339, hit.Doc.Timestamp); err == nil {
			displayTime = t.Local().Format("2006-01-02 15:04:05")
		}

		items = append(items, domain.SearchResultItem{
			Arquivo:   hit.Doc.Arquivo,
			Secao:     hit.Doc.Secao,
			Tags:      userTags,
			RawTags:   tags,
			Snippet:   snippet,
			Tipo:      noteType,
			Timestamp: displayTime,
			Line:      line,
		})
	}

	data := domain.SearchResultsData{
		Query:   query,
		Results: items,
		Total:   results.Total,
	}

	w.Header().Set("Content-Type", "text/html")
	if from > 0 {
		SearchResultsMore(data, from, size).Render(r.Context(), w)
		return
	}
	SearchResults(data, size).Render(r.Context(), w)
}
