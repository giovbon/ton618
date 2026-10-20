// Package query executa a linguagem de consulta das notas (bloco ```consulta —
// ver DECISIONS §6.29 e docs/linguagem-de-consulta.md).
//
// O motor é SOMENTE LEITURA por construção: ele só conhece a interface Reader,
// implementada por um adaptador sobre o Store. Não existe caminho de escrita
// neste pacote, e nenhum resultado é persistido (o watcher reindexaria a nota e
// o processo viraria loop — ver §6.29).
//
// A única linguagem de expressão é CEL (cel.dev/cel-go): não-Turing-completa,
// sem I/O, com teto de custo. Não existe segundo avaliador em JS — duplicar
// avaliador Go↔JS é o custo já pago nos ícones (§6.9) e no rename (§6.14).
package query

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"

	"ton618/core/internal/processor"
)

const (
	// CustoMaxCEL é o teto de custo de uma expressão (protege contra
	// expressão quadrática/exponencial montada dentro de um bloco).
	CustoMaxCEL = 10000

	// TempoMaxBloco é o teto de tempo da avaliação de UM bloco.
	TempoMaxBloco = 200 * time.Millisecond

	// MaxCandidatos é o teto de linhas que o pré-filtro entrega ao CEL — mesmo
	// espírito de maxTagOnlyCandidates (§10): nunca materializar o corpus.
	MaxCandidatos = 500
)

// Engine compila (com cache) e executa as expressões do campo `onde:`.
//
// O cache guarda o AST (parse + type-check), que NÃO depende de `agora`; o
// ambiente de execução é montado por chamada com `agora` injetado, o que mantém
// determinismo e evita estado global mutável.
type Engine struct {
	mu    sync.RWMutex
	asts  map[string]*cel.Ast
	erros map[string]string
}

// NewEngine cria um motor com o cache vazio.
func NewEngine() *Engine {
	return &Engine{asts: map[string]*cel.Ast{}, erros: map[string]string{}}
}

// Prepare compila `expr` para o ambiente com `agora` injetado.
// Devolve (programa, "") ou (nil, mensagem em português pronta para o usuário).
func (e *Engine) Prepare(expr string, agora time.Time) (cel.Program, string) {
	ast, errMsg := e.ast(expr)
	if errMsg != "" {
		return nil, errMsg
	}

	env, err := newEnv(agora)
	if err != nil {
		return nil, "onde: não foi possível preparar o avaliador"
	}
	prg, err := env.Program(ast, cel.EvalOptions(cel.OptOptimize), cel.CostLimit(CustoMaxCEL))
	if err != nil {
		return nil, traduzErroCel(expr, err)
	}
	return prg, ""
}

// ast compila e faz cache do AST (inclusive do erro: expressão errada é
// recompilada só até o arquivo ser editado de novo).
func (e *Engine) ast(expr string) (*cel.Ast, string) {
	e.mu.RLock()
	ast, okAst := e.asts[expr]
	msgAntiga, okErr := e.erros[expr]
	e.mu.RUnlock()
	if okErr {
		return nil, msgAntiga
	}
	if okAst {
		return ast, ""
	}

	env, err := newEnv(time.Time{})
	if err != nil {
		return nil, "onde: não foi possível preparar o avaliador"
	}
	novo, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		msg := traduzErroCel(expr, issues.Err())
		e.mu.Lock()
		if len(e.erros) < 500 {
			e.erros[expr] = msg
		}
		e.mu.Unlock()
		return nil, msg
	}

	e.mu.Lock()
	if len(e.asts) < 500 {
		e.asts[expr] = novo
	}
	e.mu.Unlock()
	return novo, ""
}

// newEnv monta o ambiente: variáveis `n`/`t` (mapas string→dyn, um por tipo de
// consulta), `agora` e as funções do app (`dias`, `horas`).
func newEnv(agora time.Time) (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable("n", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("t", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("agora", cel.TimestampType),
		cel.Function("dias",
			cel.Overload("dias_timestamp", []*cel.Type{cel.TimestampType}, cel.IntType,
				cel.UnaryBinding(func(v ref.Val) ref.Val { return diasAte(v, agora) })),
		),
		cel.Function("horas",
			cel.Overload("horas_timestamp", []*cel.Type{cel.TimestampType}, cel.IntType,
				cel.UnaryBinding(func(v ref.Val) ref.Val { return horasAte(v, agora) })),
		),
	)
}

// diasAte devolve quantos dias inteiros se passaram entre ts e agora (negativo
// no futuro). É o "há quantos dias" da linguagem.
func diasAte(v ref.Val, agora time.Time) ref.Val {
	ts, ok := v.(types.Timestamp)
	if !ok {
		return types.NewErr("dias() espera uma data (timestamp)")
	}
	return types.Int(int64(math.Floor(agora.Sub(ts.Time).Hours() / 24)))
}

// horasAte é a versão em horas de diasAte.
func horasAte(v ref.Val, agora time.Time) ref.Val {
	ts, ok := v.(types.Timestamp)
	if !ok {
		return types.NewErr("horas() espera uma data (timestamp)")
	}
	return types.Int(int64(math.Floor(agora.Sub(ts.Time).Hours())))
}

// Avalia uma linha: devolve o resultado do filtro e, se houver erro, a mensagem
// em português (que vira conteúdo dentro do cartão, nunca um 500).
func (e *Engine) Avalia(ctx context.Context, expr string, prg cel.Program, variaveis map[string]any) (bool, string) {
	out, _, err := prg.ContextEval(ctx, variaveis)
	if err != nil {
		if ctx.Err() != nil {
			return false, "onde: tempo esgotado (200 ms)"
		}
		return false, traduzErroCel(expr, err)
	}

	b, ok := out.(types.Bool)
	if !ok {
		return false, "onde: a expressão precisa resultar em verdadeiro ou falso"
	}
	return bool(b), ""
}

// ---------------------------------------------------------------------------
// Tradução de erros do CEL
// ---------------------------------------------------------------------------

// traduzErroCel converte a mensagem do cel-go numa explicação curta em
// português. O usuário escreve expressões dentro de uma nota: um
// "no such key: velocidade" cru não diz nada.
func traduzErroCel(expr string, err error) string {
	msg := err.Error()

	if strings.Contains(msg, "no such key:") || strings.Contains(msg, "no such field") {
		campo := campoDoErro(msg)
		dono, campos := donoDoErro(expr, campo)
		return fmt.Sprintf("onde: campo desconhecido %q — campos de %s: %s",
			campo, dono, strings.Join(campos, ", "))
	}
	if strings.Contains(msg, "no such overload") || strings.Contains(msg, "found no matching overload") {
		return "onde: tipo incompatível — " + limpaMensagemCel(msg)
	}
	if strings.Contains(msg, "cost limit") || strings.Contains(msg, "operation cancelled") || strings.Contains(msg, "exceeded") {
		return "onde: expressão excedeu o limite de custo (simplifique a expressão)"
	}
	if strings.Contains(msg, "matches") && strings.Contains(msg, "invalid") {
		return "onde: expressão regular inválida em matches()"
	}
	return "onde: " + limpaMensagemCel(msg)
}

// campoDoErro extrai o nome do campo que não existe.
func campoDoErro(msg string) string {
	for _, marca := range []string{"no such key: ", "no such field: "} {
		if i := strings.Index(msg, marca); i >= 0 {
			resto := msg[i+len(marca):]
			resto = strings.TrimLeft(resto, "'\"")
			var sb strings.Builder
			for _, r := range resto {
				if r == '\'' || r == '"' || r == ' ' || r == ')' || r == ',' || r == ']' || r == '\n' {
					break
				}
				sb.WriteRune(r)
			}
			return sb.String()
		}
	}
	return "?"
}

// donoDoErro decide de quem é a variável ausente — `t` (tarefa) ou `n` (nota) —
// para a mensagem listar só os campos que fazem sentido naquela consulta.
func donoDoErro(expr, campo string) (string, []string) {
	if campo != "" && campo != "?" && strings.Contains(expr, "t."+campo) {
		return "t", processor.QueryTaskFields()
	}
	if strings.Contains(expr, "t.") && !strings.Contains(expr, "n.") {
		return "t", processor.QueryTaskFields()
	}
	return "n", processor.QueryNoteFields()
}

// limpaMensagemCel tira ruído do erro do cel-go (posições repetidas, prefixos).
func limpaMensagemCel(msg string) string {
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.TrimSpace(msg)
	cortes := []string{
		"ERROR: ",
		"found no matching overload for ",
		"no such overload ",
	}
	for _, c := range cortes {
		msg = strings.TrimPrefix(msg, c)
	}
	if len(msg) > 220 {
		msg = msg[:220] + "…"
	}
	return msg
}
