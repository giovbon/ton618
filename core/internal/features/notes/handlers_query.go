package notes

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ton618/core/internal/processor"
	"ton618/core/internal/query"
)

// ── Painel de Consultas (linguagem de consulta nas notas) ────────────────
//
// Ver DECISIONS §6.29 e docs/linguagem-de-consulta.md.
//
// O painel aparece ABAIXO do editor, um cartão por bloco ```consulta da nota.
// Ele é carregado e atualizado pelo próprio HTMX (`hx-get="/api/query"` com
// `hx-trigger="load, query-blocks-updated from:body"`), o mesmo padrão do badge
// de Tasks (§6.15) — assim o texto da nota continua sendo editado no TipTap e o
// servidor calcula o resultado, sem duplicar avaliador no cliente.

// consultasEngine é o motor CEL compartilhado por todas as requisições. Ele
// guarda só cache (AST compilado), não estado de consulta: `agora` é injetado
// por chamada, o que mantém o resultado determinístico e testável.
var consultasEngine = query.NewEngine()

// QueryBloco é um bloco ```consulta já resolvido, pronto para o template.
type QueryBloco struct {
	Indice int                 // posição do bloco na nota (1-based)
	Rotulo string              // resumo curto do que a consulta pede
	Spec   processor.QuerySpec // vazio quando houve erro de parse
	Result query.Result
	Erro   string
}

// HandleQueryPanel renderiza o conteúdo do painel de Consultas (fragmento HTML
// para o HTMX). Nunca responde erro HTTP por causa da consulta: erro de sintaxe,
// campo inexistente ou tempo esgotado viram mensagem DENTRO do cartão.
func (ctx *HandlerContext) HandleQueryPanel(w http.ResponseWriter, r *http.Request) {
	arquivo := NoteFilename(r.URL.Query().Get("file"))

	w.Header().Set("Cache-Control", "no-store")

	// Lê do índice (notes.content) de propósito: não incrementa popularidade
	// (é um refresh, não uma abertura da nota).
	conteudo, _ := ctx.Store.GetNote(arquivo)

	blocos, excedeu := ctx.queryBlocos(arquivo, conteudo)
	QueryPanel(arquivo, blocos, excedeu).Render(r.Context(), w)
}

// queryBlocos extrai, valida e executa os blocos ```consulta de uma nota.
func (ctx *HandlerContext) queryBlocos(arquivo, conteudo string) ([]QueryBloco, bool) {
	encontrados := processor.ExtractQueryBlocks(conteudo)
	if len(encontrados) == 0 {
		return nil, false
	}
	excedeu := len(encontrados) > processor.MaxQueryBlocks
	if excedeu {
		encontrados = encontrados[:processor.MaxQueryBlocks]
	}

	agora := time.Now()
	leitor := newQueryReader(ctx.Store)

	blocos := make([]QueryBloco, 0, len(encontrados))
	for _, b := range encontrados {
		spec, err := processor.ParseQuery(b.Source)
		if err != nil {
			blocos = append(blocos, QueryBloco{
				Indice: b.Index,
				Rotulo: "consulta " + strconv.Itoa(b.Index),
				Erro:   err.Error(),
			})
			continue
		}
		res := query.Run(context.Background(), consultasEngine, spec, leitor, arquivo, agora)
		blocos = append(blocos, QueryBloco{
			Indice: b.Index,
			Rotulo: rotuloDoBloco(spec),
			Spec:   spec,
			Result: res,
			Erro:   res.Erro,
		})
	}
	return blocos, excedeu
}

// rotuloDoBloco resume a consulta para o cabeçalho do cartão — é o que o usuário
// confere de relance para saber QUAL bloco está vendo o resultado.
func rotuloDoBloco(spec processor.QuerySpec) string {
	partes := []string{spec.Tipo}
	if len(spec.Tags) > 0 {
		partes = append(partes, "tags: "+strings.Join(spec.Tags, ", "))
	}
	if len(spec.SemTags) > 0 {
		partes = append(partes, "sem-tags: "+strings.Join(spec.SemTags, ", "))
	}
	if spec.Texto != "" {
		partes = append(partes, "texto: "+spec.Texto)
	}
	if spec.Pasta != "" {
		partes = append(partes, "pasta: "+spec.Pasta)
	}
	if len(spec.Marcador) > 0 {
		partes = append(partes, "marcador: "+strings.Join(spec.Marcador, ", "))
	}
	if spec.Estado != "" {
		partes = append(partes, "estado: "+spec.Estado)
	}
	if spec.De != "" {
		partes = append(partes, "de: "+spec.De)
	}
	if spec.Onde != "" {
		partes = append(partes, "onde: "+spec.Onde)
	}
	return strings.Join(partes, " · ")
}

// ── Helpers de apresentação usados pelo template ───────────────────────────

func numero(n int) string { return strconv.Itoa(n) }

// resultadoResumo é a contagem do cabeçalho do cartão (a `contagem` mostra o
// número no corpo, então não repete no cabeçalho).
func resultadoResumo(b QueryBloco) string {
	if b.Spec.Mostrar == processor.QueryMostrarContagem {
		return ""
	}
	n := len(b.Result.Rows)
	texto := "resultados"
	if n == 1 {
		texto = "resultado"
	}
	return strconv.Itoa(n) + " " + texto
}

// rotuloContagem é o substantivo que acompanha o número em `mostrar: contagem`.
func rotuloContagem(spec processor.QuerySpec) string {
	switch spec.Tipo {
	case processor.QueryTipoTarefas:
		return "tarefas"
	case processor.QueryTipoHierarquia:
		return "itens na árvore"
	default:
		if spec.Texto != "" {
			return "notas encontradas"
		}
		return "notas"
	}
}

// colunasEfetivas aplica o padrão por tipo quando o bloco não pede `colunas:`.
func colunasEfetivas(spec processor.QuerySpec) []string {
	if len(spec.Colunas) > 0 {
		return spec.Colunas
	}
	if spec.Tipo == processor.QueryTipoTarefas {
		return []string{"texto", "marcador", "estado", "arquivo"}
	}
	return []string{"titulo", "tags", "mtime"}
}

// colunaTitulo é o rótulo humano da coluna (o nome técnico continua no bloco).
func colunaTitulo(nome string) string {
	titulos := map[string]string{
		"titulo":            "Título",
		"caminho":           "Caminho",
		"arquivo":           "Arquivo",
		"pasta":             "Pasta",
		"classe":            "Tipo",
		"pai":               "Pai",
		"tags":              "Tags",
		"n_tags":            "Nº tags",
		"filhas":            "Filhas",
		"mtime":             "Modificado",
		"modif_ha_dias":     "Dias sem editar",
		"interacao":         "Última interação",
		"interacao_ha_dias": "Dias sem interagir",
		"pai_resolvido":     "Pai existe",
		"nota_titulo":       "Nota",
		"citada_por":        "Citações",
		"tarefas_abertas":   "Tarefas abertas",
		"popularidade":      "Interações",
		"texto":             "Tarefa",
		"marcador":          "Marcador",
		"estado":            "Estado",
		"secao":             "Seção",
		"linha":             "Linha",
		"criado":            "Criada em",
	}
	if t, ok := titulos[nome]; ok {
		return t
	}
	return nome
}

// indentacao desloca a linha da `arvore` conforme a profundidade.
func indentacao(nivel int) string {
	if nivel < 1 {
		nivel = 1
	}
	return "padding-left:" + strconv.Itoa(6+nivel*16) + "px"
}

func alvoAba(r query.Row) string {
	if r.NovaAba() {
		return "_blank"
	}
	return ""
}

// linhaDetalhe é a informação secundária da linha em `mostrar: lista` — data em
// que a nota foi mexida, ou a linha da tarefa quando o resultado é de tarefas.
func linhaDetalhe(r query.Row) string {
	if r.Texto != "" && r.Linha > 0 {
		return "linha " + strconv.Itoa(r.Linha)
	}
	if r.InteracaoRel() != "" {
		return r.InteracaoRel()
	}
	return r.MtimeRel()
}

func textoVazio(spec processor.QuerySpec) string {
	switch spec.Tipo {
	case processor.QueryTipoTarefas:
		return "nenhuma tarefa corresponde"
	case processor.QueryTipoHierarquia:
		return "esta nota não tem filhas"
	default:
		if spec.Texto != "" {
			return "nenhuma nota encontrada para \"" + spec.Texto + "\""
		}
		return "nenhuma nota corresponde"
	}
}
