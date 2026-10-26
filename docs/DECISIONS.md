> Este documento deve ser atualizado sempre que uma decisão arquitetural significativa for tomada.
> ⚠️ Qualquer IA que for modificar este projeto **deve** ler este documento primeiro.

# Decisões de Arquitetura — TON-618

Este documento registra decisões arquiteturais e padrões adotados no projeto.
Serve como referência para manter consistência em contribuições futuras.


## 1. Stack Principal

| Camada | Escolha | Motivação |
|--------|---------|-----------|
| Backend | Go 1.22+ com chi router | Performance, single binary, tipagem forte |
| Banco | SQLite (modernc.org/sqlite + sqlc) | Sem dependência externa, WAL para concorrência |
| Busca textual | FTS5 (sqlite built-in) | Zero setup, stemming em português via unicode61 |
| Templates | templ (github.com/a-h/templ) | Type-safe, compilado, substitui html/template |
| Frontend build | esbuild + Tailwind CSS | Zero config, rápido, tree-shaking nativo |
| Type checking (JS) | TypeScript via `tsc --noEmit` (checkJs) | Type checking incremental sobre JSDoc, sem transpilação separada |
| IDs | CUID2 (processor/cuid2.go) | Curto, único, ordenável, sem sequência |

## 1.2 Tecnologias e Bibliotecas

### Backend (Go)

| Tecnologia | Uso | Link |
|------------|-----|------|
| chi router | Roteador HTTP | https://github.com/go-chi/chi |
| modernc.org/sqlite | SQLite puro Go (sem CGo) | https://modernc.org/sqlite |
| sqlc | Geração de código Go a partir de SQL | https://sqlc.dev |
| a-h/templ | Engine de templates type-safe | https://github.com/a-h/templ |
| fsnotify | Watcher de sistema de arquivos | https://github.com/fsnotify/fsnotify |
| gopkg.in/yaml.v3 | Parse de frontmatter YAML | https://pkg.go.dev/gopkg.in/yaml.v3 |
| go-chi/httprate | Rate limiter | https://github.com/go-chi/httprate |
| CUID2 | Geração de IDs únicos | https://github.com/paralleldrive/cuid2 |

### Frontend (JavaScript/Browser)

| Tecnologia | Uso | Link |
|------------|-----|------|
| TipTap | Editor de markdown WYSIWYG | https://tiptap.dev/ |
| Tabulator | Tabelas de dados interativas | https://tabulator.info/ |
| Excalidraw | Editor de desenhos | https://excalidraw.com/ |
| CodeJar | Editor de código leve | https://medv.io/codejar/ |
| vis-timeline | Timeline para agenda | https://visjs.github.io/vis-timeline/ |
| chrono | Parser de datas em linguagem natural | https://github.com/wanasit/chrono |
| marked | Parse markdown para HTML | https://marked.js.org/ |
| lowlight | Syntax highlight de código | https://github.com/wooorm/lowlight |
| epub.js | Leitor de arquivos EPUB no navegador | https://github.com/futurepress/epub.js |
| jszip | Manipulação de arquivos zip (requerido pelo epub.js) | https://github.com/Stuk/jszip |
| HTMX | Interatividade sem JS escrito | https://htmx.org/ |
| Alpine.js | Interatividade declarativa | https://alpinejs.dev/ |
| Tailwind CSS | Framework CSS utility-first | https://tailwindcss.com/ |
| esbuild | Bundler e minifier | https://esbuild.github.io/ |

### Infraestrutura

| Ferramenta | Uso |
|------------|-----|
| Docker + Docker Compose | Containerização |

---

## 2. Padrões de Código

### 2.1 Go

- **Handlers em packages por domínio**: `internal/features/notes/`, `internal/features/search/` etc. — cada domínio tem seu handler, context e testes.
- **Store concreto, não interface**: `db.Store` é concreto. Repository interfaces existem só onde há benefício claro (testabilidade de serviços). Não criar interface só por "bom costume".
- **sqlc para queries SQL**: Queries em `query.sql`, geradas para `internal/core/db/generated/`. Evitar SQL espalhado no código.
- **Mutex para escrita**: `WriteMu sync.Mutex` no Store serializa escritas. Leituras concorrentes são livres (WAL).
- **Mtimes sempre em UTC (RFC3339 com `Z`)**: `time.Now().UTC().Format(time.RFC3339)`. Gravar mtime em formato local (ex: `-03:00`) misturado com UTC quebra a ordenação por string da sidebar/database. A ordenação em `system/handlers.go` usa `mtimeNewer()`, que compara `time.Time` parseado (robusto a formatos mistos/legados). Corrigido em 17/08/2026 (`HandleUploadAttachment` gravava local) com testes de regressão em `system/handlers_mtime_test.go` e `notes/handlers_upload_mtime_test.go`. **`mtimeNewer` é a única regra de ordenação por mtime** (22/10/2026): a árvore do Acervo usava uma cópia própria (`mtimeIsNewer`) que divergia em valores inválidos; a função canônica agora também entende o formato de exibição do Acervo (`2006-01-02 15:04:05`, hora local) e inválidos nunca "vencem".
- **Normalização de acentos é SSOT em `search.RemoveAccents`** (22/10/2026): o pacote `features/search` mantinha uma tabela `strings.NewReplacer` duplicada que já divergia — faltava mapear `Õ` (a cópia principal tinha `"õ"` no lugar). `RemoveAccents` apenas troca diacríticos (não faz case-fold); os callers combinam com `strings.ToLower`. Teste: `internal/search/accents_test.go`.
- **Testes com banco real**: `newTestStore(t)` cria SQLite em `t.TempDir()` — sem mocks, sem abstração.
- **Testes de integração no mesmo package**: `embedding_integration_test.go` testa fluxos completos (salvar → indexar → buscar → deletar).

### 2.2 Frontend (JavaScript)

- **JSDoc apenas em APIs públicas**: O que é exposto via `window.*` ou exportado como módulo. Funções internas não recebem JSDoc — evita ruído e documentação mentirosa.
- **`web/src/global.d.ts`**: Declarações de tipos para globais `window.*` (IIFE exports) e módulos CSS. Mantenha sincronizado com as funções expostas.
- **Arquivo fonte em `web/src/`, compilado para `web/static/`**: esbuild compila e minifica. `npm run build` gera os estáticos. **Nunca editar `static/` diretamente.**
- **IIFE para scripts no browser**: O build do esbuild usa `format: "iife"` para gerar código que não polui o escopo global além do que é explicitamente exposto.
- **Web Worker para tarefas pesadas**: `semantic-worker.js` (ESM module) executa inferência ONNX em thread separada — não bloqueia UI.
- **var, function, sem arrow functions nos fontes do browser**: O build target é es2020 e algumas páginas usam IIFE. Manter compatibilidade.
- **Ícones Lucide são inline SVG server-side**: Todos os ícones são renderizados como `<svg>` direto no HTML pelo `icons.templ`. **Não depende de JS do lado do cliente.** Ícones não reconhecidos viram um círculo genérico (fallback). O pacote npm `lucide` foi removido — zero dependência de JS para ícones.
- **Escaping HTML é SSOT em `EditorCommon.escapeHtml`** (`static/editor-common.js`, arquivo manual legado): `database.js`, `tabulator-note.js` e `editor-init.js` delegam para ele (null/undefined → `""`, escapa `& < > " '`). Antes havia 4 cópias com semânticas diferentes (uma não escapava `"`/`'` e outra não tratava null). Teste: `tests/tabulator-note.unit.cjs` carrega o `editor-common.js` real.
- **Uploads XHR passam por `uploadWithProgress`** (`web/src/app.js`, 22/10/2026): PDF (`POST /upload`) e anexo (`POST /api/upload-attachment`) compartilham auth (`setXHRAuth` → `EditorCommon.getAuthHeaders`), barra de progresso, redirecionamento e mensagens de erro. Coberto por `tests/app-uploads.unit.cjs` (eventos reais em DOM/XHR fake) e pelo guard estático `auth-headers.unit.cjs`.

### 2.3 Testes

- **Go**: Testes no mesmo package (`package db`, não `package db_test`) para acesso a funções não exportadas.
- **JS (Node)**: Arquivos `.mjs` com `async/await`, sem frameworks de teste. `node web/<teste>.mjs` executa direto.

## 3. Embeddings Semânticos — ❌ REMOVIDO (16/10/2026)

> ⚠️ **Este capítulo é HISTÓRICO.** Todo o sistema de embeddings (Transformers.js no browser,
> tabelas `note_chunks`/`note_embeddings`, busca semântica, busca híbrida e o "modo Perguntar")
> foi **removido do app** em 16/10/2026 — ver §12 (Remoção definitiva).
> Nada aqui descreve o comportamento atual do TON-618. O app hoje tem **dois** modos de busca:
> Busca de Notas (nome de arquivo) e Busca Global (FTS5 sobre o conteúdo).

### 3.1 Arquitetura (histórica)

```
Browser (Transformers.js) → POST /api/embeddings/save → SQLite (vec0)
Usa o modelo: Xenova/paraphrase-multilingual-MiniLM-L12-v2
```

- Geração **exclusivamente no browser** (Transformers.js no Web Worker). Não há pipeline servidor-side.
- Modelo: `Xenova/paraphrase-multilingual-MiniLM-L12-v2` (384 dims, q8 ~120MB).
- Cacheado no IndexedDB do browser após primeiro download.
- **⚠️ e5-small REVERTIDO (09/08/2026):** o `Xenova/multilingual-e5-small` foi testado e **descartado** — no Transformers.js v4 + q8 ele produz **embeddings colapsados** (cosine ~0.85 até para textos não relacionados; o MiniLM dá ~0.15), fazendo a busca semântica retornar muito ruído. O MiniLM-L12 discrimina corretamente neste ambiente.

### 3.2 Chunking

- `chunkText(text, maxChars=700, overlapChars=100)` em `web/src/semantic.js` (parâmetros espelhados no backend via `chunkMaxChars`/`chunkOverlapChars` em `db/embeddings.go`).
- Quebra por `\n` (parágrafo) se disponível nos primeiros 60% do limite.
- Fallback para espaço. Último recurso: corte seco no limite.
- Overlap de 100 caracteres preserva contexto entre chunks.
- **Redução 1500→700 (10/08/2026):** chunks menores melhoram a precisão da busca semântica/híbrida — notas longas (transcrições) deixam de virar um vetor médio ruidoso e o voto majoritário (3.6/8) fica mais discriminativo. O fingerprint `EmbeddingModelVersion` inclui os parâmetros, então a troca **invalida e reindexa automaticamente** no próximo boot.

### 3.3 Indexação

- **Lazy**: Só indexa quando o usuário abre a busca semântica.
- Título extraído do primeiro `# ` e prefixado em cada chunk.
- **Sem prefixos de instrução:** o MiniLM-L12 não usa `query:`/`passage:` (diferente do e5). Texto enviado ao modelo = título + conteúdo limpo.
- **Limpeza preserva quebras de linha:** o markdown é limpo colapsando apenas espaços/tabs (`[ \t]+ → " "`), mantendo os `\n`. Isso permite que `chunkText` quebre em parágrafos (evitando fragmentos cortados no meio da frase que geravam embeddings ruidosos). Corrigido em 09/08/2026.
- Markdown limpo antes do chunking: remove blocos de código, imagens, mantém só texto de links.
- `Promise.all` para paralelizar chunks de uma mesma nota.

### 3.4 Staleness

- `note_chunks.indexed_mtime` armazena o mtime da nota no momento da indexação.
- `GetPendingEmbeddingNotes()` compara com `notes.mtime` para detectar desatualizados.
- **Invalidação por versão de modelo:** o backend deriva um **fingerprint** do pipeline de embeddings (modelo + chunking) — `EmbeddingModelVersion`. Quando qualquer parâmetro muda, o fingerprint muda automaticamente e `EnsureEmbeddingModelVersion` limpa os embeddings antigos na inicialização, forçando re-indexação no browser. **Sem bump manual.** Alterada em: 09/08/2026 (retorno ao MiniLM-L12 após teste do e5-small).
- **⚠️ Cache do worker (bug real, 09/08/2026):** o `semantic.js` criava o Web Worker via `/static/semantic-worker.js` **sem hash** na URL — o navegador cacheia o worker antigo (MiniLM) e a reindexação gerava vetores do modelo errado. **Corrigido:** o servidor agora injeta `window.SEMANTIC_WORKER_URL` com hash via `staticver.URL()` no `layout.templ`, e o `semantic.js` usa essa URL. O hash muda automaticamente com o conteúdo — sem `WORKER_VERSION` manual.
- Notas não-indexáveis (drawing) são excluídas via SQL.

### 3.5 Notas Indexáveis vs Não-Indexáveis (Regras de Paridade)

- **Regra Geral:** Apenas notas de texto contínuo e leitura humana são indexáveis. Dados puramente estruturados, visuais ou códigos não são indexados.
- **Tipos Indexáveis:** Markdown comuns (`NoteTypeMarkdown`), notas de transcrição do YouTube (`NoteTypeYoutube`), artigos da Web (`NoteTypeArticle`), capturas rápidas (`NoteTypeCapture`) e notas semanais (`NoteTypeSemanal`, ver 6.12).
- **Tipos Não-Indexáveis:** Desenhos/Excalidraw (`NoteTypeDrawing`), arquivos/PDFs na pasta `pdfs/` (`NoteTypePDF`), anexos na pasta `attachments/` (`NoteTypeAttachment`) e notas arquivadas na pasta `archives/` (`NoteTypeArchive`).
- **Paridade Go/SQL:** O método Go `IsNoteEmbeddable` (que valida as gravações) e as queries SQL (`GetPendingEmbeddingNotes` e `CountEmbeddableNotes`) devem estar em perfeita paridade quanto a essa lógica de exclusão de notas. Para manter a performance, a detecção de tipo é baseada apenas no caminho do arquivo, tags e heurísticas de nome de arquivo.
- **Garantia via Teste:** O teste de integração `TestIsNoteEmbeddableMatchesSQL` garante que qualquer divergência futura entre Go e SQL na lógica de exclusão de notas quebrará os testes locais e o CI/CD. Adicionalmente, o teste `TestDeleteNoteCleansEmbeddingsAndOrphanStatus` garante que a remoção de notas limpa seus respectivos chunks e embeddings, e que o cálculo de status de indexação é resiliente a registros órfãos pré-existentes.
- **Correção de paridade (09/08/2026):** o teste `TestIsNoteEmbeddableMatchesSQL` pegou uma divergência real — a tag `deletar` era excluída no SQL (`CountEmbeddableNotes`/`GetPendingEmbeddingNotes`) mas não no Go. Corrigido em `isNoteEmbeddable`, que agora também exclui notas com a tag `deletar`.

### 3.6 SimilarNotes — Estratégia do Voto Majoritário

📍 `internal/features/notes/handlers_common.go` — função `loadNoteData`

> ❌ **REMOVIDO (16/10/2026).** Esta seção já estava **desatualizada mesmo antes da remoção**:
> nenhuma feature de "Notas Semelhantes" existia no código na data da remoção (a lógica
> descrita aqui não estava em `handlers_common.go`). Fica como registro histórico. Ver §12.

O recurso **"Notas Semelhantes"** no editor usa os embeddings armazenados para recomendar notas relacionadas. A lógica implementa:

- **Dois campos**: `minDistMap` (menor distância L2 por candidato) e `matchCounts` (em quantos chunks diferentes o candidato apareceu).
- **Threshold dinâmico**: Agora configurável pelo usuário na UI (padrão 72% de similaridade de cosseno, traduzido internamente para distância L2).
- **Voto majoritário**: Se a nota atual tem ≥3 chunks (nota longa), o candidato precisa ter match em ≥2 chunks diferentes para ser recomendado — a menos que a distância seja excepcional (`< 0.60`, ~82%).
- **Ordenação**: Primária por frequência de matches (decrescente), secundária por distância L2 (crescente). Top 5 resultados exibidos.
- **Parâmetros**: `similarExcellent = 0.60`, `longNoteMinChunks = 3`, `minMatchLongNote = 2`. O limite `similarThreshold` é obtido dinamicamente das configurações.

### 3.7 Configurações Dinâmicas de Limite Semântico (Threshold) — ❌ REMOVIDO (16/10/2026)

📍 Rota `/api/settings/semantic-thresholds` | `internal/features/system/handlers.go`

> ❌ A rota, os handlers, os sliders e as settings (`semantic_search_threshold`,
> `hybrid_semantic_threshold`, `rrf_k`, `semantic_device`) foram **removidos** — a migração
> v23 apaga essas chaves do banco. Ver §12.

Para dar controle sobre a precisão da IA, adicionou-se sliders de configuração na aba **Semântica**:
- **Busca Semântica Global**: Define a similaridade mínima exigida na busca geral (padrão 35%). Controla a tolerância de resultados em `internal/features/embeddings/handlers.go`.
- **Notas Semelhantes**: Define a similaridade mínima para a aba do rodapé do editor (padrão 72%). Controla a exibição em `internal/features/notes/handlers_common.go`.
- **Persistência**: Ambos os percentuais são armazenados no SQLite na tabela de configurações como `semantic_search_threshold` e `similar_notes_threshold`.
- **Conversão de Métrica**: O banco de dados utiliza distância euclidiana L2 (sqlite-vec MATCH). A conversão a partir de porcentagem de similaridade de cosseno $c$ ocorre pela fórmula:
  $$dist_{L2} = \sqrt{2 \times (1 - c)}$$
- **Alterada em**: 14/07/2026 — implementação dos thresholds dinâmicos e UI de sliders.
- **Alterada em**: 09/08/2026 — padrão da busca global definido em 35% (validado empiricamente com o e5-small); removido o mapeamento especial que tratava 50% como 20% (o valor armazenado agora é usado literalmente, faixa válida 10–100%).

> ⚠️ A busca global (FTS5 + semântica via `POST /api/embeddings/search`) é independente e não foi afetada.

### 3.8 Momento da Indexação Semântica — ❌ REMOVIDO (16/10/2026)

> ❌ Não existe mais indexação de embeddings (nem lazy, nem no save). Ver §12.

- A indexação de embeddings é **lazy** e ocorre somente quando o usuário abre a aba de busca híbrida.
- Salvar ou editar uma nota não deve iniciar geração de embeddings nem requisições para `/api/embeddings/save`.
- Ao abrir a busca híbrida, `indexPending` identifica notas pendentes ou desatualizadas e indexa o conteúdo mais recente antes da busca.
- **Decidido em**: 10/09/2026 — evita consumo de CPU e travamentos durante a edição; o salvamento da nota permanece independente da indexação semântica.

### 3.9 Mapa Semântico (Galáxia de Notas) — PCA 2D — ❌ REMOVIDO (16/10/2026)

📍 `internal/core/db/semantic_map.go` | `internal/features/embeddings/semantic_map_handler.go` | `web/src/semantic-map.js`

> ❌ **Já não existia no código antes desta data** (nenhum `semantic_map.go`, nenhum
> `semantic-map.js`, nenhuma rota `/api/embeddings/map` ou `/mapa-semantico`).
> Os ícones órfãos (`mapa-semantico`, `semantic-map`) também foram removidos. Ver §12.

**Adicionada em**: 22/07/2026

Visualização 2D interativa de todas as notas indexadas, reduzindo os embeddings de 384 dimensões para 2 via PCA (Análise de Componentes Principais).

#### Arquitetura

```
Go (PCA 384D→2D) → JSON /api/embeddings/map → Browser (SVG + Alpine.js)
```

- **PCA server-side em Go puro** (stdlib, sem dependências):
  - Centralização dos dados (subtração da média por dimensão)
  - Matriz de covariância 384×384 (divisão por N-1)
  - Power iteration para top-2 autovetores (50 iterações)
  - Deflação de Hotelling para o segundo componente
  - Projeção de cada embedding nos 2 componentes principais
- **K-means++ pós-PCA**: até 5 clusters para atribuir cores às bolinhas
- **Cache thread-safe**: `sync.RWMutex` + checksum FNV-1a dos filenames. Invalida quando o número de notas indexadas muda.

#### Rotas

| Método | Rota | Descrição |
|--------|------|-----------|
| GET | `/api/embeddings/map` | JSON com `{points, count}` |
| GET | `/mapa-semantico` | Página HTML com scatter plot SVG |

#### Frontend

- **SVG nativo** renderizado no browser (sem D3.js, Cytoscape.js ou qualquer biblioteca de gráficos)
- **Pan e zoom** via Alpine.js (rolagem do mouse e botões +, −, ⟲)
- **Tooltip** com nome da nota ao passar o mouse
- **Clique** na bolinha → abre o editor da nota
- **5 cores de cluster** (violeta, verde, amarelo, rosa, laranja) para distinção visual

#### Guard-Clauses e Robustez

- **N < 2 notas**: retorna mapa vazio (sem erro)
- **Embeddings idênticos** (matriz de covariância zero): todos os pontos em (0,0)
- **K-means**: `K = min(5, N)`, centróides vazios são recolocados
- **Cache duplo** com double-check locking: leituras concorrentes são livres, escritas exclusivas

#### Testes

📍 `internal/core/db/semantic_map_test.go` — 27 testes unitários e de integração:

| Categoria | Testes | Cobertura |
|-----------|--------|-----------|
| PCA | 6 | Guard-clauses, embeddings idênticos, 100 pontos, determinismo, agrupamento intra-cluster |
| K-Means | 5 | 2/3 clusters, k > N, lista vazia, 1 cluster |
| Cache | 5 | Checksum ordem-independente, sem colisão, 1000 chaves, thread safety (20R/5W) |
| Ágebra Linear | 4 | Normalização, power iteration, deflação, ortogonalidade |
| Integração DB | 5 | Banco vazio, com embeddings, cache hit/miss, apenas chunk #0 |

## 4. Banco de Dados

### 4.1 Tabelas Principais

| Tabela | Função |
|--------|--------|
| `notes` | Conteúdo markdown + mtime |
| `documents` | Fragmentos de documentos indexados (FTS5) |
| `docs_fts` | Índice FTS5 para busca textual |
| `tags` | Tags por arquivo |
| `links` | Wikilinks entre notas |
| `popularity` | Score de popularidade + peso RLHF |

### 4.2 Migrações

- `migrate()` em `db.go`: cada migração tem um número de versão e é registrada na tabela `schema_versions`.
- `isApplied(v)` + `markApplied(v)` garantem que cada migração execute **uma única vez**.
- Novo padrão: adicionar `if !isApplied(N) { ... markApplied(N) }` para cada nova migração.
- Não remover migrações antigas — o código permanece para referência histórica.

## 5. API

### 5.1 Rotas

- **chi router** com agrupamento por domínio.
- Rate limiters para endpoints pesados: `searchLimiter` (30/min) — aplicado em `/search` e `/api/search/*`.
- Prefixo `/api/` para rotas JSON, sem prefixo para páginas HTML.

### 5.2 Respostas

- JSON com `Content-Type: application/json`.
- Erros: `http.Error(w, mensagem, statusCode)` — mensagens descritivas em português.

## 6. Observações Técnicas

### 6.1 `chunkText` com `maxChars=0` — ❌ HISTÓRICO (16/10/2026)

> ❌ A função `chunkText` (e todo o pipeline de chunking) foi removida junto com o índice
> semântico. Registro histórico: `chunkText(text, 0, 0)` causava loop infinito porque `start`
> nunca avançava (`end - overlap = 0`).

### 6.2 WebGPU vs WebNN — ❌ HISTÓRICO (16/10/2026)

> ❌ Não há mais runtime ONNX no app. Registro histórico: o runtime tentava WebGPU primeiro
> (se disponível) e caía para CPU (WASM); WebNN nunca foi usado.

### 6.3 `process.on('unhandledRejection')`

Usado nos testes JS para silenciar rejeições intencionais (testes de `embed_error` e timeout). Não usar em produção.

## 6.5 Cache de Estáticos e Versionamento Automático

📍 `internal/core/staticver/staticver.go`

Arquivos estáticos (`web/static/`) são servidos com **ETags automáticos** (SHA256 do conteúdo) e `Cache-Control: immutable` por 1 ano.

- `staticver.URL("/static/arquivo.js")` gera URL com hash: `/static/arquivo.js?v=a1b2c3d4e5f6`
- Quando o arquivo muda, o hash muda → URL muda → browser baixa o novo
- **Não precisa mais incrementar `?v=N` manualmente** nos templates
- Chamar `staticver.SetDefault(cache)` no `main.go` para registrar o cache global
- Exceções (strings JS dentro de `<script>`): `codejar.js` ainda usa `?v=N` manual

## 6.6 Download e Distribuição do Modelo de IA — ❌ REMOVIDO (16/10/2026)

> ❌ Toda esta seção não vale mais: o download do modelo no boot, a rota pública `/models/*`,
> `MODEL_DIR`/`MODEL_DOWNLOAD` e o `LocalModel` foram removidos. Ver §12.

### Modelo baixado no boot e servido pelo backend (Opção B — navegador offline do CDN)

📍 `internal/features/embeddings/model_files.go` | `web/src/semantic-worker.js` | `cmd/server/main.go`

- O **backend Go baixa o modelo uma única vez, no 1º boot**, para o volume persistente
  `STATE_DIR/models` (padrão; configurável via `MODEL_DIR`) e o **serve ao navegador em
  `GET /models/*`** (rota pública, sem auth — o Web Worker do Transformers.js busca sem credenciais).
- O `semantic-worker.js` aponta `env.localModelPath = "/models/"` (antes `/static/models/`).
  Com o modelo local pronto, o navegador carrega do **próprio servidor** — sem depender do CDN.
- Download **assíncrono** no boot (`go modelStore.Ensure(...)`): o servidor sobe normalmente;
  enquanto o modelo não está pronto (ou se o HF estiver fora), o worker usa o CDN como fallback
  (`allowRemoteModels = true` — mantido de propósito, ver CSP abaixo).
- Escrita **atômica** (`.part` → rename) e **retomável**: arquivos já presentes (não vazios)
  não são re-baixados — um download interrompido continua de onde parou no próximo boot.
- Persistência: como o modelo fica sob `STATE_DIR` (volume `./data` no compose), o download ocorre
  **1x por instalação** e sobrevive a restarts/recreates.
- **Trade-off (decisão consciente):** a **imagem Docker não engorda** (segue ~36MB); o custo
  (~135MB) vai para o volume persistente. Caso um dia se queira **offline total sem o boot**
  (modelo embutido na imagem), seria a "Opção A": guardar só os `.br` em `web/static/models/`
  (imagem ~+71–74MB) — o `staticver` já serve `Content-Encoding: br` transparentemente.

### `web/download_model.js` (dev / benchmark / bundle offline)

- `download_model.js` continua existindo para **uso local de desenvolvimento e benchmark**
  (`semantic_benchmark.js` lê os arquivos crus de `web/static/models/`) e para gerar um
  **bundle offline** manual (copiar a pasta para o volume/`MODEL_DIR` em redes sem internet).
- ⚠️ **`wget` é obrigatório** — lidou melhor com o XetHub/CAS Bridge do que `fetch()` ou
  `http.get()` do Node. **Não substituir.** Já houve regressão por mexer neste script.

### Arquivos baixados

| Arquivo | Tamanho |
|---------|---------|
| `config.json` | ~700B |
| `special_tokens_map.json` | ~200B |
| `tokenizer.json` | ~2.5MB |
| `tokenizer_config.json` | ~500B |
| `onnx/model_quantized.onnx` | ~120MB |

### ⚠️ CSP e download do modelo (atualizado em 09/09/2026)

- O modelo local agora é servido na **mesma origem** (`/models/*`), e o `connect-src` do CSP em
  `internal/middleware/middleware.go` **já libera os CDNs do HuggingFace** (`https://huggingface.co`,
  `https://*.xethub.hf.co`, `https://*.hf.co`, `https://*.cdn.hf.co`, `https://cdn-lfs.huggingface.co`).
- Portanto o **fallback remoto continua funcionando** enquanto o download local não terminou/falhou:
  o navegador baixa do CDN e cacheia (CacheStorage/IndexedDB).
- Para um dia desativar o fallback remoto (offline estrito): restringir o `connect-src` e garantir o
  modelo sempre presente localmente (boot concluído ou pasta embutida na imagem).

### 6.7 Variantes do ONNX Runtime WASM (redução de imagem)

📍 `web/static/models/download-ort.js`

O `download-ort.js` copia **apenas a variante do ORT realmente usada** pelo Transformers.js, reduzindo `web/static/models/ort/` de ~74MB para ~22MB:

- **`ort-wasm-simd-threaded.asyncify`** (22MB): obrigatória — é a variante usada para inferência em CPU quando o servidor **não** envia COOP/COEP (numThreads forçado a 1).
- **Removidas (CPU-only, decisão 09/08/2026)**: `jsep` (WebGPU — o `semantic_device` foi fixado em `wasm`), `ort-wasm-simd-threaded` (base, exigiria cross-origin isolated/multi-thread) e `jspi` (experimental).

> ⚠️ Para reativar WebGPU no futuro, basta adicionar `ort-wasm-simd-threaded.jsep.wasm`/`.mjs` ao `ALLOWED` e voltar `semantic_device` para `auto`. Se o servidor um dia enviar COOP/COEP (multi-thread), incluir também a variante base `ort-wasm-simd-threaded.wasm`.

## 6.8 Auto-Tag por Inatividade — Aplicação Manual (10/08/2026)

📍 `core/internal/features/notes/auto_tag_service.go` | `core/internal/features/system/handlers.go` | `core/web/layout/settings_modal.templ`

O Auto-Tag (Notas Inativas) adiciona/remove tags em notas baseado na inatividade (regras configuradas pelo usuário: `X dias → tag`). A tag é removida se a nota voltar a ficar "jovem".

**Critério de inatividade (10/08/2026):** a referência é a **última abertura** da nota — `popularity.last_interacted_at`, atualizado a cada abertura no editor via `IncrementPopularity` (`focus_zoom`). Para notas nunca abertas (sem registro em `popularity`), o critério **cai para o `mtime`** (última edição). A tag é removida na **próxima aplicação**, quando a nota tiver sido aberta recentemente (abaixo dos dias configurados) — não no momento da abertura.

**Mudança (10/08/2026):** a aplicação **deixou de rodar automaticamente em background** (agendador com `time.Ticker` de 6h em `main.go` — **removido**). Agora a aplicação é **manual**, disparada pelo usuário:

- **Rota:** `POST /api/settings/auto-tag/apply` → `HandleApplyAutoTag` chama `notes.ApplyDecayTags(store, noteSvc)` de forma **síncrona** e retorna `{"status":"success","modified": N}`.
- **Corpo opcional (10/08/2026):** o endpoint aceita as regras no corpo (mesmo formato de `POST /api/settings/auto-tag`). Se enviadas, são validadas/salvas e usadas nesta aplicação. O botão "Aplicar Tags Agora" envia as regras digitadas na tela — **não exige clicar em "Salvar Regras" antes**. Sem corpo, usa as regras já salvas.
- **`ApplyDecayTags`** agora retorna `(int, error)` — o `int` é a quantidade de notas cujas tags foram alteradas (antes só retornava `error`). Internamente busca `store.GetAllLastInteracted()` (mapa `arquivo → last_interacted_at`) para decidir a inatividade, com fallback para `mtime`.
- **UI:** botão verde **"▶️ Aplicar Tags Agora"** na aba Arquivamento das configurações, ao lado de "Salvar Regras". Ao clicar, o botão mostra spinner e o feedback `"✓ N notas atualizadas!"` (ou `"✓ Nenhuma nota precisou de atualização."`). Após aplicar, dispara o evento `reload-sidebar` (via `document.body`) para atualizar as tags na sidebar.
- **Motivação:** dar controle explícito ao usuário sobre quando as tags são aplicadas, em vez de alterações ocorrerem "sozinhas" em segundo plano.
- As regras continuam sendo salvas via `POST /api/settings/auto-tag` e lidas via `GET /api/settings/auto-tag` (armazenadas em `auto_tag_decay_config`).

## 6.9 Ícones de Notas — Fonte Única de Verdade (SSOT, 10/08/2026)

📍 `core/internal/core/domain/data.go` | `core/internal/ui/icons/config.go` | `core/internal/features/system/handlers.go` | `core/web/src/database.js`

**Problema resolvido:** notas de um tipo mudavam de ícone "sem motivo" porque o tipo e o ícone eram decididos em **múltiplos lugares com entradas diferentes**:
- `DetectNoteType` era chamado **com conteúdo** (handlers de editor, busca) e **sem conteúdo** (sidebar, banco, embeddings, `NoteIcon`). Uma nota sem a tag de tipo persistida tinha tipos diferentes conforme o caminho → ícones diferentes.
- O `database.js` tinha uma **duplicação client-side** (`detectNoteType` + `getLucideIcon`) com SVGs hardcoded que divergiam do servidor (ex: `pin` renderizava ícone de mapa no cliente e de pin no servidor).

**Refatoração (Fonte Única de Verdade):**
- **`DetectNoteType(tags, arquivo)` agora NÃO recebe conteúdo** — só tags persistidas + caminho + nome. Determinístico em toda a aplicação.
- **`DetectNoteTypeFromContent(tags, content, arquivo)`** é a variante com conteúdo, usada apenas pelos handlers de editor (decidir qual editor abrir) e pelo backfill.
- **`NoteService.EnsureTypeTags`** (rodado no `SyncDatabase`/startup) **persiste a tag canônica** de tipo na tabela `tags` para notas cujo tipo vinha só do conteúdo (ex: `type: mermaid` sem tag `mermaid`). Depois do backfill, `DetectNoteType` (sem conteúdo) é correto em 100% dos casos. Canônicas: `NoteTypeCanonicalTag`.
- **`icons.GetColor` determinístico**: mapa reverso `ícone → cor` construído uma vez com chaves ordenadas (elimina dependência da ordem de iteração de mapas em Go).
- **API do banco envia o SSOT**: `HandleGetDatabaseData` agora inclui por linha `_icon` (SVG pronto via `icons.SVGString`), `_url` e `_blank` (via `domain.NoteOpenTarget`). O `database.js` apenas injeta esses campos no formatter `abrir_link` — **removeu-se** `detectNoteType`, `getLucideIcon` e `resolveColor` (duplicação client-side).
- **Campos internos nunca são colunas**: `_blank`, `_icon`, `_url` (prefixo `_`) são helpers exclusivos do formatter `abrir_link` e **não entram no `columnSet`** — o `HandleGetDatabaseData` ignora qualquer chave que comece com `_` ao montar as colunas dinâmicas do Tabulator. Os campos continuam presentes nos dados das linhas. Garantido pelo teste `TestHandleGetDatabaseData_InternalFieldsNotColumns`.
- **Paridade**: o ícone da sidebar (server-side) e o da página do Banco de Dados agora vêm da MESMA fonte (SVG do `icons.SVGString`).

## 6.10 Web Capture — tag canônica `captura` (09/09/2026)

📍 `core/internal/features/notes/capture_service.go` | `handlers_capture_test.go`

Toda nota criada pelo **Web Capture** (artigo ou transcrição de YouTube) agora nasce com a tag `captura` no frontmatter:

```yaml
---
tags: [captura]
---
```

- **Motivação:** identificar visualmente na listagem (sidebar/Tabulator/busca) que a nota veio de uma captura. A tag faz o tipo virar `NoteTypeCapture` (via `DetectNoteType`), que renderiza o **mesmo ícone do Web Capture** (`Config["captura"]` → ícone `link`), em vez do ícone padrão de nota.
- **Implementação:** a montagem do markdown foi extraída para funções puras (`buildYouTubeMarkdown`/`buildArticleMarkdown`) usando a constante única `captureTag = "captura"` — evita divergência entre os dois fluxos e permite teste determinístico sem rede.
- **Escopo:** vale para capturas **novas**. Capturas antigas salvas sem tag não foram retroativamente marcadas (não há como identificá-las com segurança — sem tag nem prefixo de nome).

## 6.11 Qualidade no CI — gofmt, go vet e typecheck do frontend (09/09/2026)

📍 `.github/workflows/deploy_core.yml` | `internal/core/db/db.go`

O job de **teste** do CI passou a exigir, além do `go test ./...`:
- **`gofmt -l`** — a árvore Go inteira (cmd/internal/web) ficou 100% formatada (normalização única em 09/09/2026);
- **`go vet ./...`** — antes não era executado em lugar nenhum do pipeline;
- **typecheck do frontend** (`npm run typecheck` → `tsc --noEmit`) — antes existia só como script local (`npm run check`), nunca no CI.

**Motivação:** gaps encontrados numa revisão de maturidade (09/09/2026). O `go vet` apontava 2 problemas reais em `db.go`:
1. `WithRequestContext` copiava `sync.Mutex` por valor (vet: *literal copies lock value*) — era **código morto** (construía uma cópia e a descartava). Removido junto com os símbolos sem uso `StoreKey`/`requestCtxKeyType`.
2. `queryCtx()` descartava o `cancel` de `context.WithTimeout` (vet: *lostcancel*). O `cancel` agora é registrado (`queryPending`) e invocado no `Store.Close()`, com poda para não crescer sem limite.

## 6.12 Nota Semanal — tipo próprio e nome determinístico (15/09/2026)

📍 `core/internal/core/domain/data.go` | `core/internal/features/notes/handlers_weekly.go` | `core/internal/features/notes/handlers_weekly_test.go` | `core/internal/ui/icons/config.go` | `core/web/layout/navbar.templ`

O menu **Criar** da navbar ganhou o item **`+ Semana`** (`GET /semana`), que abre a nota da semana corrente.

- **Nome determinístico (não aleatório):** `notes/<ano>-S<semana>.md`, usando o **ano-semana ISO-8601** (`time.Time.ISOWeek()`), ex: `notes/2026-S38.md`. O sufixo numérico usa **2 dígitos** (`S01`..`S53`) para manter a ordenação lexicográfica da sidebar/database.
  - Por que ISO-8601: é o único esquema com 1..53 semanas consistentes (a semana começa na segunda-feira) e com **ano-semana explícito** — em 29/12/2025 o nome correto é `2026-S01`, não `2025-S53`. Testado em `TestWeeklyNoteFilename`.
- **Idempotência por semana:** como o nome é estável, clicar no botão N vezes na mesma semana sempre reusa a MESMA nota — nunca duplica nem sobrescreve o conteúdo já editado (`TestHandleWeekly_IdempotenteNaMesmaSemana`).
- **Criação sob demanda e nota EM BRANCO:** `HandleWeekly` cria a nota na primeira abertura da semana (depois só redireciona para `/editor?file=...`). O conteúdo inicial é **só uma quebra de linha** (`domain.WeeklyNoteEmptyContent`), ou seja: **sem título, sem frontmatter e sem tag**.
  - Por que não é `""`: `NoteService.processAndSave` recusa conteúdo vazio ("conteúdo vazio"). Essa proteção impede sobrescrever notas com branco e **não deve ser afrouxada** só para a nota semanal.
  - Por que sem título: a nota é uma página em branco de propósito — o nome exibido vem do próprio nome do arquivo (`domain.DisplayName`).
  - Por que sem tag: o tipo já é derivado do **nome do arquivo**, então persistir a tag seria redundante (e poluiria a sidebar com um chip `#semanal`).
- **Novo tipo `NoteTypeSemanal = "semanal"`**, detectado de forma **100% determinística** por dois caminhos (tags têm prioridade):
  1. tags `semanal`/`semana`/`weekly`;
  2. **nome do arquivo** (`IsWeeklyNoteFilename`, regex ancorada `^\d{4}-S\d{2}$` com semana 1..53).
  O caminho 2 é o **único** caminho ativo na prática: é ele que faz a nota já nascer com o tipo/ícone corretos antes de existir conteúdo salvo.
- **Ícone próprio:** `Config["semanal"]` → Lucide `calendar-days`, cor `#14B8A6` (teal-500, distinta de `agenda` sky/sky-400 e de `markmap` verde). O SVG foi adicionado nos **dois** renderizadores (`icons.templ` para `@icons.Icon` e `icons.go` para `icons.IconSVG`) — esquecer um deles faz o resultado da busca cair no ícone de fallback.
- **Editor:** `NoteTypeSemanal.EditorRoute()` é `/editor` (é uma nota markdown comum) e `NoteTypeCanonicalTag` retorna `semanal`. `NoteTypeSemanal` está em `isTypeTaggedNoteType`, então `EnsureTypeTags` **não** injeta a tag automaticamente.
- **Paridade de embeddings:** `isNoteEmbeddable` (Go) e as queries `CountEmbeddableNotes`/`GetPendingEmbeddingNotes` (SQL) **não excluem** notas semanais — elas devem participar da busca semântica assim que tiverem conteúdo. O caso foi adicionado a `TestIsNoteEmbeddableMatchesSQL`.
  - Nota: enquanto a nota está em branco o cliente envia **um chunk vazio** (`semantic.js`: `if (rawChunks.length === 0) rawChunks = [""]`), então ela é marcada como indexada e **não fica pendente para sempre**.
- **`IsDraftName` não reconhece o nome semanal** como rascunho: `2026-S38` não segue o padrão diceware — o nome é estável e nunca deve ser tratado como descartável.

## 6.13 Colar no editor — evitar markdown "sujo" (15/09/2026)

📍 `core/web/static/editor-common.js` | `core/web/src/editor-init.js` | `core/web/tests/editor-common.unit.cjs`

**Problema:** ao colar conteúdo copiado de outros programas, as marcações cruas (`**negrito**`, `*itálico*`) apareciam no editor como texto sujo.

**Causa:** muitos programas (VS Code, leitores de PDF, terminais, painéis de IA) colocam no clipboard um `text/html` que é apenas o **mesmo texto embrulhado em `<div>`/`<p>`, sem formatação real** ("HTML de fachada"). O handler antigo só interpretava o markdown quando o clipboard **não tinha HTML** (`text && !html`); existindo HTML de fachada, o ProseMirror inseria o texto literal, asteriscos incluídos.

**Regra adotada** (`EditorCommon.shouldPasteAsMarkdown(text, html)`), usada pelo `handlePaste`:

| Clipboard | Comportamento |
|---|---|
| Texto puro com sintaxe markdown | Converte via `marked` → formatação real (comportamento anterior mantido) |
| Texto puro sem markdown | Cola como texto |
| HTML **com** formatação real (`strong`, `em`, `h1-6`, `ul/ol/li`, `table`, `pre/code`, `img`, `a`, `del`, ...) | **HTML ganha** — preserva a formatação original |
| HTML **sem** formatação real ("de fachada") + texto com markdown | Converte o **texto puro** (é o que evita os asteriscos crus) |

- Tags puramente estruturais (`div`, `p`, `br`, `span`) **não** contam como formatação — são exatamente as que aparecem no HTML de fachada.
- **`Shift`+colar = texto puro** (convenção do navegador): o handler retorna `false` e o ProseMirror insere o texto cru, sem interpretar nada. É a saída manual para quem quer garantidamente sem formatação.
- A decisão é uma **função pura** em `editor-common.js` (arquivo mantido à mão em `static/`, **não** é output de build) justamente para ser testável em Node (`web/tests/editor-common.unit.cjs`).
- ⚠️ `editor-common.js` **não** é gerado pelo `build.js` (não é entrypoint do esbuild) — edite o arquivo em `static/`. Já `editor-init.js` é gerado de `src/` e exige rebuild do bundle.

### 6.13.1 "Espaço morto" entre parágrafos ao colar (15/09/2026)

📍 `EditorCommon.normalizePastedHtml` | `handlePaste` em `src/editor-init.js`

**Problema:** ao colar texto, apareciam **grandes espaços entre os parágrafos**, obrigando a reformatar na mão.

**Causa (não é CSS):** o CSS do editor é enxuto (`p { margin: 0.3em }`, `li p { margin: 0.2em }`), então a margem não explica o vão. O que aparece são **linhas em branco inteiras** (line-height 1.7) criadas por lixo de estrutura vindo do clipboard:

| Lixo no clipboard | O que vira no editor |
|---|---|
| `<p></p>`, `<div></div>`, `<p><br></p>`, `<p>&nbsp;</p>`, `<div>&nbsp;</div>` | uma **linha vazia** por bloco |
| `<p><span>&nbsp;</span></p>`, `<p><span style="…"></span></p>`, `<div><font> </font></div>` | uma **linha vazia** por bloco (bloco "vazio" com wrapper inline dentro) |
| `<br><br>` (ou mais, com/sem `&nbsp;` no meio) | uma linha vazia |
| `<br>` solto entre blocos: `<p>a</p><br><p>b</p>` | uma linha vazia |
| `<br>` nas bordas do parágrafo: `<p>a<br></p>`, `<p><br>a</p>` | linha extra no parágrafo |
| Lista "loose": `<li><p>único parágrafo</p></li>` | parágrafo extra dentro do item |
| `<script>`/`<style>` colados junto | conteúdo indesejado na nota |

**Correção:** `normalizePastedHtml(html)` roda **antes** do `insertContent` e, em **laço até estabilizar** (remover o de dentro pode esvaziar o de fora):
1. remove `<script>`/`<style>`/`<meta>`/`<link>`;
2. remove **inline vazio** (`<span></span>`, `<span> </span>`, `<span>&nbsp;</span>`, `<font></font>`, …);
3. remove **bloco vazio** (`p`/`div`/`h1-6`/`li`/`blockquote`/`section`/`article`/`header`/`footer`/`figure` só com espaço, `&nbsp;` ou `<br>`);
4. colapsa `<br><br>+` em um único `<br>`;
5. remove `<br>`/espaço **órfão** antes de abrir e logo depois de fechar um bloco;
6. remove `<br>`/espaço **nas bordas** de um bloco (quebras NO MEIO do parágrafo são preservadas);
7. desembrulha `<li><p>x</p></li>` → `<li>x</li>` (com lookahead, para **preservar** itens com 2+ parágrafos);
8. remove `<li>` que ficou vazio.

**Fora de propósito (whitespace significativo):** `pre`, `code` e células de tabela (`td`/`th`/`tr`) **nunca** entram nas listas de remoção — um `<td></td>` vazio é estrutura, não espaço morto.

Regras de segurança: a função **só remove espaço vazio e desembrulha wrappers — nunca descarta texto com conteúdo**, e se nada mudar o `handlePaste` devolve `false` (o fluxo normal do ProseMirror segue intacto, sem mudança de comportamento).

- Testes em `web/tests/editor-common.unit.cjs` (blocos vazios, inline vazio, quebras órfãs, listas loose, preservação de `<pre>`/`<td>`/conteúdo, e um cenário real completo de colagem de painel de IA).
- Se algum caso específico de colagem ainda gerar vão, o ajuste é **acrescentar uma regra na função** (e um teste), não mexer no CSS do editor.

> ⚠️ **Ao testar mudanças em `web/static/`:** o `staticver` calcula os hashes/ETags **no boot** e serve com `Cache-Control: immutable`. Se o servidor continuar rodando, o browser **não** pega o arquivo novo (ETag velho → `304`). **Reinicie o servidor** e recarregue a página. No Docker os estáticos vêm da **imagem** (os volumes montam só `docs/` e `data/`) — exige rebuild da imagem.

## 6.14 Renomear a nota aberta sem recarregar a página (15/09/2026)

📍 `core/web/src/editor-init.js` | `core/web/static/editor-common.js` | `core/web/tests/editor-common.unit.cjs`

**Problema:** renomear o título da nota com ela aberta (`#file-name`) disparava `window.location.href = "/editor?file=…"`, ou seja, um **reload completo** do navegador — perdia a posição de rolagem/cursor e o estado do editor.

**Correção:** o rename passa a ser aplicado **na própria página** por `EditorCommon.applyRenameToUI`, que sincroniza apenas o que depende do nome:

| O que | Como |
|---|---|
| URL | `history.replaceState` (mantém o histórico limpo e o F5 no lugar certo) |
| Input do nome | `value` + **`data-filename`** — é a fonte usada por salvar/excluir/duplicar (`getCurrentFilename`) |
| Título da aba | mantém o prefixo atual (`Editor - `, `Desenho - `, …) lido do próprio `document.title` |
| Sidebar (HTMX) | dispara o evento **`reload-sidebar`** já existente (mesmo usado pelo modal de configurações) |

**Quando o reload é mantido:** só quando o novo nome joga a nota para **outro editor**. O backend escolhe a rota pelo nome (`domain.DetectNoteType`: `mindmap`/`markmap` → `/mindmap`, `drawing`/`desenho` → `/drawing`), então `EditorCommon.renameChangesEditor(newName, currentBase)` retorna `true` nesses casos e o caller redireciona. A checagem é **conservadora de propósito** e considera a rota atual: renomear para `meu-desenho` a partir de `/editor` recarrega; a partir de `/drawing` não.

- Vale para o editor markdown (`editor-init.js`) **e** para desenho/markmap (`EditorCommon.doRenameContent`).
- Nada de conteúdo é regravado de forma diferente: a ordem continua `POST /file/rename` → `POST /file/save` → atualização da UI (antes: reload).
- ⚠️ Não duplicar as regras de tipo do Go no JS: `renameChangesEditor` é só um **guarda de rota** (na dúvida, recarrega). A fonte da verdade continua sendo `domain.DetectNoteType`.
- Testes: `renameChangesEditor` (nome normal/especial, rota atual, case-insensitive) e `applyRenameToUI` (URL via replaceState, input/`data-filename`, título preservando prefixo, evento da sidebar e ambiente sem DOM).

## 6.15 Contagem de tarefas no cabeçalho + marcador fora da contagem (15/09/2026)

📍 `core/internal/features/todos/badge.templ` | `handlers_markers.go` | `core/internal/core/db/db.go` | `core/internal/core/db/todos.go` | `core/web/layout/navbar.templ`

**Contagem no ícone Task:** o badge é carregado pelo próprio HTMX (`GET /api/todos/count`), não pelo handler da página — assim o cabeçalho continua sendo um componente sem parâmetros (`layout.Navbar()`) e o número pode ser atualizado de qualquer lugar.

| Peça | Detalhe |
|---|---|
| Container | `<span id="todos-badge" hx-get="/api/todos/count?r=<token>" hx-trigger="load, todos-updated from:body" hx-swap="innerHTML" hx-sync="this:replace">` (desktop e mobile, com ids distintos) |
| Fragmento | `todos.TodoBadge(count)` — **só o conteúdo** (a pílula). Contagem 0 não renderiza nada |
| Ao mexer nos marcadores | **swap out-of-band**: a resposta do `/api/todo-markers/*` traz `MarkersList` (alvo) **+** `TodoBadgeOOB(count)` com `hx-swap-oob="innerHTML:#todos-badge"` / `#mobile-todos-badge` — o HTMX atualiza os dois badges na MESMA resposta |
| Ao salvar nota | evento **`todos-updated`** (`document.body.dispatchEvent` no save do editor) — aqui não existe requisição HTMX de onde pendurar o OOB |
| Ao excluir/arquivar/restaurar | `hx-on::after-request="tonRefreshTodosBadge()"` nos botões que removem ou recriam linhas de `todos` (sidebar, arquivar/excluir em massa, restaurar backup). Sem isso a contagem ficava velha até um reload completo |
| GET `/api/todos/count` | responde com `Cache-Control: no-store` + a URL carrega `?r=<token>` gerado por `badgeCacheBuster()` em cada render da página (contagem dinâmica **nunca** pode vir do cache do browser) |

> 🚨 **CAUSA RAIZ dos “números errados/travados” (encontrada em 15/09/2026):** o `src/app.js` tinha uma função **legada** `updateTodosCount()` que sobrescrevia `#nav-todos` com `innerHTML = "<svg>… Task N"`. Isso **removia o `<span id="todos-badge">` do DOM** (com seus atributos `hx-*`) em TODA carga de página. Efeitos: (a) o número exibido no desktop vinha de `/api/todos?type=all&status=pending&format=json`, que **ignora `count_in_badge` e `active`** — medido no banco real do usuário: badge correto **1**, endpoint legado **2** (DONE com “Contar” desmarcado continuava contando); (b) sem o container, o swap OOB caía em `htmx:oobErrorNoTarget` e o evento `todos-updated` não tinha listener — o badge era o **único** elemento que nunca se atualizava; (c) no mobile o número era injetado no rótulo `TASK N`, duplicando a contagem. **Não reintroduzir JS imperativo no badge**: a responsabilidade é do HTMX e a guarda é `web/tests/todos-badge.unit.cjs`.

> ⚠️ **Nomes de evento do htmx são camelCase.** O vendor é o htmx **1.9.12**, que dispara `htmx:sendError` / `htmx:responseError` / `htmx:timeout` — **não** existe versão kebab-case. `hx-on::send-error` registrava o listener em um evento que nunca ocorre, então o “?” de servidor fora do ar **não** aparecia em falha de rede (só funcionava o `hx-on::timeout`, que já estava em camelCase). Guarda em `web/tests/todos-badge.unit.cjs`.

> ⚠️ **Já foram bugs reais (15/09/2026):** (1) o badge era atualizado só por `HX-Trigger: todos-updated` + `hx-trigger="... from:body"` e podia ficar no valor antigo — na verdade isso era sintoma do `innerHTML` do app.js (acima); (2) mesmo com a contagem certa no banco, a página exibia número velho por cache. Hoje o caminho dos marcadores é **dirigido pela própria resposta** (OOB), a URL da contagem tem **cache-buster** e `hx-sync="this:replace"` descarta resposta fora de ordem.

> ⚠️ **Ao investigar “continua contando”:** confira se o servidor está vivo antes de olhar a tela. O `run.sh` roda o binário em **primeiro plano** (`./ton618 | tee`), então **Ctrl+Z suspende o app** (saída 148 = 128+SIGTSTP) e a página continua mostrando dados velhos. Cheque com `ps aux | grep '[t]on618'`: `Z`/`<defunct>` = morto, `T` = suspenso. `./run.sh` de novo resolve (ele já faz `fuser -k "$PORT"/tcp`) — ou `nohup ./run.sh > /dev/null 2>&1 &` para não prender o terminal.

⚠️ O swap é **`innerHTML`** de propósito: com `outerHTML` o elemento que dispara a busca seria substituído e perderia os atributos `hx-get`/`hx-trigger` — o badge atualizaria **uma única vez**. O container precisa sobreviver à troca.

**Quem conta:** `todo_markers.count_in_badge` (coluna nova, migração **v12**, default **1** = comportamento anterior preservado). A contagem usa `db.CountTodosByMarkers(markers)` com os marcadores que estão **ativos E com `count_in_badge`**:
- desmarcar **Contar** nas configurações tira o marcador do número (ex: `DONE` fora da contagem) **sem afetar a listagem**;
- checkboxes comuns de markdown (tipo `TASK`) **nunca** entram, pois não são marcadores.

**Fora da listagem:** a página 🎯 TODOs passou a **ignorar itens do tipo `TASK`** (`- [ ]` / `- [x]` de markdown) em `HandleListTodos`, e o botão de filtro `TASK` foi removido da UI. A extração continua gravando esses itens na tabela `todos` (nada foi perdido), apenas a listagem e a contagem os ignoram.

**Escrita dos marcadores (reescrito em 15/09/2026):** os handlers **não** fazem mais read-modify-write da tabela (`GetTodoMarkers` → altera em memória → `SaveTodoMarkers`, que é `DELETE`+`INSERT` de tudo). Agora existe update pontual: `Store.UpdateTodoMarker(marker, TodoMarkerUpdate{...})`, `Store.AddTodoMarker` (INSERT OR IGNORE idempotente) e `Store.RemoveTodoMarker`. Motivos: (a) duas requisições concorrentes do painel (um checkbox por linha, sem `hx-sync`) perdiam uma das alterações; (b) erro de leitura era ignorado (`markers, _ := …`) e o `SaveTodoMarkers` seguinte apagava a configuração inteira; (c) `UPDATE` inexistente virou **404** e nome inválido **400** — antes o painel re-renderizava com o valor antigo como se nada tivesse falhado. `SaveTodoMarkers` continua existindo apenas para o “Restaurar padrão” (substituição completa é a intenção).

**Nome/cor de marcador são validados** (`processor.IsValidTodoMarkerName` / `IsValidTodoMarkerColor`): letras (com acento), números, espaço, `_`, `-`, `.`, até 32 chars; `TASK` é reservado (contaria checkboxes que não aparecem em lugar nenhum). A cor precisa ser `#rrggbb` (vai para um `style` inline).

> 🚨 **Marcador vira regex — `regexp.QuoteMeta` é obrigatório.** `getTodoRegex` interpola os marcadores em `(?i)^\s*(TODO|DOING):\s*(.+)$`. Sem escapar, um marcador com metacaractere (`(`, `[`, `*`…) fazia o `regexp.MustCompile` **entrar em panic** — e como o panic acontecia dentro de `ExtractTodos` **antes** do `SaveNote`, a nota **não era salva** e nenhuma outra conseguia ser salva enquanto o marcador existisse (500 via `middleware.Recovery`). `.`/`?`/`+` viravam curinga silencioso (criavam tarefas para linhas como `qualquer coisa: valor`). O `QuoteMeta` é a proteção de fato: bancos antigos podem ter marcadores gravados antes da validação. Testes em `processor/markdown_test.go` (`TestExtractTodos_MarcadorComMetacaractereNaoPanica`, `…NaoViraCuringa`).

> ⚠️ **`ExtractTodos(markers nil)` ≠ `ExtractTodos(markers []string{})`.** `nil` = “não informado” → usa `DefaultTodoMarkers`; slice **vazio** = “nenhum marcador ativo” → **não detecta nada**. Antes o vazio caía nos defaults e o app continuava criando tarefas de `TODO/DOING/DONE` **desativados**. Por isso `NoteService.processAndSave` usa `make([]string, 0, len(activeMarkers))` — `var markers []string` fica `nil` quando não há marcador ativo.

**Nomes de marcador nas URLs do painel são escapados** (`markerURL` em `markers.templ`, com `url.QueryEscape`): marcador com `&`/`#` truncava a query, `+` chegava como espaço e `%` invalidava o escape — em todos os casos o clique no checkbox “Contar” (ou ativo/cor/ordem/excluir) não fazia nada, sem aviso.

**Performance/estabilidade:** `hasMarkerColumn` (PRAGMA `table_info`) é **memoizado por Store** (`sync.Once`) — era consultado até 4× por chamada, e o badge chama `GetActiveTodoMarkers` a cada carga de página.

**Upgrade de banco existente (validado):** um banco sem a coluna e sem o registro da v12 sobe normalmente, a migração adiciona `count_in_badge` com default 1 (marcadores antigos continuam contando) e não há erro no boot. `db.CountInBadge` é lido com as colunas opcionais resolvidas via `PRAGMA table_info` (`hasMarkerColumn`), então o app também funciona se a coluna ainda não existir.

**Testes:** `internal/features/todos/markers_count_test.go` (contagem, 404/400, idempotência, escape de URL no painel), `internal/core/db/todos_count_test.go` (update pontual, add/remove idempotentes, memoize), `internal/processor/markdown_test.go` (metacaractere, nome/cor) e `web/tests/todos-badge.unit.cjs` (guarda do frontend: app.js sem innerHTML no badge, `sendError` camelCase, `innerHTML` no container, OOB dos dois badges).

## 6.16 Remoção do módulo de Ajuda/Documentação (16/09/2026)

📍 `core/internal/features/system/` | `core/web/layout/navbar.templ` | `core/web/build.js`

O módulo de ajuda foi **removido por completo** (decisão do usuário: o ícone de ajuda do header não é mais usado). O que saiu:

| Peça | O que era |
|---|---|
| `features/system/docs.templ` | página `/help`: TOC gerada em JS + `marked` via CDN + blocos `prose` do Tailwind |
| `features/system/embed.go` | `//go:embed help.md`, `TemplatesFS` e `HelpMD()` |
| `features/system/help.md` | conteúdo da documentação |
| `handlers.go` | `HandleHelp` e `HandleHelpMarkdown` |
| `cmd/server/routes.go` | `GET /help` e `GET /api/help/markdown` |
| `navbar.templ` | ícone de ajuda (desktop) e a entrada `TUTORIAL` do menu mobile |
| `ui/icons/config.go` | chaves `ajuda`/`help` (ícones de utilitários do header) |

> ℹ️ O resolvedor de SVG (`icons.templ`/`icons.go`) **mantém** os nomes `help`/`help-circle`/`circle-help`: eles também servem para nomes de notas (`GetIcon`), não são exclusivos da página de ajuda.

> ⚠️ **Efeito colateral tratado:** a página de ajuda era a **única** usuária da utility Tailwind `prose` (plugin typography). O `build.js` tinha uma "validação de sanidade" que exigia as substrings `green-500` e `prose` no `static/app.css` para provar que o `@source` de `internal/features` estava sendo escaneado. Verificou-se que o `.prose` presente no CSS vem do `@tailwindcss/typography` (emitido **independente de uso**, 328 ocorrências no build com a página já removida) — ou seja, essa exigência **não provava nada** sobre o scan do backend e ainda mascarava a remoção. A checagem agora exige apenas `green-500`, que vem de `features/system/handlers.go` (template do backend).

## 6.17 Hierarquia de Notas — Nested Data Tree no Tabulator (16/09/2026)

📍 `core/internal/features/system/note_tree.go` (novo) | `.../handlers.go` | `core/web/src/database.js` | `core/internal/features/notes/database.templ`

As notas agora podem ser **aninhadas** umas dentro das outras, e o Tabulator exibe essa hierarquia como uma árvore (setinha para abrir/fechar), usando o **módulo Data Tree** do Tabulator 6.x — que já vem embutido no bundle vendorizado (`static/tabulator.min.js`, v6.3.1).

> 🔁 **Revisto em 6.19 (mesmo dia, depois do primeiro uso):** a chave canônica virou **`pai`** (coluna "Pai"), o editor de célula foi corrigido (o `editor` em função precisa **devolver um nó**, não o nome do editor — por isso a coluna de hierarquia não era editável), abrir a nota passou a ser **duplo clique** (coluna "Abrir" oculta), o contador do topo mostra **quantas notas estão sendo exibidas** e a busca ganhou **dois modos** (com pai/irmãs ou só resultados), deixando de usar `table.setFilter`. Nesta seção, onde se lê `parent`/coluna "Parent" e `setFilter`, vale o 6.19.

### O vínculo é uma propriedade do frontmatter

A nota **filha** declara a nota-mãe (a nota-mãe não muda nada):

```yaml
---
tags: [sqlite]
parent: projeto-ton618
---
```

| Regra | Decisão |
|---|---|
| Chave | `parent` (minúscula; a leitura tolera `Parent:`). Exibida como coluna "Parent". **Alterado em 6.19: a chave canônica passou a ser `pai`; `parent` continua sendo lido (legado).** |
| Valor canônico | **nome curto**, sem pasta e sem `.md` |
| Formas aceitas na leitura e na gravação | `seila`, `seila.md`, `notes/seila.md`, `[[seila]]`, `[[seila\|alias]]`, `[[seila#secao]]`, com ou sem aspas, qualquer caixa |
| Gravação | o valor é **normalizado** (`NormalizeParentRef`) antes de ir para o YAML — a célula editada no Tabulator grava sempre a forma canônica |
| Pais por nota | **1** (árvore, não grafo) |
| Profundidade | **ilimitada** |
| Pai inexistente | **permitido**: a filha simplesmente fica na raiz até a outra existir (sem erro na UI) |
| Auto-pai e ciclo | **bloqueados** na gravação (HTTP 400) e desfeitos na leitura (segurança) |

**Nome duplicado** (existem pastas `notes/`, `pdfs/`, `archives/`, `epubs/`): vence `notes/`; em empate, o mtime mais recente. Digitando o caminho completo (`parent: pdfs/seila`) a resolução é exata. A escolha é feita varrendo a *slice* de entrada — nunca a ordem de iteração de map (mesmo cuidado do `icons.GetColor`, ver 6.9).

> ⚠️ **Cuidado com YAML (descoberto em teste manual, 16/09/2026):** `parent: [[nota]]` **sem aspas** não é string em YAML — vira uma **lista aninhada** (`[["nota"]]`) e `parent: [nota]` uma lista de um item. O resolver tolera (o valor cai no fallback `fmt.Sprintf` → `NormalizeParentRef` remove os colchetes), então **aninhar funciona**, mas a célula do Tabulator receberia um objeto. O frontend agora stringifica valores não primitivos (antes: célula vazia + "Format Error" no console). **Formato recomendado: `parent: nota`** (sem colchetes) ou `parent: "[[nota]]"` com aspas.

### Backend

- `buildNoteTree(rows)` (em `note_tree.go`) reorganiza as linhas **achatadas** do `HandleGetDatabaseData` em floresta, injetando `_children` — o campo que o módulo Data Tree lê. O JSON ganhou `meta: {hasTree, total}`; `hasTree=false` devolve as linhas como estavam (zero custo para quem não usa a propriedade).
- 🔴 **Armadilha real (testada):** o `dbCache` do Tabulator guarda o **map da linha por referência** (`context.go`). Escrever `_children` ali faria o cache acumular filhos a cada request (linhas duplicadas na tabela). Por isso `buildNoteTree` devolve **cópias rasas** dos nós e nunca escreve nos maps cacheados — `TestBuildNoteTree_DoesNotMutateInputRows` e `TestHandleGetDatabaseData_Tree` (dois requests seguidos) travam essa regressão.
- `_children` nunca vira coluna: prefixo `_` já é ignorado pelo `HandleGetDatabaseData` (convenção do SSOT de ícones, 6.9). Já a coluna **`parent` existe sempre** (mesmo sem nenhuma nota aninhada), porque é por ela que a primeira hierarquia é criada direto na tabela.
- **Validação de integridade** (`validateParentAssignment`, chamada em `HandleUpdateNoteProperty` quando a chave é `parent`): recusa auto-pai e ciclo. Para isso sobe a cadeia lendo o frontmatter das notas (`parentRefOf`), com memo e teto de 100 níveis (erra para o lado seguro). Pai inexistente **não** bloqueia — é o comportamento combinado (a nota fica na raiz).
- Ciclo na **leitura**: `hasParentCycle` anda pela cadeia com um set de visitados, então a caminhada sempre termina; o vínculo que *fecha* o laço é descartado e a nota volta para a raiz. Como a varredura é feita na ordem das linhas, o resultado é determinístico (a dupla vira uma cadeia de 2 níveis).

### Frontend (`database.js`)

- `dataTree`, `dataTreeChildField: "_children"`, `dataTreeElementColumn: "titulo"` (a setinha fica no título, não na coluna "Abrir"), `dataTreeChildIndent: 18`, `dataTreeBranchElement: false` e `dataTreeSort: false` (filhos mantêm a ordem do servidor; só o nível raiz é ordenado).
- **Sem paginação no modo árvore** (`pagination: hasTree ? false : "local"`): filhos contam como linhas e uma página cortaria uma família no meio.
- **Estado de expansão por nota** em `localStorage` (`db_tree_expanded`), atualizado pelos eventos `dataTreeRowExpanded`/`dataTreeRowCollapsed`. Primeiro nível começa aberto.
- ⚠️ **Nunca use `dataTreeExpandElement`/`dataTreeCollapseElement`** (erro cometido e corrigido no mesmo dia): no Tabulator 6.3.1 esses options **substituem o controle inteiro** — o `div.tabulator-data-tree-control` (com o `tabIndex` e o layout `inline-flex` do tema) **deixa de existir** e o HTML fornecido é inserido direto na célula, sem CSS de sizing (o SVG apareceu com 221px). O controle padrão é o caminho correto; o dark mode é ajustado por CSS em `database.templ`, que depende justamente do wrapper `.tabulator-data-tree-control`. Verificado no bundle (`web/static/tabulator.min.js`): `t.dataTreeCollapseElement ? (… this.collapseEl = e.firstChild) : (… crie div + classList.add("tabulator-data-tree-control"))`.
- **Busca com árvore (bug real, corrigido):** o filtro padrão escondia o resultado — buscando "sqlite" (nota de 2º nível) a tabela ficava **vazia**, porque o pai não casava e era removido do render, levando o filho junto. Agora o predicado é `nodeOrDescendantMatches`: um nó permanece visível se ele **ou algum descendente** casar (os filhos vêm em `_children` no dado da linha), e os nós visíveis são **auto-expandidos** ao filtrar. Resultado: `sqlite` → `projeto → backend → sqlite`.
- **Editar o `parent` de uma célula remonta a tabela** (`reloadTable()` → `destroy()` + `initTabulator()`): o Tabulator não recalcula a árvore sozinha. Colunas visíveis, última busca e expansão vivem no `localStorage`, então nada se perde.
- A restauração da última busca (`db_last_query`) passou a esperar o evento **`tableBuilt`** (`table.initialized` como atalho): antes o `setFilter` rodava antes da tabela estar pronta e o Tabulator avisava *"Table Not Initialized"* no console.
- `populateColumnCheckboxes(cols, table, lockedFields)` ganhou colunas travadas: no modo árvore a coluna **Título** não pode ser ocultada (ocultá-la desativaria o módulo Data Tree).
- O filtro de busca **ignora chaves com prefixo `_`** (antes, `_children` entrava no `for...in` e virava `"[object Object]"` na comparação de termos gerais).
- Formatter genérico das colunas editáveis **stringifica valores não primitivos** (`String(val)`): devolver objeto/array faz o Tabulator limpar a célula e logar "Format Error" — era o caso do `parent` em lista.
- Contador do topo passou a usar `meta.total`: com árvore, `data` contém só as raízes.

### Busca e embeddings

O filtro continua sendo 100% client-side e acha a nota **independente da indentação** (com o ajuste de ancestrais descrito acima). `parent` é indexado no FTS como qualquer outro campo do frontmatter — nada muda na busca semântica/híbrida.

### Validação (16/09/2026)

Toda a feature foi validada **executando o app** (sandbox isolado: `DOCS_DIR`/`DB_PATH`/`STATE_DIR` em `core/data/e2e-*`, porta 6199), já que Go e Node não existem no Windows do desenvolvedor:

- **Go** (container `golang:1.26-alpine`, com `templ generate` pinado): `go vet` OK, `go test ./...` verde, `go build -tags sqlite_fts5` OK, `gofmt` limpo.
- **Frontend** (Node do nvm no WSL Ubuntu): `npm run typecheck` limpo, `node build.js --dev` OK, `npm test` 31/31.
- **E2E no navegador**, com 6 notas de teste: aninhamento de 3 níveis (`projeto → backend → sqlite`), `parent: [[PROJETO]]` (lista aninhada) resolvido, órfã na raiz, ciclo e auto-pai recusados com 400, expandir/recolher pelo controle, expansão persistida no reload, busca por nota de 2º nível mostrando o caminho, e **zero avisos de console**.

### Testes

`core/internal/features/system/note_tree_test.go` — aninhamento de 3 níveis, órfã/auto-pai/ciclo voltando para a raiz, formato de referência (`[[alvo]]`, alias, âncora, caminho, aspas), `parent` como lista YAML (aninhada e simples), desempate de nome duplicado (pasta `notes/` e mtime), não-mutação das linhas de entrada, payload do Tabulator (`_children` presente, `_children` ausente das colunas, `parent` presente), tabela plana sem `parent`, normalização na gravação e recusa de auto-pai/ciclo (13 testes).

### Pendências (escopo futuro, combinado com o usuário)

- ✅ **Implementadas em 6.21 (28/09/2026):** botão **"Nota-mãe"** e contagem/exibição de filhas, ambos no editor e na sidebar; **rename do pai propaga o `pai:` das filhas** (elas não viram mais órfãs) com aviso na UI.

## 6.18 Fins de linha — `.gitattributes` com LF (16/09/2026)

📍 `.gitattributes` (novo)

**Problema:** desenvolver pelo checkout do Windows (VS Code) e **rodar pelo WSL** quebrava em vários pontos, todos pela mesma causa — o `core.autocrlf=true` do Git deixa a cópia de trabalho com **CRLF**:

| Sintoma | Causa |
|---|---|
| `./run.sh` → `bad interpreter: /bin/bash^M` | shebang com CR |
| `go run templ@v0.3.1020` → `version "v0.3.1020\r" invalid` | o `run.sh` extrai a versão com `grep`/`awk` do `go.mod` e o CR entra na variável |
| `gofmt -l .` listando o repo inteiro | gofmt considera CRLF como fora de formato |
| risco no `docker build` | `\` de continuação de linha + CR |

**Decisão:** declarar LF na cópia de trabalho para os arquivos de ferramenta:

```
*.sh  *.go  *.templ  *.yml  *.yaml  go.mod  go.sum  Dockerfile   →  text eol=lf
```

- O **índice do Git já era LF** em 464 dos 465 arquivos (só `DECISIONS.md` é `i/crlf`), então normalizar a cópia de trabalho foi **no-op de conteúdo** — confirmado por `git diff --stat` mostrando apenas as mudanças reais.
- ⚠️ **`DECISIONS.md` fica fora da lista** de propósito: como está commitado com CRLF, declará-lo LF geraria um diff de arquivo inteiro no próximo checkout.
- Consequência prática: os 150+ arquivos normalizados apareceram como `M` no `git status` (cache de stat), mas com conteúdo idêntico; `git add -A`/commit limpa a listagem e nada extra é commitado.
- Artefatos gerados pelo build (`web/static/*.js`) **seguem fora** do `.gitattributes` — a convenção de revertê-los após um build local (ver 6.15) continua valendo.

## 6.19 Tabulator — hierarquia `pai`, edição das células e busca em dois modos (16/09/2026)

📍 `core/internal/features/system/note_tree.go` | `.../handlers.go` | `core/web/src/database.js` | `core/internal/features/notes/database.templ` | `core/web/layout/navbar.templ`

Ajustes pedidos depois do primeiro uso da árvore (6.17).

### A chave da hierarquia passa a ser `pai`

| Regra | Decisão |
|---|---|
| Chave canônica | **`pai`** — coluna **"Pai"** no Tabulator |
| Leitura | aceita `pai` **e** o nome antigo `parent` (qualquer caixa): notas gravadas antes continuam aninhadas |
| Gravação | **sempre** em `pai`; a chave `parent` é removida do frontmatter na primeira edição da célula (migração silenciosa) |
| Linha do Tabulator | `normalizeParentKey` move qualquer variação da chave para `pai` antes de montar as colunas |

- `isParentKey` centraliza "canônica ou legada" (usado no `HandleUpdateNoteProperty`); `parentRefFromMap` procura na ordem `pai` → `parent`.
- `normalizeParentKey` roda **antes** do `columnSet` (é de lá que saem as colunas dinâmicas): sem isso, uma nota que ainda usa `parent` apareceria com **duas** colunas de hierarquia ("Pai" vazia + "Parent" preenchida).
- Testes: `TestNormalizeParentKey` (caixa alta, legada, ambas presentes), `TestHandleUpdateNoteProperty_LegacyParentKeyMigratesToPai`, `TestHandleUpdateNoteProperty_RemovalClearsLegacyKey` e `TestHandleGetDatabaseData_Tree` (payload só com `pai`, rótulo "Pai").

### Editar célula: o editor adaptativo virou editor customizado (bug real)

🔴 **Causa raiz:** a opção `editor` de uma coluna aceita **string** (editor embutido) ou **função que é o próprio editor** — e nesse caso ela precisa devolver um **nó do DOM**. O código devolvia o *nome* do editor ("input"/"date"/"number"), então o Tabulator tratava a função como editor customizado e falhava:

```
Edit Error - Editor should return an instance of Node, the editor returned: input
```

Só `Título` e `Tags` (editor embutido) eram editáveis; **a coluna de hierarquia não era**.

- Agora `autoEditor(cell, onRendered, success, cancel)` monta o `<input>` conforme o valor (checkbox para booleano, `date` para `AAAA-MM-DD`, texto para o resto — números e listas YAML continuam texto, e o `handleCellEdit` converte) e **devolve o nó**.
- Enter/blur confirmam, Esc cancela; a trava `finished` impede o commit duplo (`success()` fecha a célula, o que dispara o `blur` do input).
- Valor inalterado devolve o valor original: nada é gravado à toa.

### Abrir a nota: duplo clique (coluna "Abrir" nasce oculta)

- O clique simples numa célula editável abre o editor, então abrir a nota é **duplo clique na linha**: o 1º clique já abriu o editor e o duplo clique cai dentro do input, então o handler **cancela a edição** (`cancelEdit`) e navega.
- Cliques no controle da árvore e em links (`Abrir`) são ignorados.
- A coluna `abrir_link` continua no payload, mas com `visible: false` — quem preferir o link explícito religa no seletor "Colunas" (a escolha vive no `localStorage`).
- Teste: `TestHandleGetDatabaseData_AbrirColumnHiddenByDefault`.

### Contador do topo

Mostra **apenas o número** de notas exibidas agora (antes: "N notas registradas", que era o total e ignorava a busca). O valor sai da própria floresta filtrada (`countRows`), então bate exatamente com a tela — inclusive no modo contexto, em que pais e irmãs também são linhas.

### Busca em dois modos (botão ao lado do campo)

| Modo | O que mostra |
|---|---|
| **Pai + irmãos** (padrão) | o resultado + a cadeia de pais + as irmãs dele |
| **Só resultados** | apenas as notas que casaram (promovidas à raiz quando o pai não casou) |

- Escolha persistida em `localStorage` (`db_search_mode`); o botão mostra o modo ativo e o `title` explica o próximo clique.
- ⚠️ **O filtro deixou de usar `table.setFilter`** e passou a reconstruir a floresta e chamar `setData`. Motivo: o módulo Data Tree **esconde os filhos de uma linha filtrada** — era o que obrigava o predicado a manter os ancestrais (ver 6.17) e o que tornaria impossível o modo "só resultados" (um resultado aninhado desapareceria). Reconstruindo os `_children` no cliente, a árvore exibida é exatamente o conjunto calculado e o contador sai de graça.
- Com busca ativa, `dataTreeStartExpanded` devolve `true` para todos os níveis: o caminho até o resultado (e as irmãs) aparece sem abrir nível por nível. O estado salvo por nota (`db_tree_expanded`) não é afetado — a expansão automática não gera os eventos que persistem a escolha.
- A digitação tem **debounce de 120 ms** (`SEARCH_DEBOUNCE_MS`): cada tecla recalcula e chama `setData`, e sem agrupamento uma tabela com milhares de notas travaria.
- A sintaxe avançada da busca (`titulo:`, `tags:`, termos gerais) é a mesma de antes — só mudou o caminho de aplicação.

### Tabulator fora do mobile

O link `#mobile-nav-database` **saiu do menu mobile** (árvore com várias colunas não é utilizável no celular). O acesso continua no menu desktop (`#nav-database`) e pela URL `/database`.

## 6.20 Fontes self-hosted, TOC bidirecional e fim do "&nbsp;" (17/09/2026)

📍 `core/web/static/fonts/` (novo) | `core/web/src/input.css` | `core/web/layout/layout.templ` | `core/internal/features/{system/login,notes/editor,notes/database,notes/mindmap,notes/epub_reader}.templ` | `core/web/src/editor-init.js` | `core/web/src/editor.js` | `core/web/static/editor-common.js`

### Fontes: Noto Sans + Geist Mono, self-hosted (sem CDN)

| Peça | Decisão |
|---|---|
| Tokens | `--font-sans: "Noto Sans", …` e `--font-mono: "Geist Mono", …` no `@theme` de `web/src/input.css` — o preflight e todas as classes `font-sans`/`font-mono` do Tailwind seguem daí (fonte única de verdade) |
| Arquivos | `static/fonts/NotoSans/` e `static/fonts/GeistMono/` (woff2 **variáveis**: um arquivo por subconjunto/estilo cobre 100–900) |
| `@font-face` | `static/fonts/fonts.css`, com os mesmos `unicode-range` do Google (o browser baixa só o subconjunto usado) |
| Peso | ~490 KB no repo, mas uma página em pt-BR baixa **~60 KB** (latin normal de cada família); o resto é lazy |
| ⚠️ | **NÃO reintroduzir `<link>` do Google Fonts** (layout.templ/login.templ): o app precisa funcionar offline e a CSP é restritiva |
| Onde linkar | `layout.Layout` (via `staticver.URL`) e as páginas com `<head>` próprio (`login.templ`, `epub_reader.templ`) |
| CSS legado | `'Inter'` e as pilhas `ui-monospace, …` hardcoded em templates viraram `var(--font-sans)` / `var(--font-mono)` |

### TOC bidirecional (editar ≠ só criar)

`applyToc` só sabia mapear linhas sobre os títulos existentes (`min(linhas, títulos)`), então **apagar uma linha não removia nada da nota** — e o alinhamento dependia de um `offset` calculado na mão. Agora:

- `collectHeadings()` é compartilhado por `updateToc`/`applyToc` (títulos com texto, na ordem do documento) — título vazio é ignorado nos dois lados, senão as linhas sairiam de sincronia com os títulos;
- as mudanças (nível, texto, **remoção** e criação) são aplicadas **de trás para frente**, em uma única transação, usando as posições do documento original — uma alteração nunca invalida a posição da anterior;
- linha apagada → o título correspondente é **removido** da nota; linha nova → título criado no fim; tabulação → nível 1..6;
- se nada mudou de verdade, **nenhuma transação é disparada** (não marca a nota como suja nem agenda save);
- `updateToc` só escreve no textarea quando o valor muda — atribuir o mesmo valor jogaria o cursor para o fim enquanto o usuário digita no TOC.

### Fim do "&nbsp;" nas notas (bug real)

🔴 **Causa:** a extensão `CustomParagraph` (`web/src/editor.js`) serializava parágrafo vazio como o **texto literal** `&nbsp;`. A entidade ia crua para o arquivo (e, escapada pelo serializador, como `\&nbsp;`), reaparecendo como texto visível na busca, em previews e ao reabrir a nota.

- A extensão foi **removida** (e o `paragraph: false` do StarterKit junto): o parágrafo vazio volta à serialização padrão do `tiptap-markdown`, que produz uma linha em branco — que é o que o markdown sabe representar.
- `EditorCommon.stripNbspParagraphs()` limpa linhas que só têm o placeholder nas **três** formas (`&nbsp;`, `\&nbsp;`, U+00A0) e é aplicado **na abertura da nota e antes de cada gravação** — as notas antigas se limpam sozinhas no primeiro save. `&nbsp;` no meio de uma frase (uso legítimo) é preservado.
- Guarda: 3 casos em `web/tests/editor-common.unit.cjs` (limpeza, preservação no meio da frase, entrada vazia/nula).

### Validação (17/09/2026)

- Go: `templ generate` + `go vet` + `go test ./...` verdes; binário recompilado.
- Frontend: `tsc` limpo, `npm test` **34/34** (inclui os 3 novos casos do `&nbsp;`).
- E2E no navegador (sandbox isolado, porta 6199): fontes carregando dos arquivos locais (`document.fonts.check` = true para Noto Sans e Geist Mono, com o frontmatter/TOC em Geist Mono), nota com `&nbsp;` e `\&nbsp;` gravados abrindo **sem** o lixo, TOC renomeando/removendo/criando títulos (e o markdown salvo refletindo cada caso) e parágrafo vazio novo gravando linha em branco, sem `&nbsp;`.

## 6.21 Hierarquia: rename propaga `pai:`, barra do editor, prefixos e modelo (28/09/2026)

📍 `core/internal/features/notes/{hierarchy.go,hierarchy_bar.templ,note_service.go,handlers_file.go,handlers_editor.go,editor.templ,sidebar_tree.templ}` | `core/internal/features/system/{note_tree.go,handlers_core.go}` | `core/internal/core/domain/data.go` | `core/internal/features/embeddings/model_files.go` | `core/web/src/{editor-hierarchy.js,editor-frontmatter.js,editor-init.js}`

Fecha as pendências do 6.17 e os itens levantados na revisão de código.

### 1. Rename do pai propaga o `pai:` das filhas (fim das órfãs silenciosas)

`NoteService.UpdateParentOnRename(old, new)` roda automaticamente ao final de `Rename`: varre as notas (uma query, `GetAllNotesContent`) e, para cada uma cujo `pai:` casa com o nome-base antigo, regrava `pai: <novo-nome>` (forma canônica) e remove a chave legada `parent:`. Antes a filha voltava silenciosamente para a raiz.

- `NormalizeParentRef` e a nova `ParentCompareKey` (nome-base minúsculo, sem pasta e sem `.md`) **saíram de `system/note_tree.go` para `notes/hierarchy.go`**: Tabulator, rename e editor passam a usar a MESMA regra — antes o rename mantinha uma cópia inline da normalização.
- **Aviso na UI:** `POST /file/rename` responde com o header `X-Children-Updated: N` (contagem feita ANTES do rename, via `NoteService.GetHierarchy`). O editor lê o header em `doRename` e mostra "N notas filhas atualizadas" na barra de hierarquia; sem filhas não há header nem aviso.

### 2. Barra de hierarquia no editor ("Nota-mãe" + filhas)

`notes.HierarchyBar(data)` (`hierarchy_bar.templ`), logo abaixo do badge de backlinks:

| Peça | Comportamento |
|---|---|
| **NOTA-MÃE:** | mostra a nota-mãe atual (`—` quando raiz) e abre um picker pesquisável com a lista de `/api/notes`, excluindo a própria nota. Grava com a rota que já existia (`POST /api/notes/update-property`, `key=pai`) |
| **×** | remove a nota-mãe (a nota volta para a raiz) |
| **Badge de filhas** | popover com links para as filhas (o `/editor` redireciona para o editor especializado conforme o tipo) |

- Nenhuma validação nova: **auto-pai e ciclo continuam recusados com HTTP 400** pelo `validateParentAssignment` do Tabulator; o picker mostra o erro devolvido.
- `domain.EditorData` ganhou `Parent string` e `Children []domain.NoteRef`; `NoteService.GetHierarchy(filename)` preenche os dois numa query (`GetAllNotesContent`) e é chamado **só pelo `HandleEditor`** — drawing/mindmap/semana não pagam o custo (a nota semanal abre no editor markdown e ganha a barra de graça).
- **Sidebar:** `HandleGetSidebar` preenche `NoteItem.ChildrenCount` com `NoteService.GetChildrenCounts()` (contagem por nome-base, a mesma chave gravada em `pai:`) e `SidebarNoteItem` exibe um badge indigo quando > 0. Como a sidebar é paginada com scroll infinito (uma requisição por página) e o cálculo lê o frontmatter de TODAS as notas, o mapa é **memoizado por 5s** (`childrenCacheTTL`) e **invalidado a cada gravação do `NoteService`** (`processAndSave`/`Delete`) — o TTL cobre escritas fora dele (ex: watcher).

### 3. `HandleDuplicateNote` — uma única gravação

Havia `Store.SaveNote` **+** `GetFileTags`/`SetFileTags` **+** `Notes.Save` na mesma nota. O `SetFileTags` era inócuo (a reindexação reconstrói a tabela `tags` a partir do frontmatter: `ReplaceFileIndexes` apaga e reinsere) e o `SaveNote` era redundante, deixando o índice inconsistente por um instante.

- As tags da original agora são **mescladas no frontmatter da cópia** (`MergeFrontmatterTags`, união sem duplicatas, comparação case-insensitive) — é isso que preserva tags que existiam só no banco.
- Uma só gravação/reindexação (`Notes.Save`) e a falha dela passou a devolver HTTP 500 (antes só era logada).

### 4. Prefixos de diretório: fonte única em `domain`

As listas viviam em `allowedPrefixes` (handlers_file), `NoteFilename`, `IsNoteOrPdf` e `domain.AllowedFilePrefixes` — o risco exato apontado no §12. Agora `internal/core/domain/data.go` é a **única** definição:

| Item | Uso |
|---|---|
| consts `NotePrefix`/`PDFPrefix`/`AttachmentPrefix`/`ArchivePrefix`/`EPUBPrefix`/`ImagePrefix` e `*Dir` | strings de prefixo e nomes de pasta |
| `AllowedFilePrefixes` | validação de path traversal (`/file`, `/file/download`) |
| `NonNoteFilePrefixes` | anexos/mídia (tudo menos `notes/`) — links do markdown e rename |
| `InlineDocPrefixes` | base do `IsNoteOrPdf` |
| `FileDirs` / `SearchDirs` | `watcher.MonitoredSubDirs` e busca de arquivo sem pasta |
| `HasAllowedFilePrefix` / `InlineDocPrefix` / `NonNoteFilePrefix` | matchers |

`note_service.go`, `processor/markdown.go`, `handlers_upload.go` (`imagesPrefix = domain.ImagePrefix`), `handlers_filetype.go`, `handlers_database_tabulator.go` e `watcher.go` deixaram de repetir literais; um teste (`TestPrefixosDeDiretorio_SaoDerivadosDaFonteUnica`) trava a paridade das listas derivadas.

### 5. Download do modelo ONNX: retomável e com progresso

📍 `core/internal/features/embeddings/{model_files.go,handlers.go,context.go}` | `core/cmd/server/{main.go,routes.go}` | `core/internal/features/search/index.templ`

- **Retomada por HTTP Range:** havendo `<arquivo>.part` com bytes, o próximo boot pede `Range: bytes=<n>-` e exige `206` + `Content-Range` começando exatamente em `n`. Resposta `200` (servidor sem Range) **trunca e reinicia**; `416` descarta o `.part` e recomeça. Falha de rede no meio **preserva** o `.part` — antes o parcial era apagado e tudo recomeçava do zero.
- **Integridade:** o arquivo só é promovido (rename atômico) quando o total confere com o `Content-Length`; download truncado não se torna definitivo. `.part` órfão é removido quando o arquivo definitivo já existe.
- **Progresso granular:** `ModelProgress` (arquivo atual, bytes, total, percentual, último erro) em `GET /api/embeddings/model-status` (sem cache) + cópia em blocos de 256 KiB (`copyWithProgress`, no lugar do `io.Copy` opaco). A UI da busca híbrida mostra "Servidor N% (arquivo)" enquanto o download do servidor não termina; `WithModel` injeta o `LocalModel` no `HandlerContext` (nil → `ready=true`, nada a baixar).

### Validação (28/09/2026)

- **Go** (container `golang:1.26-alpine`, sem Go/Node no Windows): `templ generate`, `go vet -tags sqlite_fts5 ./...`, `go build -tags sqlite_fts5` e `go test ./...` verdes; `gofmt -l .` limpo.
- **Frontend** (Node do nvm no WSL Ubuntu): `tsc --noEmit` limpo, `npm test` **34/34**, `node build.js --dev` OK (bundle + Tailwind + worker).
- **Testes novos:** `hierarchy_test.go` (pai/filhas com wikilink e chave legada, caminho completo, contagens, cache/invalidação, `NormalizeParentRef`/`ParentCompareKey`, `ParentRefOfContent`, `MergeFrontmatterTags`), `TestHandleDuplicateNote_PreservaTagsDoBanco`, `TestHandleFileRename_ReportaEFilhasRepontadas`, `TestHandleFileRename_SemFilhasNaoEnviaHeader`, `TestLocalModel_DownloadRetomaDoPartViaRange`, `...ComRangeRecusadoReinicia`, `...ServidorSemRangeReinicia`, `...DownloadTruncadoNaoPromoveArquivo`, `TestLocalModel_Progress`, `TestLocalModel_EnsureRemovePartOrfao`, `TestContentRangeStart`, `TestHandleModelStatus_SemModeloLocal/_ComModelo` e `TestPrefixosDeDiretorio_SaoDerivadosDaFonteUnica`.
- **Modularização do editor (item 2 da revisão):** `slash` e `TOC` já estavam extraídos (`editor-slash.js` 207 linhas, `editor-toc.js` 127); nesta rodada saíram também o **painel de frontmatter** (`editor-frontmatter.js`, 133) e a **hierarquia** (`editor-hierarchy.js`, 234). O IIFE caiu de **1.982 → 1.567 linhas** e ficou com orquestração (estado do editor, save, rename, upload, wikilinks).
- **E2E no app real** (sandbox isolado: `DOCS_DIR`/`DB_PATH`/`STATE_DIR` em `core/data/e2e-*`, porta 6199, container `golang:1.26-alpine` rodando o binário), com 4 notas (`pai → filha → neta` + uma solta) e requisições HTTP autenticadas: pendurar filha/neta na mesma rota do botão (`POST /api/notes/update-property`, incluindo `[[FILHA]]` normalizado), barra de hierarquia no `/editor` (chip, picker e lista de filhas), badge de filhas em `/api/sidebar` (2 badges, nenhum na nota solta), auto-pai e ciclo **recusados com 400**, pai inexistente e remoção aceitos, rename com `X-Children-Updated: 1` e a filha repontada (`pai: projeto-renomeado`, neta intocada) e `/api/embeddings/model-status` reportando `files_total: 5` com percentual real de download (25% de 118MB).
- **No navegador** (mesmo sandbox): barra renderizada abaixo do badge de backlinks, picker listando as notas **sem a própria**, filtro digitando "net", seleção gravando e atualizando o chip sem reload (persistindo após F5), `×` devolvendo para `—`, popover "Abaixo desta: filha" abrindo, e o ciclo sendo barrado com a mensagem `vínculo recusado: "neta" já está abaixo desta nota (criaria um ciclo)` no próprio picker.
- 🐞 **Bug real encontrado no E2E de navegador:** `disabled={ len(children) == 0 }` no templ vira o atributo literal (`disabled="false"`) e **desabilita o botão mesmo com filhas** — o popover de filhas ficaria inacessível. Corrigido para a sintaxe booleana do templ (`disabled?={ ... }`); as guardas `TestHandleGetDatabaseData_*`/E2E passaram a checar o atributo no HTML. **Regra:** em templ, use `attr?={ cond }` para atributos booleanos (HTML), nunca `attr={ cond }`.

## 6.22 Rótulo da barra de hierarquia: "ACIMA DE" → "NOTA-MÃE:" (29/09/2026)

📍 `core/internal/features/notes/hierarchy_bar.templ` | `core/web/src/{editor-hierarchy.js,editor-init.js}`

**Problema (relatado pelo usuário):** escolhendo a nota-mãe pelo botão, o chip fica `apressado-limbo-58 · ACIMA DE · 2026-s38` e a nota aparece **aninhada dentro** da 2026-s38 no Tabulator. O comportamento está correto — o alvo é o pai (`pai: 2026-s38` no frontmatter da nota aberta) —, mas o rótulo lia-se ao contrário: lido como "esta nota ACIMA DE 2026-s38", sugeria que ela fosse o **pai**, não a filha.

**Decisão:** trocar **só o rótulo**. Nenhuma linha de lógica, de rota ou de dado mudou — o botão continua gravando `pai:` via `POST /api/notes/update-property` e o backend segue recusando auto-pai/ciclo com 400.

| Antes | Depois |
|---|---|
| `ACIMA DE <nome>` | `NOTA-MÃE: <nome>` |
| `title` do ×: "Remover o pai (a nota volta para a raiz)" | "Remover a nota-mãe (…)" |

- **Por que não inverter a semântica:** a árvore é de **pai único**. "ACIMA DE X" gravando a nota aberta como pai de X reescreveria o `pai:` que X já tinha (X trocaria de mãe em silêncio) e obrigaria um fluxo novo — o picker exclui a própria nota e `validateParentAssignment` valida "definir o MEU pai", não o de terceiros. O lado de baixo da árvore já existe: o badge de filhas com o popover **"Abaixo desta:"** (`hierarchy_bar.templ`).
- **Alternativa descartada:** `ACIMA DESTA:` (espelharia o "Abaixo desta:" do popover). `NOTA-MÃE:` ganhou por ser impossível de ler ao contrário.
- **Sem migração:** as notas já estavam certas — o frontmatter nunca representou o contrário do que a UI queria dizer.

### Validação (29/09/2026)

- **Go** (WSL, Go 1.26.3 local): `templ generate` (v0.3.1020, a pinada no `go.mod`), `gofmt -l .` limpo, `go vet -tags sqlite_fts5 ./...` limpo, `go build -tags sqlite_fts5` OK e `go test -tags sqlite_fts5 ./...` **100% verde**.
- **Frontend** (Node v24 do nvm): `tsc --noEmit` limpo e `node build.js --dev` OK (bundle + Tailwind + checagem de sanidade do CSS) — `web/static` regenerado para os comentários não ficarem defasados em relação a `src/`.
- **E2E no app real** (sandbox isolado: `DOCS_DIR`/`DB_PATH`/`STATE_DIR` em `/tmp`, porta 6199, binário `-tags sqlite_fts5`): `/editor?file=notes/apressado-limbo-58.md` (frontmatter `pai: projeto`) devolve 200 com **`NOTA-MÃE:`** e o chip com o pai resolvido (`<span id="parent-name" class="… text-sky-400">projeto</span>`), e **zero** ocorrências de `ACIMA DE` no HTML — confirmando de ponta a ponta que o alvo do botão é mesmo a nota-mãe (a nota aberta é a filha).

## 6.23 TOC navegável: Ctrl+clique pula até o título (01/10/2026)

📍 `core/web/src/editor-toc.js` | `core/web/src/editor-init.js` | `core/internal/features/notes/editor.templ` | `core/web/tests/editor-toc.unit.cjs` (novo)

**Problema (relatado pelo usuário):** o TOC era **só de edição** — renomear, renivelar e remover títulos —, mas não havia como usá-lo para **navegar** até o título dentro de uma nota longa. Faltava o caminho "índice → ponto da nota".

**Decisão:** **Ctrl+clique** (Cmd no Mac) numa linha do TOC seleciona o início do título correspondente e rola até ele. Nada no documento é alterado.

| Peça | Decisão |
|---|---|
| Widget | Mantém a `<textarea>`: edição e navegação no mesmo lugar, sem uma segunda lista de índice para manter em sincronia |
| Pareamento | A linha N do TOC é `collectHeadings(editor)[N]` — o TOC lista um título por linha (só os que têm texto), na mesma ordem do documento |
| Destino | `pos + 1` (logo após a marcação do nó = início do texto do título), com `scrollIntoView()` |
| Extração da linha | `tocLineIndex(text, caret)` — **função pura** que conta as quebras de linha antes do caret e grampeia entradas fora dos limites em `[0, length]` |
| Atalho | `Ctrl+clique` no Windows/Linux e `Cmd+clique` no macOS — no Mac o `Ctrl+clique` é o clique secundário (menu de contexto), então o handler aceita `ctrlKey` **ou** `metaKey` |
| Efeito colateral | Nenhum: é seleção/foco, **não** dispara transação — a nota **não** fica suja nem agenda save |

- **Por que não duplo-clique:** numa `textarea` o duplo-clique seleciona a palavra; usá-lo como navegação atrapalharia a edição normal do TOC.
- **Por que não uma `<ul>` clicável separada (índice de verdade):** duplicaria a UI e o estado do TOC (dois lugares mostrando os mesmos títulos, um editável e outro não). O Ctrl+clique dá o salto sem abandonar o modo de edição; se um dia o índice puramente navegável for desejado, `collectHeadings` já entrega os dados prontos.
- **Fora do intervalo** (linha vazia/inexistente): o clique é ignorado (`jumpToTocLine` devolve `false`).

### Validação (01/10/2026)

- Frontend (Node): `node --test tests/*.unit.cjs` — 5 casos novos em `editor-toc.unit.cjs` (contagem de linhas pelo caret, grampeamento de limites, salto com `pos + 1` + `scrollIntoView`, e no-op fora do intervalo / editor ausente ou destruído) — **39/39 verde**.
- `tsc --noEmit` limpo e bundle regenerado (`static/editor-init.js`).

## 7. Arquitetura de Busca

O sistema consagra **duas** modalidades complementares de pesquisa textual.

### 7.1 Os Dois Modos de Busca

| Modo | Descrição | Tecnologia | Destaque Visual |
| --- | --- | --- | --- |
| **Busca de Notas** | Filtro instantâneo no menu focado exclusivamente no nome/título dos arquivos Markdown. | Busca local indexada por correspondência parcial (`LIKE %q%`). | Azul (Sky) |
| **Busca Global** | Busca textual de termos no conteúdo interno de todas as notas do sistema. | SQLite FTS5 (tabela virtual) + Lematização (Stemming) em pt-BR. | Azul (Exato) e Roxo (Lematizado) |

> ❌ **Busca Semântica** e **Busca Híbrida (RRF)** foram removidas em 16/10/2026 — junto com
> o índice de embeddings, a UI e os endpoints. Ver §12 (Remoção definitiva).

## Como Funciona a Busca Semântica — ❌ HISTÓRICO (removida em 16/10/2026)
Vetorização (Embeddings): Cada nota markdown tem seu texto limpo e dividido em pedaços (chunks) de ~700 caracteres (com o título da nota injetado em cada pedaço para manter o contexto). O navegador gera um vetor matemático de 384 dimensões para cada chunk usando o modelo de IA local MiniLM-L12-v2.
Pesquisa KNN: Quando você digita uma busca semântica, o navegador gera o vetor da sua pergunta e o envia ao banco de dados SQLite. O banco usa a extensão vetorial sqlite-vec para rodar um cálculo KNN (Vizinhos Mais Próximos) e encontrar quais chunks de notas no banco têm a direção vetorial mais parecida (similaridade de cosseno).

## Como Funcionavam as Notas Relacionadas (Critérios) — ❌ HISTÓRICO
Para a nota que você está editando no momento, o sistema faz o seguinte:

Busca por Chunk: Ele envia cada um dos chunks da nota aberta para buscar vizinhos no banco.
Estratégia do Voto Majoritário:
Ele anota a menor distância vetorial de cada nota candidata e em quantos chunks diferentes ela deu match.
Regra para Notas Longas: Se a nota que você está editando for longa (≥ 3 chunks), uma nota relacionada só é considerada relevante se der match em pelo menos 2 chunks diferentes da nota atual. A única exceção é se a similaridade de um chunk for excepcional (acima de 82%).
Ordenação: As top 5 notas relacionadas são ordenadas por frequência de matches (notas mais consistentes ao longo do texto vêm primeiro) e depois por proximidade vetorial (distância).
Nota de Corte (Threshold): Descarta qualquer resultado abaixo do percentual configurado por você (padrão de 72%).

## 8. Busca Híbrida — Reciprocal Rank Fusion (RRF, 10/08/2026) — ❌ REMOVIDO (16/10/2026)

📍 `internal/search/rrf.go` | `internal/features/search/handlers_hybrid.go` | `internal/features/search/index.templ`

> ❌ **Arquivos e rota (`POST /api/search/hybrid`) removidos** — a fusão dependia do motor
> vetorial, que não existe mais. Ver §12.

Novo **4º modo de busca "🔀 Híbrido"** que funde os resultados do FTS5 (textual) com a busca semântica via **Reciprocal Rank Fusion**, para melhorar precisão e recall (reduz o ruído de um único motor).

$$RRF(d) = \frac{1}{k + rank_{fts}(d)} + \frac{1}{k + rank_{sem}(d)}, \quad k = 60$$

- **Fusão server-side**: `POST /api/search/hybrid` recebe `{query, embedding, limit}` (o embedding é gerado no browser pelo MiniLM já carregado). Roda os dois motores em **paralelo** (goroutines) e funde em Go.
- **Módulo puro testável**: `search.ReciprocalRankFusion(ftsRanks, semRanks, k, limit)` — só ordenação/score, sem I/O. Docs que aparecem nos **dois** motores somam as parcelas e sobem.
- **Filtros**: semântica usa o threshold configurável (`semantic_search_threshold`, padrão 35% → `maxDist`); FTS5 exclui pdfs/anexos (igual ao modo global). O FTS5 já entra re-rankeado com peso sináptico/backlinks.
- **Degradação graciosa**: se o embedding vier zerado (modelo não pronto), o backend cai para FTS5 puro — sem erro.
- **Snippet**: do FTS5 com highlight quando o termo casou (`buildSnippet`, extraído para helper reutilizável); senão prévia genérica segura (`buildPlainSnippet`).
- **Resultado JSON por item**: `filename`, `type` (determinístico via `DetectNoteType`), `rrf_score`, `rank_fts`/`rank_sem`, `sem_similarity` (%), `snippet`, `has_highlight`.
- **UI**: novo botão/input/container "🔀 Híbrido" em `index.templ`, com badge de similaridade e render unificado. Ícone novo `busca-hybrid` (sparkles, teal) em `icons/config.go`.
- **Motivação**: a busca semântica sozinha "às vezes é imprecisa"; a fusão RRF é técnica clássica de IR, barata e sem modelo novo.

## 9. Frontend — Migração Tailwind v4 e marked v18 (09/09/2026)

📍 `core/web/src/input.css` | `core/web/build.js` | `core/web/package.json`

Migração dos updates major do dependabot (PRs #5 e #7) com o app validado de ponta a ponta:

- **tailwindcss 3.4.19 → 4.3.3**: build migrado de `tailwind.config.cjs` para **CSS-first** (`input.css`). `tailwind.config.cjs` foi **removido**.
  - `@import "tailwindcss";` + `@theme` (mantém `--font-sans: Inter` e `--animate-fast-spin`) + `@plugin "@tailwindcss/typography";`.
  - **Fonte de conteúdo via `@source`** (relativo ao CSS em `src/`), cobrindo os DOIS layouts de build (local `core/internal/features` e Docker `/web/internal`):
    `../../internal/features` (local), `../internal/features` (Docker), `../layout`, `./**/*.{js,jsx}`, `../static/js`.
  - CLI agora é o binário de `@tailwindcss/cli` (o pacote `tailwindcss` v4 não embute CLI). `build.js` roda `npx tailwindcss -i src/input.css -o static/app.css --minify`.
  - Renomes de classe aplicados (preservam o visual, valores idênticos): `rounded`→`rounded-sm`, `outline-none`→`outline-hidden`, `flex-shrink`→`shrink`, `flex-shrink-0`→`shrink-0`, `placeholder-{cor}`→`placeholder:text-{cor}`, `backdrop-blur-sm`→`backdrop-blur-xs`. ⚠️ `flex-shrink` também é **propriedade CSS** — cuidado para não renomear `flex-shrink:` dentro de `style="..."`/`<style>` (já corrigido).
  - `@tailwindcss/typography` 0.5.10 → 0.5.20 (suporte v4 via `@plugin`).
- **marked 15.0.12 → 18.0.11**: uso é só `marked.parse()` (API estável, síncrona, GFM/breaks intactos) — nenhuma mudança de código além do bump. `docs.templ` usa marked via CDN (fora do escopo do npm).
- **Validação**: `go test ./...`, `go vet`, `gofmt`, `npm run typecheck`, `npm test`, `node build.js` (com checagem de sanidade `green-500`/`prose`) — todos OK; cobertura de classes conferida entre o CSS v3 e o v4 (nenhuma classe em uso foi perdida; só os renomes acima).
- Nota: o `semantic-worker.js`/`drawing.js` estáticos commitados podem divergir de um rebuild local (staleness pré-existente); reverter com `git checkout -- core/web/static` após build.

## 10. Performance — correções de gargalos (10/09/2026)

📍 `internal/search/search.go` | `internal/core/db/*.go` | `internal/features/notes/note_service.go` | `web/src/database.js` | `web/static/editor-common.js` | `web/static/js/agenda/timeline.js`

Correções guiadas por benchmarks (`perf_bench_test.go` em `internal/search` e `internal/core/db`). Números medidos num vault sintético de 3.000 notas (~21k docs) / 2 vCPU:

- **Busca "só por tag"** (`#tag`): o `LIMIT` era `99999` e materializava o corpus inteiro. Agora é limitado (máx. `maxTagOnlyCandidates = 500`); o `total` continua exato (vem do COUNT). **290ms → ~39ms e 182MB → ~8MB**.
- ❌ **Limpeza de embeddings (tabela virtual `vec0`)** — *histórico*: `chunk_id LIKE 'arquivo#%'` não usava índice; o helper `deleteEmbeddingsForFile` resolvia os `chunk_id` por índice e apagava por igualdade. **~18,5ms → ~1,1ms**. Removido com o índice semântico (§12).
- **Rename de backlinks (`UpdateBacklinksOnRename`)**: era N+1 (`GetNote` por candidato). Agora usa `GetAllNotesContent()` em uma query (método adicionado ao `NoteStore`). **~171ms → ~2ms (~86×)**.
- **`extractTerms`**: o regexp era recompilado a cada chamada (rodava em loop no gate da híbrida). Agora é `var` de pacote.
- **`HandleManualSync`**: removido o dead code (N+1 `GetNote` + `time.Parse` descartado + `if false`); agora conta notas com conteúdo em uma query. Comportamento observável inalterado.
- **Frontend**: `database.js` parseia a query de busca **uma vez** (antes rodava por linha a cada tecla); `timeline.js` guarda o `redLineInterval` em escopo de **módulo** (corrige leak de timer/canvas a cada re-render) e coalesce o redraw do `wheel` via `requestAnimationFrame`; `editor-common.js` coalesce `updateHighlight` (click/keyup/scroll/selectionchange/resize) via rAF (evita layout thrashing).

> Não alterados (avaliados, menor prioridade/risco de contrato): o `COUNT(*)` do FTS5 por busca, o filtro `tags NOT LIKE '%drawing%'` dentro da query FTS e a queda para `SearchFTSLike` (full scan `LOWER(texto) LIKE`) em buscas sem resultado.

## 11. Backup ZIP — nomes de arquivo em UTF-8 (17/09/2026)

📍 `internal/core/services/backup.go` (`zipFlagUTF8`, `writeNotesParallel`) | teste `internal/core/services/backup_test.go`

**Bug real (reportado pelo usuário):** ao exportar as notas (`/api/backup`), os **títulos com acento viravam mojibake** ao descompactar (`reunião` → `reuni├úo`, `Café` → `Caf├⌐`, `Adoção` → `Ado├º├úo`).

Causa: as notas são gravadas no ZIP via `zip.Writer.CreateRaw` (a compressão DEFLATE roda em pool paralelo e o `CreateRaw` aceita dados já comprimidos). O `CreateHeader` do stdlib aplica sozinho a heurística de encoding — seta o **bit 11 / `0x800`** quando o nome é UTF-8 válido —, mas o **`CreateRaw` grava os `Flags` exatamente como recebe**. As entradas de nota saíam com `Flags = 0x0000` e os extratores que não "adivinham" UTF-8 (Windows Explorer, gerenciadores de arquivos mobile, `unzip` antigo) decodificam o nome como **CP-437**.

Verificado empiricamente com sonda em Go 1.26 (o ponto do stdlib é a `case fh.NonUTF8: Flags &^= 0x800` / `default: Flags |= 0x800` do `CreateHeader`):

| Como foi gravado | `Flags` | flag UTF-8 |
|---|---|---|
| `CreateHeader` (Deflate ou Store) | `0x0808` | ✅ |
| `CreateRaw` | `0x0000` | ❌ |
| `CreateRaw` com `Flags: 0x800` | `0x0800` | ✅ |

**Correção:** constante `zipFlagUTF8 = 0x800` aplicada ao `zip.FileHeader` de toda entrada de nota em `writeNotesParallel`. Arquivos do disco (PDFs, anexos, imagens) já passavam por `addFileToZip` → `CreateHeader`, portanto **não** eram afetados — o que explica o sintoma aparecer apenas no backup de notas.

**Teste de regressão:** `TestBackup_NomesUTF8ComFlag` exige `Flags & 0x800 != 0` e o nome UTF-8 íntegro numa nota com acentos.

## 12. Imagens do editor em `docs/images/` (17/09/2026)

📍 `internal/features/notes/handlers_upload.go` (`imagesPrefix`) | `handlers_filetype.go` (`imageSubdir`) | `handlers_file.go` | `internal/watcher/watcher.go` | `internal/core/domain/data.go` | `internal/core/db/query.sql`

As imagens enviadas pelo editor (colar/arrastar ou botão **Imagem** → `POST /api/upload-image`) eram gravadas em **`notes/`** com o prefixo `img_<timestamp>_<nome>`, misturando binários com a pasta de notas. Agora vão para **`docs/images/`**.

- O nome continua `img_<timestampUnixMilli>_<nome sanitizado>` (evita colisão e permite a limpeza por prefixo).
- **Tipo da nota inalterado:** `domain.DetectNoteType` já classificava qualquer extensão de imagem como `NoteTypeImage`; o prefixo `images/` foi adicionado explicitamente e `notes/img_` continua reconhecido (legado).
- **Legado continua funcionando:** imagens antigas em `notes/` seguem sendo servidas (`/file?name=notes/img_...`), resolvidas (`imageSubdir` cai em `notes/` quando o arquivo não existe em `images/`) e limpas — `HandleCleanupImages` varre `images/` **e** `notes/` (e agora ignora diretório inexistente em vez de falhar).
- Prefixo `images/` registrado em: `domain.AllowedFilePrefixes`, `NoteFilename`, `IsNoteOrPdf`, `MonitoredSubDirs` (o boot/rescan passa a registrar o documento stub da imagem, como já fazia com PDFs/anexos) e nas listas de prefixos usadas por backlinks/renomeação (`processor/markdown.go`, `NoteService.UpdateBacklinksOnRename`).
  - ⚠️ **Atualizado em 6.21 (28/09/2026):** esses pontos deixaram de ser listas independentes — `internal/core/domain/data.go` é a **fonte única** dos prefixos (consts + `AllowedFilePrefixes`/`NonNoteFilePrefixes`/`InlineDocPrefixes`/`FileDirs`/`SearchDirs` + matchers). Adicione um tipo de arquivo novo apenas lá.
- **Paridade Go/SQL (regra 3.5):** `AND n.filename NOT LIKE 'images/%'` adicionado a `CountEmbeddableNotes` e `GetPendingEmbeddingNotes` (`query.sql` **e** `generated/query.sql.go`), espelhando `IsNoteEmbeddable` (que classifica `images/*` como imagem → não indexável).
- As imagens continuam **invisíveis** na sidebar/banco de dados: `NoteService.GetMany` descarta `NoteTypeImage` e o watcher não grava `file_mod` para imagens (só o documento stub).
- ⚠️ **O bundle do frontend não mudou:** o editor insere a URL absoluta devolvida pelo handler (`/file?name=images/...`), então nenhum rebuild de `web/static/` é necessário.
- ⚠️ Ao mexer nesse prefixo no futuro, mantenha em sincronia os pontos listados no comentário de `imagesPrefix`.
- ⚠️ **Desde 6.21 (28/09/2026)** o prefixo é derivado de `domain.ImagePrefix` (`const imagesPrefix = domain.ImagePrefix`) — a sincronia manual descrita acima deixou de existir.
- **Validação (17/09/2026):** `go test ./internal/features/notes/ ./internal/watcher/ ./internal/core/domain/ ./internal/core/db/ ./internal/core/services/ ./internal/processor/` — todos OK. Testes novos: `TestHandleUploadImage_Success` (grava em `images/`), `TestResolveFileInfo_ImagemEmImages`, `TestHandleCleanupImages_RemovesOrphanInImagesDir`, `TestHandleFileDelete_ImageInImagesDir`, `TestIsNoteOrPdf_Images`, casos de `DetectNoteType` e `TestMonitoredSubDirs`.

[Definição dos icones da aplicação](/core/internal/ui/icons/config.go)

https://lucide.dev/icons/

## 6.24 Correções pontuais — globais implícitas, detecção de linguagem e backlinks (03/10/2026)

📍 `core/web/src/editor-slash.js` | `core/web/src/editor-init.js` | `core/internal/features/notes/note_service.go`

### Globais implícitas em `editor-slash.js`

`_setSlashPos` e `_setSlashFilter` eram atribuídas em `initSlash()` sem declaração `var` prévia no topo do arquivo. No build do esbuild com `format: "iife"`, variáveis não declaradas viram globais implícitas no `window`. Adicionadas as declarações:

```js
var _setSlashPos = null;
var _setSlashFilter = null;
```

### `autoDetectCodeLanguage` parava no primeiro bloco

A função usava uma flag `found` e `if (found) return` dentro do `doc.descendants`, interrompendo a varredura após detectar linguagem no **primeiro** code block. Uma nota com 3 blocos sem linguagem só detectava o primeiro.

**Correção:** removida a flag; acumulados todos os `setNodeMarkup` em uma única transação (`tr`) e despachada uma só vez — evita N dispatches e preserva o comportamento sem mover o cursor.

### `wikiNotes` desatualizado após rename na mesma sessão

O autocomplete `[[` carregava `/api/notes` apenas no carregamento inicial da página. Após renomear uma nota sem reload, o nome antigo continuava aparecendo no autocomplete e o novo não aparecia.

**Correção:** `loadWikiNotes()` é chamado ao final de `doRename()` bem-sucedido (antes do `catch`), recarregando o índice de notas em background.

### `UpdateBacklinksOnRename`: fallback para `GetAllNotes` condicional

A função expandia `candidateMap` para **todas as notas** incondicionalmente, anulando o benefício do índice de backlinks implementado em §10 (de ~86× mais rápido para O(N)). Com um vault de 1.000+ notas, cada rename carregava potencialmente MBs de conteúdo sem necessidade.

**Correção:** `GetAllNotes()` agora só é chamado quando `len(candidateMap) == 0` (índice vazio — primeiro uso ou corrompido), preservando o comportamento de segurança sem penalizar o caso normal.

### Validação (03/10/2026)

- **Frontend:** `tsc --noEmit` limpo, `node --test tests/*.unit.cjs` **39/39 verde**, `node build.js --dev` OK.
- Go: requer WSL (Go não está no PATH do Windows) — executar `go test ./internal/features/notes/... -count=1` antes de subir.

## 6.25 Suporte a opções de Frontmatter no Markmap (maxWidth, etc.) (03/10/2026)

📍 `core/web/src/mindmap.js` | `core/internal/features/notes/mindmap.templ` | `core/web/build.js` | `core/web/tests/mindmap-options.unit.cjs`

### Contexto e Causa Raiz

Opções de nós no Markmap como `maxWidth: 300` (e cores, etc.) não funcionavam por três razões:
1. `mindmap.js` continha uma regex manual (`FRONTMATTER_REGEX`) que descartava qualquer bloco de frontmatter (`--- ... ---`) antes de enviar o Markdown para o `transformer.transform`.
2. As opções derivadas de frontmatter (`deriveOptions(frontmatter?.markmap)`) nunca eram passadas para `Markmap.create(...)` e `mmInstance.setData(...)`.
3. `src/mindmap.js` não constava nos `entryPoints` do `build.js`, fazendo com que alterações no arquivo fonte não fossem reconstruídas no `static/mindmap.js`.

### Mudanças Implementadas

1. **`core/web/src/mindmap.js`**:
   - Importa `deriveOptions` de `markmap-view`.
   - Remove o descarte forçado de frontmatter.
   - Adiciona tolerância: se o usuário escrever `markmap:\n  maxWidth: 300` no topo do arquivo sem os delimitadores `---`, o código envelopa automaticamente em frontmatter válido antes da compilação.
   - Passa `markmapOptions` (`{ maxWidth: 400, ...deriveOptions(frontmatter?.markmap) }`) para `Markmap.create` e `mmInstance.setData` (tanto no `update` quanto no hook `retransform`), garantindo 400px como largura máxima padrão caso a nota não especifique outro valor.
2. **`core/internal/features/notes/mindmap.templ` e `handlers_mindmap.go`**:
   - `handlers_mindmap.go`: novo mapa mental agora inicia por padrão com o bloco `markmap: maxWidth: 400` em seu frontmatter.
   - `mindmap.templ`: `ensureMindmapFrontmatter` preserva/adiciona o bloco `markmap: maxWidth: 400` ao salvar e renomear.
3. **`core/web/build.js`**:
   - Adicionado `"src/mindmap.js"` aos `entryPoints` do esbuild para que o bundle `static/mindmap.js` seja sempre gerado e comprimido junto dos demais assets.
4. **Testes (`core/web/tests/mindmap-options.unit.cjs`)**:
   - Valida a extração e derivação de `maxWidth` no `Transformer` e `deriveOptions`.
   - Valida a tolerância de sintaxe sem `---`.
   - Valida a presença de `deriveOptions` e `src/mindmap.js` no `build.js`.

### Validação (03/10/2026)

- `npx tsc --noEmit` limpo (0 erros).
- `node --test tests/*.unit.cjs` **42/42 verde**.
- `node build.js` gerou com sucesso `static/mindmap.js` (737.2KB) com Gzip (238.4KB) e Brotli (182.5KB).

## 6.24 Hierarquia com ARQUIVOS (PDF/EPUB/ZIP) como filhas — `file_metadata` (05/10/2026)

📍 `core/internal/core/db/{db.go,file_metadata.go,reindex.go}` | `core/internal/core/domain/data.go` | `core/internal/features/notes/{hierarchy.go,note_service.go}` | `core/internal/features/system/{note_tree.go,handlers_database_tabulator.go}`

**Problema:** a hierarquia (§6.17/§6.19/§6.21) vive no frontmatter da própria nota (`pai:`). PDF/EPUB/ZIP são binários no disco — não têm onde ler/escrever frontmatter — então editar a coluna "Pai" de um arquivo no Tabulator **retornava 200 sem gravar nada**.

**Decisão:** arquivos podem ser **FILHAS** (declaram `pai`), nunca **PAI**. O vínculo vive numa tabela de metadados externos por arquivo, o mesmo padrão já usado pela tabela `tags` para esses mesmos arquivos.

| Peça | Decisão |
|---|---|
| Tabela | `file_metadata (arquivo, key, value, PRIMARY KEY(arquivo,key))` — chave/valor genéricos (hoje só `pai` é gravado; futuros metadados reusam) |
| Chave | `domain.ParentKey` (`pai`) — a mesma do frontmatter; `ParentKeyLegacy` (`parent`) segue só na leitura |
| Notas | continuam no frontmatter (`notes.content`), como antes — a tabela existe só para arquivos |
| Arquivo como pai | **proibido**: o índice de resolução de pais (`newNoteTreeIndex`) considera **apenas notas** (`.md`). Um `pai: manual` apontando para `pdfs/manual.pdf` não resolve e a nota fica na raiz |
| Pai único / ciclo | mesmas regras: guarda de vírgula + `validateParentAssignment` (o alvo é resolvido só entre notas, então a cadeia é sempre de notas) |
| Gravação | `POST /api/notes/update-property` com `key=pai` em arquivo → `NoteService.SetFileParent` → `file_metadata` (valor vazio remove) |
| Leitura | `HandleGetDatabaseData` injeta os metadados nas linhas de arquivo; `buildNoteTree` monta a árvore sem mudança |
| Rename | o `pai` do arquivo é repontado por `UpdateParentOnRename` (frontmatter das notas + `file_metadata`); renomear o arquivo preserva os metadados sob o novo caminho |
| Delete | `DeleteAllFileRecords` limpa `file_metadata` (fonte única de limpeza) |
| Sidebar | `GetChildrenCounts` soma as filhas-arquivo → badge correto; `GetHierarchy` lista arquivos como filhas no popover do editor |

### Detalhes que evitaram bugs reais

- ⚠️ **Rename de arquivo pelo editor (corrigido em 22/10/2026):** só o caminho do Acervo preservava `file_metadata`; o `POST /file/rename` do editor usava uma cópia quase igual que **perdia o `pai:`** (o arquivo voltava para a raiz). O rename físico agora vive em `notes.RenameNonNoteFile` e é o único caminho para os dois handlers (Acervo e editor); teste de regressão em `notes/handlers_file_test.go` (`TestHandleFileRename_ArchivePreservaMetadados`).
- ⚠️ **Título do frontmatter tem um único leitor (corrigido em 22/10/2026):** `notes.FrontmatterTitle`/`FrontmatterTitleKey` comparam `title`/`titulo` **case-insensitive** (com precedência de `titulo`, só valor string). Antes o Acervo era case-insensitive, mas a duplicação e o rename eram case-sensitive: um `Title:` deixava a cópia sem o prefixo `copia-` e o título antigo permanecia após o rename. Testes: `frontmatter_test.go` (variantes de caixa) e regressões em `handlers_save_test.go`/`frontmatter_test.go`.
- ⚠️ **Rename de pai parseia o frontmatter uma vez (22/10/2026):** `UpdateParentOnRename` lia o mesmo conteúdo até 4× por filha (`ParseFrontmatter` + `ParentRefOfContent` + `UpdateFrontmatterProperty` + remoção da chave legada). Agora usa `parentRefFromMap(fm)` e `updateFrontmatterPropertyIn(fm, body, …)`; a remoção de `parent:` também não sobrescreve mais `updated` com `""` se a codificação falhar (evitava salvar a nota vazia).
- ⚠️ **Cache do Tabulator (bug encontrado no E2E):** a linha é cacheada por **mtime**, que não muda ao editar metadado nem ao renomear a nota-mãe. Se a injeção só preenchesse chaves ausentes, o `pai` antigo ficaria **preso** na linha. Agora, para arquivos, a injeção **remove as chaves que não existem mais** (toda chave sem prefixo `_` que não seja coluna nativa é metadado) e então aplica os valores atuais.
- SQL direto nos métodos do `Store` (como `BatchGetNotesContent`), em vez de sqlc: mantém a feature autocontida e não exige regenerar o pacote `dbgen`.
- ⚠️ Em `db.go` o schema é uma raw string Go delimitada por crase: **comentário SQL não pode conter crase** (fecha a string).
- `SetFileParent` invalida o cache de contagens de filhas (`invalidateChildrenCounts`), como qualquer outra gravação.

### Validação (05/10/2026)

- **Go** (WSL, Go 1.26): `gofmt -l` limpo, `go vet -tags sqlite_fts5 ./...` limpo, `go build -tags sqlite_fts5` OK e `go test -tags sqlite_fts5 ./...` **100% verde**.
- **Testes novos:** `db/file_metadata_test.go` (CRUD, valor vazio remove, múltiplas chaves, `DeleteAllFileRecords` limpa) | `system/file_hierarchy_test.go` (arquivo como filha, **arquivo nunca é pai**, injeção da coluna Pai, gravação/remoção via endpoint, múltiplos pais 400, **regressão do cache após rename**) | `notes/hierarchy_file_test.go` (contagens com filhas-arquivo, `GetHierarchy` com filhas-arquivo, rename propaga o `pai` do arquivo, `SetFileParent` grava/remove).
- **E2E no app real** (sandbox isolado, porta 6199, binário `-tags sqlite_fts5`): PDF como filha (`pai: [[PROJETO]]` normalizado), coluna Pai injetada, aninhamento e `hasTree`, badge de 1 filha na sidebar, arquivo **nunca** como pai, rename da nota repontando o `pai` do arquivo, remoção devolvendo à raiz e múltiplos pais recusados com 400 — **tudo OK**.

## 6.25 Tabulator — leitura visual da hierarquia e colunas (05/10/2026)

📍 `core/internal/features/notes/database.templ` | `core/web/src/database.js` | `core/internal/features/system/handlers_database_tabulator.go`

Melhorias de legibilidade da tabela (Fase 1 — só apresentação, sem mudança de dados).

| Item | O que mudou |
|---|---|
| Ícone no Título | o formatter passa a usar o `_icon` (SVG do servidor, SSOT §6.9) que **já vinha no payload**, mas só era usado na coluna "Abrir" (oculta) |
| Coluna "Tipo" | ícone + rótulo legível (mapa `TYPE_LABELS`) em vez do código cru (`pdf`, `anexo`) |
| "Embeded" → "Indexada" | corrige o typo do rótulo e mostra **✔ / —** com tooltip (em vez de `true`/`false`/`N/A`) |
| Guias de indentação | uma linha vertical por nível ancestral (ver abaixo) |
| Contador de filhas | chip no nó que tem `_children` (ex.: `projeto · 3`) |
| Marca de órfã | ponto âmbar quando a nota declara um `pai:` que não resolve |
| Expandir/Recolher tudo | botões na toolbar (aparecem só no modo árvore) |
| Colunas móveis | `movableColumns: true` + `minWidth` no Título (não espremer o controle +/-) |

### Guias verticais — como funciona (e a armadilha)

- O Tabulator **não expõe o nível** da linha (`getTreeLevel` não existe no 6.3.1). Solução: `annotateLevels()` calcula `_level` na floresta **exibida** e o `rowFormatter` publica `--lvl` na linha.
- O CSS da célula Título desenha `--lvl` linhas com `repeating-linear-gradient` + `background-size: calc(var(--lvl) * 18px)`, alinhadas ao **centro do controle +/-** (offset 20px = padding 12px + 8px).
- 🔴 **Armadilha real** (medida no navegador): com `dataTreeBranchElement: false` o Tabulator **ainda cria** um `.tabulator-data-tree-branch-empty` de **7px**. O `marginLeft` acumula o nível (18×L), mas a largura de 7px entra **uma vez** → do 2º nível em diante o controle ficava **desalinhado** das guias. Corrigido com `#notes-table .tabulator-data-tree-branch-empty { width: 0 }` (passo uniforme de 18px).
- ⚠️ `movableRows` (arrastar linhas) é **incompatível com `dataTree`** — o próprio Tabulator emite warning e desabilita. Não usar (`movableColumns` é outra coisa e é seguro).
- `_orphan` é calculado **uma vez** sobre os dados completos: o filtro de busca promove filhos à raiz e marcaria órfã falsa se fosse recalculado depois.

### Validação (05/10/2026)

- `tsc --noEmit` limpo, `node build.js` (produção) OK, `npm test` **42/42**.
- Go: `templ generate`, `gofmt -l` limpo, `go vet`, `go build` e `go test ./...` verdes.
- **E2E no navegador** (sandbox isolado, porta 6199): ícones nas colunas Título/Tipo, chip de filhas (`projeto` 3, `backend` 1), ponto de órfã, guias alinhadas em 3 níveis (centros medidos em 20/38/56px), "Expandir" (7 linhas) ↔ "Recolher" (3 raízes) e estado persistido.

## 6.26 Título "..." no Tabulator — fallback e guarda de rename (06/10/2026)

📍 `core/internal/features/system/handlers_database_tabulator.go`

**Problema:** uma nota exibia `...` na coluna Título. O rótulo vinha de `domain.DisplayName(arquivo)`; renomear uma nota para um nome só de pontuação (ex: `...`, que vira `notes/....md`) produzia um título sem nenhum caractere útil.

**Decisão:**
- **Fallback de exibição** (`displayTitle`): o nome do arquivo continua sendo o padrão, mas se ele **não tiver nenhuma letra/dígito** (`isMeaningfulName`), a coluna Título cai para (1) o primeiro heading `# ...` da nota (`processor.ExtractTitle`) e depois (2) o `titulo:`/`title:` do frontmatter. Arquivos (PDF/EPUB/ZIP) não têm conteúdo/frontmatter → seguem mostrando o nome do arquivo.
- **Guarda na origem:** `HandleUpdateNoteProperty` recusa (400) um título sem letra/dígito, impedindo que o estado ruim seja recriado. Usa o mesmo `isMeaningfulName`.
- ⚠️ Não mudou o comportamento padrão: nome normal continua vencendo sobre o heading (teste de regressão).

**Tests:** `system/handlers_titulo_test.go` (fallback para heading, fallback para frontmatter, nome normal inalterado, rename para `...` recusado).

## 6.27 Tabulator — botões únicos de expandir/recolher e filtro "Só hierarquia" (06/10/2026)

📍 `core/internal/features/notes/database.templ` | `core/web/src/database.js` (+ `static/database.js`)

**Mudanças na toolbar (modo árvore):**
- **Expandir/Recolher num só botão** (`#db-expand-toggle`): o rótulo/ícone refletem a AÇÃO disponível (`Recolher` com `chevron-right` quando expandido; `Expandir` com `chevron-down` quando recolhido). `aria-pressed` acompanha. O estado por nota continua em `localStorage[db_tree_expanded]` (ver §6.25).
- **Novo filtro "Só hierarquia"** (`#db-hierarchy-toggle`): alterna entre "Todas as notas" (`layers`) e "Só hierarquia" (`git-fork`), persistido em `localStorage[db_hierarchy_only]`. Quando ligado, `hierarchyRows` mantém apenas as raízes que **têm filhas** ou que **declaram `pai:`** (órfãs) — as notas soltas somem. Com o filtro ligado, tudo nasce expandido (`treeStartExpanded`).

**Implementação (database.js):**
- `applySearch` agora compõe os dois filtros: parte de `hierarchyRows(baseRows)` quando o filtro está ligado e só então aplica a busca. O antigo flag `filterApplied` virou `dataFiltered` (busca **ou** hierarquia).
- No boot, `applySearch` passou a rodar **sempre** no `tableBuilt` (antes só rodava se houvesse `db_last_query`), para o filtro de hierarquia persistido ser honrado mesmo sem busca.

**Validação:** `templ generate`, `tsc --noEmit`, `go build -tags sqlite_fts5` OK. `npm test`: as 2 falhas são pré-existentes (falta `markmap-common` no `node_modules`, não relacionadas).

## 6.28 Título aninhado virando "..." no Tabulator (06/10/2026)

📍 `core/internal/features/notes/database.templ` | `core/web/src/database.js` (+ `static/database.js`)

**Problema:** uma linha **aninhada** com título mais largo que a coluna exibia **apenas `...`** — nem um caractere do nome. Reproduzido em harness isolado (bundle v6.3.1 real + o `<style>` do `database.templ`, Chromium): com `layout: fitColumns` a coluna Título media 315px e o título, 582px. Não tem relação com hierarquia nem com o `displayTitle` (§6.26) — o valor nos dados estava correto.

**Causa raiz (CSS):** o formatter embrulha ícone + título + chip em `.db-title { display: inline-flex }`, que é uma **caixa atômica**. Como `.tabulator-cell` impõe `white-space: nowrap`, o **min-content** do `<strong>` é o texto INTEIRO — então a caixa `inline-flex` se dimensiona pelo *max-content* (582px), em vez de encolher para a largura disponível (shrink-to-fit = `max(min-content, available)`). A caixa **transborda** a célula e o `text-overflow: ellipsis` do tema (`.tabulator-cell`), em vez de cortar o texto, **descarta o conteúdo transbordado e desenha um único `...`** (a caixa atômica não é divisível).

**Decisão (só CSS, sem `dataTreeExpandElement` — ver §6.17):**
- A célula do Título passou a ser **`display: inline-flex; align-items: center`** — continua *inline-level*, então o layout de colunas do Tabulator (células inline-block lado a lado) não muda. Agora o **espaçador da árvore** (width 0 + `margin-left` do nível), o **controle +/-** e o título dividem a linha como flex items.
- `.db-title` recebeu **`flex: 1 1 0; min-width: 0`**: ocupa a sobra da linha e pode encolher. ⚠️ O `flex-basis` é **explícito em `0`** de propósito — com `auto` (== max-content do título) a soma das bases estoura a célula nas linhas de título longo e o encolhimento proporcional espreme os **irmãos** flex (ver “Efeito colateral” abaixo).
- **`.db-title > strong`** é o ÚNICO que encolhe: `min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap`. O ícone (`flex-shrink: 0`) e o **chip de filhas** seguem visíveis mesmo com o título truncado.
- O formatter agora publica **`title="<título completo>"`** no `<strong>`: o nome inteiro (típico de PDF/EPUB aninhados) continua recuperável no hover. Passa pelo mesmo `escapeHtml` (que já escapa aspas).

**Alternativa descartada:** resolver só com `.db-title { display: inline }` (caixa inline não-atômica) também corta o texto corretamente, mas o **chip de filhas**, que vem DEPOIS do texto no HTML, desaparece junto com o transbordo. A célula flex preserva os dois.

### Efeito colateral da célula flex: controle +/- espremido (corrigido no mesmo dia)

**Sintoma (relatado pelo usuário):** o controle `+/-` da árvore ficou **estreito** (uma pílula fina em vez do quadrado de 16px) nas linhas de título longo.

**Causa:** com `.db-title { flex: 1 1 auto }`, o `flex-basis: auto` vale o **max-content do título** (ex.: 582px). A soma das bases (espaçador 0 + controle 16 + título 582 = 598px) estoura os ~291px da célula ⇒ sobra **negativa**, e o flexbox a distribui **proporcionalmente entre TODOS os itens** (não só no que deveria encolher). O controle, com `flex-shrink: 1` (padrão), perdia a sua cota: medido no harness, **18px → 11px**; com os slugs longos do banco real, ainda menos.

**Correção (2 linhas):**
- `.db-title` passou a **`flex: 1 1 0`** — base 0, então a soma das bases = 16px (só o controle), nunca há sobra negativa por causa do título, e o título ocupa a sobra pelo `flex-grow`.
- `#notes-table .tabulator-data-tree-control` recebeu **`flex-shrink: 0 !important`** (defensivo: o controle nunca é espremido).

**Lição:** ao transformar um container do Tabulator em flex, **todo item que não deve encolher precisa de `flex-shrink: 0`** e o item elástico precisa de **base explícita (`flex: 1 1 0`)** — `flex: 1 1 auto` faz o *min-content/max-content* do texto longo virar base e “vazar” o encolhimento para os irmãos.

**Validação:**
- Harness isolado (antes/depois, mesma largura): `...` → `Placa-Mãe SOYO X99 com CPU Xeon E5…`; chip preservado em título longo COM filhas; guias de `--lvl` intactas.
- **Controle `+/-` medido:** antes `18px → 11px` na linha de título longo (o encolhimento é proporcional à largura da base de cada item); depois **18px em todas as linhas**.
- Edição da célula (`editor: "input"` do servidor) segue OK: o input vira flex item e ocupa a sobra da linha (289px de 315px).
- `templ generate`, `node build.js`, `tsc --noEmit` (limpo), `node --test tests/*.unit.cjs` (**42/42**), `go vet` e `go build -tags sqlite_fts5` OK — reexecutados após a correção do controle.

## 6.29 Linguagem de consulta nas notas — bloco `consulta` + CEL (06/10/2026)

📍 (novos) `core/internal/processor/query.go` | `core/internal/query/engine.go` | `core/internal/query/run.go` | `core/internal/features/notes/query_block.templ` | `core/internal/features/notes/handlers_query.go` · (docs) `docs/linguagem-de-consulta.md` | `docs/adr/0004-linguagem-de-consulta.md`

**Problema:** o usuário pediu "uma linguagem de script dentro das notas" para montar páginas vivas (fila de leitura, painel de manutenção, revisão do mês). O app hoje **mostra** e **filtra**, mas não combina condições (E/OU/NÃO) sobre campos; o filtro do Tabulator (§6.25) é client-side e só sobre o que já está carregado; não há valores calculados ("dias sem abrir", contagens); não há regras de organização além do Auto-Tag `X dias → tag` (§6.8); e nenhum resultado "vive" dentro da nota.

**Decisão (4 pontos):**
1. **Bloco declarativo ```` ```consulta ```` em YAML** — o usuário já escreve YAML no frontmatter (§6.17), então a curva é zero, e o parser é o `gopkg.in/yaml.v3` (já no `go.mod`) com `KnownFields(true)` ⇒ **chave desconhecida vira erro** sem uma linha de validação.
2. **Expressões em CEL** (`github.com/google/cel-go`, pura Go, sem CGO) **apenas no campo `onde:`**. CEL é não-Turing-completa: termina sempre, sem I/O nem efeitos colaterais, com *type-check* e **`cel.CostLimit`**. É a **única dependência nova**.
3. **Avaliação 100% no servidor (Go).** Nada de avaliador no cliente: duplicar Go↔JS é exatamente o custo já pago nos ícones (§6.9) e no rename (§6.14), que hoje exigem teste de paridade. Sem tocar bundle, CSP, `staticver` ou `@source` do Tailwind.
4. **Read-only e efêmero.** O motor depende de uma **interface somente-leitura** implementada pelo `*db.Store` (a única interface nova do projeto — justificada por testabilidade **e** por impedir escrita em tempo de compilação). ⚠️ **O resultado nunca é gravado no `.md`**: o watcher reindexaria a nota, o resultado mudaria e o processo se repetiria (loop de escrita).

**Duas velocidades (a razão da eficiência):**
- Chaves "de índice" (`tags`, `sem-tags`, `texto` (FTS5), `pasta`, `de`, `marcador`, `estado`) resolvem no SQL/FTS5/índice existente e entregam **no máximo 500 candidatos** — mesmo teto de `maxTagOnlyCandidates` (§10).
- `onde:` (CEL) só **refina** linha a linha esses candidatos. Nunca materializar o corpus (§10).

**Regra mental do contrato:** *tudo o que tem índice vira atalho; todo o resto vai no `onde:`*.

**Alternativas descartadas:** **JS do usuário no browser** (modelo Obsidian) ⇒ CSP restritiva, offline, mobile, ETag/`staticver` e impossível de testar com golden tests; **Lua (`gopher-lua`) / Starlark** ⇒ linguagem de propósito geral (sandbox, limites, API curada e UX de erro) — podem entrar **depois, atrás do mesmo contrato**, sem refazer nada; **SQL exposto** ⇒ vaza o schema, encoraja query cara, acopla ao banco; **parser próprio** ⇒ manutenção eterna e sem type-check; **`expr-lang/expr`** ⇒ plano B viável e mais ergonômico, mas perde type-check forte, padrão conhecido e cost limit; **`wazero`/WASM** ⇒ só se virar plataforma de plugins de terceiros.

**Contrato v1 (resumo):** `tipo` (obrigatório: `notas` | `tarefas` | `hierarquia` | `agenda` reservado) · `onde` (CEL) · `ordenar: campo [asc|desc]` · `limite: 1..200` (padrão 20) · `mostrar: lista|tabela|contagem|cartoes|arvore` · `colunas` (só com `mostrar: tabela`) · atalhos `tags`/`sem-tags`/`texto`/`pasta` (notas), `marcador`/`estado` (tarefas), `de` (hierarquia). Campos de `n.`/`t.`, macros, funções de texto e `agora`/`dias()`/`horas()` ficam na **fonte única da gramática** — 📖 [`docs/linguagem-de-consulta.md`](docs/linguagem-de-consulta.md) — para o DECISIONS **não duplicar** a gramática e não divergir dela.

**UX (decisão):** painel "Consultas" **abaixo do editor**, um cartão por bloco — contagem, resultado, botão *atualizar* via HTMX e **erro inline em pt-BR**. Motivo: as notas são editadas no TipTap (client-side) e **não existe renderizador de Markdown no servidor**; inserir HTML no meio do texto exigiria reescrever o pipeline do editor. Um **modo leitura** que inline os resultados é evolução planejada (registrado no doc de sintaxe, §13).
- Erro **é conteúdo**, nunca 500: sintaxe (com posição), campo desconhecido (com a lista de campos), tipo incompatível, custo/tempo estourado.
- `agora` é **injetado** no motor (nunca `time.Now()` interno) ⇒ resultado e testes determinísticos.

**Guardas e limites:**

| Guarda | Valor |
| :--- | :--- |
| Somente leitura | interface read-only (§ acima) |
| Nada persistido no `.md` | sempre |
| Blocos por nota | 10 |
| Candidatos pré-filtrados | 500 |
| `limite:` | máx. 200 |
| Tempo por bloco | 200 ms (`context.WithTimeout`) |
| Custo da expressão | `cel.CostLimit` |
| Cache | TTL 5 s + invalidação em qualquer escrita (padrão de `childrenCacheTTL`) |
| Env CEL | compilado **uma vez** no boot + cache de `Program` (e do erro de compilação) |

**Plano por fases (Fases 0–3 implementadas em 06/10/2026):**
- **Fase 0 — contrato e parser.** `processor/query.go`: `Spec`, `ParseBlock`, `ExtractBlocks` (fenced `consulta`) + golden tests. Aceite: `go test ./internal/processor/` verde, zero dependência nova.
- **Fase 1 — `tipo: notas` sem CEL.** `query/run.go` (interface read-only) + `mostrar: lista|contagem|tabela`, atalhos `tags`/`sem-tags`/`pasta`, `ordenar`, `limite`. Aceite: teste com banco real (`newTestStore`), ordenação estável (desempate por caminho), teto de 500 candidatos.
- **Fase 2 — `onde:` (CEL).** `query/engine.go`: `cel.Env` no boot, cache de `Program`, `CostLimit`, timeout e erros em pt-BR; campos de `n.` + `agora`/`dias()`/`horas()`. Aceite: expressão inválida → mensagem com posição; campo desconhecido → lista de campos; determinismo com `agora` fixo.
- **Fase 3 — render.** `query_block.templ` + `GET /api/query?file=…&i=…` (HTMX) + painel no editor (re-render ao salvar a nota, mesmo padrão de evento do badge de Tasks, §6.15). Aceite: E2E com 3 blocos (sucesso, erro e truncamento).
- **Fase 4 — `tipo: tarefas` e `tipo: hierarquia`** (reusam `todos` §6.15 e `GetHierarchy` §6.24).
- **(v2, fora do v1)** `similar:` (semântica em 2 etapas — o embedding é calculado no navegador, §3.1), `tipo: agenda` (depende de fixar os campos sobre o modelo real de `appointments`), `{{ }}` inline, modo leitura e ações com pré-visualização.

**Guardas de teste:**
- `processor/query_test.go`: golden (bloco válido → `Spec`; chave desconhecida; YAML inválido; `tipo` ausente; enum inválido) + **paridade docs↔parser**: a lista canônica de chaves vive num `var` único usado pela validação e pelo teste (mesmo espírito do SSOT de prefixos, §6.24).
- `query/engine_test.go`: expressão válida/inválida, erro de tipo, cost limit, `agora` injetado, mesma entrada ⇒ mesma saída.
- `query/run_test.go` (SQLite real): cada `tipo`/`mostrar`, ordenação, `limite`, teto de candidatos e nota sem bloco.
- **"Nada escreve"**: garantido pelo tipo da interface (tempo de compilação) + teste que roda o motor e compara conteúdo/`mtime` das notas antes e depois.

**Fora de escopo (explícito):** linguagem imperativa (`for`, variáveis, funções do usuário), ordenação por múltiplos campos, consulta ao conteúdo de PDF/EPUB, qualquer I/O ou rede, e qualquer escrita/ação.

**Documentação:** ADR [`docs/adr/0004-linguagem-de-consulta.md`](docs/adr/0004-linguagem-de-consulta.md) (contexto, alternativas, consequências) + referência de sintaxe [`docs/linguagem-de-consulta.md`](docs/linguagem-de-consulta.md) (fonte única da verdade, com receitas prontas, armadilhas de YAML e changelog). Índice de ADRs atualizado em `docs/adr/README.md`.

**Implementação — Fases 0 a 3 (06/10/2026)**

📍 `core/internal/processor/query.go` (+`query_test.go`) | `core/internal/query/engine.go`, `run.go`, `display.go` (+`query_test.go`) | `core/internal/features/notes/query_reader.go`, `handlers_query.go`, `query_block.templ` (+`handlers_query_test.go`) | `core/internal/features/notes/editor.templ` | `core/cmd/server/routes.go` | `core/web/src/editor-init.js`

- **Fase 0 — parser puro** (`processor`): `QuerySpec`, `ParseQuery`, `ExtractQueryBlocks`. YAML com `KnownFields(true)` ⇒ chave desconhecida vira erro nomeando as válidas; todos os erros traduzidos para pt-BR — inclusive a **dica de aspas** do `onde:`, que é a armadilha nº 1 (`onde: "x" in n.tags` precisa de aspas simples, senão o YAML quebra).
- **Fase 1 — executor** (`query.Run`): pré-filtro por índice (`tags`, `sem-tags`, `texto`/FTS5, `pasta`, `marcador`, `estado`, `de`) com teto de **500 candidatos**; `ordenar` com direção natural + desempate por caminho; `limite`; `contagem` sobre o total (antes do limite).
- **Fase 2 — CEL** (`query.Engine`): `cel.Env` com `n`/`t` como mapa `string→dyn`, `agora` injetado + `dias()`/`horas()`; cache de AST (e do erro de compilação); `CostLimit` e timeout de 200 ms; erros traduzidos (`campo desconhecido "x" — campos de n: …`).
- **Fase 3 — painel**: `GET /api/query?file=…` renderiza o fragmento (`query_block.templ`) com um cartão por bloco; o editor ganhou o container `#query-panel` (`hx-trigger="load, query-blocks-updated from:body"`, swap `innerHTML`) e o `editor-init.js` dispara `query-blocks-updated` **depois do save**, ao lado do `todos-updated` (§6.15). Apresentações: `lista`, `contagem`, `tabela` (`colunas:` validado numa lista canônica), `cartoes` (usa o snippet do FTS5) e `arvore`.
- **Tipos implementados**: `notas`, `tarefas` e `hierarquia` (com `de:` opcional e resolução por nome-base — as mesmas regras do §6.17/§6.24).
- **Read-only por construção**: o motor só conhece `query.Reader` (interface de leitura implementada por `queryReader` sobre o `*db.Store`); o texto do bloco nunca é reescrito — guardado por `TestHandleQueryPanel_NaoEscreve`.
- **Snapshot por requisição (correção de eficiência medida)**: o `queryReader` memoiza cada leitura com `sync.Once`, então uma nota com N blocos faz **uma leitura por tabela**, não N. Antes disso, medido com 300 notas em SQLite real: 3 blocos = **22 leituras** (7 por bloco) e **35,7 ms** no caminho real do painel; depois = **13,2 ms** (2,7× mais rápido; 1/3/10 blocos = 5,2/16,9/31,7 ms). Efeito colateral desejado: todos os cartões da página mostram a **mesma fotografia** do acervo (antes, uma escrita no meio podia fazer dois blocos discordarem). Guardado por `TestQueryReader_MemoizaLeituras` (identidade de fatia/mapa, não tempo).
- **Listas canônicas sem terceira fonte**: `colunas:` é **derivada** da união dos campos de `n` e `t` + as calculadas (`interacao_ha_dias`, `modif_ha_dias`, `n_tags`). Antes existia uma lista própria de colunas, e ela recusava `arquivo` — nome que o próprio renderer usa como coluna padrão de `tarefas`. `TestColunas_CobremAListaCanonica` garante que toda coluna aceita tem renderização (coluna "válida" que sai vazia é erro silencioso para o usuário).
- **`n.peso` saiu do v1**: estava anunciado na documentação e na lista de campos, mas o motor não o fornecia (daria "campo desconhecido" só em runtime). Voltou para a v2 junto de `n.tamanho`/`n.palavras` — o motor v1 lê **somente o banco**.
- **Paridade parser ↔ documentação**: `TestQueryLists_DocParity` lê `docs/linguagem-de-consulta.md` e exige que as chaves, os campos de `n`/`t` e a lista de `colunas:` do documento sejam **exatamente** as do parser (mesmo espírito do SSOT de prefixos, §6.24).
- **E2E no navegador** (`core/web/tests/query-panel.spec.ts`): prova o que nenhum teste de HTML gerado alcança — o fragmento é buscado pelo HTMX no load, o botão "atualizar" dispara uma requisição NOVA, **salvar a nota dispara `query-blocks-updated` e o painel se recarrega sozinho** (e o container sobrevive ao swap, senão o listener morreria), e bloco inválido vira texto no cartão com **HTTP 200**. As notas do teste usam etiqueta/nome únicos (`zz-e2e-<timestamp>`) e são apagadas no `finally`, porque o spec roda contra o vault de verdade.
  - ⚠️ **Achado no caminho**: os specs antigos preenchem `#user` no login, que **não existe** — o formulário atual pede só a senha (app de usuário único, "admin" implícito). O spec novo entra com `#pass` e serve de referência para consertar os antigos.

**Validação:**
- `go build -tags sqlite_fts5 ./...` OK; `gofmt -l` vazio e `go vet -tags sqlite_fts5 ./internal/...` limpo.
- `go test -tags sqlite_fts5 ./internal/processor/ ./internal/query/ ./internal/features/notes/` verde — parser (golden + paridade com o doc), motor (filtro/ordenação/limite/teto de candidatos/custo/determinismo com `agora` fixo) e painel com **SQLite real** (`newTestContext`).
- `templ generate` (v0.3.1020) + `node build.js` (Tailwind 4 + esbuild) aplicados: as classes novas estão em `web/static/app.css` e `query-blocks-updated` em `web/static/editor-init.js`. ⚠️ **Reiniciar o servidor** para o `staticver` servir os estáticos novos.
- **E2E**: `npx playwright test tests/query-panel.spec.ts` → **3/3** (Chromium headless, contra o servidor real). Exigiu `npx playwright install chromium` (o cache de browsers é por versão do Playwright).
- **Frontend**: `tsc --noEmit` limpo e `node --test tests/*.unit.cjs` **47/47** (5 guardas novas do painel).
- Cobertura negativa conferida à mão durante o desenvolvimento: campo desconhecido, erro de sintaxe, tipo incompatível, expressão não-booleana, coluna desconhecida, `de:` inexistente e expressão acima do teto de custo — todos aparecem como texto dentro do cartão, nunca como 500.

---

## 6.30 Filha que é ARQUIVO no popover "Abaixo desta" — destino pela regra canônica (15/10/2026)

📍 `core/internal/core/domain/data.go` (`NoteRef.OpenURL`/`OpenInNewTab`/`NoteType`) | `core/internal/features/notes/hierarchy_bar.templ` (`ChildrenBadge`) · testes: `core/internal/core/domain/noteref_open_test.go` | `core/internal/features/notes/hierarchy_bar_render_test.go`

**Sintoma:** abrir um PDF listado no popover **"Abaixo desta:"** (o contador de filhas da barra de hierarquia) mostrava uma **nota vazia** em vez do PDF.

**Causa:** o `<a>` do popover tinha o destino **fixo**: `templ.SafeURL("/editor?file=" + SafeFileQueryEscape(c.Filename))`. Mas a lista de filhas **mistura notas e arquivos** — o `pai` de um PDF/EPUB/ZIP vive na tabela `file_metadata`, não no frontmatter (§6.24). Para um binário o editor não tem conteúdo nenhum, e o resultado era uma nota em branco (pior que um 404: parecia perda de dado).

**Decisão:** o destino de uma referência de navegação **nunca** é montado à mão. `NoteRef` ganhou `OpenURL()`/`OpenInNewTab()`, que derivam o tipo do caminho (`DetectNoteType(nil, filename)` — o `NoteRef` não carrega as tags persistidas, e a detecção por caminho é determinística) e delegam para `domain.NoteOpenTarget`. É a **mesma** regra já usada pela sidebar (`getNoteLink`), pelo Tabulator (`handlers_database_tabulator.go`) e pelo painel de Consultas (`query.Row.OpenURL`).

| Filha | Destino | Aba |
| :--- | :--- | :--- |
| `notes/x.md` | `/editor?file=…` | mesma |
| `notes/meu-desenho.md` | `/drawing?file=…` | mesma |
| `pdfs/x.pdf` | `/file?name=…` | nova |
| `epubs/livro.epub` | `/epub/reader?file=…` | mesma |
| `attachments/x.zip` | `/file/download?name=…` | nova |

**Alternativa descartada:** repetir o `switch` de prefixos (`pdfs/`, `attachments/`, `.epub`) dentro do template — é exatamente o caminho que criou o bug (a sidebar já tinha a cópia dela) e que já foi motivo de divergência nos ícones (§6.9) e no rename (§6.14).

**Guardas:** `TestNoteRef_OpenURLSegueNoteOpenTarget` amarra o `NoteRef` ao `NoteOpenTarget` (qualquer tipo/caminho tem que dar o mesmo destino — trava contra um `switch` paralelo futuro) e `TestChildrenBadge_RotaDeCadaFilha` renderiza o componente **de verdade** e confere `href`/`target` de cada filha, incluindo a asserção negativa de que nenhum arquivo cai em `/editor?file=`.

---

## 7. Modo Perguntar & Síntese RAG via Ollama (14/10/2026) — ❌ REMOVIDO (16/10/2026)

> ❌ **Tudo removido:** as rotas `POST /api/ask` e `POST /api/ask/synthesize`, os handlers
> (`handlers_ask.go`, `handlers_ollama.go`), o botão/inputs/JS na página de busca, a aba
> "IA (Ollama)" das Configurações e as settings `ollama_url`/`ollama_model`.
> O que restou deste capítulo é **registro histórico**. Ver §12 (Remoção definitiva).

### 7.1 Arquitetura

O Modo Perguntar combina **busca híbrida local (FTS5 + KNN semântico)** com **síntese generativa local via Ollama**:

1. **Pipeline Extrativo Rápido (Backend Go - `/api/ask`)**:
   - `handlers_ask.go`: Executa busca em linguagem natural eliminando stopwords de perguntas (`como`, `qual`, `onde`, etc.) e gerando consulta FTS5 flexível com OR e delimitadores `containsWord`.
   - Fusão RRF com busca vetorial (`sqlite-vec`).
   - Extrai os 3 a 5 melhores blocos lógicos das notas com ranking de densidade semântica e contextual.
   - Responde em < 50ms com alucinação zero e links diretos para as notas.

2. **Pipeline Generativo RAG via Ollama (`POST /api/ask/synthesize`)**:
   - Substituição total do modelo in-browser (SmolLM2/Transformers.js) por chamada direta ao **Ollama local** via HTTP SSE (Server-Sent Events).
   - O modelo roda nativamente na GPU do host (ex: `llama3.2`, `qwen2.5:7b`), sem baixar pesos no navegador e sem travar a interface web.
   - Configuração de URL e modelo em **Configurações → IA (Ollama)** persistida no SQLite (`ollama_url`, `ollama_model`).
   - Se o Ollama não estiver configurado, a busca exibe apenas os cards das notas diretamente, sem tentar síntese generativa.

---

## 8. Módulo Mobile Android (`android/`) — 14/10/2026 — ❌ REMOVIDO (15/10/2026)

> ⚠️ **Removido em 15/10/2026.** A pasta `android/` e o job `android-apk` do
> `.github/workflows/deploy_core.yml` foram **deletados**. A manutenção do módulo mobile
> (NDK/cgo, extração de assets, assinatura de APK, Android Studio) atrasava o desenvolvimento
> do app principal sem retorno proporcional. O TON-618 passa a ser distribuído apenas via
> **Docker** (web). Os secrets `ANDROID_KEYSTORE_*` no GitHub podem ser removidos.
>
> A flag de config `MODEL_DOWNLOAD` (§8.7) **permanece no core** (é genérica e sem efeito
> colateral), mas o app que a usava não existe mais. O conteúdo abaixo fica como **registro
> histórico** da decisão original.

### 8.1 Arquitetura

- Estrutura autônoma na pasta `android/` (Kotlin + AndroidX/Material).
- `TonServerService` (`ForegroundService`) extrai e executa o binário Go do `core`, servindo em
  `0.0.0.0:6180` (loopback para o app + rede local Wi-Fi para o PC).
- `MainActivity` = painel de controle: status, IP na Wi-Fi, copiar link, iniciar/parar/reiniciar e logs ao vivo.
- `WebActivity` = `WebView` que carrega `http://127.0.0.1:6180` (botão "Abrir Notas") e responde ao
  BasicAuth do core em `onReceivedHttpAuthRequest` com as credenciais fixas
  (`AUTH_USER`/`AUTH_PASS` em `TonServerService`) — sem prompt ao usuário.

### 8.2 Distribuição dos artefatos

- **Binário Go**: `app/src/main/jniLibs/<abi>/libton618.so` — vive no `nativeLibraryDir`, que é
  executável (o Android 10+ bloqueia exec de arquivos em `filesDir`). **Não** há cópia em `assets/`
  (era o mesmo binário duas vezes dentro do APK).
- **Assets web**: `app/src/main/assets/web/static/**`, extraídos na primeira execução para
  `filesDir/web/static` (o `WEB_DIR` do core).
- `android/build-core-arm64.sh` produz os dois. `ANDROID_ABI` escolhe a ABI (arm64-v8a, x86_64,
  armeabi-v7a) e `ANDROID_SEMANTIC=1` inclui o onnxruntime-web.

### 8.3 ⚠️ Extração de assets precisa ser RECURSIVA

- **Sintoma**: a página abria (HTML renderizado pelo servidor, `app.css` presente), mas **nenhum
  recurso funcionava** — não criava nem pesquisava notas, parecia tudo "morto".
- **Causa**: `extractWebAssets()` usava `assets.list("web/static")`, que devolve apenas o **primeiro
  nível**, e ignorava subdiretórios. Resultado: 404 em `/static/js/vendor/htmx.min.js`,
  `/static/js/vendor/alpine.min.js`, `/static/js/app.js` e `/static/fonts/fonts.css`. O frontend é
  HTMX + Alpine desde a base — sem esses três arquivos nada responde.
- **Correção**: cópia recursiva (`copyAssetTree`), stamp de versão (`versionCode` + `lastUpdateTime`)
  para re-extrair quando o APK muda, e verificação de `REQUIRED_WEB_ASSETS`, cuja ausência agora
  aparece como erro explícito no painel de logs do app.
- **Regra**: todo arquivo novo em `web/static/**` (inclusive em subpastas) deve continuar sendo
  copiado pelo script de build; `REQUIRED_WEB_ASSETS` deve acompanhar o `layout.templ`.

### 8.4 Redução de tamanho do APK

- Removida a cópia duplicada do binário em `assets/` (~20 MB crus).
- `.gz`/`.br` não vão mais para o APK: o core já comprime as respostas em runtime (chi `Compress`),
  então os pré-comprimidos eram bytes pagos duas vezes.
- `web/static/models/ort/` (22 MB do onnxruntime-web) fica **fora por padrão**: a busca semântica no
  celular exige baixar ~120 MB de modelo + inferência ONNX em CPU. Reativável com `ANDROID_SEMANTIC=1`.
- Build do core sem CGO (`CGO_ENABLED=0`, `-buildmode=pie`): o SQLite é o `modernc.org/sqlite`
  (Go puro), logo o Android NDK era desnecessário; o fallback antigo (`GOOS=linux`) foi removido
  por poder gerar um binário não executável no Android.

### 8.5 Senha configurável no app

- A senha do core era **fixa no código** (`admin`/`ton618`). Agora vem de `TonSettings`
  (`SharedPreferences` privado do app) e é editável na `MainActivity`.
- O core valida **somente a senha**: `middleware.checkCredentials` ignora o username do token Basic
  Auth, e a tela `/login` também usa usuário interno fixo. `config.Load()` cai no default `ton618`
  quando `AUTH_PASS` é vazio — por isso a validação do app recusa senha em branco.
- Ao salvar: `AUTH_PASS` do processo Go é atualizado (vale também para o acesso pelo PC em
  `http://<ip>:6180`), o serviço é reiniciado e cookies + localStorage do WebView são descartados
  (o token `ton_auth` antigo ficaria cacheado). O localStorage do TON-618 guarda só estado de UI
  (linhas expandidas, colunas, última consulta) — nenhum conteúdo de nota é perdido.
- O `WebActivity` semeia `ton_auth=base64("admin:<senha>")` via `CookieManager` **antes** de
  navegar. Necessário porque o core **não** responde 401 em navegação de página: ele redireciona
  para `/login`. Sem o cookie, o usuário cairia na tela de login dentro do app.

### 8.6 cgo é OBRIGATÓRIO no build Android (DNS) — nunca compilar com CGO_ENABLED=0

- O SQLite é `modernc.org/sqlite` (Go puro), o que sugeriu que daria para dispensar o NDK. **Não dá**:
  no Android o Go **precisa** de cgo para resolver DNS. O sistema resolve via `netd` (socket Unix)
  e não publica um `/etc/resolv.conf` utilizável — por isso `net/conf.go` força o resolver cgo no
  Android (`goosPrefersCgo` → `case "android"`, [golang/go#10714](https://github.com/golang/go/issues/10714)),
  e `mustUseGoResolver` devolve `true` quando o binário foi compilado sem cgo (`!cgoAvailable`).
- **Sintoma** (ocorreu em 14/10/2026): `lookup huggingface.co on [::1]:53: read udp [::1]:…:
  connection refused`. Esse formato (`lookup … on <servidor>:53`) é exclusivo do resolver **Go puro** —
  é a assinatura que identifica o build sem cgo. Quebra: download do modelo de embeddings, legendas
  do YouTube, clip de artigos (readability) e notificações ntfy.
- **Build correto**: `CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC=<ndk clang> -buildmode=pie`.
  O `build-core-arm64.sh` exige o NDK (falha com mensagem clara se não achar; sem fallback para
  `GOOS=linux`, que geraria binário glibc inexequível no Android). No APK isso aparece como
  `🤖 NDK: …/aarch64-linux-android<NN>-clang`.
- **Validação automática**: o script confere o buildinfo do binário (`grep -aq "CGO_ENABLED=1"`), que
  sobrevive a `-ldflags="-s -w"`. ⚠️ Não usar `cgoLookupIP` nem `/system/bin/linker64` como
  heurística — ambos foram verificados **presentes** num build sem cgo (dariam falso OK).

### 8.7 No Android o download local do modelo de embeddings fica desligado

- O core baixa ~120MB (`Xenova/paraphrase-multilingual-MiniLM-L12-v2`) no boot para `STATE_DIR/models`.
  No aparelho isso ia para o armazenamento interno do app sem necessidade.
- Nova flag de config **`MODEL_DOWNLOAD`** (padrão `true`). O app Android sobe o processo com
  `MODEL_DOWNLOAD=false`; nesse caso o core **não** baixa e **não** injeta o modelo no
  `embeddings.HandlerContext` (equivalente a não chamar `WithModel`).
- Consequência na UI: `/api/embeddings/model-status` responde `ready: true` e **sem** `last_error`
  (caminho "sem modelo local" já previsto no handler) — o painel "Servidor X%" simplesmente não
  aparece, em vez de exibir o erro cru do resolver (era o que acontecia sem rede/cgo).
- O browser continua funcionando: o `semantic-worker.js` usa `env.localModelPath = "/models/"` com
  `allowRemoteModels = true`, então busca o modelo no CDN do HuggingFace — o mesmo caminho já usado
  no desktop enquanto o download não termina.
- Desktop/Docker **não** mudam: o download segue ligado (o volume persistente evita rebaixar).

### 8.8 Assinatura estável do APK — atualizar não pode exigir desinstalar (15/10/2026)

📍 `android/app/build.gradle.kts` | `.github/workflows/deploy_core.yml` (job `android-apk`) · doc: `android/README.md` › "Assinatura do APK"

**Problema:** era preciso **desinstalar** o app para instalar qualquer versão nova, e desinstalar **apaga todo o dado** (`filesDir` com `ton618.db` + `docs/`, `shared_prefs` com a senha, `app_webview` com o cache do WebView/modelo ONNX).

**Causa:** o CI rodava `gradle assembleDebug` **sem nenhuma `signingConfig`**. Sem configuração, o AGP assina com o *debug keystore* que ele mesmo cria em `~/.android/debug.keystore` — e essa chave é **gerada aleatoriamente** na primeira vez que o arquivo não existe. Cada runner do GitHub Actions começa limpo, então **cada build saía com uma chave diferente**; o Android recusa instalar por cima quando a assinatura muda (`INSTALL_FAILED_UPDATE_INCOMPATIBLE`). O mesmo valia para o APK gerado no Android Studio local, cuja chave de debug é de outra máquina.

**Decisão (3 pontos):**
1. **O APK de release passa a ser assinado por um keystore FIXO**, guardado **fora do repositório** e injetado pelo ambiente (`TON618_KEYSTORE_PATH`/`_PASSWORD`, `TON618_KEY_ALIAS`, `TON618_KEY_PASSWORD`). No CI os valores vêm dos GitHub Secrets `ANDROID_KEYSTORE_BASE64`/`_PASSWORD`, `ANDROID_KEY_ALIAS` e `ANDROID_KEY_PASSWORD`, com o `.jks` decodificado para `$RUNNER_TEMP`. O `android/.gitignore` ignora `*.jks`/`*.keystore`.
2. **`versionCode` deriva da tag** (`vX.Y.Z` → `MAJOR*10000 + MINOR*100 + PATCH`), passado como `-PappVersionCode=…`. É o outro requisito do Android (o código não pode diminuir) e evita um contador paralelo ao do `bump-version`.
3. **O CI nunca publica um APK inútil**: `assembleRelease` sem keystore produz `app-release-unsigned.apk`, que **não instala**. Quando o secret não existe, o workflow cai para `assembleDebug` e emite `::warning::` + a linha *Assinatura* no Job Summary — o APK continua válido, mas a chave é volátil e a atualização por cima volta a exigir desinstalar.

**Sem `signingConfig` no variant `debug`:** mexer nele quebraria o build local de quem não tem o keystore, e o `debug` já é o default do AGP. O `release` só recebe `signingConfig` **quando** o keystore existe — com isso a configuração não pode falhar por causa de variável ausente.

**Alternativa descartada:** *commitar* o keystore no repositório. Seria mais simples (zero secrets), mas o repositório é público: qualquer um assinaria um APK que o aparelho aceita como atualização do TON-618 — e o app guarda a senha e todas as notas. Também descartado depender do Auto Backup do Android (`allowBackup="false"`, §9): a restauração pós-reinstalação é opcional, não determinística e pode trazer estado velho.

**Validação:** `gradle assembleRelease -PappVersionCode=… -PappVersionName=…` produzindo `app-release.apk` assinado (e falha explícita no passo "Preparar nome do APK" se o arquivo não existir); instalação por cima (`adb install -r` / tocar no APK) preservando `ton618.db` e `docs/`; e o *stamp* de assets (`versionCode` + `lastUpdateTime`, §8.3) continuando a re-extrair o bundle a cada versão.

**Migração:** a chave do APK instalado hoje não existe mais, então a **primeira** instalação do release exige desinstalar uma última vez — coberta pelo Backup Total (§9): exportar → desinstalar → instalar → restaurar.

---

## 9. Backup Total — exportar e restaurar (15/10/2026)

A aba Configurações → Backup passa a ter 3 opções. As duas primeiras (Rápido e Completo) continuam
**somente de download**; a terceira é a única que pode ser **importada** de volta.

### 9.1 Formato

ZIP com três partes:

| Entrada | Conteúdo |
|---------|----------|
| `manifest.json` | formato/versão, data, contagens, tamanho e SHA-256 do banco |
| `db/ton618.db` | snapshot consistente do SQLite (`VACUUM INTO`) |
| `docs/**` | todos os arquivos do disco, **inclusive `archives/`** |

- **Snapshot do banco, não export por entidade.** Fidelidade total (configurações, agenda,
  marcadores de tarefas, hierarquia de arquivos em `file_metadata`) com muito menos código e sem o
  risco de esquecer um campo novo. A contrapartida é acoplar ao schema — aceitável porque
  `db.NewStore` roda as migrações no boot: um backup antigo é migrado ao restaurar.
- **Dados derivados ficam de fora**: a lista `snapshotDroppedTables` ficou **vazia** em 16/10/2026
  (a única entrada era o índice semântico, que não existe mais — §12). O mecanismo continua
  disponível para futuras tabelas derivadas. O banco vivo nunca é alterado — a limpeza acontece
  na cópia (`db.Store.Snapshot`).
- `VACUUM INTO` em vez de copiar o arquivo: aplica o WAL pendente e nunca produz cópia rasgada de um
  banco em uso.
- **Falha ao ler as notas aborta o backup (corrigido em 22/10/2026):** `CreateStreamContext` descartava
  o erro de `GetAllNotes` e gerava um ZIP **sem as notas** com status 200 (perda silenciosa). Agora o
  erro é propagado ao handler (`backup: carregar notas: …`), com regressão em `backup_test.go`.

### 9.2 Restauração (`POST /api/restore`) — sempre SUBSTITUIÇÃO TOTAL

1. o ZIP é recebido em **streaming** para um arquivo temporário em `STATE_DIR` (nunca pela memória;
   teto de 4 GiB);
2. tudo é extraído para um staging **fora** de `docs/` e o banco é validado
   (`PRAGMA integrity_check` + presença da tabela `notes`) — **antes de tocar no estado atual**;
3. o estado atual vai para quarentena (`STATE_DIR/.pre-restore-<timestamp>/`: banco + `docs/`);
4. o backup é aplicado (rename, mesmo filesystem) e o processo **encerra graciosamente**.

- Falha nos passos 1–2 deixa o app exatamente como estava (coberto por teste).
- **Por que reiniciar:** o processo mantém o banco aberto; trocar o arquivo em disco sob a conexão
  viva exigiria invalidar caches (índice de hierarquia, status de indexação). O
  encerramento é pedido por `services.RequestRestart()` e o `main` faz shutdown graciosamente — o
  supervisor traz o servidor de volta (`restart: unless-stopped` no Docker).
- **Zip slip:** todo caminho de entrada é validado (`safeZipRel`); um ZIP com `../../x` é recusado.
- **Modo único (substituir):** decisão explícita. Mesclar exigiria semântica por entidade (dedupe,
  conflito de mtime, hierarquia de arquivos) — bem mais código e mais risco de dado inconsistente.

---

## 13. Remoção definitiva do Modo Perguntar, da síntese por IA e do índice de embeddings (16/10/2026)

**Decisão:** o TON-618 **não tem mais** IA de recuperação/síntese. Foram removidos o "modo
Perguntar", a busca semântica, a busca híbrida (RRF), o índice de embeddings, o download do modelo
ONNX e toda a configuração de Ollama. O app volta a ter **dois** modos de busca (Notas e Global) e
toda a recuperação é FTS5/`LIKE`, calculada no servidor.

**Por quê:** o custo/benefício não se pagava. Os embeddings eram gerados no browser
(Transformers.js, ~120 MB de modelo carregado a cada sessão e indexação lazy só ao abrir a busca),
o que fazia o índice valer menos do que o esperado; a qualidade dependia de heurísticas de
compensação (voto majoritário por nota, thresholds em %); e o modo Perguntar precisava de um
servidor LLM externo (Ollama) configurado à mão para entregar a parte "generativa" — sem ele,
sobrava um extrator de trechos.

### 13.1 O que foi removido (código)

| Área | Removido |
| --- | --- |
| Feature | `internal/features/embeddings/` (handlers, model_files, context) |
| Handlers | `search/handlers_ask.go`, `search/handlers_ollama.go`, `search/handlers_hybrid.go` (+ testes) |
| Fusão RRF | `internal/search/rrf.go` (+ teste) |
| DB | `db/embeddings.go` (vec0/chunks/status), as queries sqlc de chunks/embeddings e o modelo `NoteChunk` |
| Server | rotas `/api/ask`, `/api/ask/synthesize`, `/api/embeddings/*`, `/api/search/hybrid`, `/api/settings/{ollama,semantic-thresholds,semantic-device}` e a rota pública `/models/*` |
| Config | `ModelDir`/`ModelDownload` (`MODEL_DIR`, `MODEL_DOWNLOAD`) e `EMBEDDING_ALL` |
| Frontend | `web/src/semantic.js`, `semantic-worker.js`, `ort-wasm.js`, `download_model.js`; aba **Semântica** e aba **IA (Ollama)** das Configurações; botão/inputs do modo Perguntar; coluna "Indexada" do Tabulator |
| Assets | `web/static/models/` (ORT + modelo), `static/semantic*.js`, dep `@huggingface/transformers` (web e scripts) |
| CSP | origens do HuggingFace e do jsdelivr (existiam só para o worker ONNX/`ort-wasm.js`) |

### 13.2 Banco de dados — migração v23

- As migrações **v3–v8** (criação/migração de `note_chunks` e da vec0 `note_embeddings`) foram
  **removidas do código**: bancos novos não criam mais essas tabelas.
- A nova migração **v23** limpa bancos já existentes: `DROP TABLE note_embeddings`,
  `DROP TABLE note_chunks` e `DELETE` das settings órfãs (`embedding_model_version`,
  `semantic_search_threshold`, `hybrid_semantic_threshold`, `rrf_k`, `semantic_device`,
  `ollama_url`, `ollama_model`).
- ⚠️ **`internal/core/db/vec_cleanup.go` é TRANSITÓRIO**: ele importa `modernc.org/sqlite/vec`
  apenas porque o SQLite recusa `DROP TABLE` de uma tabela virtual cujo módulo não está registrado —
  sem isso, a vec0 ficaria órfã no banco para sempre. Pode ser apagado (junto com o import) quando
  todos os bancos já tiverem passado pela v23.

### 13.3 O que NÃO foi removido

- **Busca Global (FTS5) e Busca de Notas (`LIKE`)**: intactas — são o motor de recuperação do app.
- `internal/search/*` (ranker, snippets, gates), o rate limiter de busca e o Tabulator.
- **Auto-Tag por inatividade** (`auto_tag_service.go`): nunca dependeu de IA.
- `modernc.org/sqlite` continua no `go.mod`; só o import do pacote `vec` sai com o
  `vec_cleanup.go`.

### 13.4 Validação

- `templ generate`, `gofmt`, `go vet ./...` e `go test ./...` — verdes.
- Frontend: `npm run build`, `npm run typecheck` e `npm test` (50 testes) — verdes.
- Testes removidos junto com o recurso: embeddings (unit/integração), hibrida, ask/synthesize,
  thresholds/device das settings, snapshot de tabelas derivadas, rename preservando chunks e os
  guardas de CSP do CDN do ONNX/HuggingFace.

## 14. Nota Tabulator — planilha de inserção de dados (18/10/2026)

📍 `core/internal/processor/tabulator.go` | `core/internal/features/notes/handlers_tabulator.go` | `core/internal/features/notes/tabulator.templ` | `core/web/src/tabulator-note.js` | `core/internal/core/services/backup.go` | `core/internal/core/domain/data.go` | `core/internal/processor/markdown.go`

**Decisão:** novo tipo de nota `Tabulator` — uma nota-planilha de **inserção de dados**
(linhas e colunas), aberta em tela cheia em `/tabulator`. É um tipo **distinto** do
`!Tabulator`/`/database`, que continua sendo a página de **administração de notas**.

**Por quê:** faltava no app um lugar para manter pequenos conjuntos de dados tabulares
(catálogos, listas, coleções) sem sair do ecossistema de notas. Como os dados vivem no
corpo da própria nota, ela reusa o que já existe — tabela `notes`, save/rename/delete,
hierarquia, tags e backups — e, diferente de desenho/markmap, participa da **busca global**.

### 14.1 Formato no disco

O corpo da nota guarda o payload JSON; o frontmatter é o habitual (`type: tabulator`):

```markdown
---
type: tabulator
---

{"columns":[{"field":"c1","title":"Nome"},{"field":"c2","title":"Idade","type":"number"}],"rows":[{"c1":"Ana","c2":30}]}
```

- `columns`: `{field, title, type?, width?}` — `field` é a chave usada nas linhas.
- `rows`: lista de mapas `field → valor`. A ordem das linhas é a posição no array
  (não há `_id`).
- `type` da coluna: `string` (default), `number`, `date`, `boolean`, `file`. O tipo é
  **opcional** e só influencia o editor da célula e o comparador da ordenação — não muda o
  formato gravado (valores são strings/números JSON como digitados). Colunas `file` guardam
  na célula o **caminho** do arquivo enviado (ver 14.5).

### 14.2 Indexação e busca

- As linhas são indexadas no FTS: **cada linha** vira um fragmento (`Document`) da seção
  `Registro N`, com texto no formato `Título: valor | Título: valor`; os nomes das colunas
  viram um fragmento extra da seção `Colunas`.
- Por isso, buscar por uma célula (`Recife`) **e** por um nome de coluna (`Cidade`)
  encontra a nota — comportamento diferente de `drawing`/`markmap`, cujo corpo não é indexado.
- `type: tabulator` adiciona a tag canônica `tabulator` (persistida) e a inclui em
  `InternalTypeTags`: ela não aparece como tag para o usuário (sidebar, Configurações e
  resultados de busca).
- Numa coluna `file` (14.5), o texto indexado é o **caminho** gravado (`datasets/<nome>`), de
  modo que buscar pelo **nome do arquivo** encontra a linha — mas o arquivo em si **não** é
  indexado como anexo (ver 14.5).

### 14.3 CSV

- Cada nota pode ser exportada como CSV pelo botão ⤓ do editor
  (`/api/tabulator/export?file=…`), com **BOM UTF-8** para o Excel reconhecer os acentos.
- No **Backup** (`/api/backup`), notas Tabulator saem como `.md` (fidelidade total,
  restaurável) **e** como `.csv` (visão de planilha). O backup total não muda: a nota vive
  no banco.

### 14.4 Ordenação, filtros e tipos

- **O tipo da coluna é opcional**: default `string`; só muda o editor da célula
  (texto/número/data/sim‑não/arquivo) e o comparador da ordenação. **Não há seletor de tipo na
  barra**: a coluna nasce como **Texto** e o tipo é definido/alterado no menu do cabeçalho
  (**⋮ → Tipo da coluna**), junto de renomear/excluir coluna.
- **Ordenação**: clique no título da coluna; o comparador (`sorter`) segue o tipo
  (`string`/`number`/`date`/`boolean`; colunas `arquivo` ordenam por `string`, pelo caminho).
- **Filtro por coluna**: cada coluna tem um input no cabeçalho (“contém”, case-insensitive).
- **Filtro rápido**: caixa “Filtrar nesta tabela…” aplica “contém” em todas as colunas.
- ⚠️ **Ordenação e filtros são apenas visuais.** A ordem gravada no arquivo segue a ordem
  canônica das linhas (campo interno `_idx`, não persistido): ordenar/filtrar para inspecionar
  nunca reordena nem descarta dados no disco. A serialização usa `getRows()` (todas as linhas,
  ignorando filtros) e reordena por `_idx`.

### 14.5 Coluna de arquivo (envio por clique na célula)

- Tipo de coluna **`file`** (`type: "file"` no JSON da coluna): a célula não tem editor de
  texto — ao clicar, abre o seletor de arquivos e o envio vai para **`POST /api/tabulator/upload`**
  (multipart, campo `file`). A célula passa a guardar o **caminho** (`datasets/<nome>`) e exibe
  um link para abrir/baixar + um “×” para remover.
- Os arquivos vão para **`docs/datasets/`** — um diretório **interno**
  (`domain.DatasetPrefix` / `InternalFilePrefixes`): está em `AllowedFilePrefixes` (por isso é
  servido por `/file/download`) mas **não** entra em `FileDirs`, então o watcher não o varre e o
  arquivo **não é indexado nem aparece na lista lateral/busca** como anexo. O invariante
  `NonNoteFilePrefixes ↔ FileDirs` continua válido (o teste de prefixos cobre o caso).
- O nome enviado é sanitizado (`processor.TabulatorUploadFilename`: só o basename, sem
  separadores/caracteres de controle, tamanho limitado preservando a extensão) e, se já existir,
  recebe sufixo `-2`, `-3`… — o arquivo anterior **não** é sobrescrito.
- Limite de **64 MiB** por arquivo; **um arquivo por célula** (para vários, use vários registros).
- **Exclusão real (sem órfãos):** quando o arquivo deixa de ser referenciado, ele é apagado do
  disco. A reconciliação roda no **save da nota** (`notes.ReconcileTabulatorFiles`, chamado por
  `doSaveNote`): compara as referências `datasets/...` do conteúdo anterior com as do novo e
  remove o que sobrou — cobre limpar a célula (×), trocar o arquivo, excluir o registro e
  excluir a coluna. Excluir a nota também apaga os arquivos dela (`HandleFileDelete`,
  `NoteService.Delete` e a exclusão em massa do Arquivamento).
- A remoção só acontece se **nenhuma outra nota** ainda referenciar o arquivo
  (`Store.CountOtherNotesReferencing`, casamento literal com os curingas do `LIKE` escapados) —
  notas **duplicadas** apontam para os mesmos arquivos, e a guarda evita que a cópia perca o
  anexo. Só caminhos válidos de `datasets/` (um único nível, sem `..`) são considerados
  (`processor.IsTabulatorFilePath`); se a verificação falhar, o arquivo é **preservado**.

### 14.6 Editor (frontend)

- `notes/tabulator.templ`: página de tela cheia (padrão de `drawing`/`mindmap`), com autosave
  por debounce em `/api/note/save`, rename/duplicar/excluir, exportar CSV e os botões
  **+ Novo registro** / **+ Nova coluna**.
- `web/src/tabulator-note.js` (bundle esbuild, declarado em `web/build.js`): monta a tabela
  Tabulator (editor por tipo de coluna, coluna `#` e coluna de ações com botão de excluir
  registro) e o menu do cabeçalho (**Renomear coluna / Tipo da coluna / Excluir coluna**).
  Usa a biblioteca já vendorizada (`static/tabulator.min.js` + tema
  `tabulator_midnight.min.css`), a mesma de `/database`.
- Entrada de criação: menu **Criar → + Tabulator** (desktop e mobile). A busca global roteia
  resultados do tipo `tabulator` para `/tabulator`.
- Tema próprio no `<style>` do template (paleta zinc do app): cabeçalho destacado, zebra e
  separadores visíveis, foco/edição em esmeralda — corrige o baixo contraste do tema
  `midnight` do Tabulator.
- **Armadilha (medida no navegador):** o Tabulator aplica a classe `.tabulator` **no próprio
  container**, ou seja, `#tabulator-table` **é** a raiz da tabela. Regras do tipo
  `#tabulator-table .tabulator …` (incluindo a que já existia no template) **nunca casavam**, e
  por isso o tema `midnight` cru aparecia (linhas `#666`, cabeçalho `#333`, bordas `#999`). Os
  seletores do tema agora são `#tabulator-table …` / `#tabulator-table.tabulator`.
- `layout: "fitColumns"` com `widthGrow: 1` nas colunas de dados (as colunas `#` e de ações têm
  largura fixa): as colunas ocupam a largura da tela, sem a faixa vazia à direita que o
  `fitDataFill` deixava. Largura ajustada manualmente pelo usuário não é persistida.

### 14.7 Validação

- `go build`, `go vet` e `go test -tags sqlite_fts5 ./...` — verdes; novos testes em
  `internal/processor/tabulator_test.go`, `internal/core/domain/data_tabulator_test.go` e
  `internal/features/notes/handlers_tabulator_test.go` (upload: grava em `datasets/`, sanitiza
  o nome e não sobrescreve em colisão).
- `tsc --noEmit`, `npm run build` e `npm test` — verdes; helpers puros em
  `core/web/tests/tabulator-note.unit.cjs`.
- E2E em `core/web/tests/tabulator.spec.ts` (executado em Chromium): criação/edição/autosave,
  tipo de coluna, **ordenação e filtros** (provando que não reordenam os dados gravados),
  **upload pela coluna de arquivo** (arquivo baixável, link persistente e remoção) e download do CSV.
- Remoção dos arquivos de célula coberta por testes: `processor` (extração/validação dos caminhos
  `datasets/...`), `internal/core/db/notes_refcount_test.go` (contagem de referências e escape dos
  curingas do `LIKE`), `internal/features/notes/tabulator_files_test.go` (apaga removido/trocado,
  preserva o de nota duplicada, nunca sai de `datasets/`) e no E2E (o arquivo some do disco
  depois do “×” e depois de excluir o registro).
- Verificado que um arquivo enviado em `datasets/` **não** entra na tabela `documents` (ou seja,
  não é indexado/listado).
- Verificado com o binário real (DATA_DIR isolado): render, save, busca por célula/coluna,
  CSV com acentos, ZIP com `.md`+`.csv`, redirect `/editor`→`/tabulator`, ícone próprio na
  sidebar e tipo `Tabulator` na página `/database`.

### 14.8 Visual compartilhado com a `/database` e ícone próprio do tipo (18/10/2026)

- O desenho/cores da tabela passou a ter **uma única fonte**: `web/static/tabulator-theme.css`,
  aplicada pela classe `.ton-tabulator` no container (que é a própria raiz da tabela — ver 14.6).
  A nota `/tabulator` e a administração `/database` carregam o mesmo arquivo; cada página guarda no
  seu `<style>` só o que é local (altura total, padding das células, árvore de hierarquia,
  paginação e as decorações das células).
- Motivo: os dois blocos de CSS tinham **divergido** (cabeçalho `#18181b`, linhas `#09090b` e sem
  zebra na `/database` × `#1b1b20`/`#121215` com zebra na nota) e o contraste reforçado existia só
  na nota. Agora ambos usam a mesma paleta: cabeçalho `#1b1b20` com borda inferior de 2px
  `#3f3f46`, linhas `#121215` (zebra `#18181d`), hover `#24242c`, separadores `#26262c` e acento
  esmeralda (`#10b981`/`#34d399`) no foco, na edição e na ordenação.
- O padding das células **não** é totalmente compartilhado: a `/database` depende do `padding-left: 12px` para
  o deslocamento das guias da hierarquia (§6.28). A nota usa `6px 8px`; a `/database`, `6px 12px`
  (vertical igual, horizontal maior por causa da árvore — ver §20).
- **Ícone próprio do tipo:** a `/database` monta o ícone de cada linha no servidor
  (`icons.SVGString` → campo `_icon`), enquanto o navbar/sidebar e os templates usam o renderizador
  templ (`icons.Icon`). Os dois switches tinham **divergido** (56 nomes existiam só no template) e
  `Config["tabulator"].Icon = "container"` não tinha `case` em `SVGString` — o tipo caía no
  **fallback genérico de arquivo** (que era o “sem ícone próprio”). Foram portados os 56 ícones
  faltantes (e 6 que só existiam no Go), deixando os dois renderizadores com os mesmos 111 nomes.
- Testes de regressão: `internal/ui/icons/config_test.go` passou a comparar os dois arquivos-fonte
  (paridade de nomes — um `case` novo tem que entrar nos dois lados) e a validar que **todo** ícone
  referenciado no `Config` existe em ambos. Sem isso, tipos como `desenho`, `youtube`, `agenda`,
  `task` e `captura` também apareciam com o ícone genérico quando renderizados por `SVGString`/
  `IconSVG` — isto é, na página `/database` (campo `_icon`) e nas listas do bloco `consulta`.
- E2E: `database.spec.ts` verifica que a `/database` usa o tema compartilhado (as mesmas cores) e
  que o tipo `Tabulator` renderiza o ícone `container`; `tabulator.spec.ts` verifica as **mesmas**
  cores na nota (se uma tela mudar sozinha, um dos dois testes quebra).

### 14.9 Seletor visual do tipo da coluna (submenu) (18/10/2026)

- Antes, **⋮ → Tipo da coluna** abria um `prompt()` que exigia **digitar** o tipo (“número”, “data”…).
  Agora o item é um **submenu nativo do Tabulator** (`menu: [...]`): ao clicar, a lista dos 5 tipos
  (Texto / Número / Data / Sim‑Não / Arquivo) abre **ao lado** do item; o **tipo atual vem marcado**
  (✓ + realce esmeralda) e basta um clique — sem digitar nada. (O nome da coluna em **+ Nova coluna**
  / **Renomear coluna** continua em `prompt`, porque ali a entrada é texto livre por natureza.)
- **Por que o submenu nativo e não um modal:** o Tabulator já suporta item com `menu: [...]`
  (ganha a classe `tabulator-menu-item-submenu`, a seta ▸ e o posicionamento à direita via
  `popup.child()`). Assim não há nada de posicionamento/fechamento/foco para manter à mão: o
  submenu herda o comportamento do menu ⋮ (mesmo visual, fecha ao clicar fora). A primeira
  tentativa foi um `<dialog>` modal **centralizado**; funcionava, mas era mais pesado e menos “no
  fluxo” do que o desejado.
- Cada item do submenu é um `{label, action}`; o `label` é uma **função** reavaliada a cada
  abertura (lê `colTypes[field]`), então o ✓ acompanha o tipo atual sem reconstruir o menu. O
  conteúdo é um HTML próprio (ícone + nome + ✓) — o Tabulator aceita `label` como função que
  devolve HTML (`innerHTML`) ou um `Node`.
- ⚠️ **Mesma escala de valores:** as opções expõem exatamente os 5 valores canônicos
  (`string`/`number`/`date`/`boolean`/`file`) e são exportadas em `TabulatorNoteUtils.columnTypes`;
  um teste unitário garante `normalizeType(value) === value`, ou seja, o que o submenu grava é o
  mesmo valor que o JSON da nota persiste. `applyColumnType` só dispara autosave quando o tipo
  **muda**.
- Os estilos do item (`.tab-type-item`/`.tab-type-ico`/`.tab-type-name`/`.tab-type-check`) ficam no
  `<style>` do `notes/tabulator.templ`: o menu é montado no fim do `<body>`, **fora** de
  `#tabulator-root`, então os seletores são globais (mas com prefixo próprio). Não vão para
  `tabulator-theme.css`, que é o tema da **tabela** compartilhado com a `/database`.
- E2E (`tabulator.spec.ts`): clicar em **Tipo da coluna** abre o submenu com os 5 tipos, o atual
  marcado (`.tab-type-item.is-current`) e **nenhum `<input>`**; escolher o tipo atual fecha o menu
  sem alterar nada. A coluna `arquivo` do teste passa a ser criada pelo submenu (`pickColumnType`),
  restando só o `prompt` do nome.

## 15. Execução sem processos fantasmas — `run.sh` (18/10/2026)

- Sintoma: depois de rodar o projeto sobrava um `ton618` **suspenso** (`stat T`, típico de
  `Ctrl+Z`) segurando a porta — o socket continuava em `LISTEN` e o app parava de responder, sem
  erro visível. O `core/run.sh` antigo terminava com `./ton618 … | tee …`, sem `trap` e sem tratar
  sinais, e dependia de um único `fuser -k` silencioso.
- `core/run.sh` agora:
  - termina com **`exec`** (`exec ./ton618 > >(tee -a "$LOG_FILE") 2>&1`): o shell é substituído
    pelo servidor, então não existe processo-pai para ficar órfão segurando a porta e o `Ctrl+C`
    vai direto para ele (o log continua em arquivo e na tela, via `tee`);
  - **ignora `SIGTSTP`** antes do `exec` (disposição herdada): `Ctrl+Z` não deixa mais o servidor
    suspenso prendendo a porta;
  - libera a porta antes de compilar **e** de novo imediatamente antes de subir (`free_port`:
    descobre o PID pelo dono do socket com `ss`, manda `SIGTERM`, espera, cai para `SIGKILL`), e
    aborta com mensagem clara se a porta continuar ocupada — em vez de falhar com “address already
    in use”;
  - grava o PID em `$DATA_DIR/ton618.pid` e ganhou os subcomandos **`stop`** (encerra o que estiver
    na porta, idempotente) e **`status`** (mostra o dono da porta e **avisa quando o processo está
    SUSPENSO**). O `run.sh` da raiz repassa `stop`/`status`.
- `core/.gitignore` passou a ignorar os byproducts de compressão do build
  (`web/static/**/*.gz` e `web/static/**/*.br`), que apareciam como não rastreados a cada build.

## 16. Busca global — cor do nome da nota = cor do tipo (18/10/2026)

- Sintoma: no resultado da busca global o **nome da nota** aparecia sempre na mesma família de cor,
  sem relação com o tipo — o ícone do badge seguia o `icons.Config`, mas o título tinha uma cor
  **fixa por ramo do `if`** (`text-lime-400` no tabulator, `text-emerald-400` no markmap,
  `text-sky-400` no desenho e no markdown, `text-orange-400` no pdf). Era o mesmo tipo de divergência
  já corrigido nos ícones (§14.8), só que aplicado a cores: `desenho` (`#9C63F8`, roxo) e `markdown`
  (`#F54927`) saíam em **azul**; `tabulator` (`#10C809`) saía em `lime-400`, um verde **diferente** do
  do próprio ícone do tipo.
- Correção: o título passou a usar a **cor do tipo** como fonte única
  (`icons.GetColor(res.Tipo)` → `icons.ParseClassAndStyle`, que manda classe Tailwind para o `class` e
  cor raw/hex para o `style` inline). O mesmo valor já alimenta o ícone do badge, a sidebar e a
  `/database`, então nome + ícone + sidebar ficam na mesma cor.
- De quebra, os 5 blocos `<a>` duplicados (um por tipo) viraram **um só**, com o destino isolado em
  `searchResultHref(res)` (mesma tabela de rotas de antes: `/file`, `/drawing`, `/mindmap`,
  `/tabulator`, `/editor#line-N`); o `target="_blank"` continua **só** no pdf (atributo condicional do
  templ). O `style` vazio nos tipos cuja cor é classe sai como `style=""` (inócuo).
- Teste: `internal/features/search/search_results_color_test.go` renderiza o item para
  `tabulator`/`markdown`/`markmap`/`desenho`/`pdf`, extrai a tag `<a>` do título e confere que ela
  carrega **a mesma cor** de `icons.GetColor(tipo)` (o valor esperado é **derivado** do Config, então
  voltar a fixar `text-lime-400` na mão quebra o teste), que o `href` é o esperado, que só o pdf tem
  `target` e que tipos com cores distintas **não** compartilham a mesma cor aplicada.
- Não muda o destaque do termo buscado (`.search-highlight`/`.stem-highlight`), que continua em
  azul/violeta como indicador de “onde casou” — inclusive quando o termo casa no próprio nome.
## 17. Campo `tipo:` no frontmatter — filtro por tipo de nota (21/10/2026)

- Motivo: o tipo de nota era só inferido (`DetectNoteType`: tags → caminho → nome/conteúdo) e nunca
  persistido num campo visível; na prática alguns tipos eram impossíveis de filtrar na busca — a
  busca global só tinha sintaxe `tags:` e o FTS5 **excluía duramente** desenhos/PDFs/anexos dos
  resultados (`fts.go`), então `tipo:desenho`/`tipo:pdf` jamais casariam.
- Decisão: gravar `tipo:` no frontmatter de TODA nota `.md`, no vocabulário PT snake_case fixo
  (`nota, desenho, mapa_mental, tabulator, pdf, epub, anexo, arquivo, imagem, youtube, artigo,
  captura, semanal`). É um **espelho** determinístico de `DetectNoteType` — a detecção continua
  sendo a fonte da verdade; uma edição manual do valor é normalizada no próximo save/backfill.
  `mapa_mental` substitui o valor interno `markmap` **só** no frontmatter (rotas/tags/ícones
  continuam `markmap` — ver `core/internal/core/domain/note_tipo.go`).
- Gravação: `NoteService.processAndSave` (ponto único de todo save: editor, rename, duplicação,
  editores especiais) chama `ensureNoteTipoField`, que detecta o tipo pelas mesmas tags do banco +
  conteúdo + caminho e usa `UpdateFrontmatterProperty`. Backfill idempotente de todas as notas roda
  no boot via `EnsureNoteTypeField` (chamado pelo `SyncDatabaseWithContext`, depois do
  `EnsureTypeTags`). Arquivos binários (pdf/anexo/arquivo/epub/imagem) não têm frontmatter (§6.24)
  — o tipo deles é derivado do caminho pelo indexador.
- Índice: migração v24 adiciona a coluna `note_type` a `documents` e `docs_fts` (a FTS5 é recriada —
  `ALTER ADD COLUMN` não existe em tabela virtual — e `file_mods` é limpo para forçar reindex
  completa). `processor.Document.NoteType` carrega o valor content-aware (markdown/tabulator); os
  escritores do db derivam de tags + caminho (`DeriveNoteType`). O sqlc não foi regenerado (CLI
  ausente): `Store.InsertDocument` faz o INSERT do sqlc + um `UPDATE note_type` logo atrás.
- Busca global: nova sintaxe `tipo:x` (alias `type:`, case-insensitive, aceita aspas e múltiplos
  filtros em OR). O valor é normalizado (`domain.NormalizeNoteTipo`: espaços/hífens → `_`, aliases
  markmap/drawing/...) e vira predicado `note_type:"x"` na query MATCH; com filtro de tipo as
  exclusões duras do FTS/LIKE são relaxadas (mantendo só `deletar`) — senão `tipo:pdf` jamais
  casaria. O fallback LIKE recebe `AND note_type = ?`.
- Acervo: a coluna Tipo (`row["type"]`) passa a usar o vocabulário do frontmatter (frontmatter se
  existir, senão a detecção) e a célula exibe **apenas o valor cru** (`mapa_mental`, `anexo`, ...)
  como texto simples — o formatter de ícone + rótulo legível foi removido (`TYPE_LABELS`/`typeLabel`
  do database.js ficaram órfãos e saíram junto; o `typeLabel` do `tabulator-note.js` é outro módulo
  e permanece). O filtro `tipo:` do `parseSearchQuery`/`makeMatcher` normaliza o valor digitado. O
  campo `tipo` do frontmatter NÃO vira coluna dinâmica (é o mesmo dado da coluna fixa "Tipo").
- Nota: as notas **imagem** continuam fora das linhas do Acervo (comportamento pré-existente do
  `GetMany`) — `tipo:imagem` funciona na busca global.

## 18. Busca global — todos os trechos da nota no card (22/10/2026)

- Problema: a busca global colapsava os hits POR ARQUIVO mantendo **um único fragmento** por nota
  (o de maior score, ou o primeiro que contém o termo). Como notas Tabulator são indexadas linha a
  linha (uma linha da planilha = um fragmento com `secao: "Registro N"` — ver
  `ProcessTabulatorContent`), pesquisar o conteúdo de uma célula casava em N linhas e o card
  mostrava só 1 — as outras ocorrências eram descartadas em silêncio.
- Decisão (escolha do usuário): o card da nota passa a listar **TODOS** os trechos que casaram,
  cada um com o rótulo da seção (ex.: `Registro 3`) e o destaque próprio, com teto de
  `maxSnippetsPerNote = 5`; o excedente vira "+N ocorrências" no rodapé do card. Os trechos que
  contêm o termo ficam na frente (preservando a ordem de ranking) — o primeiro é o principal.
- Contagem: o "X resultados" (e o COUNT das queries FTS/LIKE em `fts.go`) passa a contar
  **arquivos distintos** (notas), não fragmentos — uma planilha com 30 linhas que casando é
  1 resultado. A página continua sendo de notas: `search.Search` agora carrega
  `size * searchFragmentsPerNote` (= 5×) fragmentos por página para o agrupamento no handler ter
  material dos extras (antes o trim cortava em `size` fragmentos, matando os extras).
- Arquivos: `domain.SearchResultItem` ganha `Snippets []SearchSnippet` (Secao + HTML) e
  `ExtraCount int`; o handler (`handlers_search.go`) agrupa por bucket `with/without` termo; o
  template (`search_results.templ`) renderiza a lista indentada + o contador.
- Limite conhecido: a paginação ("carregar mais") ainda desloca por FRAGMENTO no SQL — um card
  pode se repetir entre páginas quando as linhas de uma nota atravessam a borda do offset (era
  possível antes também); corrigir isso de verdade exige paginar por `GROUP BY arquivo` no FTS,
  o que muda a semântica do ranking e fica para uma iteração futura se incomodar.

## 19. Correções de consistência do ciclo de refatoração (22/10/2026)

- **Tipo do frontmatter**: o indexador lê `tipo:` (canônico, gravado pelo app) e `type:`
  (legado), em qualquer caixa, e normaliza **uma única vez** por `domain.NormalizeNoteTipo`.
  `applyFrontmatterType` passa a receber o valor canônico (`desenho`, `mapa_mental`,
  `tabulator`, ...) em vez do texto cru: `type: Drawing` agora limpa o JSON do Excalidraw e
  `tipo: tabulator`/`tipo: desenho` disparam o comportamento especial. A detecção de nota
  Tabulator (`tabulatorBodyFromNote`) usa o mesmo parser de frontmatter — não procura mais a
  substring `tabulator` no YAML (que casava até `description: "type: tabulator"`).
- **JSON do Tabulator**: `ParseTabulatorData` exige `io.EOF` após o primeiro valor. Corpo
  truncado seguido de lixo/outro JSON não é mais aceito como planilha válida.
- **Filtro de tags**: a chave de comparação é `normalizeTagKey` (minúscula + `RemoveAccents`),
  usada na query FTS, em `extractTags` e no filtro Go de segunda linha. `tags:saude` casa com
  a tag `saúde` (antes o FTS casava e o filtro Go descartava).
- **Limpeza de imagens**: usa `DeleteAllFileRecords` (fonte única) em vez de seis deletes
  seletivos. Isso remove também `file_metadata`, evitando que uma imagem apagada continue
  contando como filha fantasma (`pai:`) em `GetChildrenCounts`/`GetHierarchy`.
- **Delete de arquivo/nota**: se `DeleteAllFileRecords` falhar, o handler responde 500 e
  **não** executa `ReconcileTabulatorFiles`. Antes, a nota continuava viva no índice e a
  reconciliação podia apagar arquivos de `datasets/` ainda referenciados.
- **Restore de backup total**: entradas de diretório do ZIP são ignoradas no `RestoreResult.Files`
  (só arquivos contam). `extractZipEntry` já as ignorava; o contador somava +1.
- **Frontend**: `_organizeHeadings` do editor é declarado explicitamente (ESM/strict não pode
  atribuir a variável implícita); o formatter de tags do Acervo usa `escapeHtml` (XSS por tag
  de frontmatter). Testes unitários cobrem os dois.

## 20. Acervo — cabeçalho compacto e área de resultados com moldura (22/10/2026)

📍 `core/internal/features/system/handlers_database_tabulator.go` | `core/internal/features/notes/database.templ` | `core/web/src/database.js` | `core/web/tests/database.spec.ts`

**Problema:** o cabeçalho do Acervo (título + contador + botões + busca) consumia ~118px e a
tabela usava `height: calc(100vh - 190px)` chumbado. A área de resultados era esticada com o hack
`width: 100vw; left: 50%; margin-left: -50vw` para escapar do `max-w-4xl` do layout — como `100vw`
inclui a largura da barra de rolagem, a página ganhava overflow horizontal e a **barra de rolagem
vertical da tabela ficava parcialmente escondida** no canto direito.

**Decisão:**

- **Layout full width de verdade:** `HandleDatabasePage` passa a renderizar com
  `is_full_width` no contexto (mesmo padrão da Agenda). O layout entrega `w-full px-3 md:px-6`,
  então a tabela ganha o padding das bordas sem precisar de `100vw` e sem estourar a viewport.
- **Cabeçalho compacto:** `gap-4 mb-6` → `gap-2 mb-3`, título `text-2xl` → `text-xl`, botões
  `py-1.5` → `py-1` e busca `py-2.5` → `py-1.5` (foco em esmeralda, o mesmo acento do filtro da
  nota Tabulator). O cabeçalho é `shrink-0`; a área de resultados é `flex-1 min-h-0` e a tabela usa
  `height: "100%"`.
- **Altura sem número mágico no JS:** `#database-page` mede `calc(100dvh - 105px)` (navbar de 57px
  medido no DOM + `py-6` do layout). Se o cabeçalho crescer (ex.: quebra de linha no mobile), o flex
  reduz a tabela automaticamente.
- **Moldura e respiro:** a tabela fica dentro de `#notes-table-wrap` (`rounded-xl`, borda zinc e
  `overflow-hidden`), que garante o padding lateral da página (12px no mobile, 24px no desktop) e
  mantém a barra de rolagem vertical inteira visível.
- **Padronização com a nota Tabulator (§14.8):** o padding vertical das células passou de 8px para
  **6px** (a nota usa `6px 8px`), mantendo o horizontal em 12px porque as guias da hierarquia
  dependem dele (§6.28). O resto do desenho continua vindo de `tabulator-theme.css`.

**Regressão:** `database.spec.ts` verifica que a página não tem overflow horizontal, que a moldura
tem padding real nas bordas e que a área da tabela ocupa a maior parte da viewport.

## 21. Nota markdown — botão de baixar o `.md` exato (24/10/2026)

📍 `core/internal/features/notes/editor.templ` | `core/internal/ui/icons/config.go` | `core/web/src/editor-init.js` | `core/web/static/editor-common.js` | `core/web/src/global.d.ts` | `core/web/tests/editor-common.unit.cjs` | `core/web/tests/notes.spec.ts`

**Problema:** a nota markdown só tinha "Duplicar" e "Excluir". Não havia como exportar o markdown
da nota: o conteúdo vive dentro do editor (TipTap + painel de frontmatter separado) e
`/file/download` devolve o arquivo do **disco**, que pode estar atrasado em relação ao autosave
(debounce de 2s) — um download logo após editar sairia velho.

**Decisão:**

- **Botão ao lado de duplicar** (`#download-editor-btn`, `onclick="downloadCurrentNote()"`), com
  o mesmo estilo/título dos vizinhos. Fica **só** em `editor.templ`, que é usado exclusivamente
  por `/editor` (`handlers_editor.go`): tabulator, desenho, mapa mental e demais tipos têm
  template próprio e não mudaram.
- **Fonte única do conteúdo:** `buildCurrentMarkdown()` no `editor-init.js` concentra o pipeline
  que antes estava escrito **duas vezes** (`doSave` e `doRename`): markdown do TipTap →
  `stripNbspParagraphs` → frontmatter do painel reanexado → `wikilinksToMarkdown`. Salvar,
  renomear e baixar leem da mesma função, então o `.md` baixado é o mesmo que seria gravado,
  inclusive com alterações ainda não salvas.
- **Download 100% no cliente:** `EditorCommon.downloadMarkdown(filename, content)` cria um `Blob`
  (`text/markdown;charset=utf-8`), monta um `<a download>` temporário e revoga a object URL em
  seguida. Usa só o **basename** (o prefixo `notes/` não faz sentido no disco do usuário) e ganha
  `.md` se faltar. Nenhuma rota nova no backend.
- **Ícone:** chaves `baixar`/`download` no `icons.Config` apontando para o Lucide `download` — sem
  a chave, `GetIcon("baixar")` caía no fallback `sticky-note`. Herda `text-zinc-500`, a cor dos
  botões de ação da nota; o teste de paridade de ícones cobre o novo caso nos dois renderizadores.

**Regressão:** `editor-common.unit.cjs` cobre o download (conteúdo intacto, MIME, basename,
`.md` automático e revogação da URL) — `npm test` **74/74**; `notes.spec.ts` ganhou um teste E2E
que baixa a nota com frontmatter preenchido e confere `tags:` + corpo no arquivo baixado e o nome
`.md`. `templ generate`, `gofmt`, `go vet`, `go test ./...` e `node build.js --dev` verdes.
⚠️ Reiniciar o servidor para o `staticver` servir os estáticos novos.

## 22. Nota markmap — botões de baixar o `.md` e o mapa mental em PNG (25/10/2026)

📍 `core/internal/features/notes/mindmap.templ` | `core/internal/ui/icons/{icons.go,icons.templ,config.go}` | `core/web/static/editor-common.js` | `core/web/src/global.d.ts` | `core/web/tests/editor-common.unit.cjs` | `core/web/tests/mindmap-download.unit.cjs`

**Problema:** a nota markmap só tinha "Duplicar" e "Excluir" (§21 deu o botão de download só para
`/editor`). Não havia como levar o conteúdo markdown para fora nem como obter uma imagem do mapa
mental — e copiar o `<svg>` à mão não serve: ele é estilizado pelo CSS **da página** e pelas
`<style>`/`<defs>` internas, então uma cópia serializada sai sem cor, sem fonte e sem o HTML dos
`<foreignObject>` do markmap.

**Decisão:**

- **Dois botões ao lado de duplicar**, na topbar da `mindmap.templ`: `#download-md-btn`
  (`onclick="downloadCurrentNote()"`) e `#download-image-btn`
  (`onclick="downloadMindmapImage()"`), mesmo estilo/título dos vizinhos; a ordem ficou
  duplicar → markdown → imagem → excluir. Ficam **só** nesse template (desenho, tabulator etc.
  têm template próprio e não mudaram).
- **Markdown = fonte única do save:** `downloadCurrentNote()` chama
  `EditorCommon.downloadMarkdown(filename, ensureMindmapFrontmatter(jar.toString()))` — exatamente
  o que `doSave`/`doRename` gravam, inclusive com alterações ainda não salvas (autosave com
  debounce). Reaproveita o helper do §21; nenhuma rota nova.
- **PNG 100% no cliente** em `EditorCommon.downloadSvgPng(svgEl, opts)`:
  1. mede o **bbox do conteúdo** (`getBBox()` já inclui o `transform` do zoom/pan aplicado por
     markmap no `<g>` raiz) e monta `viewBox = bbox + 48px` de margem;
  2. clona o SVG e **inline o CSS computado** de cada elemento (e do HTML dentro de cada
     `foreignObject`) — a imagem é renderizada isolada, sem o CSS da página;
  3. embute as `@font-face` **realmente usadas** (`/static/fonts/fonts.css` → `data:` URL,
     best-effort: falhou, exporta sem);
  4. serializa → `Image` → canvas com fundo `#09090b` → `toBlob("image/png")` → `<a download>`.
- **Resolução padrão, com piso e teto:** `EditorCommon.svgPngScale(vbW, vbH, opts)` é a fonte
  única do cálculo — **2x** por padrão, **ampliado** até o piso de **1600px** no maior lado
  (mapas pequenos saem nítidos) e **reduzido** para nunca passar de **8192px** por lado (teto
  seguro para canvas). Retorna `basename.png` (só o basename, como no §21).
- **Guardas:** sem prévia renderizada (`mmWrapper` nulo — mobile) o botão avisa em vez de
  exportar um SVG vazio; `SecurityError` do canvas (imagem externa "suja") vira mensagem em
  português. Erros caem no `.catch` e viram `alert`.
- **Ícone:** nova chave `baixar-imagem` → Lucide `image-down`, com o case adicionado **nos dois**
  renderizadores (`icons.go` e `icons.templ`) — o teste de paridade (§6.9) falha se faltar um.

**Regressão:** `editor-common.unit.cjs` ganhou os testes de `svgPngScale` (2x, piso, teto,
entradas inválidas) e do fluxo `downloadSvgPng` (viewBox do conteúdo + margem, escala aplicada ao
SVG e ao canvas, `image/png`, data URL, basename `.png`); `mindmap-download.unit.cjs` trava os
dois botões, a ordem na topbar, os handlers globais e que o `.md` usa a mesma fonte do save —
`npm test` **80/80**. `templ generate` (CLI pinado v0.3.1070), `go build -tags sqlite_fts5`,
`go test ./internal/ui/icons ./internal/features/notes` e `tsc --noEmit` verdes.
⚠️ Reiniciar o servidor para o `staticver` servir os estáticos novos.

## 23. MCP local somente leitura — qualquer IA acessa as notas (25/10/2026)

📍 `core/cmd/server/{main.go,mcp.go}` | `core/internal/mcp/{server.go,tools.go,format.go}` |
`core/internal/core/db/db.go` (`NewReaderStore`) | `core/internal/features/notes/query_reader.go`
(`NewQueryReader`) | `docs/mcp.md`

**Problema:** não havia uma porta padrão para IA. A API HTTP existente não serve: `/search` e
`/api/query` devolvem HTML (HTMX), a única credencial é a senha única que também autoriza
`POST /api/restore` e exclusões, e dar o diretório do banco a um agente não estabelece escopo
nem formato. O acesso precisava ser universal (qualquer cliente MCP), local e **sem poder
alterar o acervo**.

**Decisão:**

- **Subcomando `ton618 mcp`**, despachado no topo de `main()` **antes** do bootstrap HTTP. O
  protocolo usa `stdout`; por isso o subcomando troca o `slog` para `stderr` e não chama
  `EnsureDirs`, `NewStore`, migração nem indexação. Flags: `--db` e `--docs`; sem `--db`, resolve
  `DB_PATH` → `./data/ton618.db` → `<binário>/data/ton618.db`, com erro que lista os caminhos
  tentados.
- **`db.NewReaderStore(path)`**: abre o banco existente com `mode=ro` e `_busy_timeout=5000`, sem
  `backupBeforeMigration` nem `initSchema`. `Ping()` no boot falha cedo com mensagem clara
  (em vez de o erro aparecer na primeira tool). Como o SQLite em `mode=ro` rejeita escrita, a
  garantia não depende das tools: é do driver.
- **Cinco tools, todas `readOnlyHint: true`** — nenhuma tool de escrita é registrada:
  `search_notes`, `get_note`, `list_notes`, `query_notes`, `backlinks`.
- **Reuso, não reimplementação:** `search.Search` (busca rankeada) com
  `Store.GetBacklinkCount`/`GetSynapticWeight`; o mesmo `query.Engine` dos blocos de consulta do
  editor, via `processor.ParseQuery` + `query.Run`; `NoteService.GetMany`/`GetBacklinks` para
  listagem e links. O único código novo é apresentação (agrupamento de fragmentos por nota, trecho
  de 280 caracteres, JSON indentado).
- **Leitura do conteúdo vem do SQLite** (`notes.content`), não do disco — decisão do §14/§6.24:
  notas vivem no banco. `get_note` pagina com `offset`/`max_chars` (padrão 50k, teto 200k) para
  não estourar o contexto em notas Tabulator.
- **Padronização de nomes e tipos:** `notes.NoteFilename` normaliza `nome` → `notes/nome.md`;
  `..` e `\` são rejeitados. O `tipo` devolvido é sempre o vocabulário do frontmatter
  (`domain.FrontmatterTipo`/`NormalizeNoteTipo`): `nota`, `mapa_mental`, `desenho`, `tabulator`…
- **Instruções do servidor** (enviadas no `initialize`) explicam o modelo de dados, os filtros,
  o vocabulário `tipo:` e a limitação do FTS (corpo de desenho/markmap/tabulator não indexado).
- **Descoberta do leitor de consultas:** `notes.newQueryReader` virou `notes.NewQueryReader`,
  devolvendo `query.Reader` — a interface só tem operações de leitura. O painel HTML e os testes
  usam a mesma função.

**Não implementado (de propósito):** transporte HTTP/remoto, token, OAuth e tools de escrita. Para
IA fora da máquina seria preciso um transporte remoto com Bearer token read-only, TLS/túnel e rate
limit; a senha do app continua sendo a única credencial com poder de escrita/restauração.

**Regressão:** `internal/mcp/tools_test.go` sobe um acervo temporário com `db.NewStore` +
`NoteService.Save`, conecta um cliente MCP em memória e cobre busca, filtro por tag, paginação,
nota inexistente, listagem, consulta YAML por texto/tag, erro de consulta, backlinks e a ausência
de tools fora da allowlist. `internal/core/db/db_reader_test.go` prova que `NewReaderStore` não
cria arquivo, lê o banco e rejeita `SaveNote`. `go test -tags sqlite_fts5 ./...`, `go vet` e
`gofmt` verdes. Instruções de uso em [`docs/mcp.md`](mcp.md).

## 24. `/help` — índice navegável gerado dos próprios títulos (25/10/2026)

📍 `core/internal/features/system/{help_toc.go,help.templ,handlers_help.go,handlers_help_test.go,help_toc_test.go}`

**Problema:** o `/help` virou um documento longo (14 seções, 69 subseções). Sem índice, achar um
assunto exige rolar tudo ou usar o Ctrl+F do navegador, que não mostra a estrutura. Um índice
escrito à mão no `HELP.md` resolveria hoje e apodreceria na primeira seção nova — a lista e o
documento passariam a discordar sem ninguém notar.

**Decisão:**

- **O índice não existe no markdown: é derivado a cada requisição.** `HandleHelpPage` parseia o
  `HELP.md` **uma vez** com goldmark e usa a mesma árvore para duas coisas: dar `id` aos títulos
  (`helpHeadingIDs`) e renderizar o HTML. Nenhuma lista paralela para manter; seção nova aparece
  sozinha no índice.
- **Estrutura:** `h2` = seção, `h3` = subseção. Um `h3` antes de qualquer `h2` entra como raiz,
  para não sumir do índice. `helpHeadingText` extrai o texto puro do título (nós `Text`/`String`):
  um título que no markdown tem crase em volta de `/database` chega ao índice sem a crase.
- **Âncoras determinísticas:** `helpSlug` gera minúsculas ASCII sem acento (`helpAccents`), com
  hífen no lugar da pontuação (`4.5 Nota semanal` → `4-5-nota-semanal`); letras não latinas ficam
  de fora para o `href` não virar percent-encoding. Títulos repetidos recebem sufixo numérico
  (`-2`, `-3`) via `helpUniqueID`, com um mapa compartilhado pela varredura — o link é o mesmo em
  toda requisição. Cada título recebe também `style="scroll-margin-top:5.5rem"` inline (não numa
  classe) porque a navbar é `sticky top-0` e cobriria o alvo do salto.
- **Ponto de inserção:** `handlers_help.go` corta o HTML no primeiro `<h2>`; o índice entra entre a
  apresentação e a seção 1. Em `help.templ`, o layout é `HelpPage(title, intro, toc, body)` e o
  `<nav>` usa `not-prose`, senão o plugin de tipografia estiliza a lista como se fosse conteúdo.
- **Filtro do índice compara só o título da entrada**, não o texto da seção. A primeira versão
  comparava a seção inteira: buscar `restauração` casava com a seção 9 e mantinha as 5 subseções
  visíveis (inclusive "Sinônimos" e "Colunas"), que era exatamente o "mostra um monte de coisa
  nada a ver" relatado. Regras atuais: sem acento e sem caixa (`configuracoes` acha
  "Configurações"); vários termos = todos precisam aparecer (`backup nota`); só ficam visíveis as
  subseções cujo próprio título casa; contador ao vivo ("2 de 14 seções", voltando para
  "14 seções · 69 subseções" ao limpar) e estado vazio. O placeholder diz "Buscar por título…",
  deixando claro que o campo filtra o índice, não o documento.
- **Tamanho do índice:** `helpTOCLabel`/`helpTOCSize` geram o resumo do cabeçalho sem contagem
  escrita à mão.

**Regressão:** `help_toc_test.go` cobre slug (acentos, pontuação, não latino, vazio) e ids únicos;
`handlers_help_test.go` ganhou `GeraIndiceComAncoras`, `IndiceAcompanhaNovasSecoes` (documento
temporário com seção inventada) e `TitulosRepetidosGanhamAncoraUnica`. No documento real: 14
seções, 69 subseções, 83 links, 0 quebrados; 15 `<hr>`, 37 tabelas e 56 `<pre>` preservados. Filtro
validado em navegador real: `backup` → seção 9 (só 9.2 e 9.3) e seção 12 (2 itens); `configuracoes`
→ seção 9 (só 9.1); `agenda` → só a seção 7; `zzzz` → 0 e mensagem de vazio.

## 25. Nota de desenho — baixar o `.excalidraw` e o `.png` do estado atual (25/10/2026)

📍 `core/internal/features/notes/drawing.templ` | `core/web/src/drawing.jsx` | `core/web/src/global.d.ts` | `core/web/tests/drawing-download.unit.cjs` | `core/HELP.md` (§4.6)

**Problema:** a nota de desenho só tinha "Duplicar" e "Excluir" (§21/§22 deram download ao editor
markdown e ao markmap, cada um no seu template). Não havia como levar o desenho para fora nem
como obter uma imagem dele. Baixar o `.md` do disco também não serve: o corpo da nota é **JSON do
Excalidraw embutido no markdown**, não um arquivo `.excalidraw`, e o autosave é debounced — um
download logo após editar sairia velho.

**Decisão:**

- **Dois botões ao lado de duplicar** na topbar de `drawing.templ`: `#download-excalidraw-btn`
  (`onclick="downloadCurrentDrawing()"`) e `#download-png-btn`
  (`onclick="downloadDrawingImage()"`), mesmo estilo/título dos vizinhos; ordem duplicar →
  `.excalidraw` → `.png` → excluir. Ícones: `baixar` (download) e `baixar-imagem` (`image-down`,
  criado no §22, com o case nos dois renderizadores).
- **Fonte dos dois downloads é o estado VIVO do editor** — `getSceneElements()`/`getAppState()`/
  `getFiles()` da API imperativa do `drawing.jsx` —, e não o último snapshot salvo: alterações
  ainda não gravadas (debounce de ~1,5 s) entram no arquivo.
- **`.excalidraw`:** `serializeAsJSON(elements, appState, files, "local")`, MIME
  `application/vnd.excalidraw+json`, nome derivado do arquivo atual sem o `.md` (só o basename,
  como no §21). É o JSON nativo, que pode ser aberto no próprio Excalidraw.
- **`.png`:** `exportToBlob({ elements, appState, files, mimeType: "image/png" })` **sem**
  `maxWidthOrHeight` nem `scale` — resolução padrão do Excalidraw (tamanho natural do desenho, com
  o fundo e a margem do `appState`). Diferente do §22, não há inlining de CSS: quem renderiza a
  cena é o próprio Excalidraw, não um SVG da página.
- **Os helpers não estão na API imperativa.** `exportToBlob`/`serializeAsJSON` não aparecem em
  `ExcalidrawImperativeAPI`, então `drawing.jsx` publica
  `window.excalidrawExport = { exportToBlob, serializeAsJSON }` a partir do **mesmo bundle** —
  sem carregar uma segunda cópia da biblioteca e sem rota nova no servidor.
- **Guardas:** PNG antes de o editor terminar de carregar avisa em português em vez de exportar
  vazio; erros de exportação caem no `.catch` e viram `alert`.
- **Documentação:** `HELP.md` §4.6 cita os dois downloads e que eles usam o estado do editor,
  inclusive o que ainda não foi salvo.

**Regressão:** `drawing-download.unit.cjs` (4 testes) trava os dois botões, a ordem na topbar, os
handlers globais, o uso do estado vivo, o MIME/nome do `.excalidraw`, o `mimeType: "image/png"`, a
ausência de `maxWidthOrHeight`/`scale` e a publicação de `window.excalidrawExport`. Validado em
navegador real (Playwright, com download salvo em disco): `.excalidraw` de 1.216 bytes com
`type: excalidraw`, `version: 2` e 1 elemento; `.png` de 316×24 com assinatura PNG válida.
`templ generate` + `node build.js`, `go build -tags sqlite_fts5`, `go test ./internal/features/notes`,
`tsc --noEmit` e os 84 testes JS verdes.
⚠️ Reiniciar o servidor: `drawing_templ.go` e `static/drawing.js` mudaram.

---

## 26. Rotina — linha do tempo do dia, pessoal e opcional (25/10/2026)

**Problema:** a Agenda (`/agenda`) guarda compromissos com data e hora, mas o dia a dia não é feito
só de compromissos: academia, almoço, foco, deslocamento. Não havia onde enxergar **o dia inteiro
como uma faixa de tempo contínua** nem saber quanto dele está ocupado e quanto sobra. O projeto de
referência em `New/` (CasalAgenda) já resolvia isso com um **gantt Mermaid** por pessoa e um resumo
de ocupação — mas era multi-usuário ("casal"), o que não serve ao TON-618.

**Decisão:** portar a ideia como **Rotina** (`/rotina`), **pessoa única**, com o visual do app.

- **Nome e escopo:** "Rotina", rota `/rotina`, pacote `core/internal/features/dayplan`, tabela
  `day_activities`. Nada de usuário/dono: a única credencial é o dono do cofre (BasicAuth). O modelo
  é `domain.DayActivity{ID, Day, StartTime, EndTime, Description, CreatedAt, UpdatedAt}` — **sem**
  coluna de usuário. Um teste de guarda de schema (`TestDayActivities_SchemaSemDono`) falha se
  alguém introduzir `user`/`owner`/`account_id`/etc.
- **Persistência:** `CREATE TABLE IF NOT EXISTS day_activities` no schema de boot (e em
  `schema.sql`), de modo que bancos existentes ganham a tabela sem migração manual. CRUD em **SQL
  cru** (`internal/core/db/day_activities.go`), não sqlc: a tabela é pequena e o gerador não está
  disponível no build. Escritas usam um lock próprio, `dayPlanMu`, no mesmo padrão dos appointments
  e settings — o domínio não participa das transações de notas/FTS e por isso não bloqueia nem é
  bloqueado pelo `WriteMu` do núcleo.
- **Resumo:** o tempo ocupado é a **união dos intervalos** (duas atividades sobrepostas contam uma
  vez), com livres = `24h − ocupado`, porcentagem para a barra e um contador de atividades que se
  sobrepõem — aviso informativo, nunca soma dupla. Intervalos inválidos (fim ≤ início, hora fora de
  00:00–23:59) são ignorados no cálculo e rejeitados na entrada, para o resumo não quebrar com dado
  antigo.
- **Linha do tempo:** gantt Mermaid com uma única seção `Rotina`, barras `sky-500` (`#0ea5e9`, o azul
  da Agenda) e `dateFormat YYYY-MM-DD HH:mm` (sem `HH:mm` o Mermaid desenha barras de largura zero).
  Uma tarefa-fantasma `edge :edge0, <dia> 00:00, <dia> 23:59` força o eixo a cobrir o dia inteiro; o
  JS esconde a barra e redesenha as horas no rodapé. A descrição é sanitizada antes de entrar na
  fonte (sem vírgula/dois-pontos/aspas, colapsando espaços, truncando em 80 caracteres). Mermaid
  v10.9.1 vendorizado em `static/js/vendor/mermaid.min.js` — o app continua funcionando offline.
- **Tela:** um único fragmento `#rotina-schedule` (resumo + timeline + lista + formulário). Criar,
  editar e excluir devolvem **esse fragmento pronto**; o JS troca o elemento e re-renderiza o
  Mermaid. Sem estado de cliente além do formulário de edição. Formulário valida `HH:MM`, fim
  depois do início e descrição ≤ 200 caracteres. Modal "Ampliar" mostra o dia inteiro; no cartão,
  telas ≤ 820 px compactam o eixo para o intervalo real (folga de 2 h, mínimo de 8 h).
- **Navegação e flag:** ícone **Rotina** (`calendar-clock`) no cabeçalho **ao lado da Agenda**
  (desktop e menu mobile), com destaque quando ativo. Novo pacote
  `internal/core/featureflags` com `Manager` cacheado sobre a tabela `settings`
  (`rotina_enabled`, padrão **ligado**) e middleware `WithFeatureFlags` injetando o retrato no
  contexto; a Navbar decide **no servidor** (`display:none`) se renderiza o ícone. Nova aba
  **Rotina** em Configurações com toggle: desligar persiste, esconde o ícone ao vivo
  (`window.applyRotinaNav`) e, se você estiver em `/rotina`, volta para a home. `/rotina` com a flag
  desligada redireciona (303) para `/`; nenhuma atividade é apagada.
- **Rotas:** `GET /rotina`, `GET /api/rotina/schedule`, `POST /api/rotina/create|update`,
  `DELETE /api/rotina/delete`, `GET|POST /api/settings/rotina`.
- **Cópia automática da semana anterior (25/10/2026):** ao iniciar o dia, a agenda de hoje recebe
  as atividades do **mesmo dia da semana anterior** (`hoje − 7 dias`) — segunda copia segunda,
  terça copia terça, e assim por diante. Se a origem estiver vazia, nada é criado. A cópia é feita
  por `EnsureDayCopied(target, source)` em uma transação e registrada em `day_activity_seeds` por
  dia de destino, então rodar de novo no mesmo dia (ou reiniciar o servidor) não duplica nem copia
  retroativamente. `HandlerContext.SeedToday()` é chamado no boot do servidor e nas rotas
  `/rotina` e `/api/rotina/schedule`, cobrindo tanto o início do app quanto a virada do dia com o
  processo já em execução. A cópia respeita a flag `rotina_enabled`: com a Rotina desligada, nada é
  copiado. Ids e timestamps das cópias são novos: a rotina do dia é independente da origem.
- **Documentação:** `HELP.md` §7.4 explica o resumo, a timeline, o modal, a cópia automática e o
  desligamento; a tabela de abas (§9.1) ganhou a linha **Rotina**.

**Regressão:** testes Go de `dayplan` (resumo/união/sobreposição, gantt/ordem/sanitização, handlers
incluindo validação, 404 e flag desligada), de `featureflags`, do middleware e do schema
single-owner; `EnsureDayCopied`/`SeedToday` têm testes próprios de cópia, origem vazia,
idempotência e flag desligada. `rotina.unit.cjs` (5 testes) trava `tightenSource`, o
formulário/fragmento/Mermaid e a integração com a Navbar e as Configurações. `templ generate`
(v0.3.1070), `node build.js`, `gofmt`, `go vet`, `go test -tags sqlite_fts5 ./...`,
`tsc --noEmit` e os 89 testes JS verdes.
⚠️ Reiniciar o servidor: `rotina_templ.go`, `navbar_templ.go`, `settings_modal_templ.go` e o bundle
`static/rotina.js` são novos/mudaram.

---

## Rotina: linha do tempo nativa (26/10/2026)

**Problema:** o gantt do Mermaid custava **3,2 MB** de JS por visita e obrigava a uma camada de
compensações: tarefa-fantasma `edge` para forçar o eixo de 24 h, `sanitizeMermaid` para o texto não
quebrar a sintaxe, `postProcess`/`relabelAxis` para repintar barras e mover horas no SVG, e
`tightenSource` para reescrever a fonte no mobile. O SVG também não dava clique, tooltip, zoom ou
arrastar.

**Decisão:** a Rotina passa a desenhar a própria linha do tempo em DOM, no mesmo padrão da Agenda
(`web/static/js/agenda/timeline.js`), mas especializada em um dia:

- **Servidor:** `ScheduleData.TimelineNative` escolhe o renderer. No modo nativo o Go não gera mais
  Mermaid (`MermaidSrc` fica vazio) e o template entrega as atividades como JSON
  (`templ.JSONScript("rotina-timeline-data", s.Activities)`). O Mermaid só é carregado quando a flag
  de comparação está desligada.
- **Cliente:** `web/src/rotina-timeline.js` converte tempo→pixel, distribui sobreposições em raias
  (greedy), desenha eixo de horas, sombreamento fora do dia útil, linha do "agora" (atualizada a cada
  minuto) e barras clicáveis. `layoutBars`/`parseClock`/`formatDuration` são puras e testadas no Node.
- **Interação:** clique na barra preenche o formulário (`window.rotinaEdit`); hover/focus mostra
  tooltip com horário e duração; `+`/`−`/`Ajustar` e Ctrl+scroll dão zoom; arrastar (mouse) rola;
  no toque vale o scroll nativo. O modal "Ampliar" monta uma segunda instância com barras maiores.
- **Flag:** `rotina_timeline_native` (padrão **ligado**) em `settings`, exposta na aba Rotina das
  Configurações. Desligar volta ao Mermaid e recarrega a página, permitindo comparar os dois antes
  de remover o vendor. `GET|POST /api/settings/rotina` passam a devolver/aceitar
  `timeline_native`/`native`, sem quebrar o `enabled` existente.
- **Custo:** `static/rotina.js` fica em ~19,5 KB (6,9 KB gzip) e a página deixa de baixar o vendor de
  3,2 MB no modo padrão. O Mermaid continua no repositório apenas para a comparação.

**Regressão:** testes Go de `dayplan` cobrem o renderer padrão (sem Mermaid), a flag desligada
(gera Mermaid) e o endpoint de settings; `featureflags` cobre o padrão e a persistência da nova
flag. `rotina.unit.cjs` ganhou testes de `parseClock`, `formatDuration` e `layoutBars` (posição,
raias, sobreposição, largura mínima) e de integração do template/JSON. Verificado no Chromium
(Playwright) em desktop e mobile: barras, tooltip, edição por clique, zoom, ajuste, modal e
re-render após criar/excluir. `templ generate` (v0.3.1070), `go test`, `tsc --noEmit`, `node
build.js` e os 96 testes JS verdes.
