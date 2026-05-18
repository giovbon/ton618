## ✅ Projeto TON-618 v2 — Estrutura Final

### Estrutura atual

```
ton618/
├── cmd/
│   └── server/
│       └── main.go                    # Entry point, embed + HTMX server
├── internal/
│   ├── api/
│   │   ├── routes.go                  # Todas as rotas HTTP
│   │   ├── handlers.go                # Handlers: busca, CRUD, API, páginas, notas
│   │   ├── handlers_test.go           # Testes unitários (36 testes)
│   │   ├── middleware.go              # BasicAuth, Recovery
│   │   └── render.go                  # Server-side rendering com html/template
│   ├── config/
│   │   └── config.go                  # Config via env vars
│   ├── db/
│   │   ├── db.go                      # Conexão SQLite + schema (9 tabelas + FTS5)
│   │   ├── documents.go               # CRUD documentos
│   │   ├── fts.go                     # FTS5 busca + LIKE fallback
│   │   ├── state.go                   # Popularidade, tags, links, file_mods, settings
│   │   └── vectors.go                 # Embeddings (vector BLOB + encode/decode)
│   ├── processor/
│   │   └── markdown.go                # Parse YAML frontmatter, hashtags, wikilinks, headers
│   ├── search/
│   │   ├── search.go                  # Motor: FTS5 → LIKE fallback → re-rank → sort
│   │   └── ranker.go                  # Scoring com pesos (título, tag, frase, frescor, path)
│   ├── semantic/
│   │   ├── provider.go                # Interface + factory + cache
│   │   ├── gemini.go                  # Google Gemini embeddings
│   │   ├── ollama.go                  # Ollama local embeddings
│   │   └── openai.go                  # OpenAI embeddings
│   ├── template/
│   │   ├── embed.go                   # Embed dos templates HTML
│   │   ├── layout.html                # Base: Tailwind CDN + HTMX + TipTap condicional
│   │   ├── index.html                 # Página inicial com busca compacta + global
│   │   ├── search_results.html        # Partial com resultados + highlight
│   │   ├── editor.html                # Editor TipTap + frontmatter + bubble menu + slash
│   │   └── graph.html                 # Mapa semântico D3.js
│   └── watcher/
│       └── watcher.go                 # fsnotify + polling, processamento de arquivos
├── web/
│   ├── package.json                   # Dependências npm (TipTap, marked, esbuild)
│   ├── build.js                       # Build com esbuild
│   ├── src/
│   │   └── editor.js                  # Entry point: expõe TipTap + marked no window
│   └── static/
│       └── editor.js                  # Bundle gerado (servido em /static/editor.js)
├── go.mod
├── README.md
├── run.sh
├── deploy.sh
├── .env.example
├── Dockerfile
├── docker-compose.yml
└── LEGADO/                            # Projeto original (referência)
```

### Modos de Pesquisa

A aplicação possui **dois modos de busca** que o usuário alterna por um botão no topo:

| Modo | Ícone | Descrição | Endpoint | Filtro |
|---|---|---|---|---|
| **Compacto** (padrão) | ☰ Lista | Lista de notas por nome, ordenadas por data de modificação. Mostra tags do frontmatter e botão de exclusão. | `GET /api/notes` (JSON) | Client-side: filtra pelo nome do arquivo |
| **Global** | 🔍 Busca | Busca full-text no conteúdo das notas via FTS5. Retorna snippets com contexto do termo e destaque (highlight). | `POST /search` (HTML parcial) | Server-side: busca no índice FTS5 |

**Modo Compacto:**
- Carregado automaticamente ao abrir a página inicial
- Requisição `GET /api/notes` retorna JSON com `[{arquivo, tags, mtime}]`
- Ordenado por `mtime` decrescente (mais recentes primeiro)
- Filtro instantâneo via JS (sem requisição ao servidor)
- Destaque visual do termo digitado no nome do arquivo

**Modo Global:**
- Acionado ao clicar no botão de alternância
- Envia `POST /search` com `q=termo`
- Server-side: consulta FTS5, extrai contexto ao redor do termo, remove tags HTML
- Resultado: snippet limpo com ~200 chars ao redor do termo + destaque (highlight) client-side
- Termos de busca são destacados com classe `.search-highlight` (fundo azul)

### Sintaxe de Pesquisa (Modo Global)

O campo de busca global suporta os seguintes operadores:

| Operador | Exemplo | Efeito |
|---|---|---|
| **Palavra simples** | `golang` | Busca o termo em todas as colunas (texto, título, nome do arquivo, tags) |
| **Frase exata** | `"goroutines channels"` | Busca a frase exata (palavras adjacentes na ordem) |
| **Prefixo** | `concorren*` | Busca palavras que começam com "concorren" (wildcard automático para palavras > 2 chars) |
| **AND implícito** | `golang channels` | Todos os termos devem aparecer (comportamento padrão) |
| **Exclusão** | `golang -java` | Exclui notas que contenham "java" |
| **Tag** | `#urgente #programacao` | Filtra por tags específicas |
| **Filtro de tag** | `+tags:urgente` | Forma alternativa de filtrar por tag |

#### Detalhes técnicos

- **Stopwords** (de, da, do, em, no, na, um, uma, os, as, com, por, para, que, o, a, e, etc.) são ignoradas automaticamente
- **Case-insensitive:** a busca não diferencia maiúsculas de minúsculas
- **Prefix wildcard:** termos com mais de 2 caracteres ganham sufixo `*` automaticamente para busca parcial
- **Colunas:** cada termo é buscado em todas as colunas do índice FTS5 com pesos: `tags` (50×), `arquivo` (20×), `secao` (10×), `texto` (1×)

#### Exemplos

```
# Notas sobre concorrência em Go
goroutines channels

# Frase exata
"aprendendo goroutines"

# Excluindo resultados de Java
golang -java

# Filtrando por tag
#urgente golang

# Múltiplos termos com tag
#programacao django python
```

#### Modo Compacto

No modo compacto (padrão), o campo filtra apenas pelo **nome do arquivo** (client-side, sem requisição ao servidor). Tags podem ser usadas digitando `#`:

- `#tag` → mostra apenas notas com essa tag
- `#tag termo` → filtra por tag + texto no nome do arquivo
- `#tag1 #tag2` → mostra notas que tenham AMBAS as tags

### Virtualização (performance com muitas notas)

#### LEGADO (Preact + react-virtuoso)
O LEGADO usava `react-virtuoso` (biblioteca de virtualização React) para renderizar a lista de resultados de busca. Isso era necessário porque o Preact processava milhares de resultados no DOM, causando lentidão. A virtualização garantia que apenas os itens visíveis na tela fossem renderizados.

#### v2 atual (Vanilla JS)
Atualmente, a v2 **não tem virtualização**. A lista de notas no modo compacto é renderizada inteira no DOM. Isso é aceitável para até ~500 notas. Acima disso, pode causar lentidão.

#### Opções para implementar virtualização

**Opção 1: Paginação simples (recomendado inicialmente)**
- Adicionar botão "Carregar mais" ou scroll infinito
- Backend já suporta `from` e `size` na busca global
- Para o modo compacto, o `GET /api/notes` poderia aceitar `?from=0&size=50`
- Implementação puramente Go + JS, sem dependências
- **Prós:** Simples, sem novas dependências
- **Contras:** Não é virtualização real, apenas lazy loading

**Opção 2: Intersection Observer (nativo do browser)**
- Usar `IntersectionObserver` JS API para detectar quando o último item fica visível
- Carregar mais resultados automaticamente (scroll infinito)
- **Prós:** API nativa do browser, zero dependências
- **Contras:** Não virtualiza os itens fora da tela (DOM cresce)

**Opção 3: Virtualização manual com `content-visibility` (CSS)**
- CSS `content-visibility: auto` no Chrome: só renderiza itens próximos à viewport
- Combinado com `contain-intrinsic-size` para altura correta
- **Prós:** Zero JS, puro CSS, funciona no Chrome/Edge
- **Contras:** Não funciona no Firefox (precisa de polyfill ou fallback)

**Opção 4: Clusterize.js (biblioteca leve)**
- [Clusterize.js](https://clusterize.js.org/) - 5KB, vanilla JS, virtualização de listas
- Mantém apenas os elementos visíveis no DOM, recicla nós
- **Prós:** Leve, sem dependências, funciona em todos browsers
- **Contras:** Mais uma dependência (CDN ou bundled)

**Opção 5: Web Component customizado**
- Criar um `<virtual-list>` web component com virtualização
- Reutilizável em toda a aplicação
- **Prós:** Customizável, sem dependências externas
- **Contras:** Mais complexo de implementar

**Recomendação:** Começar com **Opção 1 (paginação)** combinada com **Opção 2 (Intersection Observer)** para scroll infinito. É a abordagem mais simples, sem dependências externas, e resolve o problema para a maioria dos casos de uso. Se houver necessidade de lidar com > 2000 notas, migrar para **Opção 4 (Clusterize.js)**.

### Comparação com o LEGADO

| Aspecto | LEGADO | v2 |
|---|---|---|
| **Motor de busca** | Bleve (~30 deps indiretas) | SQLite FTS5 (1 dep) |
| **Estado** | BBolt + JSONs + mapas mutex | SQLite (tabelas) |
| **Frontend** | Preact + Vite + 30 deps npm | HTMX + vanilla JS (0 deps npm) |
| **Frameworks** | React Query, Router, Tailwind build | Tailwind CDN, sem build step |
| **Editor** | TipTap via `@tiptap/react` | TipTap via bundle esbuild |
| **Virtualização** | react-virtuoso (Preact) | Nenhuma (planejado: paginação + Intersection Observer) |
| **Pesquisa compacta** | Requisição `POST` com `compact: true` | `GET /api/notes` (JSON) |
| **Pesquisa global** | Requisição `POST` com `compact: false` | `POST /search` (HTML parcial) |
| **Highlight** | Client-side via React | Client-side via `innerHTML` + regex |
| **Snippet** | Server-side com Bleve | Server-side: strip HTML + contexto ao redor do termo |
| **Frontmatter** | Editor inline no modal | Toggle FRONTMATTER no editor |
| **Bubble Menu** | `@tiptap/react` BubbleMenu | Vanilla JS popup com `mousedown` listener |
| **Slash Commands** | Extensão custom `SlashCommandExtension` | Função `makeSlashAction` + `deleteRange` |
| **WikiLinks** | Mention + WikiLinkNode | Autocomplete vanilla JS com `[[` trigger |
| **Backup** | 5 arquivos/pastas | 1 arquivo `ton618.db` |
| **Dependências Go** | ~30 indiretas | 3 (sqlite3, fsnotify, yaml.v3) |
| **Dependências npm** | 30+ | 16 (TipTap + marked + esbuild) |
| **Startup** | 2-3 segundos | < 100ms |

### Sistema de Pesos (Search Ranking)

O ranking dos resultados de busca combina **FTS5 BM25** (SQLite) com **re-ranker heurístico** em Go. O algoritmo é inspirado no LEGADO (Bleve + ScoreFragment).

#### Arquitetura

```
Usuário digita → buildFTSQuery(query)
  ↓
FTS5 busca com pesos por coluna: (tags:term OR arquivo:term OR secao:term OR texto:term)
  ↓
BM25 retorna `rank` bruto (score nativo)
  ↓
scoreFragment() re-ranking heurístico (8 fatores)
  ↓
Ordenação por FinalScore, desempate por timestamp
```

#### Pesos por coluna (FTS5)

Cada termo da query é expandido para buscar em todas as colunas com pesos implícitos:

| Coluna | Peso FTS5 | Efeito |
|---|---|---|
| `tags` | 50× | Match em tag tem relevância máxima |
| `arquivo` | 20× | Match no nome do arquivo |
| `secao` | 10× | Match no título/seção |
| `texto` | 1× | Match no corpo do texto |

#### Fatores do re-ranker (scoreFragment)

| # | Fator | Peso | Descrição | Fórmula |
|---|---|---|---|---|
| 1 | **BM25 Base** | ×1.0 | Score nativo do FTS5 | `hit.Score × BaseMultiplier` |
| 2 | **Título** | +10.0 / +4.0 | Match exato (+10) ou parcial (+4) no título | `10 × (BoostTitleExact ou BoostTitlePartial)` |
| 3 | **Frase exata** | +120% | A frase completa aparece no texto | `score × BoostPhrase` |
| 4 | **Caminho** | +0.5/termo | Match no nome do arquivo | `termos × BoostPathContext` |
| 5 | **Keywords** | +3.0 tag / +1.0 texto / +0.5 radical | Match em tag, texto ou radical (4 primeiras letras) | por termo |
| 6 | **Recência** | +0.5 → +0.25 → +0.1 | Decai conforme idade: <1d / <7d / <30d | `BoostFreshness × decay` |
| 7 | **Riqueza** | +1.0 + 0.5 (tabela/código) | Conteúdo estruturado (tabelas, código, palavras longas) | palavras longas > 8 chars |
| 8 | **Popularidade + Links** | log₂ | Bônus logarítmico baseado em acessos e backlinks | `log₂(count+1) × multiplicador` |

#### Pesos configurados (defaults)

```go
var weights = RankWeights{
    BaseMultiplier:     1.0,  // Score base do FTS5
    BoostTitleExact:    1.0,  // ×10 na prática (ver fórmula)
    BoostTitlePartial:  0.4,  // ×4 na prática
    BoostPathContext:   0.5,  // +0.5 por termo no arquivo
    BoostPhrase:        1.2,  // +120% do score atual
    BoostFreshness:     0.5,  // Máximo bônus para notas de hoje
    BoostTechnical:     0.5,  // Bônus para tabelas/código
    BoostLinkAuthority: 1.5,  // Multiplicador log₂(backlinks)
}
```

#### Cálculo do Score Final

```
Score Final = BM25_base
            + (title_match × 10)
            + (score_atual × BoostPhrase) se frase exata
            + (path_matches × 0.5)
            + keyword_bonus (tag=3, texto=1, radical=0.5)
            + recencia (0.5/0.25/0.1)
            + riqueza_estrutural
            + log₂(popularity+1)
            + log₂(linkCount+1) × BoostLinkAuthority
```

### Tags

As tags podem ser adicionadas às notas de duas formas:

#### 1. Via Frontmatter (recomendado)

No editor, clique em **FRONTMATTER** para expandir o editor de metadados YAML. Adicione as tags no formato:

```yaml
title: Minha Nota
tags: [golang, programacao, concorrencia]
```

Ou no formato de lista:

```yaml
title: Minha Nota
tags:
  - golang
  - programacao
```

#### 2. Sugestão de tags ao abrir FRONTMATTER

Ao expandir o FRONTMATTER no editor, uma lista de **tags disponíveis** (já usadas em outras notas, carregada de `GET /api/tags`) aparece abaixo do campo de texto. Clique em qualquer tag para adicioná-la automaticamente ao campo `tags: [...]` no frontmatter.

#### 3. Busca por tags

As tags são indexadas na busca global com **peso 50× maior** que o texto comum. Os operadores de busca por tag são:

| Operador | Exemplo | Efeito |
|---|---|---|
| `#tag` | `#urgente` | Busca notas com a tag "urgente" |
| `+tags:tag` | `+tags:programacao` | Forma alternativa |
| `#tag1 #tag2` | `#go #concorrencia` | Notas que tenham AMBAS as tags |

### Embeddings

Embeddings são vetores numéricos que representam o **significado semântico** do texto de uma nota. São gerados automaticamente via API do **Google Gemini** (modelo `gemini-embedding-2`, 768 dimensões).

#### Pipeline

```
[Arquivo .md modificado]
        ↓
watcher.ProcessFile(store, ev, embedProvider, embedAll)
        ↓
ProcessMarkdown() → parseia frontmatter, extrai texto
        ↓
shouldEmbed(tags, embedAll)?
  ├─ EMBEDDING_ALL=true → sim (sempre)
  ├─ tag "embed" no frontmatter → sim
  └─ senão → não
        ↓
embed.Embed(texto) → POST /v1beta/models/gemini-embedding-2:embedContent
        ↓
store.SetEmbedding(docID, vector) → SQLite (tabela embeddings)
```

#### Configuração

| Variável | Padrão | Descrição |
|---|---|---|
| `EMBEDDING_PROVIDER` | `gemini` | `gemini`, `ollama` ou `openai` |
| `EMBEDDING_API_KEY` | — | Chave da API (obrigatório para Gemini/OpenAI) |
| `EMBEDDING_MODEL` | `gemini-embedding-2` | Modelo de embedding |
| `EMBEDDING_DIM` | `768` | Dimensionalidade do vetor |
| `EMBEDDING_ALL` | `false` | `true` = embeda todas as notas automaticamente |

#### Armazenamento

Os vetores são armazenados na tabela `embeddings` do SQLite como BLOBs (4 bytes por float, little-endian):

```go
EncodeVector(vec []float32) → []byte
DecodeVector(data []byte) → []float32
```

#### Cache

O provedor de embedding implementa cache LRU em memória: textos já embedados são retornados sem chamar a API novamente.

#### Indicadores visuais

- **✦** verde = nota embedada
- **○** cinza = não embedada

Presentes no modo compacto (lista de notas) e no editor (ao lado do status).

#### WikiLinks

O editor suporta `[[wikilink]]` com autocomplete:
- Digitar `[[` abre dropdown com notas existentes
- Setas ↑↓ navegam, Enter/Tab seleciona, Escape fecha
- Notas carregadas de `GET /api/notes`
- O wikilink é salvo como `[[nota]]` no markdown

### Como rodar

```bash
# 1. Configurar
cp .env.example .env   # editar EMBEDDING_API_KEY

# 2. Build do bundle web (TipTap)
cd web && npm install && node build.js && cd ..

# 3. Rodar servidor
go run -tags sqlite_fts5 ./cmd/server/

# 4. Abrir
# http://localhost:6180
```

Ou usar o script:
```bash
./run.sh
```

### Testes

```bash
go test -tags sqlite_fts5 -v ./internal/api/
```

Atualmente **36 testes**:
- 9 testes do handler `/file/save`
- 3 testes do handler `/file/delete`
- 1 teste de rename flow (save + delete)
- 2 testes do handler `/api/notes`
- 1 teste de template com 21 sub-checks
- 2 testes de autenticação
- 2 testes de performance/limite

### Próximos passos opcionais

1. **Virtualização** — paginação + Intersection Observer para scroll infinito
2. **Stemming português** — RSLP no `processor/markdown.go`
3. **OCR de imagens** — reassimilar `ProcessImage` + Google Vision do LEGADO
4. **WikiLinks** — `[[link]]` com sugestão (Mention extension)
5. **Hashtags** — `#tag` highlighting no editor
6. **Docker** — Dockerfile + docker-compose prontos
7. **Sincronização de embeddings** — background após indexar
8. **Export/Backup** — download do `ton618.db` compactado
