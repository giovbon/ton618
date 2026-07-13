package notes

import (
	"log/slog"
	"net/http"
	"net/url"
	"sort"
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

	// Carrega Notas Semelhantes usando os embeddings armazenados
	embeddings, err := ctx.Store.GetNoteEmbeddings(filename)
	if err != nil {
		slog.Error("loadNoteData: erro ao obter embeddings", "file", filename, "error", err)
	}

	var similarNotes []domain.SimilarNoteItem
	if len(embeddings) > 0 {
		similarMap := make(map[string]float64)
		for _, emb := range embeddings {
			hits, err := ctx.Store.SearchSimilar(emb, 10)
			if err != nil {
				continue
			}
			for _, hit := range hits {
				if hit.Filename == filename {
					continue // descarta a si mesma
				}
				dist, exists := similarMap[hit.Filename]
				if !exists || hit.Distance < dist {
					similarMap[hit.Filename] = hit.Distance
				}
			}
		}

		type tempItem struct {
			filename string
			distance float64
		}
		var list []tempItem
		for fname, dist := range similarMap {
			// sqlite-vec MATCH retorna a distância L2 por padrão.
			// Para vetores normalizados: dist_L2 = sqrt(2 * (1 - cos_sim))
			// Uma similaridade de cosseno >= 55% (cos_sim >= 0.55) corresponde a dist_L2 <= sqrt(0.9) = 0.948.
			if dist <= 0.95 {
				list = append(list, tempItem{filename: fname, distance: dist})
			}
		}

		// Ordena por distância (menor distância = mais similar)
		sort.Slice(list, func(i, j int) bool {
			return list[i].distance < list[j].distance
		})

		// Limita aos top 5
		limit := 5
		if len(list) < limit {
			limit = len(list)
		}
		for i := 0; i < limit; i++ {
			// Converte distância L2 para percentual de similaridade de cosseno
			cosSim := 1.0 - (list[i].distance * list[i].distance / 2.0)
			pct := int(cosSim * 100)
			if pct < 0 {
				pct = 0
			} else if pct > 100 {
				pct = 100
			}
			similarNotes = append(similarNotes, domain.SimilarNoteItem{
				Filename:    list[i].filename,
				DisplayName: domain.DisplayName(list[i].filename),
				Percentage:  pct,
			})
		}
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
