package search

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
	"ton618/core/internal/search"
)

// askMaxSources é quantas notas-fonte a resposta traz (e o que o browser envia
// para a síntese por IA). askExcerptChars é o tamanho do trecho por nota.
//
// Juntos definem o contexto que o modelo local recebe (~4.800 caracteres no
// padrão). Era 5 × 500 = 2.500 caracteres, o que sufocava perguntas amplas do
// tipo "resuma tudo sobre X" — o modelo tinha pouco material para resumir.
const (
	askMaxSources   = 6
	askExcerptChars = 800
)

// askRequest é o payload recebido do browser para a funcionalidade "Perguntar".
// O browser embeda a query com o mesmo worker usado na busca híbrida e envia o
// vetor junto com o texto da pergunta.
type askRequest struct {
	Query     string    `json:"query"`
	Embedding []float32 `json:"embedding"`
	// Limit máximo de notas-fonte a retornar (padrão 5, máx 10).
	Limit int `json:"limit"`
}

// askSource representa uma nota-fonte com o trecho mais relevante extraído.
type askSource struct {
	Filename string  `json:"filename"`
	Tipo     string  `json:"type"`
	Name     string  `json:"name"`    // nome legível (sem path/extensão)
	Excerpt  string  `json:"excerpt"` // parágrafo(s) mais relevante(s) da nota
	Score    float64 `json:"score"`   // score de relevância (0-1)
	IsSem    bool    `json:"is_sem"`  // veio da busca semântica?
	IsFTS    bool    `json:"is_fts"`  // veio da busca textual?
	SemSim   float64 `json:"sem_sim"` // similaridade semântica (0-1), se disponível
}

// askResponse é o payload devolvido para o browser.
type askResponse struct {
	Query   string      `json:"query"`
	Sources []askSource `json:"sources"`
	// Found indica se alguma fonte relevante foi encontrada.
	Found bool `json:"found"`
}

// HandleAsk responde a perguntas baseando-se APENAS no conteúdo das notas.
// Funciona sem LLM: recupera os chunks mais relevantes via busca híbrida (FTS5 +
// semântica) e extrai os trechos mais densos de cada nota como resposta.
// O browser exibe os trechos com os links para as notas-fonte.
//
// POST /api/ask
func (ctx *HandlerContext) HandleAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invalido: "+err.Error(), http.StatusBadRequest)
		return
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		http.Error(w, "query obrigatoria", http.StatusBadRequest)
		return
	}

	limit := req.Limit
	if limit <= 0 || limit > 10 {
		limit = askMaxSources
	}

	rCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// ── 1. Busca híbrida: FTS5 + semântica (igual ao HandleHybridSearch) ──
	hasSemantic := len(req.Embedding) == db.EmbeddingDim && !isZeroEmbedding(req.Embedding)

	pct := 55
	maxDist := math.MaxFloat64
	exceptionalSim := 0.82
	if hasSemantic {
		pct = hybridThresholdPct(ctx.Store)
		maxDist = math.Sqrt(2.0 * (1.0 - float64(pct)/100.0))
		exceptionalSim = semanticExceptionalSimilarity(pct)
	}

	// engineN: candidatos por motor — generoso para garantir cobertura máxima
	engineN := limit * 6
	if engineN > hybridMaxEngineCandidates {
		engineN = hybridMaxEngineCandidates
	}

	ftsRanks := make(map[string]int)
	semRanks := make(map[string]int)
	ftsDocs := make(map[string]db.Document)
	ftsHits := make(map[string]search.SearchHit)
	semSim := make(map[string]float64)

	var wg sync.WaitGroup
	var ftsErr, semErr error
	var semCandidates []db.SimilarResult

	// FTS5 textual com fallback flexível para linguagem natural
	wg.Add(1)
	go func() {
		defer wg.Done()
		// 1. Tenta a busca estrita com search.Search
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

		// 2. Se a busca estrita retornou poucos candidatos (típico de perguntas com
		// palavras interrogativas como "qual", "como", "cria"), executa a busca flexível
		// por termos-chave da pergunta com OR ponderado no FTS5.
		if len(ftsRanks) < limit {
			keywords := extractQuestionKeywords(req.Query)
			if len(keywords) > 0 {
				flexQuery := buildFTSFlexibleQuery(keywords)
				if flexQuery != "" {
					flexResults, _, errFlex := ctx.Store.SearchFTSWithContext(rCtx, flexQuery, 0, engineN)
					if errFlex == nil && len(flexResults) > 0 {
						docIDs := make([]string, len(flexResults))
						for i, r := range flexResults {
							docIDs[i] = r.DocID
						}
						timestampMap, _ := ctx.Store.BatchGetTimestamps(docIDs)

						for _, r := range flexResults {
							arquivo := r.Arquivo
							if seen[arquivo] {
								continue
							}
							seen[arquivo] = true
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
							ftsRanks[arquivo] = rank
							ftsDocs[arquivo] = doc
							ftsHits[arquivo] = search.SearchHit{
								ID:         r.DocID,
								Score:      r.Rank,
								Doc:        doc,
								FinalScore: calculateFlexibleScore(doc, keywords),
							}
							rank++
							if rank > engineN {
								break
							}
						}
					}
				}
			}
		}
	}()

	// KNN semântico
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
		slog.Error("ask: fts", "error", ftsErr)
		http.Error(w, "erro na busca: "+ftsErr.Error(), http.StatusInternalServerError)
		return
	}
	if semErr != nil {
		slog.Warn("ask: sem degraded", "error", semErr)
	}

	// ── 2. Filtra e ranqueia os candidatos semânticos ──
	rank := 1
	for _, h := range semCandidates {
		if h.Distance > maxDist {
			continue
		}
		if _, exists := semRanks[h.Filename]; exists {
			continue
		}
		sim := 1.0 - (h.Distance*h.Distance)/2.0
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

	// Gate de evidência: se a busca semântica está ativa, candidatos textuais
	// precisam de menção no texto/seção/arquivo ou aval semântico.
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

	// ── 3. Fusão RRF (Reciprocal Rank Fusion) ──
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
	}

	fusedFiles := search.ReciprocalRankFusionWeighted(ftsRanks, semRanks, ftsScore, semSim, k, limit)

	type fileCandidate struct {
		filename string
		score    float64
	}
	candidates := make([]fileCandidate, 0, len(fusedFiles))
	for _, f := range fusedFiles {
		score := search.WeightedFusionScore(ftsRanks[f], k, ftsScore[f]) +
			search.WeightedFusionScore(semRanks[f], k, semSim[f])
		candidates = append(candidates, fileCandidate{filename: f, score: score})
	}

	// ── 4. Carrega conteúdo completo e extrai trechos relevantes ──
	missingFiles := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if _, ok := ftsHits[c.filename]; !ok {
			missingFiles = append(missingFiles, c.filename)
		}
	}
	var contents map[string]string
	if len(missingFiles) > 0 {
		contents, _ = ctx.Store.BatchGetNotesContent(missingFiles)
	}

	allTags, _ := ctx.Store.GetAllFileTags()

	sources := make([]askSource, 0, len(candidates))
	for _, c := range candidates {
		filename := c.filename
		compositeScore := c.score

		var noteContent string
		if hit, ok := ftsHits[filename]; ok {
			noteContent = hit.Doc.Texto
		} else {
			noteContent = contents[filename]
		}

		sim := 0.0
		if s, ok := semSim[filename]; ok {
			sim = s
		}

		excerpt, matchScore, distinct := extractBestExcerpt(noteContent, req.Query, askExcerptChars)
		// Se a nota não tem nenhum trecho que case com os termos e não tem alta similaridade semântica, ignora
		if excerpt == "" {
			if sim < 0.65 {
				continue
			}
			// Se a similaridade é alta, extrai o início substancial
			excerpt = extractFallbackExcerpt(noteContent, askExcerptChars)
			if excerpt == "" {
				continue
			}
		}

		// Se já encontramos uma fonte muito relevante (ex: nota com vários termos da pergunta),
		// descarta notas subsequentes que só bateram 1 palavra isolada e sem suporte semântico
		if len(sources) > 0 && distinct <= 1 && sim < 0.60 {
			continue
		}

		// Tipo da nota
		tags := allTags[filename]
		if doc, ok := ftsDocs[filename]; ok && doc.Tags != "" {
			tags = db.TagsToSlice(doc.Tags)
		}
		tipo := string(domain.DetectNoteType(tags, filename))

		// Nome legível
		name := filename
		name = strings.TrimPrefix(name, "notes/")
		name = strings.TrimSuffix(name, ".md")
		name = strings.TrimPrefix(name, "captura-")

		sources = append(sources, askSource{
			Filename: filename,
			Tipo:     tipo,
			Name:     name,
			Excerpt:  excerpt,
			Score:    compositeScore,
			IsSem:    semRanks[filename] > 0,
			IsFTS:    ftsRanks[filename] > 0,
			SemSim:   sim,
		})
		_ = matchScore
	}

	resp := askResponse{
		Query:   req.Query,
		Sources: sources,
		Found:   len(sources) > 0,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// sanitizeFTS5TermSafe limpa um termo para uso seguro em queries FTS5.
func sanitizeFTS5TermSafe(term string) string {
	var b strings.Builder
	for _, r := range term {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}

// questionStopwords contém palavras interrogativas, verbos auxiliares e stopwords comuns
// que não devem ser exigidas rigidamente na busca textual de perguntas.
var questionStopwords = map[string]bool{
	// Pronomes e interrogativas PT
	"qual": true, "quais": true, "como": true, "onde": true, "aonde": true,
	"quando": true, "quem": true, "porque": true, "quanto": true, "quanta": true,
	"quantos": true, "quantas": true, "qualquer": true, "cujo": true, "cuja": true,

	// Verbos comuns em perguntas PT
	"fazer": true, "faz": true, "faco": true,
	"cria": true, "criar": true, "crio": true, "criando": true,
	"usa": true, "usar": true, "uso": true, "usando": true,
	"serve": true, "servir": true, "funciona": true, "funcionar": true,
	"posso": true, "pode": true, "podem": true, "podemos": true,
	"devo": true, "deve": true, "devem": true, "devemos": true,
	"tenho": true, "tem": true, "temos": true, "tendo": true,
	"saber": true, "sei": true, "sabe": true,
	"ver": true, "vejo": true, "ve": true,
	"acha": true, "achar": true, "acho": true,
	"obter": true, "obtenho": true, "pegar": true, "pego": true,
	"instalar": true, "instala": true, "instalo": true,
	"executar": true, "executa": true, "executo": true,
	"rodar": true, "roda": true, "rodo": true,

	// Formatos e genéricos PT
	"forma": true, "jeito": true, "maneira": true, "modo": true,
	"passo": true, "passos": true, "tutorial": true, "guia": true,
	"exemplo": true, "exemplos": true,

	// Conectivos, preposições e artigos PT
	"de": true, "da": true, "do": true, "dos": true, "das": true,
	"em": true, "no": true, "na": true, "nos": true, "nas": true,
	"um": true, "uma": true, "uns": true, "umas": true,
	"o": true, "a": true, "os": true, "as": true,
	"para": true, "pra": true, "pro": true, "pros": true, "pras": true,
	"por": true, "pelo": true, "pela": true, "pelos": true, "pelas": true,
	"com": true, "sem": true, "sob": true, "sobre": true,
	"e": true, "ou": true, "se": true, "que": true, "mas": true,
	"meu": true, "minha": true, "meus": true, "minhas": true,
	"seu": true, "sua": true, "seus": true, "suas": true,
	"dele": true, "dela": true, "deles": true, "delas": true,
	"este": true, "esta": true, "estes": true, "estas": true, "isto": true,
	"esse": true, "essa": true, "esses": true, "essas": true, "isso": true,
	"aquele": true, "aquela": true, "aqueles": true, "aquelas": true, "aquilo": true,

	// Interrogativas e auxiliares EN
	"what": true, "how": true, "where": true, "when": true, "who": true, "why": true,
	"which": true, "is": true, "are": true, "was": true, "were": true, "be": true,
	"does": true, "did": true, "can": true, "could": true, "should": true,
	"would": true, "the": true, "to": true, "in": true, "on": true, "at": true, "for": true,
}

// extractQuestionKeywords extrai os termos substantivos essenciais da pergunta.
func extractQuestionKeywords(query string) []string {
	clean := removeAccents(strings.ToLower(query))
	// Substitui pontuações por espaço
	clean = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			return r
		}
		return ' '
	}, clean)

	words := strings.Fields(clean)
	var keywords []string
	seen := make(map[string]bool)

	for _, w := range words {
		w = strings.Trim(w, "-_")
		if len(w) <= 1 {
			continue
		}
		if questionStopwords[w] {
			continue
		}
		if seen[w] {
			continue
		}
		seen[w] = true
		keywords = append(keywords, w)
	}

	// Se todos os termos foram filtrados, mantém palavras originais maiores que 2 chars
	if len(keywords) == 0 {
		for _, w := range words {
			if len(w) > 2 && !seen[w] {
				seen[w] = true
				keywords = append(keywords, w)
			}
		}
	}

	return keywords
}

// buildFTSFlexibleQuery monta query FTS5 unindo termos por OR para ranqueamento BM25 por cobertura.
func buildFTSFlexibleQuery(keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}
	var parts []string
	for _, kw := range keywords {
		clean := sanitizeFTS5TermSafe(kw)
		if clean == "" {
			continue
		}
		if len(clean) > 2 {
			clean += "*"
		}
		parts = append(parts, `(tags:`+clean+` OR arquivo:`+clean+` OR secao:`+clean+` OR texto:`+clean+`)`)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " OR ")
}

// containsWord verifica se a palavra ocorre como termo inteiro no texto (limite de palavra).
// Evita falsos positivos como "uv" casando com "houve" ou "chuva".
func containsWord(textLower, wordLower string) bool {
	idx := 0
	wordLen := len(wordLower)
	for {
		pos := strings.Index(textLower[idx:], wordLower)
		if pos == -1 {
			return false
		}
		start := idx + pos
		end := start + wordLen

		// Caractere anterior deve ser delimitador
		leftBoundary := (start == 0) || !isWordRune(rune(textLower[start-1]))
		// Caractere posterior deve ser delimitador
		rightBoundary := (end == len(textLower)) || !isWordRune(rune(textLower[end]))

		if leftBoundary && rightBoundary {
			return true
		}
		idx = start + 1
		if idx >= len(textLower) {
			return false
		}
	}
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// calculateFlexibleScore calcula pontuação de cobertura de termos (0.0 a 1.0).
func calculateFlexibleScore(doc db.Document, keywords []string) float64 {
	if len(keywords) == 0 {
		return 1.0
	}
	textLower := removeAccents(strings.ToLower(doc.Texto))
	fileLower := removeAccents(strings.ToLower(doc.Arquivo))
	secLower := removeAccents(strings.ToLower(doc.Secao))

	matched := 0
	for _, kw := range keywords {
		kwNorm := removeAccents(strings.ToLower(kw))
		if containsWord(textLower, kwNorm) || containsWord(fileLower, kwNorm) || containsWord(secLower, kwNorm) {
			matched++
		}
	}
	return float64(matched) / float64(len(keywords))
}

// splitTextIntoLogicalBlocks divide o texto da nota em blocos lógicos (parágrafos, itens de lista principais).
func splitTextIntoLogicalBlocks(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	rawBlocks := strings.Split(text, "\n\n")
	var blocks []string

	for _, b := range rawBlocks {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		// Se o bloco é grande (> 220 chars) e contém itens de lista de primeiro nível, divide-os
		if len(b) > 220 && (strings.Contains(b, "\n- ") || strings.Contains(b, "\n* ")) {
			lines := strings.Split(b, "\n")
			var current strings.Builder
			for _, line := range lines {
				trimmed := strings.TrimRight(line, " \t")
				isTopLevel := (strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "#")) &&
					!strings.HasPrefix(trimmed, "  ") && !strings.HasPrefix(trimmed, "\t")

				if isTopLevel && current.Len() > 0 {
					cb := strings.TrimSpace(current.String())
					if cb != "" {
						blocks = append(blocks, cb)
					}
					current.Reset()
				}
				current.WriteString(line)
				current.WriteString("\n")
			}
			if current.Len() > 0 {
				cb := strings.TrimSpace(current.String())
				if cb != "" {
					blocks = append(blocks, cb)
				}
			}
		} else {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

// extractBestExcerpt extrai o trecho mais denso e explicativo da nota em relação à query.
// extractBestExcerpt extrai o trecho mais denso e explicativo da nota em relação à query.
// Retorna o trecho recortado, o score de relevância e a quantidade de termos distintos casados.
func extractBestExcerpt(text, query string, maxChars int) (string, int, int) {
	if text == "" {
		return "", 0, 0
	}

	// Remove frontmatter YAML
	if strings.HasPrefix(strings.TrimSpace(text), "---") {
		idx := strings.Index(text[3:], "\n---")
		if idx != -1 {
			text = strings.TrimSpace(text[idx+7:])
		}
	}

	keywords := extractQuestionKeywords(query)
	if len(keywords) == 0 {
		keywords = extractSearchTerms(query)
	}

	normalizedKeywords := make([]string, 0, len(keywords))
	for _, k := range keywords {
		normalizedKeywords = append(normalizedKeywords, removeAccents(strings.ToLower(k)))
	}

	blocks := splitTextIntoLogicalBlocks(text)
	type scoredBlock struct {
		text     string
		score    int
		distinct int
	}
	scored := make([]scoredBlock, 0, len(blocks))

	for _, b := range blocks {
		b = strings.TrimSpace(b)
		if b == "" || len(b) < 8 {
			continue
		}
		// Ignora linhas que são só separadores markdown
		if strings.HasPrefix(b, "---") || strings.HasPrefix(b, "***") {
			continue
		}
		bLower := removeAccents(strings.ToLower(b))
		score := 0
		distinct := 0
		for _, kw := range normalizedKeywords {
			if containsWord(bLower, kw) {
				score += 2
				distinct++
				// Bônus se a palavra aparece em código inline (`comando`) ou negrito
				if strings.Contains(b, "`"+kw) || strings.Contains(b, "**"+kw) {
					score += 2
				}
			}
		}
		scored = append(scored, scoredBlock{text: b, score: score, distinct: distinct})
	}

	if len(scored) == 0 {
		return "", 0, 0
	}

	// Localiza o bloco com maior score
	bestIdx := 0
	bestScore := -1
	for i, sb := range scored {
		if sb.score > bestScore {
			bestScore = sb.score
			bestIdx = i
		}
	}

	// Se nenhum bloco teve match com as palavras da pergunta, não inventa resposta
	if bestScore <= 0 {
		return "", 0, 0
	}

	result := scored[bestIdx].text
	distinctCount := scored[bestIdx].distinct

	// Se sobrar espaço, tenta anexar o bloco adjacente se ele tiver relevância
	if bestIdx+1 < len(scored) && len(result)+len(scored[bestIdx+1].text)+2 < maxChars {
		next := scored[bestIdx+1]
		if next.score > 0 || len(result) < maxChars/2 {
			result = result + "\n\n" + next.text
			distinctCount += next.distinct
		}
	}

	if len(result) > maxChars {
		cut := maxChars
		for cut > maxChars-50 && cut > 0 && result[cut] != ' ' && result[cut] != '\n' {
			cut--
		}
		result = result[:cut] + "…"
	}

	return result, bestScore, distinctCount
}

// extractFallbackExcerpt extrai o primeiro bloco substancial de uma nota quando o match foi 100% semântico.
func extractFallbackExcerpt(text string, maxChars int) string {
	if strings.HasPrefix(strings.TrimSpace(text), "---") {
		idx := strings.Index(text[3:], "\n---")
		if idx != -1 {
			text = strings.TrimSpace(text[idx+7:])
		}
	}
	blocks := splitTextIntoLogicalBlocks(text)
	for _, b := range blocks {
		b = strings.TrimSpace(b)
		if len(b) >= 30 && !strings.HasPrefix(b, "#") {
			if len(b) > maxChars {
				return b[:maxChars] + "…"
			}
			return b
		}
	}
	if len(text) > maxChars {
		return text[:maxChars] + "…"
	}
	return text
}
