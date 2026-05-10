# TON-618 — Contexto do Projeto

**Última atualização:** 2026-05-09

> ⚠️ **INSTRUÇÃO AO AGENTE:** Sempre que realizar qualquer alteração no projeto, ADICIONE UMA ENTRADA no final da seção `## HISTÓRICO DE ALTERAÇÕES` abaixo, com a data e descrição do que foi feito. Isto é obrigatório para rastreamento do projeto.

---

## VISÃO GERAL

PKM (Personal Knowledge Management) de alta performance. Motor de busca Bleve embutido que indexa notas, documentos, imagens e links. Stack: Go 1.25 + Preact/Vite 8, single binary, embedded databases (Bleve + BBolt), Ollama opcional para embeddings.

**Core Value:** Capture e busque sem atrito.

---

## STATUS DE ESTABILIDADE: 🟢 ESTÁVEL

### Backend — 🟢 100% Funcional

| Componente | Status | Detalhes |
|------------|--------|----------|
| Servidor HTTP | ✅ OK | `go build` + `go vet` + 12 pacotes de teste passando |
| API REST | ✅ OK | 20+ endpoints: search, file, graph, settings, backup, maintenance |
| Busca Bleve | ✅ OK | Full-text + ranking Rico Score v3 |
| BBolt State | ✅ OK | Persistência ACID, 10 managers, recuperação de corrupção |
| Watcher fsnotify | ✅ OK | Com polling fallback |
| Autenticação Basic Auth | ✅ OK | |
| Ollama embeddings | ✅ OK | Suporte a múltiplos hosts (failover/seleção manual) |
| Rate limiting | ✅ OK | 30 req/s por IP |
| SPA fallback | ✅ OK | Sem 404 em refresh |

### Frontend — 🟢 Estável

| Componente | Status | Detalhes |
|------------|--------|----------|
| Build Vite | ✅ OK | ~5s, ~8 arquivos JS, ~5MB |
| Editor TipTap | ✅ OK | WYSIWYG, slash commands, tabelas, BubbleMenu |
| Busca | ✅ OK | Virtualizada com Virtuoso, compacta e global |
| Knowledge Map | ✅ OK | Fallback de tag #embed para notas sem cache de vetor |
| Configurações | ✅ OK | Gestão dinâmica de múltiplos hosts Ollama |
| Tags | ✅ OK | Autocomplete |
| Upload | ✅ OK | PDF + Imagem com OCR |
| DocsPanel | ✅ OK | Documentação interna via Markdown |
| Scrollbars | ✅ OK | Tema escuro global |

---

## ESTRUTURA DO PROJETO

```
TON618/
├── etl/                    # Backend Go
│   ├── main.go             # Entry point, server bootstrap, rotas (slog migrado)
│   ├── go.mod              # Go 1.25.0
│   └── internal/
│       ├── api/            # Handlers HTTP, middleware, SSE
│       ├── clustering/     # K-Means, PCA, cluster labeling
│       ├── config/         # Config por env vars
│       ├── events/         # SSE event hub
│       ├── ingest/         # Watcher, syncer, processor, state (BBolt)
│       ├── middleware/     # Logger, Recovery, RateLimiter
│       ├── models/         # Tipos compartilhados
│       ├── query/          # Engine de consulta SQL-like
│       ├── search/         # Bleve engine, ranker, cache
│       ├── semantic/       # Ollama provider, circuit breaker, cache
│       └── utils/          # Helpers (i18n, safe_go, slugify)
├── web/                    # Frontend Preact + Vite
│   ├── src/
│   │   ├── App.tsx         # Componente raiz
│   │   ├── components/     # UI components
│   │   │   ├── TiptapEditor.tsx    # Editor WYSIWYG (TipTap)
│   │   │   ├── WeightsSettings.tsx # Tela de configurações (Pesos + APIs + Múltiplos Ollamas)
│   │   │   ├── KnowledgeMap.tsx    # Mapa semântico D3
│   │   │   ├── DocsPanel.tsx       # Documentação interna
│   │   │   └── ... (20+ componentes)
│   │   ├── hooks/          # Custom hooks
│   │   ├── utils/          # Utilitários
│   │   └── __tests__/      # Testes Vitest
│   ├── tests/              # E2E Playwright (5 specs)
│   ├── public/help/        # Documentação em Markdown
│   └── vite.config.js      # Vite 8 + Preact + Tailwind
├── run.sh                  # Script de build + run
├── deploy.sh               # Deploy multi-arch Docker
├── Dockerfile              # Multi-stage build
├── docker-compose.yml      # Dev
├── docker-compose.prod.yml # Produção
├── context.md              # Contexto do projeto (este arquivo)
└── .planning/              # Planejamento e estado
```

---

## DÍVIDAS TÉCNICAS RESTANTES

### 🔴 Críticas (segurança)

| # | Dívida | Local | Detalhes |
|---|--------|-------|----------|
| 01 | Migração parcial para `slog` | `api/*.go`, `ingest/*.go` | `main.go` migrado, handlers ainda usam `log.Printf` |
| 02 | `context.TODO()` em produção | `clustering/keywords.go:107` | `TermFieldReader` sem timeout |
| 03 | Credenciais hardcoded | `config/config.go:40` | `AUTH_PASS="ton618_secret"` como fallback |
| 04 | Sem `.env.example` | Raiz | Variáveis não documentadas |
| 05 | Sem `.golangci.yml` | Raiz | Zero linter no Go |

### 🟡 Médias

| # | Dívida | Local | Detalhes |
|---|--------|-------|----------|
| 06 | Sem CI/CD | — | Tudo manual (./run.sh) |
| 07 | Testes E2E quebrados | `web/tests/` | 5 specs criados mas falham |
| 08 | Sem backup BBolt automático | — | Só endpoint manual |
| 09 | Sem TLS | `docker-compose.prod.yml` | Basic Auth over HTTP |

### 🟢 Leves

| # | Dívida | Local | Detalhes |
|---|--------|-------|----------|
| 10 | state.go > 350 linhas | `ingest/state.go` | 363 linhas |
| 11 | state.json vs state.db | `config/config.go` | Resíduo de refatoração |
| 12 | Comentários pt/en misturados | `etl/` | Hora português, hora inglês |

---

## ESTADO DOS TESTES

### Go Tests — ✅ Todos passando (12 pacotes)

```
etl                          0.006s
etl/internal/api             0.854s (incluindo graph_regression_test.go)
etl/internal/clustering      0.042s (incluindo clustering_test.go)
etl/internal/config          0.011s
etl/internal/ingest          8.120s (incluindo state_settings_test.go, projection_test.go)
etl/internal/query           0.087s
etl/internal/search          1.833s
etl/internal/semantic        0.011s
etl/internal/utils           0.006s
```

### Frontend Tests — Não verificados na última execução

- Unit: Vitest, testes em `web/src/__tests__/`
- E2E: Playwright, 5 specs em `web/tests/` (seletores precisam ajuste)

---

## COMO RODAR

```bash
./run.sh                    # Dev (build + start)
npm --prefix web run build  # Apenas frontend
docker compose -f docker-compose.prod.yml up -d  # Produção
```

**Porta:** 6180
**Credenciais:** admin / ton618_secret (ou gerada aleatória se AUTH_PASS não definida)
**Dados:** `/home/giobon/ton618_data/` (state.db, índice Bleve, logs)

---

## DEPENDÊNCIAS PRINCIPAIS

### Backend (Go)
- `github.com/blevesearch/bleve/v2` — Motor de busca full-text
- `go.etcd.io/bbolt` — Banco embedded ACID
- `codeberg.org/readeck/go-readability` — Extração de conteúdo web
- `golang.org/x/time/rate` — Rate limiting
- `gonum.org/v1/gonum` — PCA, K-Means

### Frontend (JS)
- `preact` + `@preact/preset-vite` — UI framework
- `@tiptap/*` — Editor WYSIWYG (StarterKit + Table + Mention + Placeholder + Markdown)
- `@tanstack/react-query` — Server state
- `react-virtuoso` — Lista virtualizada
- `d3-*` — Knowledge Map
- `tailwindcss` — CSS
- `marked` + `highlight.js` — Renderização Markdown

---

## HISTÓRICO DE ALTERAÇÕES

> Instrução ao agente: SEMPRE adicione uma nova entrada neste histórico ao finalizar qualquer alteração no projeto.

| Data | Descrição |
|------|-----------|
| 2026-05-09 | **Múltiplos Provedores Ollama.** Implementação de gestão dinâmica de hosts Ollama no frontend (`WeightsSettings`) e backend. Suporte a lista de URLs e seleção de host ativo para failover manual. Campo `OllamaHosts` e `OllamaHostActive` adicionados ao `AppSettings`. |
| 2026-05-09 | **Correção de Visibilidade no Mapa Semântico.** Implementado fallback no `HandleKnowledgeMap` para incluir notas com a tag `#embed` que ainda não possuíam vetor no cache, garantindo que apareçam no reindexador. Adicionado `graph_regression_test.go`. |
| 2026-05-09 | **Migração Structured Logging (slog).** `main.go` agora utiliza o pacote `slog` com saída JSON por padrão. Preparação para migração total dos handlers da API. |
| 2026-05-07 | **Pesquisa no Mapa Semântico (Query Point).** Nova funcionalidade "Consultar o Mapa" que projeta uma consulta de texto diretamente no mapa semântico. Exibe conexões visuais animadas para as 3 notas mais relevantes (top-3 similares). Botão dedicado na UI do mapa e endpoint `/api/graph/query-point`. |
| 2026-05-06 | **Busca por frase exata corrigida.** `NewPhraseQuery` trocado por `NewMatchPhraseQuery`. `IncludeTermVectors` removido. Teste `TestExecuteSearch_PhraseExact` adicionado para regressao. |
| 2026-05-06 | **Tabelas no editor TipTap.** Implementação completa: extensões `Table/TableCell/TableHeader/TableRow`, slash command `/table`, BubbleMenu com add/delete linha/coluna, CSS escuro para tabelas. |
| 2026-05-06 | **DocsPanel (documentação interna).** Componente que lê `web/public/help/README.md`. Suporta SVGs reais dos botões via `:icon-nome:`. Botão 📖 no painel superior. |
| 2026-05-05 | **Análise de viabilidade Wails v2** adicionada ao `context.md`. Conclusão: abordagem híbrida recomendada, mas usuário optou por não fazer. |
| 2026-05-05 | **Correção WeightsSettings.** Loop infinito resolvido via `useCallback`. Scrollbars padronizadas (tema escuro global). Slash commands no TiptapEditor. |
| 2026-05-05 | **Migração CodeMirror → TipTap.** Substituição completa. Fim dos erros TDZ do Rolldown com CodeMirror. |
| 2026-05-04 | **SPA fallback.** 404 em refresh/navegação direta corrigido. |
| 2026-05-04 | **Circuit breaker Ollama** (tri-state: closed/open/half-open). **Migração go-readability** de pseudo-version para fork mantido. **Testes de corrupção BBolt** + recuperação automática. |
| 2026-05-04 | **state.go fatiado** (497→339 linhas), bucket vars extraídos, FileModCache, HashCache, Popularity, FileMetadata, Settings em arquivos separados. |
| 2026-02-05 | **state.go fatiado** (state_tags, state_links, state_vectors criados). Injeção de dependência. Rate limiting middleware. fsnotify watcher. |
