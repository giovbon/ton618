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

// maxSnippetsPerNote é quantos trechos de UMA nota o card de resultado lista
// (o principal + extras). O que exceder vira "+N ocorrências" no rodapé do
// card — típico de notas Tabulator, onde cada linha da planilha é um fragmento
// indexado e o termo pode casar em dezenas de linhas.
const maxSnippetsPerNote = 5

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

	// A página é de NOTAS (cards), não de fragmentos: agrupa os hits por
	// arquivo e o card passa a listar TODOS os trechos que casaram (até
	// maxSnippetsPerNote; o resto vira "+N ocorrências" no rodapé do card).
	// Antes cada nota virava um card com UM fragmento e as outras linhas da
	// planilha/seções que casavam eram descartadas em silêncio.
	// Os hits chegam ordenados (HasOriginalTerm, depois FinalScore); dentro
	// de cada nota os trechos que contêm o termo ficam na frente preservando a
	// ordem relativa — o primeiro vira o trecho principal do card.
	type bucket struct{ with, without []*search.SearchHit }
	buckets := make(map[string]*bucket)
	var order []string
	for i := range results.Hits {
		hit := &results.Hits[i]
		b, ok := buckets[hit.Doc.Arquivo]
		if !ok {
			b = &bucket{}
			buckets[hit.Doc.Arquivo] = b
			order = append(order, hit.Doc.Arquivo)
		}
		if containsSearchTerm(hit.Doc.Texto, query) {
			b.with = append(b.with, hit)
		} else {
			b.without = append(b.without, hit)
		}
	}

	weightCache := make(map[string]float64)
	var items []domain.SearchResultItem
	for _, arq := range order {
		frags := append(buckets[arq].with, buckets[arq].without...)
		primary := *frags[0]

		snippet := buildSnippet(primary, query)
		var extras []domain.SearchSnippet
		for _, h := range frags[1:] {
			if len(extras) >= maxSnippetsPerNote-1 {
				break
			}
			extras = append(extras, domain.SearchSnippet{
				Secao: h.Doc.Secao,
				HTML:  buildSnippet(*h, query),
			})
		}
		extraCount := len(frags) - 1 - len(extras)

		tags := db.TagsToSlice(primary.Doc.Tags)
		// FilterUserTags remove as tags internas de tipo de editor (drawing,
		// markmap, tabulator, ...) — o usuário não deve vê-las como tags.
		userTags := domain.FilterUserTags(tags)

		weight, ok := weightCache[primary.Doc.Arquivo]
		if !ok {
			weight = ctx.Store.GetSynapticWeight(primary.Doc.Arquivo)
			weightCache[primary.Doc.Arquivo] = weight
		}
		if weight <= 0.105 {
			userTags = append(userTags, "esquecida")
		} else if weight <= 0.25 {
			userTags = append(userTags, "fria")
		}

		noteType := string(domain.DetectNoteType(tags, primary.Doc.Arquivo))

		line := findQueryLineInText(primary.Doc.Texto, query)

		displayTime := primary.Doc.Timestamp
		if t, err := time.Parse(time.RFC3339, primary.Doc.Timestamp); err == nil {
			displayTime = t.Local().Format("2006-01-02 15:04:05")
		}

		items = append(items, domain.SearchResultItem{
			Arquivo:    primary.Doc.Arquivo,
			Secao:      primary.Doc.Secao,
			Tags:       userTags,
			RawTags:    tags,
			Snippet:    snippet,
			Snippets:   extras,
			ExtraCount: extraCount,
			Tipo:       noteType,
			Timestamp:  displayTime,
			Line:       line,
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
