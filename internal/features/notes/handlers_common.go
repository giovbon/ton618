package notes

import (
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"ton618/internal/core/domain"
	"ton618/internal/processor"
)

// SafeFileQueryEscape escapes a file path for a query string but keeps slashes intact to avoid reverse proxy (e.g. Cloudflare) blocks.
func SafeFileQueryEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "%2F", "/")
}

// noteLoadResult agrupa todos os dados necessários para renderizar qualquer editor de nota.
type noteLoadResult struct {
	Content      string
	FileTags     []string // tags brutas incluindo tags internas de tipo
	UserTags     []string // tags filtradas para display (sem tags internas)
	AllTags      []string
	Backlinks    *domain.BacklinksResult
	SimilarNotes []domain.SimilarNoteItem
	Exists       bool // true se o conteúdo foi carregado do banco (nota existente)
}

// loadNoteData carrega todos os dados comuns necessários para renderizar qualquer editor.
// Centraliza: GetNote + IncrementPopularity + GetFileTags + FilterUserTags +
// GetAllTags + GetBacklinks + SimilarNotes.
func (ctx *HandlerContext) loadNoteData(filename string) (noteLoadResult, error) {
	var r noteLoadResult

	if data, err := ctx.Store.GetNote(filename); err == nil && data != "" {
		r.Content = data
		r.Exists = true
		ctx.Store.IncrementPopularity(filename)
	}

	fileTags, _ := ctx.Store.GetFileTags(filename)
	r.FileTags = fileTags
	r.UserTags = domain.FilterUserTags(fileTags)

	allTags, _ := ctx.Store.GetAllTags()
	r.AllTags = allTags

	backlinks, err := ctx.Notes.GetBacklinks(filename)
	if err != nil {
		slog.Error("get backlinks", "file", filename, "error", err)
		backlinks = &domain.BacklinksResult{}
	}
	r.Backlinks = backlinks

	// ── SimilarNotes: Estratégia de Pontuação Acumulada ──
	// Para cada chunk da nota atual, busca vizinhos via sqlite-vec.
	// Usa dois mapas:
	//   accumulatedScores[fname] = soma das similaridades de cosseno dos matches
	//   maxSimilarities[fname]  = maior similaridade de cosseno encontrada para cada nota [0.0 - 1.0]
	//
	// Ordenação: Ordenado pela pontuação acumulada (decrescente). Em caso de empate, pela maior similaridade (decrescente).

	// Carrega limite do banco ou assume padrão (40%)
	similarThresholdPct := 40
	if val, err := ctx.Store.GetSetting("similar_notes_threshold"); err == nil && val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			if v == 72 {
				similarThresholdPct = 40
			} else {
				similarThresholdPct = v
			}
		}
	}

	embeddings, err := ctx.Store.GetNoteEmbeddings(filename)
	if err != nil {
		slog.Error("loadNoteData: erro ao obter embeddings", "file", filename, "error", err)
	}

	var similarNotes []domain.SimilarNoteItem
	if len(embeddings) > 0 {
		accumulatedScores := make(map[string]float64)
		maxSimilarities := make(map[string]float64)

		for _, emb := range embeddings {
			hits, err := ctx.Store.SearchSimilar(emb, 10)
			if err != nil {
				continue
			}
			for _, hit := range hits {
				if hit.Filename == filename {
					continue // descarta a si mesma
				}
				// Converte distância L2 para similaridade de cosseno
				cosSim := 1.0 - (hit.Distance*hit.Distance)/2.0
				pct := cosSim * 100.0

				if int(pct) < similarThresholdPct {
					continue // threshold dinâmico como pré-filtro
				}

				accumulatedScores[hit.Filename] += cosSim
				if m, exists := maxSimilarities[hit.Filename]; !exists || cosSim > m {
					maxSimilarities[hit.Filename] = cosSim
				}
			}
		}

		similarNotes = filterAndRankSimilarNotes(accumulatedScores, maxSimilarities)
	}
	r.SimilarNotes = similarNotes

	return r, nil
}

// ensureNoteFilename normaliza o filename da query string e redireciona se necessário.
// Retorna o filename normalizado e true se um redirect foi enviado.
func ensureNoteFilename(w http.ResponseWriter, r *http.Request, route string) (string, bool) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		filename = "notes/" + processor.GenerateCUID2() + ".md"
	}
	sanitized := NoteFilename(filename)
	if sanitized != filename {
		http.Redirect(w, r, route+"?file="+SafeFileQueryEscape(sanitized), http.StatusFound)
		return sanitized, true
	}
	return sanitized, false
}

// redirectIfWrongEditor redireciona para o editor correto se o tipo da nota
// não corresponder à rota atual. Retorna true se um redirect foi enviado.
// Só deve ser chamado quando a nota já existe (nd.Exists == true).
func redirectIfWrongEditor(w http.ResponseWriter, r *http.Request, noteType domain.NoteType, currentRoute, filename string) bool {
	correctRoute := noteType.EditorRoute()
	if correctRoute != currentRoute {
		http.Redirect(w, r, correctRoute+"?file="+SafeFileQueryEscape(filename), http.StatusFound)
		return true
	}
	return false
}

// buildEditorData constrói o EditorData a partir de um noteLoadResult.
func buildEditorData(title, filename string, nd noteLoadResult) domain.EditorData {
	return domain.EditorData{
		Title:        title,
		Filename:     filename,
		DisplayName:  domain.DisplayName(filename),
		Content:      nd.Content,
		Tags:         nd.UserTags,
		AllTags:      nd.AllTags,
		Backlinks:    nd.Backlinks,
		SimilarNotes: nd.SimilarNotes,
	}
}

// filterAndRankSimilarNotes ordena os candidatos pela pontuação acumulada e limita aos top 5.
// É uma função pura (sem acesso a banco) para facilitar testes unitários.
func filterAndRankSimilarNotes(
	accumulatedScores map[string]float64,
	maxSimilarities map[string]float64,
) []domain.SimilarNoteItem {
	type tempItem struct {
		filename string
		score    float64
		maxSim   float64
	}
	var list []tempItem
	for fname, score := range accumulatedScores {
		list = append(list, tempItem{
			filename: fname,
			score:    score,
			maxSim:   maxSimilarities[fname],
		})
	}

	// Ordena primariamente por pontuação acumulada (decrescente),
	// secundariamente por maior similaridade (decrescente).
	sort.Slice(list, func(i, j int) bool {
		if math.Abs(list[i].score-list[j].score) > 1e-9 {
			return list[i].score > list[j].score
		}
		return list[i].maxSim > list[j].maxSim
	})

	// Limita aos top 5
	limit := 5
	if len(list) < limit {
		limit = len(list)
	}
	result := make([]domain.SimilarNoteItem, 0, limit)
	for i := 0; i < limit; i++ {
		pct := int(list[i].maxSim * 100.0)
		if pct < 0 {
			pct = 0
		} else if pct > 100 {
			pct = 100
		}
		result = append(result, domain.SimilarNoteItem{
			Filename:    list[i].filename,
			DisplayName: domain.DisplayName(list[i].filename),
			Percentage:  pct,
		})
	}
	return result
}
