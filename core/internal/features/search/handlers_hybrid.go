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
	From      int       `json:"from"`
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

// hybridMaxEngineCandidates limita quantos candidatos cada motor entrega para a
// fusão (offset + página cobertos até esse teto). Acima dele, a paginação
// simplesmente termina — evita KNN caro em corpora gigantes.
const hybridMaxEngineCandidates = 200

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
	from := req.From
	if from < 0 {
		from = 0
	}
	if from > 500 {
		from = 500 // trava contra paginação profunda/abusiva
	}
	// Cada motor entrega candidatos suficientes para cobrir offset + página,
	// com teto para não explodir o custo do KNN em corpora enormes.
	engineN := (from + limit) * 2
	if engineN > hybridMaxEngineCandidates {
		engineN = hybridMaxEngineCandidates
	}

	// Timeout no mesmo padrão da busca global (HandleSearch): evita request
	// pendurado em lock/DB lento. Vale para as duas goroutines abaixo (o FTS5 e
	// o KNN agora propagam o contexto para as queries).
	rCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// A parte semântica só participa se houver um embedding válido.
	pct := 55
	exceptionalSim := 0.82
	hasSemantic := len(req.Embedding) == db.EmbeddingDim && !isZeroEmbedding(req.Embedding)
	maxDist := math.MaxFloat64
	if hasSemantic {
		pct = hybridThresholdPct(ctx.Store)
		maxDist = math.Sqrt(2.0 * (1.0 - float64(pct)/100.0))
		// O "excepcional" do voto majoritário acompanha o threshold configurado
		// (com piso histórico de 82%), em vez de ser um valor fixo.
		exceptionalSim = semanticExceptionalSimilarity(pct)
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
		results, err := search.Search(rCtx, ctx.Store, req.Query, 0, engineN,
			ctx.Store.GetBacklinkCount, ctx.Store.GetSynapticWeight)
		if err != nil {
			ftsErr = err
			return
		}
		seen := make(map[string]bool)
		rank := 1
		for _, hit := range results.Hits {
			arquivo := hit.Doc.Arquivo
			if seen[arquivo] {
				continue
			}
			seen[arquivo] = true
			ftsRanks[arquivo] = rank
			ftsDocs[arquivo] = hit.Doc
			ftsHits[arquivo] = hit
			rank++
			if rank > engineN {
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
		similar, err := ctx.Store.SearchSimilarWithConsensus(rCtx, req.Embedding, engineN, maxDist)
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
	// precisa de match em ≥2 chunks, exceto similaridade excepcional (que segue
	// o threshold configurado — ver semanticExceptionalSimilarity).
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
		if !anchored && !semanticConsensusPass(h.TotalChunks, h.ChunkMatches, sim, exceptionalSim) {
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

	// 3. Fusão RRF ponderada pela qualidade de cada motor (k configurável via
	// Configurações > Semântica). Normaliza o FinalScore do FTS (0..1) para o
	// peso — como o score é monotônico com o rank, a ordem interna de cada
	// motor é preservada; a ponderação apenas equilibra a força da evidência
	// entre os dois motores (item: fusão ponderada).
	k := rrfK(ctx.Store)

	ftsScore := make(map[string]float64, len(ftsHits))
	maxFinal := 0.0
	for _, hit := range ftsHits {
		if hit.FinalScore > maxFinal {
			maxFinal = hit.FinalScore
		}
	}
	if maxFinal > 0 {
		for arquivo, hit := range ftsHits {
			if s := hit.FinalScore / maxFinal; s > 0 {
				ftsScore[arquivo] = s
			}
		}
	} else {
		for arquivo := range ftsHits {
			ftsScore[arquivo] = 1.0
		}
	}
	// semScore já é a similaridade (cosseno) em [0,1] → semSim.

	// Fusão sobre a união completa dos candidatos para permitir paginação real
	// (from/offset) com total aproximado e estável.
	unionCap := len(ftsRanks) + len(semRanks)
	fusedAll := search.ReciprocalRankFusionWeighted(ftsRanks, semRanks, ftsScore, semSim, k, unionCap)
	total := len(fusedAll)
	start := from
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	fused := fusedAll[start:end]
	hasMore := end < total

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
			res.RRFScore += search.WeightedFusionScore(rank, k, ftsScore[filename])
		}
		if rank, ok := semRanks[filename]; ok {
			rp := rank
			res.RankSem = &rp
			res.RRFScore += search.WeightedFusionScore(rank, k, semSim[filename])
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
		"query":    req.Query,
		"results":  results,
		"total":    total,
		"has_more": hasMore,
	})
}

// semanticExceptionalSimilarity retorna o limiar "excepcional" do voto
// majoritário para nota longa com 1 chunk: segue o threshold configurado com
// uma margem de 5 p.p., com piso histórico de 82% (comportamento antigo
// preservado para thresholds baixos/padrão). Para thresholds altos (ex.: 90%)
// o "excepcional" sobe junto, evitando que uma nota longa passe com 1 chunk
// apenas por estar no limiar.
func semanticExceptionalSimilarity(pct int) float64 {
	const floor = 0.82
	fromThreshold := float64(pct)/100.0 + 0.05
	if fromThreshold <= floor {
		return floor
	}
	if fromThreshold > 1 {
		return 1
	}
	return fromThreshold
}

// semanticConsensusPass aplica o voto majoritário aos candidatos só-semânticos:
// nota longa (≥3 chunks) precisa de match em ≥2 chunks, exceto similaridade
// excepcional (exceptionalSim, derivado do threshold configurável).
// Notas curtas (1-2 chunks) passam com match único.
func semanticConsensusPass(totalChunks, chunkMatches int, similarity, exceptionalSim float64) bool {
	if totalChunks <= 2 {
		return true
	}
	if chunkMatches >= 2 {
		return true
	}
	return chunkMatches >= 1 && similarity >= exceptionalSim
}
