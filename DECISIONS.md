> Este documento deve ser atualizado sempre que uma decisão arquitetural significativa for tomada.
> ⚠️ Qualquer IA que for modificar este projeto **deve** ler este documento primeiro.
> Utilize o arquivo `repomix-output.xml` como referência rápida de código, a menos que consultas diretas aos arquivos do workspace por outras ferramentas (como busca textual ou leitura direta) sejam mais adequadas para a tarefa.

# Decisões de Arquitetura — TON-618

Este documento registra decisões arquiteturais e padrões adotados no projeto.
Serve como referência para manter consistência em contribuições futuras.

---

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
| jspreadsheet | Editor de planilhas | https://jspreadsheet.com/ |
| Tabulator | Tabelas de dados interativas | https://tabulator.info/ |
| Excalidraw | Editor de desenhos | https://excalidraw.com/ |
| Mermaid | Diagramas e gráficos | https://mermaid.js.org/ |
| Markmap | Mapas mentais | https://markmap.js.org/ |
| Leaflet | Mapas interativos (OpenStreetMap) | https://leafletjs.com/ |
| CodeJar | Editor de código leve | https://medv.io/codejar/ |
| vis-timeline | Timeline para agenda | https://visjs.github.io/vis-timeline/ |
| chrono | Parser de datas em linguagem natural | https://github.com/wanasit/chrono |
| Transformers.js | Modelos ONNX no navegador (embeddings) | https://huggingface.co/docs/transformers.js |
| marked | Parse markdown para HTML | https://marked.js.org/ |
| lowlight | Syntax highlight de código | https://github.com/wooorm/lowlight |
| HTMX | Interatividade sem JS escrito | https://htmx.org/ |
| Alpine.js | Interatividade declarativa | https://alpinejs.dev/ |
| Tailwind CSS | Framework CSS utility-first | https://tailwindcss.com/ |
| esbuild | Bundler e minifier | https://esbuild.github.io/ |

### Infraestrutura

| Ferramenta | Uso |
|------------|-----|
| Docker + Docker Compose | Containerização |
| Typst | Compilador de documentos (texto para PDF via tipst) |
| Nominatim (OSM) | Geocoding reverso para mapas |
| OSRM | Cálculo de rotas para mapas |

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
- **TypeScript incremental via `checkJs`**: TypeScript (`tsc --noEmit`) é usado apenas para checagem de tipos em cima de JSDoc, sem transpilação. `npm run typecheck` executa a validação. O build continua exclusivamente com esbuild.
- **Migração gradual para `.ts`**: Arquivos em `web/src/` podem ser renomeados para `.ts`/`.tsx` conforme forem sendo migrados. O esbuild aceita TypeScript nativamente — basta atualizar o `entryPoints` em `build.js`. O JS inline em arquivos `.templ` não é verificável por TS e permanece como está até ser extraído.
- **`web/src/global.d.ts`**: Declarações de tipos para globais `window.*` (IIFE exports), bibliotecas sem types (Leaflet, jSuites, markmap) e módulos CSS. Mantenha sincronizado com as funções expostas.
- **Arquivo fonte em `web/src/`, compilado para `web/static/`**: esbuild compila e minifica. `npm run build` gera os estáticos. **Nunca editar `static/` diretamente.**
- **IIFE para scripts no browser**: O build do esbuild usa `format: "iife"` para gerar código que não polui o escopo global além do que é explicitamente exposto.
- **Web Worker para tarefas pesadas**: `semantic-worker.js` (ESM module) executa inferência ONNX em thread separada — não bloqueia UI.
- **var, function, sem arrow functions nos fontes do browser**: O build target é es2020 e algumas páginas usam IIFE. Manter compatibilidade.

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

### 3.2 Chunking

- `chunkText(text, maxChars=1500, overlapChars=200)` em `web/src/semantic.js`.
- Quebra por `\n` (parágrafo) se disponível nos primeiros 60% do limite.
- Fallback para espaço. Último recurso: corte seco no limite.
- Overlap de 200 caracteres preserva contexto entre chunks.

### 3.3 Indexação

- **Lazy**: Só indexa quando o usuário abre a busca semântica.
- Título extraído do primeiro `# ` e prefixado em cada chunk.
- Markdown limpo antes do chunking: remove blocos de código, imagens, mantém só texto de links.
- `Promise.all` para paralelizar chunks de uma mesma nota.

### 3.4 Staleness

- `note_chunks.indexed_mtime` armazena o mtime da nota no momento da indexação.
- `GetPendingEmbeddingNotes()` compara com `notes.mtime` para detectar desatualizados.
- Notas não-indexáveis (drawing, spreadsheet, mermaid, mapa) são excluídas via SQL.

### 3.5 Notas Indexáveis vs Não-Indexáveis (Regras de Paridade)

- **Regra Geral:** Apenas notas de texto contínuo e leitura humana são indexáveis. Dados puramente estruturados, visuais ou códigos não são indexados.
- **Tipos Indexáveis:** Markdown comuns (`NoteTypeMarkdown`), documentos Typst (`NoteTypeTypst`), mapas mentais/markmaps (`NoteTypeMindmap`), notas de transcrição do YouTube (`NoteTypeYoutube`), artigos da Web (`NoteTypeArticle`) e capturas rápidas (`NoteTypeCapture`).
- **Tipos Não-Indexáveis:** Desenhos/Excalidraw (`NoteTypeDrawing`), planilhas (`NoteTypeSpreadsheet`), diagramas Mermaid (`NoteTypeMermaid`), mapas geográficos (`NoteTypeMap`), arquivos/PDFs na pasta `pdfs/` (`NoteTypePDF`), anexos na pasta `attachments/` (`NoteTypeAttachment`) e notas arquivadas na pasta `archives/` (`NoteTypeArchive`).
- **Paridade Go/SQL:** O método Go `IsNoteEmbeddable` (que valida as gravações) e as queries SQL (`GetPendingEmbeddingNotes` e `CountEmbeddableNotes`) devem estar em perfeita paridade quanto a essa lógica de exclusão de notas. Para manter a performance, a detecção de tipo é baseada apenas no caminho do arquivo, tags e heurísticas de nome de arquivo (ex: conter `mapa.` ou `mapa-` no nome), sem abrir o conteúdo completo das notas.
- **Garantia via Teste:** O teste de integração `TestIsNoteEmbeddableMatchesSQL` garante que qualquer divergência futura entre Go e SQL na lógica de exclusão de notas quebrará os testes locais e o CI/CD. Adicionalmente, o teste `TestDeleteNoteCleansEmbeddingsAndOrphanStatus` garante que a remoção de notas limpa seus respectivos chunks e embeddings, e que o cálculo de status de indexação é resiliente a registros órfãos pré-existentes.

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

`chunkText(text, 0, 0)` causa loop infinito porque `start` nunca avança (`end - overlap = 0`). **Não usar.** Os parâmetros reais (1500, 200) são seguros.

### 6.2 WebGPU vs WebNN

O runtime ONNX tenta WebGPU primeiro (se disponível), depois cai para CPU (WASM). WebNN não é usado atualmente.

### 6.3 `process.on('unhandledRejection')`

Usado nos testes JS para silenciar rejeições intencionais (testes de `embed_error` e timeout). Não usar em produção.

## 7. Pendentes

| ID | Item | Esforço | Motivo |
|----|------|---------|--------|
| P1 | ~~Migrar queries de embeddings para sqlc~~ | ✅ Feito | `HasEmbedding`, `GetEmbeddedFiles`, `GetEmbeddingStatus`, `GetPendingEmbeddingNotes`, `DeleteEmbedding`, `SaveNoteChunks` migradas para sqlc. `SearchSimilar`, `SaveEmbedding`, `GetNoteEmbeddings` mantidas como SQL cru por usarem a tabela virtual `note_embeddings` (vec0) que sqlc não reconhece. |
| P2 | **Stemmer pt-BR para FTS5** | Baixo | `unicode61` não faz stemming. "navegador" não encontra "navegação". O fallback LIKE já cobre, mas um stemmer melhoraria precisão. |
| P3 | ~~Corrigir erros do typecheck (30 erros)~~ | ✅ Feito | JSDoc corrigido em `semantic.js`, `mindmap.js`, `semantic-worker.js`, `map.js`, `drawing.jsx`. Ajustadas declarações em `global.d.ts`. `npm run typecheck` agora passa limpo. |