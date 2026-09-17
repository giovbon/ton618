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
| Busca semântica | sqlite-vec (vizinhos próximos) | Embeddings no próprio SQLite, sem serviço externo |
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
| sqlite-vec | Busca por similaridade de vetores (vec0) | https://github.com/asg017/sqlite-vec |
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
| Transformers.js | Modelos ONNX no navegador (embeddings) | https://huggingface.co/docs/transformers.js |
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
- **Mtimes sempre em UTC (RFC3339 com `Z`)**: `time.Now().UTC().Format(time.RFC3339)`. Gravar mtime em formato local (ex: `-03:00`) misturado com UTC quebra a ordenação por string da sidebar/database. A ordenação em `system/handlers.go` usa `mtimeNewer()`, que compara `time.Time` parseado (robusto a formatos mistos/legados). Corrigido em 17/08/2026 (`HandleUploadAttachment` gravava local) com testes de regressão em `system/handlers_mtime_test.go` e `notes/handlers_upload_mtime_test.go`.
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

### 2.3 Testes

- **Go**: Testes no mesmo package (`package db`, não `package db_test`) para acesso a funções não exportadas.
- **JS (Node)**: Arquivos `.mjs` com `async/await`, sem frameworks de teste. `node web/<teste>.mjs` executa direto.
- **JS (Browser)**: Teste de chunking (`chunk_test.js`) roda em Node puro por ser função pura.

## 3. Embeddings Semânticos

### 3.1 Arquitetura

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

O recurso **"Notas Semelhantes"** no editor usa os embeddings armazenados para recomendar notas relacionadas. A lógica implementa:

- **Dois campos**: `minDistMap` (menor distância L2 por candidato) e `matchCounts` (em quantos chunks diferentes o candidato apareceu).
- **Threshold dinâmico**: Agora configurável pelo usuário na UI (padrão 72% de similaridade de cosseno, traduzido internamente para distância L2).
- **Voto majoritário**: Se a nota atual tem ≥3 chunks (nota longa), o candidato precisa ter match em ≥2 chunks diferentes para ser recomendado — a menos que a distância seja excepcional (`< 0.60`, ~82%).
- **Ordenação**: Primária por frequência de matches (decrescente), secundária por distância L2 (crescente). Top 5 resultados exibidos.
- **Parâmetros**: `similarExcellent = 0.60`, `longNoteMinChunks = 3`, `minMatchLongNote = 2`. O limite `similarThreshold` é obtido dinamicamente das configurações.

### 3.7 Configurações Dinâmicas de Limite Semântico (Threshold)

📍 Rota `/api/settings/semantic-thresholds` | `internal/features/system/handlers.go`

Para dar controle sobre a precisão da IA, adicionou-se sliders de configuração na aba **Semântica**:
- **Busca Semântica Global**: Define a similaridade mínima exigida na busca geral (padrão 35%). Controla a tolerância de resultados em `internal/features/embeddings/handlers.go`.
- **Notas Semelhantes**: Define a similaridade mínima para a aba do rodapé do editor (padrão 72%). Controla a exibição em `internal/features/notes/handlers_common.go`.
- **Persistência**: Ambos os percentuais são armazenados no SQLite na tabela de configurações como `semantic_search_threshold` e `similar_notes_threshold`.
- **Conversão de Métrica**: O banco de dados utiliza distância euclidiana L2 (sqlite-vec MATCH). A conversão a partir de porcentagem de similaridade de cosseno $c$ ocorre pela fórmula:
  $$dist_{L2} = \sqrt{2 \times (1 - c)}$$
- **Alterada em**: 14/07/2026 — implementação dos thresholds dinâmicos e UI de sliders.
- **Alterada em**: 09/08/2026 — padrão da busca global definido em 35% (validado empiricamente com o e5-small); removido o mapeamento especial que tratava 50% como 20% (o valor armazenado agora é usado literalmente, faixa válida 10–100%).

> ⚠️ A busca global (FTS5 + semântica via `POST /api/embeddings/search`) é independente e não foi afetada.

### 3.8 Momento da Indexação Semântica

- A indexação de embeddings é **lazy** e ocorre somente quando o usuário abre a aba de busca híbrida.
- Salvar ou editar uma nota não deve iniciar geração de embeddings nem requisições para `/api/embeddings/save`.
- Ao abrir a busca híbrida, `indexPending` identifica notas pendentes ou desatualizadas e indexa o conteúdo mais recente antes da busca.
- **Decidido em**: 10/09/2026 — evita consumo de CPU e travamentos durante a edição; o salvamento da nota permanece independente da indexação semântica.

### 3.9 Mapa Semântico (Galáxia de Notas) — PCA 2D

📍 `internal/core/db/semantic_map.go` | `internal/features/embeddings/semantic_map_handler.go` | `web/src/semantic-map.js`

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
| `note_chunks` | Chunks de texto para busca semântica |
| `note_embeddings` | Tabela virtual vec0 — vetores FLOAT[384] |
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
- Rate limiters para endpoints pesados: `searchLimiter` (30/min), `embLimiter` (30/min).
- Prefixo `/api/` para rotas JSON, sem prefixo para páginas HTML.

### 5.2 Respostas

- JSON com `Content-Type: application/json`.
- Cache-Control: `no-cache, max-age=10` para status de embeddings (dados dinâmicos).
- Erros: `http.Error(w, mensagem, statusCode)` — mensagens descritivas em português.

## 6. Observações Técnicas

### 6.1 `chunkText` com `maxChars=0`

`chunkText(text, 0, 0)` causa loop infinito porque `start` nunca avança (`end - overlap = 0`). **Não usar.** Os parâmetros reais (700, 100) são seguros.

### 6.2 WebGPU vs WebNN

O runtime ONNX tenta WebGPU primeiro (se disponível), depois cai para CPU (WASM). WebNN não é usado atualmente.

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

## 6.6 Download e Distribuição do Modelo de IA (atualizado em 09/09/2026)

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

- Botão **"Acima de"** no editor (abaixo do badge de backlinks) para escolher o pai sem digitar YAML.
- **Rename do pai** ainda não propaga o `parent` das filhas — hoje a filha vira órfã e volta para a raiz (`UpdateBacklinksOnRename` é o gancho natural).
- Contagem/exibição de filhas no editor e na sidebar.

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

## 7. Arquitetura de Busca

O sistema consagra três modalidades complementares de pesquisa textual e semântica, integrando tecnologias específicas para cada propósito.

### 7.1 Os Três Modos de Busca

| Modo | Descrição | Tecnologia | Destaque Visual |
| --- | --- | --- | --- |
| **Busca de Notas** | Filtro instantâneo no menu focado exclusivamente no nome/título dos arquivos Markdown. | Busca local indexada por correspondência parcial (`LIKE %q%`). | Azul (Sky) |
| **Busca Global** | Busca textual de termos no conteúdo interno de todas as notas do sistema. | SQLite FTS5 (tabela virtual) + Lematização (Stemming) em pt-BR. | Azul (Exato) e Roxo (Lematizado) |
| **Busca Semântica** | Pesquisa por aproximação conceitual e sentido (IA), lidando com sinônimos e contextos distantes. | Embeddings vetoriais locais gerados por IA (`MiniLM-L12-v2` via Transformers.js no browser). | Sem realce textual direto (exibe % de similaridade) |


## Como Funciona a Busca Semântica
Vetorização (Embeddings): Cada nota markdown tem seu texto limpo e dividido em pedaços (chunks) de ~700 caracteres (com o título da nota injetado em cada pedaço para manter o contexto). O navegador gera um vetor matemático de 384 dimensões para cada chunk usando o modelo de IA local MiniLM-L12-v2.
Pesquisa KNN: Quando você digita uma busca semântica, o navegador gera o vetor da sua pergunta e o envia ao banco de dados SQLite. O banco usa a extensão vetorial sqlite-vec para rodar um cálculo KNN (Vizinhos Mais Próximos) e encontrar quais chunks de notas no banco têm a direção vetorial mais parecida (similaridade de cosseno).

## Como Funcionam as Notas Relacionadas (Critérios)
Para a nota que você está editando no momento, o sistema faz o seguinte:

Busca por Chunk: Ele envia cada um dos chunks da nota aberta para buscar vizinhos no banco.
Estratégia do Voto Majoritário:
Ele anota a menor distância vetorial de cada nota candidata e em quantos chunks diferentes ela deu match.
Regra para Notas Longas: Se a nota que você está editando for longa (≥ 3 chunks), uma nota relacionada só é considerada relevante se der match em pelo menos 2 chunks diferentes da nota atual. A única exceção é se a similaridade de um chunk for excepcional (acima de 82%).
Ordenação: As top 5 notas relacionadas são ordenadas por frequência de matches (notas mais consistentes ao longo do texto vêm primeiro) e depois por proximidade vetorial (distância).
Nota de Corte (Threshold): Descarta qualquer resultado abaixo do percentual configurado por você (padrão de 72%).

## 8. Busca Híbrida — Reciprocal Rank Fusion (RRF, 10/08/2026)

📍 `internal/search/rrf.go` | `internal/features/search/handlers_hybrid.go` | `internal/features/search/index.templ`

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
- **Limpeza de embeddings (tabela virtual `vec0`)**: `chunk_id LIKE 'arquivo#%'` não usa índice (full scan). Agora resolve-se os `chunk_id` pelo índice `idx_note_chunks_filename` e apaga-se por igualdade (`chunk_id = ?`), que usa o índice da vec0. Helper único `deleteEmbeddingsForFile` usado em `SaveNoteChunks`, `DeleteEmbedding`, `DeleteNote`, `RenameNote`, `ReplaceFileIndexes` e `DeleteAllFileRecords`. **~18,5ms → ~1,1ms (~16×)**. ⚠️ Deve rodar **antes** de apagar as linhas de `note_chunks`.
- **Rename de backlinks (`UpdateBacklinksOnRename`)**: era N+1 (`GetNote` por candidato). Agora usa `GetAllNotesContent()` em uma query (método adicionado ao `NoteStore`). **~171ms → ~2ms (~86×)**.
- **`GetEmbeddingStatus`** (rota pollada): as 3 contagens agregadas custavam ~410ms em 3k notas. Resultado passou a ser **memoizado** no `Store` (`embeddingStatusCacheTTL = 10s`), **invalidado em toda escrita** que altere notes/chunks/embeddings/tags (SaveNote, DeleteNote, RenameNote, SaveNoteChunks, SaveEmbedding, DeleteEmbedding, ResetAllEmbeddings, ReplaceFileIndexes, DeleteAllFileRecords, Set/Add/RemoveTag). HTTP `Cache-Control: private, max-age=10` (antes `no-cache` forçava ida ao servidor). **~410ms → ~62ns no hit**.
- **`extractTerms`**: o regexp era recompilado a cada chamada (roda em loop no gate da híbrida). Agora é `var` de pacote.
- **`HandleManualSync`**: removido o dead code (N+1 `GetNote` + `time.Parse` descartado + `if false`); agora conta notas com conteúdo em uma query. Comportamento observável inalterado.
- **Frontend**: `database.js` parseia a query de busca **uma vez** (antes rodava por linha a cada tecla); `timeline.js` guarda o `redLineInterval` em escopo de **módulo** (corrige leak de timer/canvas a cada re-render) e coalesce o redraw do `wheel` via `requestAnimationFrame`; `editor-common.js` coalesce `updateHighlight` (click/keyup/scroll/selectionchange/resize) via rAF (evita layout thrashing).

> Não alterados (avaliados, menor prioridade/risco de contrato): o `COUNT(*)` do FTS5 por busca, o filtro `tags NOT LIKE '%drawing%'` dentro da query FTS e a queda para `SearchFTSLike` (full scan `LOWER(texto) LIKE`) em buscas sem resultado.

[Definição dos icones da aplicação](/core/internal/ui/icons/config.go)

https://lucide.dev/icons/