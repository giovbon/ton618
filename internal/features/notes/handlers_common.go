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

	// ── SimilarNotes: Estratégia do Voto Majoritário ──
	// Para cada chunk da nota atual, busca vizinhos via sqlite-vec.
	// Usa dois mapas:
	//   minDistMap[fname]  = menor distância L2 encontrada
	//   matchCounts[fname] = em quantos chunks diferentes o candidato apareceu
	//
	// Threshold: dist <= 0.75 (~72% similaridade cosseno)
	// Notas longas (≥3 chunks): exigem ≥2 matches, a menos que dist < 0.60 (~82%)
	// Ordenação: primária por frequência (decrescente), secundária por distância (crescente)

	// Carrega limite do banco ou assume padrão (72%)
	similarThresholdPct := 72
	if val, err := ctx.Store.GetSetting("similar_notes_threshold"); err == nil && val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			similarThresholdPct = v
		}
	}

	// Converte % para distância L2
	cosSimLimit := float64(similarThresholdPct) / 100.0
	similarThreshold := math.Sqrt(2.0 * (1.0 - cosSimLimit))

	const similarExcellent = 0.60

	embeddings, err := ctx.Store.GetNoteEmbeddings(filename)
	if err != nil {
		slog.Error("loadNoteData: erro ao obter embeddings", "file", filename, "error", err)
	}

	var similarNotes []domain.SimilarNoteItem
	if len(embeddings) > 0 {
		minDistMap := make(map[string]float64) // filename → menor distância
		matchCounts := make(map[string]int)    // filename → quantos chunks deram match

		for _, emb := range embeddings {
			hits, err := ctx.Store.SearchSimilar(emb, 10)
			if err != nil {
				continue
			}
			for _, hit := range hits {
				if hit.Filename == filename {
					continue // descarta a si mesma
				}
				if hit.Distance > similarThreshold {
					continue // threshold rigoroso
				}
				matchCounts[hit.Filename]++
				if d, exists := minDistMap[hit.Filename]; !exists || hit.Distance < d {
					minDistMap[hit.Filename] = hit.Distance
				}
			}
		}

		similarNotes = filterAndRankSimilarNotes(minDistMap, matchCounts, len(embeddings), similarThreshold, similarExcellent)
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

// filterAndRankSimilarNotes aplica voto majoritário, ordenação e limite aos candidatos.
// É uma função pura (sem acesso a banco) para facilitar testes unitários.
//
// Parâmetros:
//   - minDistMap: nome do arquivo → menor distância L2 encontrada
//   - matchCounts: nome do arquivo → em quantos chunks diferentes apareceu
//   - totalChunks: total de chunks da nota original (para determinar se é "longa")
//   - threshold: limite de distância L2 (notas com distância maior são descartadas)
//   - excellent: distância abaixo da qual dispensa voto majoritário
//
// Constantes internas:
//   - longNoteMinChunks = 3 (nota longa)
//   - minMatchLongNote  = 2 (matches necessários para nota longa)
func filterAndRankSimilarNotes(
	minDistMap map[string]float64,
	matchCounts map[string]int,
	totalChunks int,
	threshold float64,
	excellent float64,
) []domain.SimilarNoteItem {
	const longNoteMinChunks = 3
	const minMatchLongNote = 2

	type tempItem struct {
		filename string
		distance float64
		matches  int
	}
	var list []tempItem
	for fname, dist := range minDistMap {
		matches := matchCounts[fname]

		// Voto majoritário: notas longas (≥3 chunks) precisam de ≥2 matches,
		// a menos que a similaridade seja excepcional (dist < excellent)
		if totalChunks >= longNoteMinChunks && matches < minMatchLongNote && dist >= excellent {
			continue
		}

		list = append(list, tempItem{filename: fname, distance: dist, matches: matches})
	}

	// Ordena primariamente por frequência (mais matches primeiro),
	// em caso de empate pela menor distância L2
	sort.Slice(list, func(i, j int) bool {
		if list[i].matches != list[j].matches {
			return list[i].matches > list[j].matches
		}
		return list[i].distance < list[j].distance
	})

	// Limita aos top 5
	limit := 5
	if len(list) < limit {
		limit = len(list)
	}
	result := make([]domain.SimilarNoteItem, 0, limit)
	for i := 0; i < limit; i++ {
		// Converte distância L2 para percentual de similaridade de cosseno
		cosSim := 1.0 - (list[i].distance*list[i].distance)/2.0
		pct := int(cosSim * 100)
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
