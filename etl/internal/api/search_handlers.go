package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"

	"etl/internal/ingest"
	"etl/internal/models"
	"etl/internal/search"
	"etl/internal/utils"
)

// HandleSearch recebe a query do Frontend, autentica e manda pro backend motor de busca local.
func (ctx *HandlerContext) HandleSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	if r.Method != http.MethodPost {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Erro ao ler corpo", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload struct {
		Query struct {
			Term string `json:"term"`
		} `json:"query"`
		Compact  bool `json:"compact"`
		Semantic bool `json:"semantic"`
		From     int  `json:"from"`
		Size     int  `json:"size"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	// 1. Tentar recuperar do CacheInterno (body contém from/size, então o cache é seguro)
	if cachedResult, found := search.GetCachedResult(body); found {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		json.NewEncoder(w).Encode(cachedResult)
		return
	}

	// 2. Executar Busca no Motor Interno (Bleve)
	searchSize := payload.Size
	if searchSize <= 0 {
		searchSize = 50
	}

	searchResult, err := search.ExecuteSearch(r.Context(), payload.Query.Term, payload.Compact, payload.From, searchSize)
	if err != nil {
		log.Printf("[Search] Erro ao buscar no Bleve: %v\n", err)
		http.Error(w, "Erro no motor de busca interno", http.StatusInternalServerError)
		return
	}

	// 3. Pós-processamento (Popularidade e Refinamento)
	queryTerms := search.GetHeuristicTerms(payload.Query.Term)
	searchResult.Hits.Hits = PostProcessSearchHits(searchResult.Hits.Hits, queryTerms, payload.Query.Term, payload.Compact, ctx.State)

	// 4. Ordenação Final
	if payload.Compact {
		// No modo compacto, priorizamos recência pura
		sort.Slice(searchResult.Hits.Hits, func(i, j int) bool {
			return searchResult.Hits.Hits[i].Source.Timestamp > searchResult.Hits.Hits[j].Source.Timestamp
		})
	} else {
		// No modo padrão (Cards), sempre respeitamos o Rank recalculado
		search.SortHitsByScore(searchResult.Hits.Hits)
	}

	if len(searchResult.Hits.Hits) > searchSize {
		searchResult.Hits.Hits = searchResult.Hits.Hits[:searchSize]
	}

	// 5. Salvar no Cache e Responder
	search.SetCachedResult(body, searchResult)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(searchResult)
}

func (ctx *HandlerContext) HandleTrack(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("name")
	if filename == "" {
		http.Error(w, "Nome do arquivo é obrigatório", http.StatusBadRequest)
		return
	}

	utils.SafeGo(func() {
		ctx.State.IncrementPopularity(filename)
		if ctx.State.GetPopularity(filename)%10 == 1 {
			ctx.State.Save(ctx.Cfg)
		}
	})

	w.WriteHeader(http.StatusNoContent)
}

func PostProcessSearchHits(hits []models.SearchHit, queryTerms []string, rawQuery string, isCompact bool, state *ingest.AppState) []models.SearchHit {
	filteredHits := []models.SearchHit{}
	cleanedQuery := search.CleanQueryForMatch(rawQuery)

	for i := range hits {
		pop := state.GetPopularity(hits[i].Source.Arquivo)
		hits[i].Source.IsIndexing = ingest.IsFileIndexing(hits[i].Source.Arquivo)
		hits[i].Source.IsNoEmbed = ingest.HasNoEmbedTag(hits[i].Source.Tags)

		if isCompact {
			hits[i].FinalScore = 1.0
		} else {
			// Buscar autoridade de links (Backlinks)
			linkCount := state.GetLinkCount(strings.ToLower(hits[i].Source.Arquivo))

			// Passamos a query limpa para evitar re-cálculo interno no ScoreFragment
			finalScore, details := search.ScoreFragment(&hits[i], queryTerms, rawQuery, cleanedQuery, pop, linkCount)
			hits[i].FinalScore = finalScore
			hits[i].ScoreDetails = details
		}

		filteredHits = append(filteredHits, hits[i])
	}
	return filteredHits
}
