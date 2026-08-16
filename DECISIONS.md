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
- **Tipos Indexáveis:** Markdown comuns (`NoteTypeMarkdown`), notas de transcrição do YouTube (`NoteTypeYoutube`), artigos da Web (`NoteTypeArticle`) e capturas rápidas (`NoteTypeCapture`).
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

### 3.8 Mapa Semântico (Galáxia de Notas) — PCA 2D

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

## 6.6 Download do Modelo de IA

📍 `web/download_model.js`

- O modelo `Xenova/paraphrase-multilingual-MiniLM-L12-v2` (ONNX q8, ~120MB) é baixado do **HuggingFace** usando `wget`.
- **`wget` é obrigatório** — lidou melhor com o XetHub/CAS Bridge do que `fetch()` ou `http.get()` do Node. Não substituir.
- O script gera automaticamente versões comprimidas (`.gz` e `.br`) ao lado do arquivo original.
- O Dockerfile **não** executa este script. O modelo é baixado pelo navegador via Transformers.js (CDN do HuggingFace + IndexedDB).
- **Esta decisão não deve ser alterada sem validação manual.** Já houve regressão por mexer neste arquivo.

### Arquivos baixados

| Arquivo | Tamanho |
|---------|---------|
| `config.json` | ~700B |
| `special_tokens_map.json` | ~200B |
| `tokenizer.json` | ~2.5MB |
| `tokenizer_config.json` | ~500B |
| `onnx/model_quantized.onnx` | ~120MB |

### ⚠️ Dependência do CSP para fallback remoto

Se os arquivos do modelo **não estiverem disponíveis localmente** (ex: `download_model.js` não foi executado), o Transformers.js tenta baixá-los via `fetch()` do CDN do HuggingFace. Essa conexão é **bloqueada pelo CSP** em `internal/middleware/middleware.go`:

```
connect-src 'self'
```

`huggingface.co` **não está listado** no `connect-src`, então o download remoto falha silenciosamente. **O modelo só funciona via arquivos locais servidos pelo próprio servidor Go** (`/static/models/`).

Caso no futuro seja necessário suportar fallback remoto, é preciso:
1. Adicionar `https://huggingface.co` (e possivelmente `https://cdn-lfs.huggingface.co`) ao `connect-src` do CSP.
2. Testar manualmente, pois o bloqueio do CSP não gera erro no servidor — aparece apenas no console do navegador.

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

## 7. Arquitetura de Busca

O sistema consagra três modalidades complementares de pesquisa textual e semântica, integrando tecnologias específicas para cada propósito.

### 7.1 Os Três Modos de Busca

| Modo | Descrição | Tecnologia | Destaque Visual |
| --- | --- | --- | --- |
| **Busca de Notas** | Filtro instantâneo no menu focado exclusivamente no nome/título dos arquivos Markdown. | Busca local indexada por correspondência parcial (`LIKE %q%`). | Azul (Sky) |
| **Busca Global** | Busca textual de termos no conteúdo interno de todas as notas do sistema. | SQLite FTS5 (tabela virtual) + Lematização (Stemming) em pt-BR. | Azul (Exato) e Roxo (Lematizado) |
| **Busca Semântica** | Pesquisa por aproximação conceitual e sentido (IA), lidando com sinônimos e contextos distantes. | Embeddings vetoriais locais gerados por IA (`MiniLM-L12-v2` via Transformers.js no browser). | Sem realce textual direto (exibe % de similaridade) |


## Como Funciona a Busca Semântica
Vetorização (Embeddings): Cada nota markdown tem seu texto limpo e dividido em pedaços (chunks) de ~1500 caracteres (com o título da nota injetado em cada pedaço para manter o contexto). O navegador gera um vetor matemático de 384 dimensões para cada chunk usando o modelo de IA local MiniLM-L12-v2.
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

[HELP do sistema](core/internal/features/system/help.md)
[Definição dos icones da aplicação](/core/internal/ui/icons/config.go)

https://lucide.dev/icons/