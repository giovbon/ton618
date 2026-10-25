package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
	"ton618/core/internal/features/notes"
	"ton618/core/internal/processor"
	"ton618/core/internal/query"
	"ton618/core/internal/search"
)

// ── search_notes ─────────────────────────────────────────────────────────

type searchInput struct {
	Query string `json:"query" jsonschema:"texto livre e/ou filtros da busca do app (ex.: 'gatos', 'tags:livros', 'tipo:mapa_mental projeto')"`
	Limit int    `json:"limit,omitempty" jsonschema:"máximo de NOTAS no resultado (padrão 10, máximo 50)"`
}

type searchNote struct {
	File      string   `json:"file"`
	Title     string   `json:"title"`
	Type      string   `json:"type,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Sections  []string `json:"sections,omitempty"`
	Excerpt   string   `json:"excerpt,omitempty"`
	Score     float64  `json:"score,omitempty"`
	Fragments int      `json:"fragments"`
}

type searchOutput struct {
	Query      string       `json:"query"`
	TotalHits  int          `json:"total_hits"` // trechos indexados que casaram, não notas
	NotesFound int          `json:"notes_found"`
	Notes      []searchNote `json:"notes"`
}

func (s *Server) searchNotes(ctx context.Context, _ *sdk.CallToolRequest, in searchInput) (*sdk.CallToolResult, any, error) {
	limit := clampInt(in.Limit, 10, 1, 50)
	queryText := strings.TrimSpace(in.Query)

	// size é o número de FRAGMENTOS: buscamos alguns a mais para conseguir
	// agrupar por nota sem devolver menos notas que o pedido.
	results, err := search.Search(ctx, s.store, queryText, 0, min(limit*3, 100),
		s.store.GetBacklinkCount, s.store.GetSynapticWeight)
	if err != nil {
		return toolError("busca falhou: %v", err), nil, nil
	}

	out := searchOutput{Query: queryText, TotalHits: results.Total, Notes: []searchNote{}}
	index := make(map[string]int)
	for _, hit := range results.Hits {
		file := hit.Doc.Arquivo
		i, ok := index[file]
		if !ok {
			if len(out.Notes) >= limit {
				continue
			}
			i = len(out.Notes)
			index[file] = i

			tags := db.TagsToSlice(hit.Doc.Tags)
			tipo := hit.Doc.NoteType
			if tipo == "" {
				tipo = domain.FrontmatterTipo(domain.DetectNoteType(tags, file))
			}
			out.Notes = append(out.Notes, searchNote{
				File:  file,
				Title: domain.DisplayName(file),
				Type:  tipo,
				Tags:  displayTags(tags),
			})
		}

		n := &out.Notes[i]
		n.Fragments++
		if hit.FinalScore > n.Score {
			n.Score = hit.FinalScore
		}
		if hit.Doc.Secao != "" {
			n.Sections = append(n.Sections, hit.Doc.Secao)
		}
		if n.Excerpt == "" && strings.TrimSpace(hit.Doc.Texto) != "" {
			n.Excerpt = excerpt(hit.Doc.Texto, 280)
		}
	}

	for i := range out.Notes {
		out.Notes[i].Sections = dedupStrings(out.Notes[i].Sections)
		out.Notes[i].Score = float64(int(out.Notes[i].Score*1000+0.5)) / 1000
	}
	out.NotesFound = len(out.Notes)
	if out.Notes == nil {
		out.Notes = []searchNote{}
	}
	return reply(out), nil, nil
}

// ── get_note ─────────────────────────────────────────────────────────────

type getNoteInput struct {
	File     string `json:"file" jsonschema:"nome/caminho da nota (ex.: 'gambiarra-clone-25' ou 'notes/gambiarra-clone-25.md')"`
	Offset   int    `json:"offset,omitempty" jsonschema:"deslocamento em caracteres para paginar notas grandes (padrão 0)"`
	MaxChars int    `json:"max_chars,omitempty" jsonschema:"máximo de caracteres a devolver (padrão 50000, máximo 200000)"`
}

func (s *Server) getNote(_ context.Context, _ *sdk.CallToolRequest, in getNoteInput) (*sdk.CallToolResult, any, error) {
	name, ok := normalizeNoteName(in.File)
	if !ok {
		return toolError("arquivo inválido: %q (use um nome como %q)", in.File, "minha-nota"), nil, nil
	}
	if !s.store.NoteExists(name) {
		return toolError("nota %q não encontrada; use search_notes ou list_notes para descobrir o nome", name), nil, nil
	}

	content, err := s.store.GetNote(name)
	if err != nil {
		return toolError("não foi possível ler %q: %v", name, err), nil, nil
	}
	tags, _ := s.store.GetFileTags(name)
	mtime, _ := s.store.GetNoteMtime(name)

	runes := []rune(content)
	total := len(runes)
	offset := clampInt(in.Offset, 0, 0, total)
	maxChars := clampInt(in.MaxChars, 50000, 1, 200000)
	end := min(offset+maxChars, total)
	part := string(runes[offset:end])

	var b strings.Builder
	fmt.Fprintf(&b, "Nota: %s\n", name)
	fmt.Fprintf(&b, "Título: %s\n", domain.DisplayName(name))
	fmt.Fprintf(&b, "Tipo: %s\n", noteType(tags, content, name))
	if userTags := displayTags(tags); len(userTags) > 0 {
		fmt.Fprintf(&b, "Tags: %s\n", strings.Join(userTags, ", "))
	} else {
		b.WriteString("Tags: (nenhuma)\n")
	}
	if mtime != "" {
		fmt.Fprintf(&b, "Modificada: %s\n", mtime)
	}
	fmt.Fprintf(&b, "Caracteres: %d\n", total)
	if end < total || offset > 0 {
		fmt.Fprintf(&b, "Trecho: %d–%d de %d (use offset/max_chars para paginar)\n", offset, end, total)
	}
	b.WriteString("\n---\n\n")
	b.WriteString(part)

	return textReply(b.String()), nil, nil
}

// ── list_notes ───────────────────────────────────────────────────────────

type listInput struct {
	Tipo    string `json:"tipo,omitempty" jsonschema:"filtra pelo tipo do frontmatter: nota, desenho, mapa_mental, tabulator, semanal, pdf, anexo, artigo, captura"`
	Tag     string `json:"tag,omitempty" jsonschema:"filtra por uma tag exata (sem #)"`
	Ordenar string `json:"ordenar,omitempty" jsonschema:"recentes (padrão), antigas ou nome"`
	Limit   int    `json:"limit,omitempty" jsonschema:"máximo de notas (padrão 50, máximo 200)"`
}

type noteSummary struct {
	File  string   `json:"file"`
	Title string   `json:"title"`
	Type  string   `json:"type,omitempty"`
	Tags  []string `json:"tags,omitempty"`
	Mtime string   `json:"mtime,omitempty"`
}

type listOutput struct {
	Tipo     string        `json:"tipo,omitempty"`
	Tag      string        `json:"tag,omitempty"`
	Total    int           `json:"total"` // notas que casaram o filtro
	Returned int           `json:"returned"`
	Notes    []noteSummary `json:"notes"`
}

func (s *Server) listNotes(_ context.Context, _ *sdk.CallToolRequest, in listInput) (*sdk.CallToolResult, any, error) {
	items, err := s.notes.GetMany()
	if err != nil {
		return toolError("não foi possível listar as notas: %v", err), nil, nil
	}

	tipoFiltro := domain.NormalizeNoteTipo(in.Tipo)
	tagFiltro := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(in.Tag), "#"))

	all := make([]noteSummary, 0, len(items))
	for _, item := range items {
		tipo := domain.NormalizeNoteTipo(string(item.Type))
		if tipoFiltro != "" && tipo != tipoFiltro {
			continue
		}
		if tagFiltro != "" && !hasTag(item.Tags, tagFiltro) {
			continue
		}
		all = append(all, noteSummary{
			File:  item.Arquivo,
			Title: domain.DisplayName(item.Arquivo),
			Type:  tipo,
			Tags:  item.Tags,
			Mtime: item.Mtime,
		})
	}

	switch strings.ToLower(strings.TrimSpace(in.Ordenar)) {
	case "antigas":
		sort.SliceStable(all, func(i, j int) bool { return all[i].Mtime < all[j].Mtime })
	case "nome":
		sort.SliceStable(all, func(i, j int) bool {
			return strings.ToLower(all[i].Title) < strings.ToLower(all[j].Title)
		})
	default: // recentes
		sort.SliceStable(all, func(i, j int) bool { return all[i].Mtime > all[j].Mtime })
	}

	total := len(all)
	limit := clampInt(in.Limit, 50, 1, 200)
	if len(all) > limit {
		all = all[:limit]
	}

	return reply(listOutput{
		Tipo:     tipoFiltro,
		Tag:      tagFiltro,
		Total:    total,
		Returned: len(all),
		Notes:    all,
	}), nil, nil
}

// ── query_notes ──────────────────────────────────────────────────────────

type queryInput struct {
	Consulta string `json:"consulta" jsonschema:"YAML de um bloco consulta, ex.: 'tipo: notas\\ntexto: gatos\\nlimite: 10'"`
	Nota     string `json:"nota,omitempty" jsonschema:"opcional: nota raiz para tipo: hierarquia (ex.: 'projetos')"`
}

type queryOutput struct {
	Total     int              `json:"total"` // linhas após o filtro onde:, antes do limite
	Returned  int              `json:"returned"`
	Truncated bool             `json:"truncated,omitempty"`
	Capped    bool             `json:"capped,omitempty"`
	Error     string           `json:"error,omitempty"`
	Rows      []map[string]any `json:"rows"`
}

func (s *Server) queryNotes(ctx context.Context, _ *sdk.CallToolRequest, in queryInput) (*sdk.CallToolResult, any, error) {
	spec, err := processor.ParseQuery(in.Consulta)
	if err != nil {
		return toolError("consulta inválida: %v", err), nil, nil
	}

	nota := ""
	if strings.TrimSpace(in.Nota) != "" {
		if name, ok := normalizeNoteName(in.Nota); ok {
			nota = name
		}
	}

	res := query.Run(ctx, s.engine, spec, notes.NewQueryReader(s.store), nota, time.Now())

	out := queryOutput{
		Total:     res.Total,
		Returned:  len(res.Rows),
		Truncated: res.Truncado,
		Capped:    res.Capado,
		Error:     res.Erro,
		Rows:      rowsToMaps(res.Rows),
	}
	if res.Erro != "" {
		result := reply(out)
		result.IsError = true
		return result, nil, nil
	}
	return reply(out), nil, nil
}

// rowsToMaps converte as linhas do motor em JSON. Campos vazios/zero são
// omitidos — o modelo não precisa distinguir "campo ausente" de "valor zero".
func rowsToMaps(rows []query.Row) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		m := map[string]any{}
		put := func(k string, v any) { m[k] = v }

		if r.Caminho != "" {
			put("arquivo", r.Caminho)
		}
		if r.Titulo != "" {
			put("titulo", r.Titulo)
		}
		if r.Pasta != "" {
			put("pasta", r.Pasta)
		}
		if r.Classe != "" {
			put("classe", r.Classe)
		}
		if len(r.Tags) > 0 {
			put("tags", r.Tags)
		}
		if !r.Mtime.IsZero() {
			put("mtime", r.Mtime.Format(time.RFC3339))
		}
		if r.Pai != "" {
			put("pai", r.Pai)
			put("pai_resolvido", r.PaiResolvido)
		}
		if len(r.Filhas) > 0 {
			put("filhas", r.Filhas)
		}
		if r.CitadaPor > 0 {
			put("citada_por", r.CitadaPor)
		}
		if r.TarefasAbertas > 0 {
			put("tarefas_abertas", r.TarefasAbertas)
		}
		if r.Popularidade > 0 {
			put("popularidade", r.Popularidade)
		}
		if r.Nivel > 0 {
			put("nivel", r.Nivel)
		}
		if r.Texto != "" {
			put("texto", r.Texto)
		}
		if r.Marcador != "" {
			put("marcador", r.Marcador)
		}
		if r.Estado != "" {
			put("estado", r.Estado)
		}
		if r.Secao != "" {
			put("secao", r.Secao)
		}
		if r.Linha > 0 {
			put("linha", r.Linha)
		}
		if !r.Criado.IsZero() {
			put("criado", r.Criado.Format(time.RFC3339))
		}
		if r.Snippet != "" {
			put("snippet", r.Snippet)
		}
		out = append(out, m)
	}
	return out
}

// ── backlinks ────────────────────────────────────────────────────────────

type backlinksInput struct {
	File string `json:"file" jsonschema:"nome/caminho da nota"`
}

type backlinksOutput struct {
	File      string   `json:"file"`
	Backlinks []string `json:"backlinks"` // notas que apontam PARA esta
	Links     []string `json:"links"`     // notas que esta aponta
}

func (s *Server) backlinks(_ context.Context, _ *sdk.CallToolRequest, in backlinksInput) (*sdk.CallToolResult, any, error) {
	name, ok := normalizeNoteName(in.File)
	if !ok {
		return toolError("arquivo inválido: %q", in.File), nil, nil
	}
	if !s.store.NoteExists(name) {
		return toolError("nota %q não encontrada", name), nil, nil
	}

	result, err := s.notes.GetBacklinks(name)
	if err != nil {
		return toolError("não foi possível ler os backlinks: %v", err), nil, nil
	}
	outgoing, err := s.store.GetLinks(name)
	if err != nil {
		return toolError("não foi possível ler os links: %v", err), nil, nil
	}

	return reply(backlinksOutput{
		File:      name,
		Backlinks: nonNil(result.Level1),
		Links:     nonNil(outgoing),
	}), nil, nil
}

// ── registro ─────────────────────────────────────────────────────────────

func (s *Server) registerTools(srv *sdk.Server) {
	sdk.AddTool(srv, &sdk.Tool{
		Name:  "search_notes",
		Title: "Buscar notas",
		Description: "Busca rankeada no acervo (FTS5 + sinônimos + acentos). Aceita texto livre e os " +
			"filtros do app: tags:tag, tipo:mapa_mental, tipo:desenho, etc. Devolve as notas " +
			"agrupadas, com trecho, seções e score. total_hits conta trechos indexados, não notas.",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, s.searchNotes)

	sdk.AddTool(srv, &sdk.Tool{
		Name:  "get_note",
		Title: "Ler nota",
		Description: "Devolve o conteúdo markdown completo de uma nota, com título, tipo, tags e data " +
			"de modificação. Notas grandes podem ser paginadas com offset/max_chars. " +
			"Use para ler desenhos, mapas mentais e tabelas Tabulator, cujo corpo não está no FTS.",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, s.getNote)

	sdk.AddTool(srv, &sdk.Tool{
		Name:  "list_notes",
		Title: "Listar notas",
		Description: "Lista metadados (arquivo, título, tipo, tags, mtime) de todas as notas, " +
			"opcionalmente filtradas por tipo e/ou tag e ordenadas por data ou nome.",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, s.listNotes)

	sdk.AddTool(srv, &sdk.Tool{
		Name:  "query_notes",
		Title: "Consultar acervo",
		Description: "Executa a linguagem de consulta do TON-618 (mesmo motor dos blocos ```consulta). " +
			"Recebe o YAML com tipo: notas|tarefas|hierarquia e filtros texto:, tags:, sem-tags:, " +
			"pasta:, onde: (CEL), ordenar:, limite:, colunas:. Ex.: \"tipo: notas\\ntexto: gatos\\nlimite: 5\".",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, s.queryNotes)

	sdk.AddTool(srv, &sdk.Tool{
		Name:  "backlinks",
		Title: "Links da nota",
		Description: "Devolve os backlinks (notas que apontam para esta) e os links de saída " +
			"(notas citadas no conteúdo).",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, s.backlinks)
}

// ── utilidades ───────────────────────────────────────────────────────────

func clampInt(v, def, lo, hi int) int {
	if v == 0 {
		return def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if strings.EqualFold(strings.TrimPrefix(t, "#"), want) {
			return true
		}
	}
	return false
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
