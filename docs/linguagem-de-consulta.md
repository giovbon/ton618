# Linguagem de Consulta do TON-618 — referência completa

> **Status:** v1 **implementada** (Fases 0–3: `tipo: notas`, `tarefas` e `hierarquia`, todas as apresentações e o painel no editor). Pendentes para a v2: `tipo: agenda`, `similar:`, `n.tamanho`/`n.palavras`, `{{ }}` inline e modo leitura.
> **Decisão arquitetural:** [`ADR 0004`](adr/0004-linguagem-de-consulta.md) · `DECISIONS.md` §6.29.
> **Este documento é a fonte única da verdade da sintaxe.** O código referencia estes nomes; se a
> gramática mudar, este arquivo muda junto (e o changelog, no fim).

---

## 0. O que é (e o que não é)

A **linguagem de consulta** deixa você escrever, **dentro de uma nota**, uma pergunta sobre o seu
acervo — e ver a resposta atualizada ali mesmo.

````markdown
```consulta
tipo: notas
tags: [livros]
onde: n.interacao == null || dias(n.interacao) > 90
ordenar: mtime desc
limite: 10
mostrar: lista
```
````

Você usa isso para montar **páginas vivas**: uma nota "Minha fila de leitura", uma "Revisão do mês",
um "Painel de projetos". A resposta é recalculada a partir do acervo real — não é texto que você
precisa atualizar na mão.

**A linguagem:**

| É | Não é |
|---|---|
| Declarativa: você **descreve** o que quer | Código imperativo (não tem `for`, `print`, atribuição) |
| Somente leitura: só **lê** o acervo | Nunca escreve, move, renomeia, apaga ou cria nada |
| Server-side: roda no Go, no servidor | Não roda no navegador, não baixa nada, não pede internet |
| Determinística: mesma entrada, mesma saída | Não depende do relógio do formato nem da ordem de abertura |
| Efémera: o resultado é **renderizado na hora** | O resultado **não é gravado** dentro do seu `.md` |

> ⚠️ **O resultado nunca é salvo no arquivo.** Isto é uma decisão de arquitetura, não uma limitação
> temporária: se o resultado fosse escrito na nota, o watcher reindexaria a nota, o resultado mudaria,
> e o processo se repetiria — um loop de escrita. O texto do bloco é a *pergunta*; a resposta existe
> só na tela.

---

## 1. Onde escrever a consulta

Dentro de qualquer nota Markdown, num bloco de código cujo idioma é **`consulta`**:

````markdown
Veja o que eu tenho de livros por ler:

```consulta
tipo: notas
tags: [livros]
onde: !("lido" in n.tags)
```

E o total de notas do acervo:

```consulta
tipo: notas
mostrar: contagem
```
````

Regras:

* O idioma **`consulta`** é obrigatório (```` ```consulta ````). Um bloco ```` ``` ```` normal, ou
  ```` ```sql ````, ```` ```go ```` etc., é ignorado — continua sendo código comum na sua nota.
* Podem existir **vários blocos por nota** (até 10). Cada bloco é uma consulta independente.
* Cada bloco é identificado pela **posição** (1º, 2º, 3º...) — o painel mostra um cartão por bloco,
  na mesma ordem do texto.
* O conteúdo é **YAML** (o mesmo formato do frontmatter): `chave: valor`, uma por linha. Listas podem
  ser `[a, b, c]` ou em bloco com `- a`.

---

## 2. Onde ver o resultado

No editor da nota, **abaixo do texto**, aparece o painel **Consultas**, com um cartão por bloco:

| Elemento do cartão | Significado |
|---|---|
| Ícone + contagem | tipo da consulta e quantos itens foram devolvidos |
| Botão **atualizar** | recalcula **aquele** bloco (via HTMX) |
| Corpo | o resultado, conforme `mostrar:` |
| Rodapé `mostrando N de M` | aparece quando o resultado foi truncado por `limite:` |
| Faixa de erro | a mensagem em português, dentro do cartão (a nota nunca dá erro 500) |

O painel é re-renderizado no servidor quando você **salva a nota** (mesmo mecanismo de eventos do
badge de Tasks, §6.15) e também quando clica em **atualizar**.

> **Por que abaixo do texto e não no meio dele?** As notas são editadas no TipTap (WYSIWYG client-side)
> e **não existe renderizador de Markdown no servidor** — inserir HTML no meio do documento exigiria
> reescrever o pipeline do editor. O painel entrega o mesmo valor com risco zero. Um **modo leitura**
> que inline os resultados no meio do texto é evolução planejada (ver §13).

---

## 3. As chaves do bloco (visão geral)

Todas as chaves, com obrigatoriedade, valores aceitos e padrão:

| Chave | Obrigatória | Valores | Padrão |
|---|---|---|---|
| `tipo` | **sim** | `notas` · `tarefas` · `hierarquia` · `agenda` | — |
| `onde` | não | expressão **CEL** (§7) | vazio = todos |
| `ordenar` | não | `<campo>` + ` asc`/` desc` | do tipo (ver §8) |
| `limite` | não | inteiro `1`–`200` | `20` |
| `mostrar` | não | `lista` · `tabela` · `contagem` · `cartoes` · `arvore` | `lista` |
| `colunas` | não | lista de campos (só com `mostrar: tabela`) | `[titulo, tags, mtime]` |

**Atalhos de índice** (por tipo — cada um responde por uma query já existente e indexada):

| Chave | Tipos | Valores | Efeito |
|---|---|---|---|
| `tags` | `notas` | lista de tags | nota precisa ter **todas** elas |
| `sem-tags` | `notas` | lista de tags | nota **não pode ter nenhuma** delas |
| `texto` | `notas` | string | busca **FTS5** (stemming pt-BR) |
| `pasta` | `notas` | `notes` · `pdfs` · `epubs` · `attachments` · `images` · `archives` | restringe a raiz do acervo |
| `marcador` | `tarefas` | lista de marcadores | filtra pela coluna `type` do `todos` |
| `estado` | `tarefas` | `pendente` · `concluida` · `todas` | filtra por `status` |
| `de` | `hierarquia` | nome/caminho de nota | raiz da árvore (padrão: a nota atual) |
| `janela` | `agenda` (v2) | `hoje` · `semana` · `mes` | recorte de datas — **reservado, ainda não suportado** |

> 💡 **A regra mental do v1:** *tudo que tem índice vira atalho; todo o resto vai no `onde:`*. Se a
> informação não está no índice (ex.: `n.citada_por >= 3`), quem resolve é a expressão CEL — sobre os
> candidatos que os atalhos já reduziram.

Chave desconhecida **é erro** (não é ignorada em silêncio): o bloco mostra a lista das chaves válidas.

---

## 4. `tipo: notas`

O tipo principal: consulta o acervo de arquivos (`notes`, `pdfs`, `epubs`, `attachments`, `images`,
`archives`).

**Atalhos disponíveis:** `tags`, `sem-tags`, `texto`, `pasta`.
**Campos aceitos em `ordenar:` e em `colunas:`:** ver tabela em §7.5.
**`mostrar:` permitido:** todos, exceto `arvore`.

````markdown
```consulta
tipo: notas
tags: [projeto, ativo]
onde: n.filhas.size() == 0
ordenar: citada_por desc
limite: 15
mostrar: tabela
colunas: [titulo, tags, citada_por, interacao]
```
````

Semântica dos atalhos:

* `tags: [a, b]` ⇒ **E** lógico (precisa ter as duas). Para **ou**, use `onde:`:
  `onde: "a" in n.tags || "b" in n.tags`.
* `sem-tags: [rascunho]` ⇒ a nota **não** pode ter `rascunho`.
* `texto: "guerra"` ⇒ FTS5: encontra por radical (`guerra`, `guerras`); combinável com `onde:`.
* `pasta: pdfs` ⇒ só arquivos daquela raiz.

---

## 5. `tipo: tarefas`

Consulta a tabela de marcadores de tarefa (`todos`), que já alimenta o badge (§6.15).

**Atalhos disponíveis:** `marcador`, `estado`.
**Campos:** `t.*` (§7.5). **`mostrar:` permitido:** `lista`, `tabela`, `contagem`.

````markdown
```consulta
tipo: tarefas
marcador: [TODO]
estado: pendente
onde: t.texto.contains("publicar")
ordenar: criado asc
limite: 50
mostrar: lista
```
````

| Chave | Valores | Padrão | Nota |
|---|---|---|---|
| `marcador` | `[TODO]`, `[FIXME]`, `[REVISAR]`… | todos | casa com a coluna `type` |
| `estado` | `pendente` · `concluida` · `todas` | `pendente` | `concluida` = `[x]` |

---

## 6. `tipo: hierarquia`

Percorre a árvore de **pai → filhas** (relação `pai:` no frontmatter, §6.24).

**Atalhos disponíveis:** `de` (raiz). **`mostrar:` permitido:** `lista`, `arvore`, `contagem`.

````markdown
```consulta
tipo: hierarquia
de: Projeto TON-618
onde: n.mtime > timestamp("2026-09-01T00:00:00Z")
mostrar: arvore
```
````

* Sem `de:`, a raiz é **a própria nota onde o bloco está escrito** — perfeito para um "índice" que
  lista os filhos.
* A profundidade é limitada (padrão: 3 níveis; `arvore` mostra a indentação com `└`).

---

## 7. Expressões (`onde:`) — a linguagem CEL

O campo `onde:` é avaliado por **CEL** (*Common Expression Language*, a mesma linguagem de regras do
Kubernetes/Istio). É uma linguagem de **expressões**: cada linha produz um `true`/`false`.

### 7.1 Tipos e operadores

| Categoria | Operadores / literais |
|---|---|
| Lógicos | `&&` (E) · `\|\|` (OU) · `!` (NÃO) |
| Comparação | `==` · `!=` · `<` · `<=` · `>` · `>=` |
| Aritmética | `+` · `-` · `*` · `/` · `%` (inteiros) |
| Agrupamento | `( … )` |
| Literais | `42` · `3.14` · `"texto"` · `'texto'` · `true` / `false` · `null` |
| Listas | `["a", "b"]` · `[]` · `in` (`"a" in n.tags`) |
| Mapa | `{"a": 1}` · `n.mapa["chave"]` |

Precedência segue o usual: `!` > aritmética > comparação > `&&` > `||`. Na dúvida, **use parênteses**.

### 7.2 Macros (as mais usadas no dia a dia)

| Macro | Assinatura | Para que serve |
|---|---|---|
| `exists` | `lista.exists(x, condicao)` | "existe algum que satisfaz" |
| `all` | `lista.all(x, condicao)` | "todos satisfazem" |
| `exists_one` | `lista.exists_one(x, condicao)` | "exatamente um satisfaz" |
| `map` | `lista.map(x, expressao)` | transforma a lista |
| `filter` | `lista.filter(x, condicao)` | filtra a lista |
| `has` | `has(mapa.chave)` | testa se o campo existe |
| `size` | `lista.size()` / `"txt".size()` | quantidade / comprimento |

```cel
// alguma tag começa com "livro"?
n.tags.exists(t, t.startsWith("livro"))

// TODAS as tags são curtas (menos de 12 caracteres)?
n.tags.all(t, t.size() < 12)

// quantas tags começam com "proj"?
n.tags.filter(t, t.startsWith("proj")).size() >= 2
```

### 7.3 Funções de texto

| Função | Exemplo | Observação |
|---|---|---|
| `contains` | `n.titulo.contains("ATA")` | sensível a maiúsculas |
| `startsWith` | `n.caminho.startsWith("notes/2026")` | |
| `endsWith` | `n.titulo.endsWith(".pdf")` | |
| `matches` | `n.caminho.matches("^notes/2026-\\d\\d")` | **RE2** (Go) — sem *backtracking* |
| `size` | `n.titulo.size() > 60` | comprimento em caracteres |
| `lowerAscii` / `upperAscii` | `n.titulo.lowerAscii().contains("ata")` | normalização simples |

Para busca **sem acento/radical**, prefira o atalho `texto:` (FTS5) em vez de `matches`.

### 7.4 Tempo e datas

| Recurso | Exemplo | Significado |
|---|---|---|
| `agora` | `agora` | instante da execução (**injetado**, determinístico) |
| `duration("2160h")` | `duration("720h")` | duração (sufixos `h`, `m`, `s`) |
| `timestamp("2026-09-01T00:00:00Z")` | idem | instante fixo (RFC 3339) |
| `dias(ts)` | `dias(n.mtime)` | **função do app**: dias inteiros até `agora` |
| `horas(ts)` | `horas(n.interacao)` | **função do app**: horas até `agora` |
| `-` entre timestamps | `agora - n.mtime` | duração (`duration`) |

```cel
// não mexo há mais de 90 dias
dias(n.mtime) > 90

// editada nas últimas 2 semanas
agora - n.mtime < duration("336h")

// nunca interagi
n.interacao == null

// "esfriou": interagi antes, mas não nos últimos 30 dias
n.interacao != null && dias(n.interacao) > 30
```

### 7.5 Variáveis e campos

Variáveis disponíveis por tipo:

| Variável | Tipo | Existe em |
|---|---|---|
| `n` | nota / arquivo | `tipo: notas`, `tipo: hierarquia` |
| `t` | tarefa | `tipo: tarefas` |
| `a` | compromisso | `tipo: agenda` (reservado, §13 — campos na v2) |
| `agora` | timestamp | todos |

**Campos de `n` (nota/arquivo):**

| Campo | Tipo | Significado | Custo |
|---|---|---|---|
| `n.caminho` | string | caminho relativo (`notes/pessoal/x.md`) | índice |
| `n.titulo` | string | nome de exibição (sem extensão) | índice |
| `n.pasta` | string | raiz (`notes`, `pdfs`, `epubs`, `attachments`, `images`, `archives`) | índice |
| `n.classe` | string | tipo do item (ver abaixo) | índice |
| `n.tags` | lista de string | tags (`tags`) | índice |
| `n.mtime` | timestamp | última modificação no disco (`file_mods`) | índice |
| `n.pai` | string | valor de `pai:` (`""` se não tiver) | índice |
| `n.pai_resolvido` | bool | o `pai:` aponta para uma nota que existe | agregado |
| `n.filhas` | lista de string | nomes-base das filhas diretas | agregado |
| `n.citada_por` | int | quantos backlinks apontam para ela (`links`) | agregado |
| `n.tarefas_abertas` | int | tarefas pendentes dentro dela (`todos`) | agregado |
| `n.popularidade` | int | contador de interações (`popularity.count`) | índice |
| `n.interacao` | timestamp ou `null` | última interação no app (`popularity.last_interacted_at`) | índice |

> ⚠️ **Não existe "última abertura".** O banco registra a **última interação**
> (`popularity.last_interacted_at`, que também muda ao editar/buscar). É ela que alimenta a coluna
> **Abrir** da tabela de notas — por isso o campo se chama `n.interacao` e não `n.abertura`.

Valores de `n.classe` (espelham o tipo detectado na varredura do acervo — SSOT dos ícones, §6.9):
`nota` (Markdown), `desenho`, `markmap`, `youtube`, `artigo`, `captura`, `semanal`, `pdf`, `epub`,
`anexo`, `imagem`, `arquivo`.

**Campos de `t` (tarefa):**

| Campo | Tipo | Significado |
|---|---|---|
| `t.texto` | string | texto da tarefa |
| `t.marcador` | string | `TODO`, `FIXME`, `REVISAR`… |
| `t.estado` | string | `pendente` ou `concluida` |
| `t.arquivo` | string | caminho da nota onde ela vive |
| `t.nota_titulo` | string | título dessa nota |
| `t.secao` | string | seção (`##`) onde a tarefa está |
| `t.linha` | int | linha no arquivo |
| `t.criado` | timestamp | quando foi detectada |

**Fontes (o motor só lê):** `file_mods`/`notes` (caminho, mtime, conteúdo), `tags`, `links`
(`citada_por`), `popularity` (`popularidade`, `interacao`), `todos` (`tarefas_abertas`),
`file_metadata` + frontmatter (`pai`, `pai_resolvido`, `filhas`) e a varredura do acervo (`classe`).
Nenhuma tabela nova.

### 7.6 Erros de expressão (e como o app responde)

| Situação | Mensagem no cartão |
|---|---|
| Sintaxe | `onde: erro de sintaxe na posição 17 — esperado ")", encontrado "&&"` |
| Campo inexistente | `onde: campo desconhecido "n.velocidade" — campos de n: caminho, citada_por, classe, filhas, interacao, mtime, pai, …` |
| Tipo incompatível | `onde: tipo incompatível — n.tags é lista e 3 é inteiro` |
| Expressão cara | `onde: expressão excedeu o limite de custo` |
| Demorou demais | `onde: tempo esgotado (200 ms)` |
| Regex inválida | `onde: expressão regular inválida em "matches"` |

Toda mensagem é **conteúdo do bloco**, em português, com a posição quando aplicável — nunca um erro
500 e nunca um bloco silenciosamente vazio.

---

## 8. Ordenação e limite

```
ordenar: mtime desc
ordenar: titulo
limite: 25
```

| Regra | Detalhe |
|---|---|
| Direção natural | `titulo` e `caminho` ordenam **asc**; os demais campos, **desc** |
| `asc` / `desc` | sufixo explícito sobrepõe a direção natural |
| Um campo só | o v1 ordena por **um** campo; multi-campo (`mtime desc, titulo asc`) está fora do escopo |
| Campos de lista | `filhas` e `tags` ordenam pelo **tamanho** da lista |
| `limite` | `1`–`200`; padrão `20`; valores acima de 200 são **rejeitados** (não truncados) |
| Truncamento | quando há mais resultados que o limite, o cartão mostra `mostrando 20 de 137` |
| Desempate | sempre **caminho asc**, para o resultado ser estável entre execuções |

---

## 9. Apresentações (`mostrar:`)

### `lista` (padrão)

Lista de links, com ícone do tipo (§6.9) e data relativa.

```
• 📄 Livros por ler            (editado há 2 dias)
• 📄 Fila do mês               (editado há 3 semanas)
```

**Permitido em:** `notas`, `tarefas`, `hierarquia`.

### `tabela`

Tabela compacta; `colunas:` escolhe e ordena as colunas.

````markdown
```consulta
tipo: notas
tags: [livros]
ordenar: citada_por desc
mostrar: tabela
colunas: [titulo, tags, interacao_ha_dias, citada_por]
```
````

| Título | Tags | Interação há (dias) | Citações |
|---|---|---|---|
| A História Secreta do Ocidente | `livros, história` | 12 | 5 |
| Tecnofeudalismo | `livros, economia` | — | 3 |

* Coluna `interacao_ha_dias` = `dias(n.interacao)` (vazio quando nunca interagida).
* Colunas válidas: `arquivo`, `caminho`, `citada_por`, `classe`, `criado`, `estado`, `filhas`, `interacao`, `interacao_ha_dias`, `linha`, `marcador`, `modif_ha_dias`, `mtime`, `n_tags`, `nota_titulo`, `pai`, `pai_resolvido`, `pasta`, `popularidade`, `secao`, `tags`, `tarefas_abertas`, `texto`, `titulo`.
* A lista é a **união dos campos de `n` e de `t`** (§7.5) com as calculadas — `interacao_ha_dias`, `modif_ha_dias` (`dias(...)`) e `n_tags` (tamanho da lista); `filhas` mostra o **tamanho** da lista.
* `colunas:` só é válido com `mostrar: tabela` — em outro modo, é erro.
* Coluna desconhecida é erro, com a lista das válidas (nada de coluna silenciosamente vazia).

### `contagem`

Um número e um rótulo. Sem lista. Ideal para dashboards.

```
12 notas
```

### `cartoes`

Grade com título + trecho. Como o trecho vem do índice FTS5, combina com `texto:`.

````markdown
```consulta
tipo: notas
texto: "foco profundo"
mostrar: cartoes
limite: 6
```
````

### `arvore`

Só em `tipo: hierarquia`: a subárvore com indentação.

```
Projeto TON-618
├─ Arquitetura
│  └─ Busca híbrida
└─ Diário
   └─ 2026-09-30
```

---

## 10. Limites e segurança

| Guarda | Valor | Por quê |
|---|---|---|
| Somente leitura | por construção | o motor só recebe uma interface de leitura do banco |
| Nada é persistido | sempre | o watcher reindexaria e causaria loop de escrita |
| Blocos por nota | 10 | uma página não vira um *dump* |
| Candidatos pré-filtrados | 500 | mesmo teto de `maxTagOnlyCandidates` (§10) |
| `limite:` | máx. `200` | resultado sempre pequeno para a tela |
| Tempo por bloco | 200 ms | a nota abre rápido mesmo com expressão ruim |
| Custo da expressão | `cel.CostLimit` | impede expressão quadrática/exponencial |
| Cache | TTL 5 s + invalidação em qualquer escrita | mesmo padrão de `childrenCacheTTL` |
| Sem rede, sem I/O, sem `exec` | por linguagem | CEL não tem I/O nenhum |
| Sem `agora` livre | `agora` injetado | determinismo e testes reproduzíveis |

Consequências visíveis destas guardas:

* Resultado com mais de 500 candidatos: o pré-filtro corta e o cartão avisa `avaliadas as 500 mais
  recentes de 1.204` — nunca "consulta lenta".
* Expressão patológica: mensagem de limite, não travamento.

---

## 11. Receitas prontas (copiar e colar)

**1. Fila de leitura (livros que ainda não terminei)**

````markdown
```consulta
tipo: notas
tags: [livros]
onde: !("lido" in n.tags)
ordenar: mtime asc
limite: 10
mostrar: lista
```
````

**2. Total de notas do acervo**

````markdown
```consulta
tipo: notas
mostrar: contagem
```
````

**3. Esfriando: não abro há mais de 60 dias**

````markdown
```consulta
tipo: notas
onde: n.interacao != null && dias(n.interacao) > 60
ordenar: interacao asc
mostrar: tabela
colunas: [titulo, tags, interacao_ha_dias]
```
````

**4. Capturas não processadas**

````markdown
```consulta
tipo: notas
onde: n.classe == "captura" && !("processado" in n.tags)
ordenar: mtime asc
mostrar: lista
```
````

**5. Órfãs: têm `pai:` mas o pai não existe**

````markdown
```consulta
tipo: notas
onde: n.pai != "" && !n.pai_resolvido
mostrar: tabela
colunas: [titulo, pai]
```
````

**6. Notas-mãe mais carregadas**

````markdown
```consulta
tipo: notas
onde: n.filhas.size() >= 5
ordenar: filhas desc
mostrar: tabela
colunas: [titulo, filhas, citada_por]
```
````

**7. Sem etiqueta nenhuma (limpeza)**

````markdown
```consulta
tipo: notas
pasta: notes
onde: n.tags.size() == 0 && n.classe == "nota"
ordenar: mtime desc
limite: 30
mostrar: lista
```
````

**8. Mais citadas do acervo**

````markdown
```consulta
tipo: notas
onde: n.citada_por >= 3
ordenar: citada_por desc
limite: 10
mostrar: tabela
colunas: [titulo, citada_por, tags]
```
````

**9. Notas semanais recentes**

````markdown
```consulta
tipo: notas
onde: n.titulo.matches("^2026-S")
ordenar: titulo desc
limite: 8
mostrar: lista
```
````

**10. Tarefas pendentes de um projeto**

````markdown
```consulta
tipo: tarefas
marcador: [TODO]
estado: pendente
onde: t.arquivo.contains("projeto-ton618")
mostrar: lista
```
````

**11. Tarefas concluídas nesta semana**

````markdown
```consulta
tipo: tarefas
estado: concluida
onde: agora - t.criado < duration("168h")
mostrar: contagem
```
````

**12. Índice dos filhos desta nota**

````markdown
```consulta
tipo: hierarquia
mostrar: arvore
```
````

**13. Trechos de busca em cartões**

````markdown
```consulta
tipo: notas
texto: "foco"
onde: n.classe == "artigo"
mostrar: cartoes
limite: 6
```
````

**14. Desenhos/mapas abandonados**

````markdown
```consulta
tipo: notas
onde: (n.classe == "desenho" || n.classe == "markmap") && dias(n.mtime) > 180
ordenar: mtime asc
mostrar: lista
```
````

**15. Painel de manutenção da semana**

````markdown
```consulta
tipo: notas
onde: agora - n.mtime < duration("168h")
ordenar: mtime desc
limite: 20
mostrar: tabela
colunas: [titulo, tags, modif_ha_dias]
```
````

---

## 12. Armadilhas do YAML (as mensagens de erro que você vai ver)

O bloco é YAML. Três regras resolvem 95% dos problemas:

| Situação | Como escrever |
|---|---|
| A expressão tem `#` ou `: ` | **Ponha entre aspas**: `onde: 'n.titulo.contains("#1")'` |
| Lista de tags | `tags: [a, b]` ou em bloco (`- a` em linhas seguintes) |
| String que é só número (`"2026"`) | Use aspas para não virar inteiro: `onde: n.titulo == "2026"` |
| Regex com `\` | Barra dupla: `n.caminho.matches("^notes/\\d{4}")` |
| `[` no começo da expressão | Ponha entre aspas: `onde: '["a","b"].exists(t, t in n.tags)'` — evita conflito com lista YAML |

Erros de YAML aparecem como: `bloco: YAML inválido (linha 4) — …`.

---

## 13. O que ainda NÃO existe (e por quê)

| Recurso | Situação | Motivo |
|---|---|---|
| `similar:` — semântico no bloco | v2 | o embedding é calculado **no navegador**; exige duas etapas (como a busca híbrida, §8) |
| `tipo: agenda` | v2 | precisa fixar os campos sobre o modelo real de `appointments` (`descricao`, `data`, `ano`, `mes`, `semana` — ainda sem nome-canônico na spec) |
| `n.tamanho`, `n.palavras` e `n.peso` | v2 | custam I/O por linha (o motor v1 lê **somente o banco**); `n.peso` exigiria uma query nova de `popularity.weight` |
| `janela:` | v2 | só faz sentido com `tipo: agenda` |
| Resultado **dentro** do texto da nota | planejado | exige modo leitura / renderizador server-side |
| `{{ }}` inline no meio de um parágrafo | planejado | reusa o mesmo motor; depende do item acima |
| Várias ordenações (`mtime desc, titulo asc`) | fora do v1 | simplicidade do contrato |
| Ações (`mover para`, `aplicar tag`) | fora | a linguagem é **somente leitura** por decisão; ações serão recurso separado e com pré-visualização |
| `for` / variáveis / funções próprias | nunca | seria programação; CEL já cobre filtro e cálculo |
| Busca em conteúdo de PDF/EPUB | não | só metadados e texto indexado |
| Consultas em notas remotas / rede | nunca | o app é local e offline |

---

## 14. Changelog da especificação

| Versão | Data | Mudanças |
|---|---|---|
| v1 (rascunho aprovado) | 2026-10-06 | Especificação inicial: bloco `consulta`, chaves gerais, `tipo: notas/tarefas/hierarquia`, `onde:` em CEL, ordenação/limite, 5 apresentações, limites, receitas |
| v1 (implementada) | 2026-10-06 | Parser + motor CEL + executor + painel no editor (Fases 0–3 do §6.29 do DECISIONS). Ajustes do contrato contra o schema real: `n.interacao` (não "abertura"), `n.classe` = `nota/desenho/markmap/…`, `n.tamanho`/`n.palavras`/`n.peso` adiados; lista canônica de `colunas:` validada no parser |
| v1 (ajustes) | 2026-10-06 | `colunas:` passa a ser a **união dos campos de `n`/`t`** + calculadas (a lista própria recusava `arquivo`); leitor vira **snapshot por requisição** (medido: 3 blocos = 22 → 7 leituras, 35,7 ms → 13,2 ms) |

---

## Apêndice A — Cola rápida

```
tipo: notas | tarefas | hierarquia            (obrigatório)
onde: <expressão CEL>                         (opcional)
ordenar: campo [asc|desc]
limite: 1..200                                (padrão 20)
mostrar: lista | tabela | contagem | cartoes | arvore
colunas: [campo, ...]                         (só com tabela)

tags: [a, b]        sem-tags: [c]      texto: "fts5"
pasta: notes        marcador: [TODO]   estado: pendente|concluida|todas
de: Nome da nota
```

```
Operadores:  && || !  == != < <= > >=  + - * / %  in  ( )
Macros:      exists  all  exists_one  map  filter  has  size
Texto:       contains startsWith endsWith matches size lowerAscii upperAscii
Tempo:       agora  duration("720h")  timestamp("2026-01-01T00:00:00Z")
App:         dias(ts)  horas(ts)
n:           caminho titulo pasta classe tags mtime pai pai_resolvido filhas
             citada_por tarefas_abertas popularidade interacao
t:           texto marcador estado arquivo nota_titulo linha criado
```
