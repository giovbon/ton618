# ADR 0004: Linguagem de Consulta nas notas (bloco `consulta` + CEL)

* **Status:** Aceito — implementação pendente (ver plano em `DECISIONS.md` §6.29)
* **Data:** 2026-10-06

## Contexto e Problema

O app sabe **mostrar** e **filtrar**: busca FTS5, busca semântica, busca híbrida (RRF) e o Tabulator
com filtro client-side. O que ele não dá conta hoje:

1. **Combinar condições** (E / OU / NÃO) — a busca filtra por termo e uma tag; o Tabulator só filtra
   o que já está carregado e com sintaxe simples (`titulo:`, `tags:`).
2. **Ter o resultado dentro da nota, sempre atualizado** — hoje a informação é "caçada" na mão.
3. **Valores calculados** (contagens, "há quantos dias não abro", tamanho).
4. **Regras de organização** além do `X dias → tag` do Auto-Tag (§6.8).

O usuário pediu uma "linguagem de script dentro das notas".

## Decisão

1. **Bloco declarativo** ` ```consulta `, escrito em **YAML** — o usuário já escreve YAML no
   frontmatter (§6.17), então a curva de aprendizado é zero e o parser é a dependência que já existe
   (`gopkg.in/yaml.v3`, com `KnownFields(true)` ⇒ campo desconhecido vira erro na hora).
2. **Expressões em CEL** (`github.com/google/cel-go`) **apenas no campo `onde:`**. CEL é
   não-Turing-completa: termina sempre, não tem I/O nem efeitos colaterais, tem *type-check* e
   **`cel.CostLimit`**. É o "esperto" do bloco, não uma linguagem de programação.
3. **Avaliação 100% no servidor (Go).** Nada de CEL/JS no cliente: duplicar avaliador Go↔JS é
   exatamente o erro já registrado nos ícones (§6.9) e no rename (§6.14) — e o projeto tem teste de
   paridade justamente por isso. Consequência: **zero mudança** de bundle, CSP, `staticver` ou mobile.
4. **Duas velocidades (a razão da eficiência)**: as chaves "de índice" (`tags`, `sem-tags`, `texto`,
   `pasta`, `de`, `marcador`, `estado`, `janela`) resolvem no **SQL/FTS5/índice existente** e
   entregam no máximo 500 candidatos; `onde:` (CEL) só **refina** linha a linha. Mesmo espírito do §10:
   nunca materializar o corpus.
5. **Read-only por construção.** O motor depende de uma **interface somente-leitura** implementada
   pelo `*db.Store` (única interface nova, justificada por testabilidade **e** por impedir escrita em
   tempo de compilação). Nenhuma consulta escreve; **nada é persistido no `.md`** — o watcher
   reindexaria e viraria loop.
6. **Resultado é efêmero e renderizado no servidor** (`templ`), num painel abaixo do editor, com
   botão de atualizar via HTMX (mesmo padrão de eventos do badge de Tasks, §6.15).
7. **Determinismo:** `agora` é **injetado** no motor (nunca `time.Now()` interno) ⇒ golden tests
   reproduzíveis.

## Alternativas descartadas

| Alternativa | Motivo |
|---|---|
| JS do usuário no browser (modelo Obsidian) | CSP restritiva, offline, mobile, ETag/`staticver`, não testável com golden tests — e não é o modelo do projeto (binário único + render server-side) |
| Lua (`gopher-lua`) / Starlark | Linguagem de propósito geral ⇒ sandbox, limites, API curada e UX de erro. Pode entrar **depois, atrás do mesmo contrato**, sem refazer nada |
| Expor SQL ao usuário | Vaza o schema, encoraja query cara, acopla ao banco |
| Parser próprio | Manutenção eterna e sem *type-check* — é o caminho mais caro no fim |
| `expr-lang/expr` | Plano B viável e mais ergonômico para filtros; perde o type-check forte, o padrão conhecido e o cost limit |
| WASM (`wazero`) | Só se virar plataforma de plugins de terceiros |

## Consequências

* **Positivas:** uma única dependência nova (pura Go, sem CGO — alinhada a `modernc.org/sqlite`);
  reaproveita FTS5, RRF, hierarquia `pai`/filhas e a tabela `todos`; funciona no mobile; erro vira
  conteúdo no bloco (padrão do projeto: erro na UI, nunca 500); testável com banco real.
* **Pontos de atenção:**
  1. No v1 o resultado aparece num **painel abaixo do editor**, não no meio do texto. Um "modo
     leitura" que inline os resultados é evolução natural (registrado como pendência).
  2. Blocos custam CPU a cada abertura de nota ⇒ **cache com invalidação na escrita**, tetos de
     linhas/tempo e no máximo **10 blocos por nota**.
  3. `similar:` (busca semântica) **não roda no servidor** — o embedding só existe no navegador
     (§3.1). Fica para a **v2**, em duas etapas, como já acontece na busca híbrida (§8).
  4. A gramática passa a ser **contrato público**: mudanças exigem atualização do documento de
     sintaxe e do changelog dele.

## Referências

* **Sintaxe completa e possibilidades:** [`docs/linguagem-de-consulta.md`](../linguagem-de-consulta.md)
* **Log detalhado da decisão, plano por fases e guardas de teste:** `DECISIONS.md` §6.29
