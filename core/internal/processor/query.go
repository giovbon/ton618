package processor

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Linguagem de consulta das notas — bloco ```consulta
//
// Ver DECISIONS §6.29 (decisão) e docs/linguagem-de-consulta.md (sintaxe).
//
// Este arquivo é o PARSEADOR PURO: recebe o texto do bloco e devolve um Spec
// validado, ou uma mensagem de erro em português. Nada aqui toca banco, disco
// ou rede — quem executa é internal/query, com um leitor somente-leitura.
//
// ⚠️ As listas abaixo (chaves do bloco, campos de n/t, campos ordenáveis) são o
// CONTRATO público da linguagem: elas alimentam a validação E o teste de
// paridade com a documentação (query_parity_test.go). Ao adicionar/renomear
// qualquer item, atualize docs/linguagem-de-consulta.md no mesmo commit.
// ---------------------------------------------------------------------------

const (
	// QueryFenceInfo é a info-string da cerca que marca um bloco de consulta.
	QueryFenceInfo = "consulta"

	// MaxQueryBlocks é o teto de blocos executados por nota (§13 do doc).
	MaxQueryBlocks = 10

	// DefaultQueryLimit / MaxQueryLimit são os limites de linhas do resultado.
	DefaultQueryLimit = 20
	MaxQueryLimit     = 200

	// MaxQueryOndeBytes evita expressão gigante (a cerca já limita, mas YAML
	// aceita multiline).
	MaxQueryOndeBytes = 2000
)

// Tipos de consulta aceitos em `tipo:`.
const (
	QueryTipoNotas      = "notas"
	QueryTipoTarefas    = "tarefas"
	QueryTipoHierarquia = "hierarquia"
	// QueryTipoAgenda é reconhecido só para dar uma mensagem melhor (v2).
	QueryTipoAgenda = "agenda"
)

// Apresentações aceitas em `mostrar:`.
const (
	QueryMostrarLista    = "lista"
	QueryMostrarTabela   = "tabela"
	QueryMostrarContagem = "contagem"
	QueryMostrarCartoes  = "cartoes"
	QueryMostrarArvore   = "arvore"
)

// Estados de tarefa aceitos em `estado:`.
const (
	QueryEstadoPendente  = "pendente"
	QueryEstadoConcluida = "concluida"
	QueryEstadoTodas     = "todas"
)

// QueryPastas são as raízes do acervo aceitas em `pasta:`.
var QueryPastas = []string{"notes", "pdfs", "epubs", "attachments", "images", "archives"}

// queryKeys é a ordem canônica das chaves do bloco (usada nas mensagens de erro).
var queryKeys = []string{
	"colunas", "de", "estado", "janela", "limite", "marcador", "mostrar",
	"onde", "ordenar", "pasta", "sem-tags", "tags", "texto", "tipo",
}

// queryNoteFields são os campos de `n` (nota/arquivo) disponíveis no `onde:`.
var queryNoteFields = []string{
	"caminho", "citada_por", "classe", "filhas", "interacao", "mtime", "pai",
	"pai_resolvido", "pasta", "popularidade", "tags", "tarefas_abertas", "titulo",
}

// queryTaskFields são os campos de `t` (tarefa) disponíveis no `onde:`.
var queryTaskFields = []string{
	"arquivo", "criado", "estado", "linha", "marcador", "nota_titulo", "secao", "texto",
}

// queryColunas são as colunas aceitas em `colunas:`. É DERIVADA da união dos
// campos de `n` e `t` mais as calculadas — assim não existe uma terceira lista
// para sair de sincronia com as outras duas (uma coluna recusada que o próprio
// motor preenche é o tipo de inconsistência que passa despercebida).
var queryColunas = buildQueryColunas()

// queryColunasCalculadas são colunas que não existem no banco: o motor deriva
// na hora (dias desde..., tamanho de lista).
var queryColunasCalculadas = []string{"interacao_ha_dias", "modif_ha_dias", "n_tags"}

func buildQueryColunas() []string {
	set := map[string]bool{}
	for _, f := range append(append([]string{}, queryNoteFields...), queryTaskFields...) {
		set[f] = true
	}
	for _, c := range queryColunasCalculadas {
		set[c] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// QueryKeys devolve a lista canônica de chaves aceitas no bloco.
func QueryKeys() []string { return append([]string(nil), queryKeys...) }

// QueryNoteFields devolve os campos de `n` (nota/arquivo) disponíveis no `onde:`.
func QueryNoteFields() []string { return append([]string(nil), queryNoteFields...) }

// QueryTaskFields devolve os campos de `t` (tarefa) disponíveis no `onde:`.
func QueryTaskFields() []string { return append([]string(nil), queryTaskFields...) }

// QueryColunas devolve as colunas aceitas em `colunas:`.
func QueryColunas() []string { return append([]string(nil), queryColunas...) }

// QueryTipoFields devolve os campos de `n`/`t` conforme o tipo da consulta.
func QueryTipoFields(tipo string) []string {
	if tipo == QueryTipoTarefas {
		return append([]string(nil), queryTaskFields...)
	}
	return append([]string(nil), queryNoteFields...)
}

// QueryOrderFields devolve os campos aceitos em `ordenar:` para o tipo.
func QueryOrderFields(tipo string) []string {
	if tipo == QueryTipoTarefas {
		return []string{"arquivo", "criado", "estado", "linha", "marcador", "nota_titulo", "secao", "texto"}
	}
	return []string{
		"caminho", "citada_por", "classe", "filhas", "interacao", "mtime", "pai",
		"pasta", "popularidade", "tags", "tarefas_abertas", "titulo",
	}
}

// QuerySpec é uma consulta já validada, pronta para o motor executar.
type QuerySpec struct {
	Tipo    string   // notas | tarefas | hierarquia
	Onde    string   // expressão CEL ("" = sem filtro)
	Ordenar string   // campo de ordenação ("" = padrão do tipo)
	Ordem   string   // "asc" | "desc" | "" (direção natural do campo)
	Limite  int      // 1..200
	Mostrar string   // lista | tabela | contagem | cartoes | arvore
	Colunas []string // só com Mostrar == tabela

	// Atalhos de índice
	Tags     []string
	SemTags  []string
	Texto    string
	Pasta    string
	Marcador []string
	Estado   string
	De       string
}

// QueryBlock é um bloco ```consulta encontrado no texto da nota.
type QueryBlock struct {
	Index  int    // 1-based: 1º, 2º... bloco de consulta da nota
	Source string // conteúdo entre as cercas (YAML)
	Line   int    // linha (1-based) da primeira linha do conteúdo
}

// queryBlockYAML é o espelho cru do bloco. Os nomes das tags YAML são o que o
// usuário escreve — KnownFields(true) faz chave desconhecida virar erro, com o
// nome do campo na mensagem.
type queryBlockYAML struct {
	Tipo     string   `yaml:"tipo"`
	Onde     string   `yaml:"onde"`
	Ordenar  string   `yaml:"ordenar"`
	Limite   *int     `yaml:"limite"`
	Mostrar  string   `yaml:"mostrar"`
	Colunas  []string `yaml:"colunas"`
	Tags     []string `yaml:"tags"`
	SemTags  []string `yaml:"sem-tags"`
	Texto    string   `yaml:"texto"`
	Pasta    string   `yaml:"pasta"`
	Marcador []string `yaml:"marcador"`
	Estado   string   `yaml:"estado"`
	De       string   `yaml:"de"`
	Janela   string   `yaml:"janela"`
}

var (
	queryYAMLFieldRe = regexp.MustCompile(`field ([^\s]+) not found`)
	queryYAMLLineRe  = regexp.MustCompile(`line (\d+)`)
	queryFenceRe     = regexp.MustCompile("^[ \t]*(`{3,}|~{3,})[ \t]*([A-Za-z0-9_-]*)[ \t]*$")
	queryColunasRe   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// ParseQuery valida o conteúdo de um bloco ```consulta.
//
// A mensagem de erro é escrita para o usuário final (aparece dentro do cartão
// do painel de Consultas) e por isso é curta, em português, e sempre diz o que
// fazer — nunca devolve erro genérico de YAML sem tradução.
func ParseQuery(src string) (QuerySpec, error) {
	var raw queryBlockYAML
	dec := yaml.NewDecoder(strings.NewReader(src))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		if errors.Is(err, io.EOF) {
			return QuerySpec{}, errors.New("bloco: consulta vazia — informe ao menos `tipo:`")
		}
		return QuerySpec{}, queryYAMLError(src, err)
	}

	spec := QuerySpec{
		Tipo:     strings.ToLower(strings.TrimSpace(raw.Tipo)),
		Onde:     strings.TrimSpace(raw.Onde),
		Mostrar:  strings.ToLower(strings.TrimSpace(raw.Mostrar)),
		Limite:   DefaultQueryLimit,
		Tags:     cleanQueryList(raw.Tags),
		SemTags:  cleanQueryList(raw.SemTags),
		Texto:    strings.TrimSpace(raw.Texto),
		Pasta:    strings.ToLower(strings.TrimSpace(raw.Pasta)),
		Marcador: cleanQueryList(raw.Marcador),
		Estado:   strings.ToLower(strings.TrimSpace(raw.Estado)),
		De:       strings.TrimSpace(raw.De),
	}
	if raw.Limite != nil {
		spec.Limite = *raw.Limite
	}
	if spec.Mostrar == "" {
		spec.Mostrar = QueryMostrarLista
	}
	spec.Colunas = cleanQueryList(raw.Colunas)

	if raw.Janela != "" {
		return QuerySpec{}, errors.New(`janela: só existe para "tipo: agenda", que ainda não é suportado (planejado para a v2)`)
	}

	switch spec.Tipo {
	case "":
		return QuerySpec{}, errors.New("bloco: campo obrigatório ausente: tipo")
	case QueryTipoNotas, QueryTipoTarefas, QueryTipoHierarquia:
	case QueryTipoAgenda:
		return QuerySpec{}, errors.New(`tipo: "agenda" ainda não é suportado (planejado para a v2)`)
	default:
		return QuerySpec{}, fmt.Errorf("tipo: valor inválido %q — use notas, tarefas ou hierarquia", raw.Tipo)
	}

	campo, ordem, err := parseQueryOrder(raw.Ordenar)
	if err != nil {
		return QuerySpec{}, err
	}
	spec.Ordenar, spec.Ordem = campo, ordem
	if campo != "" && !containsString(QueryOrderFields(spec.Tipo), campo) {
		return QuerySpec{}, fmt.Errorf("ordenar: campo desconhecido %q para tipo: %s — campos ordenáveis: %s",
			campo, spec.Tipo, strings.Join(QueryOrderFields(spec.Tipo), ", "))
	}

	if spec.Limite < 1 || spec.Limite > MaxQueryLimit {
		return QuerySpec{}, fmt.Errorf("limite: use um valor entre 1 e %d", MaxQueryLimit)
	}

	switch spec.Mostrar {
	case QueryMostrarLista, QueryMostrarTabela, QueryMostrarContagem, QueryMostrarCartoes, QueryMostrarArvore:
	default:
		return QuerySpec{}, fmt.Errorf("mostrar: valor inválido %q — use lista, tabela, contagem, cartoes ou arvore", raw.Mostrar)
	}
	if spec.Mostrar == QueryMostrarArvore && spec.Tipo != QueryTipoHierarquia {
		return QuerySpec{}, errors.New(`mostrar: "arvore" só existe para "tipo: hierarquia"`)
	}
	if len(spec.Colunas) > 0 && spec.Mostrar != QueryMostrarTabela {
		return QuerySpec{}, errors.New(`colunas: só faz sentido com "mostrar: tabela"`)
	}
	for _, c := range spec.Colunas {
		if !queryColunasRe.MatchString(c) {
			return QuerySpec{}, fmt.Errorf("colunas: nome de coluna inválido %q", c)
		}
		if !containsString(queryColunas, c) {
			return QuerySpec{}, fmt.Errorf("colunas: coluna desconhecida %q — colunas válidas: %s",
				c, strings.Join(queryColunas, ", "))
		}
	}

	if len(spec.Onde) > MaxQueryOndeBytes {
		return QuerySpec{}, fmt.Errorf("onde: expressão longa demais (máximo %d caracteres)", MaxQueryOndeBytes)
	}

	if spec.Pasta != "" && !containsString(QueryPastas, spec.Pasta) {
		return QuerySpec{}, fmt.Errorf("pasta: valor inválido %q — use %s", raw.Pasta, strings.Join(QueryPastas, ", "))
	}
	if spec.Estado != "" {
		switch spec.Estado {
		case QueryEstadoPendente, QueryEstadoConcluida, QueryEstadoTodas:
		default:
			return QuerySpec{}, fmt.Errorf("estado: valor inválido %q — use pendente, concluida ou todas", raw.Estado)
		}
	}

	// Chaves que não se aplicam ao tipo: avisar é melhor que ignorar em silêncio.
	if err := queryCheckTipoKeys(spec); err != nil {
		return QuerySpec{}, err
	}

	return spec, nil
}

// queryCheckTipoKeys recusa atalhos que não fazem nada no tipo escolhido.
func queryCheckTipoKeys(spec QuerySpec) error {
	notaOnly := []struct {
		nome  string
		usado bool
	}{
		{"tags", len(spec.Tags) > 0},
		{"sem-tags", len(spec.SemTags) > 0},
		{"texto", spec.Texto != ""},
		{"pasta", spec.Pasta != ""},
	}
	for _, k := range notaOnly {
		if k.usado && spec.Tipo != QueryTipoNotas {
			return fmt.Errorf(`%s: só existe para "tipo: notas" — use onde: com os campos de n`, k.nome)
		}
	}
	if len(spec.Marcador) > 0 && spec.Tipo != QueryTipoTarefas {
		return fmt.Errorf(`marcador: só existe para "tipo: tarefas"`)
	}
	if spec.Estado != "" && spec.Tipo != QueryTipoTarefas {
		return fmt.Errorf(`estado: só existe para "tipo: tarefas"`)
	}
	if spec.De != "" && spec.Tipo != QueryTipoHierarquia {
		return fmt.Errorf(`de: só existe para "tipo: hierarquia"`)
	}
	return nil
}

// parseQueryOrder interpreta `ordenar: campo [asc|desc]`.
func parseQueryOrder(raw string) (campo, ordem string, err error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	switch len(fields) {
	case 0:
		return "", "", nil
	case 1:
		return strings.ToLower(fields[0]), "", nil
	case 2:
		campo = strings.ToLower(fields[0])
		dir := strings.ToLower(fields[1])
		if dir != "asc" && dir != "desc" {
			return "", "", fmt.Errorf("ordenar: direção inválida %q — use asc ou desc", fields[1])
		}
		return campo, dir, nil
	default:
		return "", "", fmt.Errorf("ordenar: formato inválido %q — use \"campo [asc|desc]\"", raw)
	}
}

// queryYAMLError traduz o erro do YAML para pt-BR, destacando chave desconhecida
// (o caso mais comum: erro de digitação numa chave) e a armadilha de aspas do
// `onde:` — que é, de longe, o erro que mais trava quem começa a escrever
// consultas (`onde: "projeto" in n.tags` sem aspas simples, ou expressão
// começando com `!`).
func queryYAMLError(src string, err error) error {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	linha := ""
	if m := queryYAMLLineRe.FindStringSubmatch(msg); m != nil {
		linha = " (linha " + m[1] + ")"
	}
	if m := queryYAMLFieldRe.FindStringSubmatch(msg); m != nil {
		return fmt.Errorf("bloco: campo desconhecido %q%s — chaves válidas: %s",
			m[1], linha, strings.Join(queryKeys, ", "))
	}
	msg = strings.ReplaceAll(msg, "unmarshal errors:\n", "")
	msg = strings.TrimSpace(strings.ReplaceAll(msg, "\n", " "))

	dica := ""
	if queryOndePrecisaAspas(src) {
		dica = " — dica: ponha o valor do onde: entre aspas simples, ex: onde: 'n.tags.size() > 0'"
	}
	return fmt.Errorf("bloco: YAML inválido%s — %s%s", linha, msg, dica)
}

// queryOndePrecisaAspas detecta o valor de `onde:` que o YAML não aceita solto:
// começando com aspas, `!` (tag), `[`/`{` (coleção), `*`/`&` (âncora/alias).
func queryOndePrecisaAspas(src string) bool {
	for _, linha := range strings.Split(src, "\n") {
		depois, ok := strings.CutPrefix(strings.TrimSpace(linha), "onde:")
		if !ok {
			continue
		}
		v := strings.TrimSpace(depois)
		if v != "" && strings.ContainsRune("\"'![{*&", rune(v[0])) {
			return true
		}
	}
	return false
}

// ExtractQueryBlocks devolve, na ordem do texto, todos os blocos ```consulta de
// um markdown. Blocos de código comuns (```go, ```sql, ``` sem info) são
// ignorados, e um bloco de consulta sem cerca de fechamento NÃO é executado
// (mesma semântica do markdown: enquanto a cerca não fecha, não existe bloco).
func ExtractQueryBlocks(markdown string) []QueryBlock {
	lines := strings.Split(markdown, "\n")

	var blocks []QueryBlock
	fenceOpen := false
	collecting := false
	var fenceChar byte
	var fenceLen int
	var buf []string
	startLine := 0

	for i, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")

		if !fenceOpen {
			m := queryFenceRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			fenceChar = m[1][0]
			fenceLen = len(m[1])
			fenceOpen = true
			collecting = strings.EqualFold(m[2], QueryFenceInfo)
			buf = nil
			startLine = i + 2
			continue
		}

		if queryIsClosingFence(line, fenceChar, fenceLen) {
			if collecting {
				blocks = append(blocks, QueryBlock{
					Index:  len(blocks) + 1,
					Source: strings.Join(buf, "\n"),
					Line:   startLine,
				})
			}
			fenceOpen = false
			collecting = false
			continue
		}

		if collecting {
			buf = append(buf, line)
		}
	}

	return blocks
}

// queryIsClosingFence diz se a linha fecha a cerca aberta: mesma "família"
// (``` ou ~~~), comprimento >= abertura, sem info-string.
func queryIsClosingFence(line string, char byte, minLen int) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < minLen {
		return false
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] != char {
			return false
		}
	}
	return true
}

// cleanQueryList limpa itens vazios e espaços de uma lista YAML.
func cleanQueryList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
