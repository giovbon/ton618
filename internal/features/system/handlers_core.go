package system

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/features/notes"
	"ton618/core/internal/features/search"
	"ton618/core/internal/httputil"
)

// ── Pages e Handlers Core do Sistema ──

func (ctx *HandlerContext) HandleIndex(w http.ResponseWriter, r *http.Request) {
	search.Index("TON-618").Render(r.Context(), w)
}

func (ctx *HandlerContext) HandleStatus(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, map[string]interface{}{
		"status":    "ok",
		"documents": ctx.Store.GetDocumentCount(),
	})
}

func (ctx *HandlerContext) HandleHealth(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, map[string]string{
		"status":    "up",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (ctx *HandlerContext) HandleGetTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	tags, err := ctx.Store.GetAllTags()
	if err != nil {
		tags = nil
	}
	// Usa InternalTypeTags para filtrar todas as tags de tipo de editor
	filtered := domain.FilterUserTags(tags)
	httputil.WriteJSON(w, map[string]interface{}{
		"tags": filtered,
	})
}

func (ctx *HandlerContext) HandleGetAllNotes(w http.ResponseWriter, r *http.Request) {
	noteList, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Slice(noteList, func(i, j int) bool {
		return mtimeNewer(noteList[i].Mtime, noteList[j].Mtime)
	})
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	httputil.WriteJSON(w, map[string]interface{}{
		"notes": noteList,
		"total": len(noteList),
	})
}

func (ctx *HandlerContext) HandleGetSidebar(w http.ResponseWriter, r *http.Request) {
	// Delega para o NoteService que consolida file_mods + notes
	noteList, err := ctx.Notes.GetMany()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sort.Slice(noteList, func(i, j int) bool {
		return mtimeNewer(noteList[i].Mtime, noteList[j].Mtime)
	})

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	filteredNotes := filterNotes(noteList, q)

	// Badge de filhas na sidebar: o número de notas que declaram cada uma como
	// pai (`pai:` do frontmatter). As filhas são referenciadas pelo nome-base, a
	// mesma chave usada na árvore do banco (baseNameKey).
	if counts, cErr := ctx.Notes.GetChildrenCounts(); cErr == nil && len(counts) > 0 {
		for i := range filteredNotes {
			filteredNotes[i].ChildrenCount = counts[baseNameKey(filteredNotes[i].Arquivo)]
		}
	}

	// Paginação do scroll infinito da busca de Notas (from>0 = "carregar mais").
	from, _ := strconv.Atoi(r.URL.Query().Get("from"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 {
		size = 50
	}
	if from < 0 {
		from = 0
	}
	if size > 200 {
		size = 200
	}

	total := len(filteredNotes)
	var page []domain.NoteItem
	if from < total {
		end := from + size
		if end > total {
			end = total
		}
		page = filteredNotes[from:end]
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")

	if from > 0 {
		notes.SidebarTreeMore(page, q, from, size, total).Render(r.Context(), w)
		return
	}
	notes.SidebarTreePaginated(page, q, 0, size, total).Render(r.Context(), w)
}

func filterNotes(noteList []domain.NoteItem, query string) []domain.NoteItem {
	if query == "" {
		return noteList
	}

	queryLower := strings.ToLower(query)
	nameSearch := strings.TrimSpace(queryLower)

	var nameMatches []domain.NoteItem
	for _, n := range noteList {
		filenameLower := strings.ToLower(n.Arquivo)
		if strings.Contains(filenameLower, nameSearch) {
			nameMatches = append(nameMatches, n)
		}
	}

	return nameMatches
}

// parseMtime interpreta os formatos de mtime usados no app:
//   - RFC3339 com fuso (banco/sidebar, ex.: "2026-08-17T18:55:00Z");
//   - exibição do Tabulator/Acervo ("2006-01-02 15:04:05", hora local — ver
//     handlers_database_tabulator.go, que usa t.Local().Format).
//
// O segundo formato não carrega fuso; ele é gerado em hora local, então é
// parseado em time.Local para comparar instantes corretamente.
func parseMtime(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// mtimeNewer compara dois mtimes e retorna true se a é estritamente mais
// recente que b. É a ÚNICA regra de ordenação por mtime do app — usada pela
// sidebar (RFC3339) e pelo Acervo/árvore (formato de exibição); antes a árvore
// tinha uma cópia própria (mtimeIsNewer) que divergia em valores inválidos.
// Inválidos nunca "vencem": a inválido => false; b inválido => true.
func mtimeNewer(a, b string) bool {
	ta, okA := parseMtime(a)
	tb, okB := parseMtime(b)
	if !okA {
		return false
	}
	if !okB {
		return true
	}
	return ta.After(tb)
}

func (ctx *HandlerContext) HandleManualSync(w http.ResponseWriter, r *http.Request) {
	contents, err := ctx.Store.GetAllNotesContent()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	count := 0
	for _, content := range contents {
		if content != "" {
			count++
		}
	}

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("HX-Trigger", "reload-sidebar")
	w.Write([]byte(`<div class="text-green-500">✓ Sincronização concluída (` + strconv.Itoa(count) + ` notas processadas)</div>`))
}

func (ctx *HandlerContext) HandleLogin(w http.ResponseWriter, r *http.Request) {
	Login().Render(r.Context(), w)
}
