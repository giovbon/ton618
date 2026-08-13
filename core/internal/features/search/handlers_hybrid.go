package search

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
	"ton618/core/internal/httputil"
	"ton618/core/internal/search"
	"ton618/core/internal/ui/icons"
)

type hybridSearchRequest struct {
	Query     string    `json:"query"`
	Embedding []float32 `json:"embedding"`
	Limit     int       `json:"limit"`
}

type hybridSearchResult struct {
	Filename      string   `json:"filename"`
	Tipo          string   `json:"type"`
	Icon          string   `json:"_icon"`
	Secao         string   `json:"section,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	RRFScore      float64  `json:"rrf_score"`
	RankFTS       *int     `json:"rank_fts,omitempty"`
	RankSem       *int     `json:"rank_sem,omitempty"`
	SemSimilarity *float64 `json:"sem_similarity,omitempty"`
	Snippet       string   `json:"snippet"`
	HasHighlight  bool     `json:"has_highlight"`
}

// isZeroEmbedding indica se o embedding veio zerado (inválido) — usado para
// degradar graciosamente para FTS5 puro quando a IA não está pronta.
func isZeroEmbedding(emb []float32) bool {
	for _, v := range emb {
		if v != 0 {
			return false
		}
	}
	return true
}

// hasDeletarTag indica se a nota está marcada para exclusão (tag "deletar") —
// mesma regra do lado semântico (isNoteEmbeddable), para os dois motores
// cobrirem o mesmo universo.
func hasDeletarTag(tags string) bool {
	for _, t := range db.TagsToSlice(tags) {
		if strings.ToLower(strings.TrimSpace(t)) == "deletar" {
			return true
		}
	}
	return false
}

// hybridThresholdPct lê o threshold da busca HÍBRIDA (default 55%). Se a setting
// dedicada não existir, usa a da busca semântica pura (sem migração).
func hybridThresholdPct(store *db.Store) int {
	pct := 55
	if val, err := store.GetSetting("hybrid_semantic_threshold"); err == nil && val != "" {
		if v, err := strconv.Atoi(val); err == nil && v >= 10 && v <= 100 {
			return v
		}
	}
	if val, err := store.GetSetting("semantic_search_threshold"); err == nil && val != "" {
		if v, err := strconv.Atoi(val); err == nil && v >= 10 && v <= 100 {
			pct = v
		}
	}
	return pct
}

// rrfK lê a constante k do RRF configurável (default 60, faixa 10-100).
// Menor k => o rank (posição) pesa mais na fusão; maior k => empata menos.
func rrfK(store *db.Store) int {
	k := search.DefaultRRFK
	if val, err := store.GetSetting("rrf_k"); err == nil && val != "" {
		if v, err := strconv.Atoi(val); err == nil && v >= 10 && v <= 100 {
			k = v
		}
	}
	return k
}

// HandleHybridSearch funde a busca textual (FTS5) com a busca semântica via
// Reciprocal Rank Fusion. Recebe o embedding da query (gerado no browser) e o
// texto bruto; roda os dois motores em paralelo no servidor e devolve JSON unificado.
// POST /api/search/hybrid
func (ctx *HandlerContext) HandleHybridSearch(w http.ResponseWriter, r *http.Request) {
	var req hybridSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invalido: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		http.Error(w, "query obrigatoria", http.StatusBadRequest)
		return
	}

	limit := req.Limit
	if limit <= 0 || limit > 50 {
		limit = 15
	}

	// Timeout no mesmo padrão da busca global (HandleSearch): evita request
	// pendurado em lock/DB lento. Vale para as duas goroutines abaixo (o FTS5 e
	// o KNN agora propagam o contexto para as queries).
	rCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// A parte semântica só participa se houver um embedding válido.
	hasSemantic := len(req.Embedding) == db.EmbeddingDim && !isZeroEmbedding(req.Embedding)
	maxDist := math.MaxFloat64
	if hasSemantic {
		pct := hybridThresholdPct(ctx.Store)
		maxDist = math.Sqrt(2.0 * (1.0 - float64(pct)/100.0))
	}

	ftsRanks := make(map[string]int)
	semRanks := make(map[string]int)
	ftsDocs := make(map[string]db.Document)
	ftsHits := make(map[string]search.SearchHit)
	semSim := make(map[string]float64)

	var wg sync.WaitGroup
	var ftsErr, semErr error
	var semCandidates []db.SimilarResult

	// 1. FTS5 (textual)
	wg.Add(1)
	go func() {
		defer wg.Done()
		results, err := search.Search(rCtx, ctx.Store, req.Query, 0, limit*2,
			ctx.Store.GetBacklinkCount, ctx.Store.GetSynapticWeight)
		if err != nil {
			ftsErr = err
			return
		}
		seen := make(map[string]bool)
		rank := 1
		for _, hit := range results.Hits {
			arquivo := hit.Doc.Arquivo
			if strings.HasPrefix(arquivo, "pdfs/") || strings.HasSuffix(strings.ToLower(arquivo), ".pdf") {
				continue
			}
			if strings.HasPrefix(arquivo, "attachments/") {
				continue
			}
			// Notas marcadas para exclusão não participam da fusão (paridade com a semântica).
			if hasDeletarTag(hit.Doc.Tags) {
				continue
			}
			if seen[arquivo] {
				continue
			}
			seen[arquivo] = true
			ftsRanks[arquivo] = rank
			ftsDocs[arquivo] = hit.Doc
			ftsHits[arquivo] = hit
			rank++
			if rank > limit*2 {
				break
			}
		}
	}()

	// 2. Semântica (KNN) — coleta os candidatos; threshold + consenso são
	// aplicados depois do wg.Wait, porque o consenso depende dos ranks do FTS
	// (que ainda estariam em escrita concorrente aqui).
	wg.Add(1)
	go func() {
		defer wg.Done()
		if !hasSemantic {
			return
		}
		similar, err := ctx.Store.SearchSimilarWithConsensus(rCtx, req.Embedding, limit*2, maxDist)
		if err != nil {
			semErr = err
			return
		}
		semCandidates = similar
	}()
	wg.Wait()

	if ftsErr != nil {
		slog.Error("hybrid search: fts", "error", ftsErr)
		http.Error(w, "erro na busca textual: "+ftsErr.Error(), http.StatusInternalServerError)
		return
	}
	if semErr != nil {
		// A semântica falhar não deve derrubar a busca inteira — segue só com FTS5.
		slog.Error("hybrid search: sem", "error", semErr)
	}

	// ── Semântica: threshold + consenso de chunks ──
	// O voto majoritário vale para candidatos SEM evidência de conteúdo no FTS
	// (só-semânticos OU com match só por tag/hashtag): nota longa (≥3 chunks)
	// precisa de match em ≥2 chunks, exceto similaridade excepcional (≥82%).
	// Notas curtas (1-2 chunks) passam com match único — o chunk é a nota inteira.
	// Docs com o termo no conteúdo já têm evidência e passam direto.
	rank := 1
	for _, h := range semCandidates {
		if h.Distance > maxDist {
			continue
		}
		if _, exists := semRanks[h.Filename]; exists {
			continue
		}
		sim := 1.0 - (h.Distance*h.Distance)/2.0 // cosseno
		anchored := false
		if doc, ok := ftsDocs[h.Filename]; ok && search.HasContentEvidence(doc, req.Query) {
			anchored = true
		}
		if !anchored && !semanticConsensusPass(h.TotalChunks, h.ChunkMatches, sim) {
			continue
		}
		semRanks[h.Filename] = rank
		semSim[h.Filename] = sim
		rank++
	}

	// ── Gate de evidência ──
	// Match no FTS só por tag/hashtag é evidência fraca: o doc só participa da
	// fusão se a semântica também o aceitar — OU se a query pediu tag
	// explicitamente (tags:/#). Não se aplica no modo degradado (sem IA), onde a
	// fusão é FTS puro e o comportamento atual é mantido.
	if hasSemantic && !search.HasExplicitTagFilter(req.Query) {
		for arquivo := range ftsRanks {
			if search.HasContentEvidence(ftsDocs[arquivo], req.Query) {
				continue
			}
			if _, ok := semRanks[arquivo]; ok {
				continue
			}
			delete(ftsRanks, arquivo)
			delete(ftsDocs, arquivo)
			delete(ftsHits, arquivo)
		}
	}

	// 3. Fusão RRF (k configurável via Configurações > Semântica)
	k := rrfK(ctx.Store)
	fused := search.ReciprocalRankFusion(ftsRanks, semRanks, k, limit)

	// Carrega em batch as tags (uma query) e o conteúdo das notas só-semânticas
	// (uma query) — antes eram N chamadas individuais (GetFileTags/GetNote).
	allTags, _ := ctx.Store.GetAllFileTags()
	var needContent []string
	for _, filename := range fused {
		if _, ok := ftsHits[filename]; !ok {
			needContent = append(needContent, filename)
		}
	}
	contents, _ := ctx.Store.BatchGetNotesContent(needContent)

	results := make([]hybridSearchResult, 0, len(fused))
	for _, filename := range fused {
		res := hybridSearchResult{Filename: filename}
		if rank, ok := ftsRanks[filename]; ok {
			rp := rank
			res.RankFTS = &rp
			res.RRFScore += search.FusionScore(rank, k)
		}
		if rank, ok := semRanks[filename]; ok {
			rp := rank
			res.RankSem = &rp
			res.RRFScore += search.FusionScore(rank, k)
			sim := semSim[filename] * 100
			res.SemSimilarity = &sim
		}

		// Snippet: do FTS (com highlight) quando o termo casou; senão prévia genérica.
		if hit, ok := ftsHits[filename]; ok {
			res.Snippet = buildSnippet(hit, req.Query)
			res.HasHighlight = true
		} else {
			res.Snippet = buildPlainSnippet(contents[filename], 200)
		}

		// Tipo (determinístico — tags + caminho).
		tags := db.TagsToSlice(ftsDocs[filename].Tags)
		if len(tags) == 0 {
			tags = allTags[filename]
		}
		res.Tipo = string(domain.DetectNoteType(tags, filename))

		// SSOT do ícone (paridade com a busca global / banco de dados).
		res.Icon = icons.IconSVG(res.Tipo, "w-3 h-3")

		// Seção (do documento FTS quando o termo casou).
		if doc, ok := ftsDocs[filename]; ok {
			res.Secao = doc.Secao
		}

		// Tags de usuário (sem as tags internas de tipo).
		for _, t := range tags {
			if !domain.InternalTypeTags[strings.ToLower(t)] {
				res.Tags = append(res.Tags, t)
			}
		}

		results = append(results, res)
	}

	httputil.WriteJSON(w, map[string]interface{}{
		"query":   req.Query,
		"results": results,
	})
}

// semanticConsensusPass aplica o voto majoritário aos candidatos só-semânticos:
// nota longa (≥3 chunks) precisa de match em ≥2 chunks, exceto similaridade
// excepcional (≥82%). Notas curtas (1-2 chunks) passam com match único.
func semanticConsensusPass(totalChunks, chunkMatches int, similarity float64) bool {
	if totalChunks <= 2 {
		return true
	}
	if chunkMatches >= 2 {
		return true
	}
	return chunkMatches >= 1 && similarity >= 0.82
}
