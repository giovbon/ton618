package query

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cel.dev/cel-go/common/types"

	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
	"ton618/core/internal/processor"
)

// Reader é a ÚNICA porta do motor para o acervo. Todas as operações são de
// leitura — não existe (nem pode existir) um caminho de escrita a partir deste
// pacote (DECISIONS §6.29).
type Reader interface {
	Files() ([]db.FileModTag, error)
	Contents() (map[string]string, error)
	Links() (map[string][]string, error)
	Popularity() (map[string]int, error)
	LastInteracted() (map[string]string, error)
	Todos(typeFilter map[string]bool, status string) ([]processor.TodoItem, error)
	SearchText(term string, limit int) ([]db.FTSResult, error)
	Hierarchy() (Hierarchy, error)
}

// Hierarchy é a fotografia da árvore `pai:` usada pelo motor.
type Hierarchy struct {
	Pai       map[string]string   // arquivo -> referência escrita em `pai:` ("" se não tem)
	Resolvido map[string]string   // arquivo -> caminho da nota-mãe resolvida
	Filhas    map[string][]string // arquivo (mãe) -> caminhos das filhas
}

// Row é uma linha do resultado (uma nota, uma tarefa ou um item da árvore).
type Row struct {
	// Campos de `n` (nota/arquivo)
	Caminho        string
	Titulo         string
	Pasta          string
	Classe         string
	Tags           []string
	Mtime          time.Time
	Pai            string
	PaiResolvido   bool
	Filhas         []string
	CitadaPor      int
	TarefasAbertas int
	Popularidade   int
	Interacao      time.Time
	Nivel          int // profundidade na hierarquia (0 = raiz listada)

	// Campos de `t` (tarefa)
	Texto    string
	Marcador string
	Estado   string
	Secao    string
	Linha    int
	Criado   time.Time

	Snippet string
}

// Result é o que o painel de Consultas desenha. Nada aqui é persistido.
type Result struct {
	Spec             processor.QuerySpec
	Rows             []Row
	Total            int // linhas depois do filtro (`onde:`) e antes do limite
	Candidatos       int // linhas que o pré-filtro entregou ao CEL
	CandidatosTotais int // linhas antes do teto de MaxCandidatos
	Capado           bool
	Truncado         bool
	Erro             string
	Duracao          time.Duration
}

// Run executa um bloco validado. Nunca devolve erro Go: qualquer problema vira
// Result.Erro, porque a mensagem é CONTEÚDO do cartão (o documento fala em
// "erro dentro do bloco", nunca 500 — DECISIONS §5.x/§6.29).
func Run(ctx context.Context, eng *Engine, spec processor.QuerySpec, r Reader, nota string, agora time.Time) Result {
	inicio := time.Now()
	res := Result{Spec: spec}

	dados, err := carregar(r, spec)
	if err != nil {
		res.Erro = "bloco: não foi possível ler o acervo agora"
		return res
	}

	var rows []Row
	var capado bool
	var candidatosTotais int
	switch spec.Tipo {
	case processor.QueryTipoNotas:
		rows, capado, candidatosTotais, res.Erro = linhasDeNotas(spec, dados)
	case processor.QueryTipoTarefas:
		rows, res.Erro = linhasDeTarefas(spec, dados)
	case processor.QueryTipoHierarquia:
		rows, res.Erro = linhasDeHierarquia(spec, dados, nota)
	default:
		res.Erro = fmt.Sprintf("tipo: %q não é suportado", spec.Tipo)
	}
	res.Capado = capado
	res.CandidatosTotais = candidatosTotais
	if res.Erro != "" {
		return res
	}
	res.Candidatos = len(rows)

	if spec.Onde != "" {
		filtradas, msg := filtrar(ctx, eng, spec, rows, agora)
		if msg != "" {
			res.Erro = msg
			return res
		}
		rows = filtradas
	}

	res.Total = len(rows)
	if spec.Tipo != processor.QueryTipoHierarquia || spec.Ordenar != "" {
		ordenar(rows, spec)
	}

	limite := spec.Limite
	if limite <= 0 {
		limite = processor.DefaultQueryLimit
	}
	res.Truncado = len(rows) > limite
	if res.Truncado {
		rows = rows[:limite]
	}
	res.Rows = rows
	res.Duracao = time.Since(inicio)
	return res
}

// ---------------------------------------------------------------------------
// Carga (tudo de leitura, em massa, uma consulta por tabela)
// ---------------------------------------------------------------------------

type dados struct {
	files      []db.FileModTag
	contents   map[string]string
	links      map[string][]string
	citacoes   map[string]int
	pop        map[string]int
	last       map[string]string
	hierarchy  Hierarchy
	todosAbert map[string]int
	todos      []processor.TodoItem // só para tipo: tarefas
	snippets   map[string]string
	ftsOrdem   map[string]int
}

func carregar(r Reader, spec processor.QuerySpec) (dados, error) {
	var d dados

	// `tipo: tarefas` não precisa do acervo de notas — só da tabela `todos`.
	if spec.Tipo == processor.QueryTipoTarefas {
		todos, err := r.Todos(nil, "")
		if err != nil {
			return d, err
		}
		d.todos = todos
		return d, nil
	}

	var err error
	if d.files, err = r.Files(); err != nil {
		return d, err
	}
	if d.contents, err = r.Contents(); err != nil {
		d.contents = map[string]string{}
	}
	if d.links, err = r.Links(); err != nil {
		d.links = map[string][]string{}
	}
	if d.pop, err = r.Popularity(); err != nil {
		d.pop = map[string]int{}
	}
	if d.last, err = r.LastInteracted(); err != nil {
		d.last = map[string]string{}
	}
	if d.hierarchy, err = r.Hierarchy(); err != nil {
		d.hierarchy = Hierarchy{}
	}

	// Invertido uma vez só: evita O(linhas × links) ao montar cada linha.
	d.citacoes = map[string]int{}
	for _, tos := range d.links {
		for _, to := range tos {
			d.citacoes[to]++
		}
	}

	pending, err := r.Todos(nil, "pending")
	if err != nil {
		pending = nil
	}
	d.todosAbert = map[string]int{}
	for _, t := range pending {
		d.todosAbert[t.File]++
	}

	if spec.Texto != "" {
		resultados, err := r.SearchText(spec.Texto, processor.MaxQueryLimit*4)
		if err != nil {
			return d, err
		}
		d.snippets = map[string]string{}
		d.ftsOrdem = map[string]int{}
		for i, fr := range resultados {
			if _, ok := d.snippets[fr.Arquivo]; ok {
				continue
			}
			d.snippets[fr.Arquivo] = fr.Snippet
			d.ftsOrdem[fr.Arquivo] = i
		}
	}

	return d, nil
}

// ---------------------------------------------------------------------------
// tipo: notas
// ---------------------------------------------------------------------------

func linhasDeNotas(spec processor.QuerySpec, d dados) ([]Row, bool, int, string) {
	rows := make([]Row, 0, len(d.files))

	for _, f := range d.files {
		tags := splitTags(f.Tags)
		if !temTodas(tags, spec.Tags) || temAlguma(tags, spec.SemTags) {
			continue
		}
		if spec.Pasta != "" && pastaOf(f.Arquivo) != spec.Pasta {
			continue
		}
		snippet := ""
		if spec.Texto != "" {
			s, ok := d.snippets[f.Arquivo]
			if !ok {
				continue
			}
			snippet = s
		}
		rows = append(rows, buildNotaRow(f, d, snippet))
	}

	// Quando há busca textual, o teto de candidatos preserva a ordem do FTS5
	// (ranqueado). Sem ela, mantém as mais recentes — e o cartão avisa.
	if len(rows) > MaxCandidatos {
		total := len(rows)
		if spec.Texto != "" {
			sort.SliceStable(rows, func(i, j int) bool {
				return d.ftsOrdem[rows[i].Caminho] < d.ftsOrdem[rows[j].Caminho]
			})
		} else {
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].Mtime.After(rows[j].Mtime) })
		}
		return rows[:MaxCandidatos], true, total, ""
	}
	return rows, false, len(rows), ""
}

func buildNotaRow(f db.FileModTag, d dados, snippet string) Row {
	tags := splitTags(f.Tags)
	conteudo := d.contents[f.Arquivo]

	row := Row{
		Caminho:        f.Arquivo,
		Titulo:         domain.DisplayName(f.Arquivo),
		Pasta:          pastaOf(f.Arquivo),
		Classe:         string(domain.DetectNoteTypeFromContent(tags, conteudo, f.Arquivo)),
		Tags:           tags,
		Mtime:          parseTempo(f.Mtime),
		Pai:            d.hierarchy.Pai[f.Arquivo],
		CitadaPor:      d.citacoes[f.Arquivo],
		TarefasAbertas: d.todosAbert[f.Arquivo],
		Popularidade:   d.pop[f.Arquivo],
		Interacao:      parseTempo(d.last[f.Arquivo]),
		Snippet:        snippet,
	}
	row.PaiResolvido = d.hierarchy.Resolvido[f.Arquivo] != ""
	for _, filha := range d.hierarchy.Filhas[f.Arquivo] {
		row.Filhas = append(row.Filhas, domain.DisplayName(filha))
	}
	return row
}

// ---------------------------------------------------------------------------
// tipo: tarefas
// ---------------------------------------------------------------------------

func linhasDeTarefas(spec processor.QuerySpec, d dados) ([]Row, string) {
	todos := d.todos
	marcadores := map[string]bool{}
	for _, m := range spec.Marcador {
		marcadores[strings.ToUpper(m)] = true
	}

	rows := make([]Row, 0, len(todos))
	for _, t := range todos {
		if spec.Estado != "" && spec.Estado != processor.QueryEstadoTodas {
			pendente := t.Status == "pending"
			if spec.Estado == processor.QueryEstadoPendente && !pendente {
				continue
			}
			if spec.Estado == processor.QueryEstadoConcluida && pendente {
				continue
			}
		}
		if len(marcadores) > 0 && !marcadores[strings.ToUpper(t.Type)] {
			continue
		}
		estado := processor.QueryEstadoPendente
		if t.Status != "pending" {
			estado = processor.QueryEstadoConcluida
		}
		rows = append(rows, Row{
			Caminho:  t.File,
			Titulo:   domain.DisplayName(t.File),
			Texto:    t.Text,
			Marcador: t.Type,
			Estado:   estado,
			Secao:    t.Section,
			Linha:    t.Line,
			Criado:   t.Created,
		})
	}
	if len(rows) > MaxCandidatos {
		return rows[:MaxCandidatos], ""
	}
	return rows, ""
}

// ---------------------------------------------------------------------------
// tipo: hierarquia
// ---------------------------------------------------------------------------

func linhasDeHierarquia(spec processor.QuerySpec, d dados, nota string) ([]Row, string) {
	raiz := nota
	if spec.De != "" {
		raiz = ""
		for _, f := range d.files {
			if strings.EqualFold(f.Arquivo, spec.De) || strings.EqualFold(domain.DisplayName(f.Arquivo), spec.De) {
				raiz = f.Arquivo
				break
			}
		}
		if raiz == "" {
			return nil, fmt.Sprintf("de: nota não encontrada: %q", spec.De)
		}
	}

	porCaminho := map[string]db.FileModTag{}
	for _, f := range d.files {
		porCaminho[f.Arquivo] = f
	}

	const profundidadeMax = 3
	var rows []Row
	var visitar func(pai string, nivel int)
	visitar = func(pai string, nivel int) {
		if nivel > profundidadeMax {
			return
		}
		filhas := append([]string(nil), d.hierarchy.Filhas[pai]...)
		sort.Strings(filhas)
		for _, filha := range filhas {
			f, ok := porCaminho[filha]
			if !ok {
				continue
			}
			row := buildNotaRow(f, d, "")
			row.Nivel = nivel
			rows = append(rows, row)
			visitar(filha, nivel+1)
		}
	}
	visitar(raiz, 1)

	if len(rows) > MaxCandidatos {
		return rows[:MaxCandidatos], ""
	}
	return rows, ""
}

// ---------------------------------------------------------------------------
// Filtro CEL
// ---------------------------------------------------------------------------

func filtrar(ctx context.Context, eng *Engine, spec processor.QuerySpec, rows []Row, agora time.Time) ([]Row, string) {
	prg, msg := eng.Prepare(spec.Onde, agora)
	if msg != "" {
		return nil, msg
	}

	evalCtx, cancel := context.WithTimeout(ctx, TempoMaxBloco)
	defer cancel()

	out := make([]Row, 0, len(rows))
	for _, row := range rows {
		ok, msg := eng.Avalia(evalCtx, spec.Onde, prg, variaveis(row, spec.Tipo, agora))
		if msg != "" {
			return nil, msg
		}
		if ok {
			out = append(out, row)
		}
	}
	return out, ""
}

// variaveis monta a ativação do CEL. Listas viram listas de string nativas e
// datas viram timestamps (ou null, quando nunca aconteceram) — é o que
// possibilita `... in n.tags`, `n.tags.exists(...)` e `n.interacao == null`.
func variaveis(row Row, tipo string, agora time.Time) map[string]any {
	vars := map[string]any{"agora": types.Timestamp{Time: agora}}
	if tipo == processor.QueryTipoTarefas {
		vars["t"] = map[string]any{
			"texto":       row.Texto,
			"marcador":    row.Marcador,
			"estado":      row.Estado,
			"arquivo":     row.Caminho,
			"nota_titulo": row.Titulo,
			"secao":       row.Secao,
			"linha":       int64(row.Linha),
			"criado":      tempoValor(row.Criado),
		}
		return vars
	}

	vars["n"] = map[string]any{
		"caminho":         row.Caminho,
		"titulo":          row.Titulo,
		"pasta":           row.Pasta,
		"classe":          row.Classe,
		"tags":            listaValor(row.Tags),
		"mtime":           tempoValor(row.Mtime),
		"pai":             row.Pai,
		"pai_resolvido":   row.PaiResolvido,
		"filhas":          listaValor(row.Filhas),
		"citada_por":      int64(row.CitadaPor),
		"tarefas_abertas": int64(row.TarefasAbertas),
		"popularidade":    int64(row.Popularidade),
		"interacao":       tempoValor(row.Interacao),
	}
	return vars
}

func listaValor(v []string) any {
	return types.NewStringList(types.DefaultTypeAdapter, v)
}

func tempoValor(t time.Time) any {
	if t.IsZero() {
		return types.NullValue
	}
	return types.Timestamp{Time: t}
}

// ---------------------------------------------------------------------------
// Ordenação
// ---------------------------------------------------------------------------

func ordenar(rows []Row, spec processor.QuerySpec) {
	campo := spec.Ordenar
	dir := spec.Ordem
	if campo == "" {
		campo = campoPadrao(spec.Tipo)
	}
	if dir == "" {
		dir = direcaoNatural(campo)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		c := compara(rows[i], rows[j], campo)
		if c == 0 {
			return rows[i].Caminho < rows[j].Caminho
		}
		if dir == "desc" {
			return c > 0
		}
		return c < 0
	})
}

func campoPadrao(tipo string) string {
	if tipo == processor.QueryTipoTarefas {
		return "arquivo"
	}
	return "mtime"
}

// direcaoNatural: texto ordena de A a Z, contagens/datas do maior para o menor.
func direcaoNatural(campo string) string {
	switch campo {
	case "titulo", "caminho", "pasta", "classe", "arquivo", "texto", "marcador", "estado", "secao", "pai":
		return "asc"
	default:
		return "desc"
	}
}

func compara(a, b Row, campo string) int {
	switch campo {
	case "titulo", "nota_titulo":
		return strings.Compare(strings.ToLower(a.Titulo), strings.ToLower(b.Titulo))
	case "caminho", "arquivo":
		return strings.Compare(a.Caminho, b.Caminho)
	case "pasta":
		return strings.Compare(a.Pasta, b.Pasta)
	case "classe":
		return strings.Compare(a.Classe, b.Classe)
	case "texto":
		return strings.Compare(a.Texto, b.Texto)
	case "marcador":
		return strings.Compare(a.Marcador, b.Marcador)
	case "estado":
		return strings.Compare(a.Estado, b.Estado)
	case "secao":
		return strings.Compare(a.Secao, b.Secao)
	case "pai":
		return strings.Compare(a.Pai, b.Pai)
	case "mtime":
		return a.Mtime.Compare(b.Mtime)
	case "interacao":
		return a.Interacao.Compare(b.Interacao)
	case "criado":
		return a.Criado.Compare(b.Criado)
	case "citada_por":
		return a.CitadaPor - b.CitadaPor
	case "tarefas_abertas":
		return a.TarefasAbertas - b.TarefasAbertas
	case "popularidade":
		return a.Popularidade - b.Popularidade
	case "linha":
		return a.Linha - b.Linha
	case "tags":
		return len(a.Tags) - len(b.Tags)
	case "filhas":
		return len(a.Filhas) - len(b.Filhas)
	default:
		return 0
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// pastaOf devolve a raiz do acervo de um caminho (`notes/x/y.md` → `notes`).
func pastaOf(arquivo string) string {
	if i := strings.Index(arquivo, "/"); i > 0 {
		return strings.ToLower(arquivo[:i])
	}
	return ""
}

func splitTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	partes := strings.Split(raw, ",")
	out := make([]string, 0, len(partes))
	for _, p := range partes {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func temTodas(tags, exigidas []string) bool {
	for _, e := range exigidas {
		if !containsFold(tags, e) {
			return false
		}
	}
	return true
}

func temAlguma(tags, proibidas []string) bool {
	for _, p := range proibidas {
		if containsFold(tags, p) {
			return true
		}
	}
	return false
}

func containsFold(list []string, v string) bool {
	for _, item := range list {
		if strings.EqualFold(item, v) {
			return true
		}
	}
	return false
}

// parseTempo aceita RFC3339 (formato usado no banco). Valor vazio ou inválido
// vira tempo zero, que o CEL enxerga como `null`.
func parseTempo(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
