# TON-618 — Análise do Projeto

## 1. Visão Geral

**TON-618** é uma aplicação de **gestão de conhecimento pessoal (PKM)** e anotações, construída como um **binário único Go** com frontend renderizado no servidor. Projetada para rodar em container Docker, otimizada para ambientes de baixo recurso como Raspberry Pi. O projeto é escrito em português brasileiro e visa usuários lusófonos.

### Características Principais
- Binário único Go (sem dependências externas em runtime)
- Banco de dados SQLite (Go puro, sem CGo)
- UI renderizada no servidor com HTMX para interatividade
- Notas baseadas em Markdown com YAML frontmatter
- Busca full-text via SQLite FTS5
- File system watcher para indexação em tempo real
- Deploy Docker com suporte multi-arch (AMD64 + ARM64)

---

## 2. Estrutura do Projeto

```
ton618/
├── .github/
│   ├── workflows/deploy_core.yml    # CI/CD: teste → versionamento → build & push Docker
│   ├── dependabot.yml               # Atualizações automáticas: Go, npm, GitHub Actions
│   └── copilot-instructions.md      # Instruções para agentes de IA
├── .gitattributes                   # Força LF para .sh, .go, .templ, .yml
├── CLAUDE.md                        # Instruções para agentes de IA
├── README.md                        # Snippet Docker Compose
├── run.sh                           # Runner raiz (delega para core/run.sh)
├── ANALISE.md                       # Este arquivo
├── docs/
│   ├── DECISIONS.md                 # ★ Documento mestre de decisões de arquitetura
│   ├── linguagem-de-consulta.md     # Referência da linguagem de consulta
│   └── adr/                         # Architecture Decision Records
│       ├── 0001-stack-principal-e-tecnologias.md
│       ├── 0002-sqlite-fts5-e-busca-semantica.md
│       ├── 0003-templating-com-templ-e-htmx.md
│       └── 0004-linguagem-de-consulta.md
└── core/                            # ★ Aplicação principal
    ├── cmd/server/
    │   ├── main.go                  # Entry point
    │   ├── routes.go                # Registro de rotas
    │   └── main_test.go
    ├── internal/
    │   ├── core/
    │   │   ├── config/              # Configuração (env vars)
    │   │   ├── db/                  # Camada de banco (Store, migrations, queries)
    │   │   ├── domain/              # Tipos de domínio
    │   │   ├── services/            # Backup, notificações Ntfy, restart
    │   │   ├── staticver/           # Versionamento de arquivos estáticos
    │   │   └── timeutil/            # Utilitários de tempo
    │   ├── features/
    │   │   ├── notes/               # ★ Feature de notas (editor, arquivos, hierarquia)
    │   │   ├── search/              # Feature de busca (FTS5, arquivos, snippets)
    │   │   ├── system/              # Páginas do sistema (index, config, login, database)
    │   │   ├── todos/               # Feature de marcadores todo
    │   │   └── appointments/        # Feature de agenda/compromissos
    │   ├── middleware/              # Auth, rate limiting, headers de segurança
    │   ├── processor/               # Processamento de documentos (markdown, PDF, query)
    │   ├── query/                   # Motor de linguagem de consulta (CEL)
    │   ├── repository/              # Interfaces de repositório
    │   ├── search/                  # Lógica de ranking/scoring de busca
    │   ├── httputil/                # Utilitários HTTP
    │   ├── ui/icons/                # Sistema de ícones SVG inline
    │   └── watcher/                 # File system watcher (fsnotify)
    ├── web/
    │   ├── src/                     # Frontend (JS/TS)
    │   ├── static/                  # Assets compilados (gerados, nunca editar)
    │   ├── layout/                  # Componentes de layout Templ
    │   ├── tests/                   # Playwright + testes unitários Node
    │   ├── build.js                 # Script de build esbuild + Tailwind
    │   └── package.json
    ├── data/                        # Dados em runtime (SQLite, modelos, logs)
    ├── scripts/                     # Scripts de dev/debug
    ├── Dockerfile                   # Build multi-stage
    ├── docker-compose.yml
    ├── deploy.sh                    # Deploy multi-arch Docker Hub
    ├── go.mod / go.sum
    └── run.sh                       # Script de build & run completo
```

---

## 3. Stack Tecnológica

### Backend (Go 1.26.3)
| Tecnologia | Propósito |
|---|---|
| **chi/v5** | Router HTTP |
| **modernc.org/sqlite** | SQLite Go puro (sem CGo), com FTS5 |
| **a-h/templ** | Templates HTML type-safe compilados para Go |
| **cel-go** | Avaliação de expressões CEL para linguagem de consulta |
| **gopkg.in/yaml.v3** | Parsing de YAML frontmatter |
| **fsnotify** | File system watching |
| **go-readability** | Extração de artigos web |
| **html-to-markdown** | Conversão HTML para markdown |
| **youtube-transcript-api-go** | Busca de transcrições do YouTube |

### Frontend (JavaScript/Browser)
| Tecnologia | Propósito |
|---|---|
| **TipTap 3.x** | Editor WYSIWYG markdown |
| **Tailwind CSS 4.x** | CSS utility-first |
| **esbuild** | Bundler JS |
| **HTMX** | UI orientada ao servidor |
| **Alpine.js** | Interatividade declarativa |
| **Excalidraw** | Editor de desenhos |
| **markmap** | Visualização de mindmap |
| **vis-timeline** | Timeline da agenda |
| **chrono-node** | Parsing de datas em linguagem natural |
| **marked** | Parsing de markdown |
| **lowlight/PrismJS** | Syntax highlighting |
| **epub.js** | Leitor EPUB |

### Infraestrutura
- **Docker** build multi-stage (Node → Go → Alpine)
- **UPX** compressão de binário
- **GitHub Actions** CI/CD
- **Dependabot** para atualizações de dependências

---

## 4. Arquitetura Detalhada

### 4.1 Entry Point (`cmd/server/main.go`)

A função `main()` orquestra o startup:
1. **Carregamento de config** — `config.Load()` lê env vars com defaults
2. **Banco de dados** — `db.NewStore()` abre SQLite, roda migrations
3. **Migração disco→DB** — `MigrateNotesFromDisk()` importa arquivos `.md`
4. **Indexação inicial** — `watcher.ScanAndIndexAllParallel()` scan paralelo
5. **Scheduler de notificações** — `NtfyService` faz polling de lembretes (goroutine)
6. **Contexts das features** — HandlerContexts para notes, search, todos, appointments, system
7. **Servidor HTTP** — router chi com chain de middleware
8. **Graceful shutdown** — SIGINT/SIGTERM ou restart por backup/restore

### 4.2 Chain de Middleware
```
Request → Logging → Recovery → SecurityHeaders → RequestContext → Compress → [BasicAuth] → Routes
```

- **BasicAuth**: Autenticação por senha (username ignorado), suporta header Basic Auth, Authorization manual e cookie
- **Rate limiting**: Token bucket por IP (30 req/min para busca)
- **Headers de segurança**: CSP, X-Frame-Options, X-Content-Type-Options, etc.
- **Arquivos estáticos**: Servidos com ETag + cache imutável (1 ano)

### 4.3 Camada de Banco de Dados

**Schema** (10 tabelas):
- `notes` — Conteúdo markdown + mtime (armazenamento principal)
- `documents` — Fragmentos de documentos indexados (fonte FTS5)
- `docs_fts` — Tabela virtual FTS5 (tokenizer unicode61)
- `tags` — Tags de arquivos (many-to-many)
- `links` — Wikilinks entre notas
- `popularity` — Scores de popularidade estilo RLHF
- `todo_markers` — Definições de marcadores todo customizados
- `todos` — Items todo extraídos de notas
- `appointments` — Eventos de calendário
- `settings` — Configuração chave-valor
- `notification_log` — Histórico de notificações enviadas
- `file_mods` — Tempos de modificação de arquivos

**Padrões-chave:**
- `db.Store` é uma **struct concreta** (não interface) com `WriteMu sync.Mutex` para serialização de escrita
- Modo WAL para leituras concorrentes
- `sqlc` para geração de queries SQL type-safe
- Migrations rastreadas na tabela `schema_versions`
- Timeout de query de 30 segundos com cancelamento por context

### 4.4 Arquitetura de Features

Cada feature segue um padrão consistente:
- `context.go` — HandlerContext struct com dependências
- `handlers_*.go` — Handlers HTTP
- `*_templ.go` — Funções de render geradas pelo Templ
- `*_test.go` — Testes (mesmo pacote, DB real)

**Feature Notes** (maior):
- `NoteService` — Lógica de negócio para CRUD de notas, reindexação, gestão de tags/links
- `CaptureService` — Funcionalidade de captura rápida
- `AutoTagService` — Auto-tagging baseado em inatividade
- Editor, Desenho, Mindmap, Semanal, Leitor EPUB
- Hierarquia (relacionamento pai/filho)
- Painel de consulta (linguagem de consulta baseada em CEL)
- Upload/download/renomear/excluir arquivos
- Integração com backup/restore

**Feature Search**:
- Busca full-text baseada em FTS5
- Scoring BM25 + bônus heurísticos (match de título, frase exata, caminho, frescor)
- Integração de peso RLHF (peso sináptico + bônus estrutural de backlink)
- Operações de exclusão em massa/arquivamento
- Gestão de stopwords

**Feature System**:
- Página index, configurações, login, visualizador de banco
- Configurações de notificação Ntfy
- Configurações de auto-tag
- Sidebar com árvore de notas
- Tabelas de dados baseadas em Tabulator

**Feature Appointments**:
- CRUD para eventos de calendário
- Visualização em árvore da agenda
- Visualização em timeline
- Notificações semanais de resumo

**Feature Todos**:
- Marcadores customizados (definidos pelo usuário)
- Extração de todos de markdown
- Contadores em badges

### 4.5 Linguagem de Consulta (CEL-based)

Feature única: notas podem conter blocos ` ```consulta ` com configuração YAML e expressões CEL:

```yaml
tipo: notas
tags: [livros]
onde: n.interacao == null || dias(n.interacao) > 90
ordenar: mtime desc
limite: 10
mostrar: lista
```

- **Parser** (`processor/query.go`) — Parsing YAML puro, sem acesso ao DB
- **Engine** (`query/engine.go`) — Compilação CEL com cache, limites de custo
- **Runner** (`query/run.go`) — Execução somente-leitura contra o store
- **Duas fases**: Pré-filtro SQL/FTS5 (máx 500 candidatos) → Refinamento CEL
- **Determinístico**: `agora` injetado por chamada, nunca `time.Now()` internamente

### 4.6 File System Watcher

- Usa `fsnotify` para monitorar subdiretórios `docs/`
- Processa: `.md`, `.pdf`, imagens, `.zip`, `.epub`
- Deduplicação via map `recentlyProcessed` (janela de 3 segundos)
- Indexação inicial paralela com workers configuráveis
- Extensões suportadas mapeadas para tipos de documento

### 4.7 Versionamento de Arquivos Estáticos (`staticver`)

- SHA256 hashes de arquivos estáticos → ETag + URLs com cache-busting
- `staticver.URL("/static/file.js")` → `/static/file.js?v=abc123`
- `Cache-Control: immutable` por 1 ano
- Elimina bumps de versão manuais

---

## 5. Arquitetura Frontend

### Pipeline de Build
```
web/src/*.js → esbuild (bundle, minify, IIFE, es2020) → web/static/*.js
web/src/input.css → Tailwind CSS → web/static/app.css
Todos os estáticos → Compressão Gzip + Brotli
```

### Módulos Frontend-chave
- `editor.js` — Setup do editor TipTap (StarterKit, tabelas, tasks, code blocks, markdown)
- `editor-init.js` — Inicialização do editor
- `editor-slash.js` — Comandos slash
- `editor-toc.js` — Sumário
- `editor-hierarchy.js` — Navegação pai/filho
- `editor-frontmatter.js` — Edição de frontmatter
- `database.js` — Tabelas de dados Tabulator
- `mindmap.js` — Renderização de mindmap Markmap
- `app.js` — Lógica principal da app
- `drawing.jsx` — Editor de desenho Excalidraw

### Sistema de Layout
- `layout.templ` — Layout HTML base com HTMX, Alpine.js, Tailwind
- `navbar.templ` — Barra de navegação
- `settings_modal.templ` — Modal de configurações
- Dark mode por padrão (`class="dark"`)

---

## 6. CI/CD & Deploy

### GitHub Actions (`.github/workflows/deploy_core.yml`)
1. **Job de teste**: Go tests, gofmt check, go vet, npm typecheck
2. **Versionamento**: Auto-incremento SemVer via Conventional Commits (`feat`→minor, `fix`→patch, `BREAKING`→major) ou tag manual — ver [`docs/versionamento.md`](docs/versionamento.md)
3. **Job de deploy**: Docker build multi-arch (AMD64 + ARM64), push para Docker Hub

### Docker
- **Multi-stage**: Node 24 (web build) → Go 1.26 (binário) → Alpine 3.21 (runtime)
- **Compressão UPX** para binário menor
- **Fontes self-hosted** (Fira Sans) para capacidade offline
- **Health check**: `wget http://localhost:6180/api/health`
- **Volumes**: `/app/docs`, `/app/data`

### Docker Compose
- Imagem: `giovbon/ton618plus:latest`
- Porta: 6180
- Limites de memória: 256MB max, 64MB reservado (otimizado para Raspberry Pi)
- Environment: `DOCS_DIR`, `DB_PATH`, `STATE_DIR`, `WEB_DIR`, `PORT`, `AUTH_PASS`, `TZ`

---

## 7. Padrões de Testes

### Testes Go
- **Mesmo pacote** (não `_test` package) para acesso a funções não exportadas
- **SQLite real** em `t.TempDir()` — sem mocks
- **Testes de integração** para fluxos completos (salvar → indexar → buscar → excluir)
- **Benchmarks** para caminhos críticos de performance
- **Testes de paridade** para garantir alinhamento Go/SQL
- **Testes golden** para avaliação determinística de consultas

### Testes JavaScript
- **Testes unitários Node**: `node --test tests/*.unit.cjs`
- **Playwright E2E**: `tests/*.spec.ts` para automação de browser
- **TypeScript checking**: `tsc --noEmit`

### Arquivos de Teste (60+ total)
- `*_test.go` — 50+ arquivos de teste Go
- `*.unit.cjs` — Testes unitários Node
- `*.spec.ts` — Testes E2E Playwright

---

## 8. Decisões de Design (do DECISIONS.md)

1. **Binário único** — Sem serviços externos, deploy fácil
2. **SQLite com WAL** — Leituras concorrentes, escritas serializadas
3. **FTS5 para busca** — Zero config, stemming português via unicode61
4. **Templ + HTMX** — UI orientada ao servidor, JS mínimo
5. **CUID2 IDs** — Curtos, únicos, ordenáveis
6. **SQLite Go puro** — Sem CGo, cross-compilation amigável
7. **Embeddings semânticos REMOVIDOS** (Out 2026) — Era Transformers.js no browser, agora só FTS5
8. **Linguagem de consulta** — CEL-based, apenas no servidor, somente-leitura por construção
9. **Versionamento estático** — ETags SHA256, sem cache busting manual
10. **Erro na UI** — Erros renderizados na interface, nunca 500 para problemas do usuário

---

## 9. Segurança

- **BasicAuth** com validação apenas por senha
- **Headers CSP** restringindo origens de script/style/font/img/connect
- **X-Frame-Options: DENY**
- **Rate limiting** em endpoints pesados (busca)
- **Limites de custo de consulta** (CEL cost limit, máximo de candidatos, timeout)
- **Motor de consulta somente-leitura** — Sem caminhos de escrita no sistema de consulta
- **Sanitização de input** — Normalização de filenames, sanitização de termos FTS5

---

## 10. Features Notáveis

- **Notas semanais** — Filenames determinísticos baseados na semana ISO-8601
- **Hierarquia de notas** — Relacionamentos pai/filho via frontmatter
- **Backlinks** — Rastreamento de backlinks diretos e indiretos
- **Auto-tagging** — Tagging baseado em inativação (trigger manual)
- **Integração Ntfy** — Notificações push para lembretes
- **Backup/restore completo** — ZIP com manifest, snapshot do DB, todos os arquivos
- **Leitor EPUB** — Leitura EPUB no browser com rastreamento de posição
- **Desenho/Excalidraw** — Editor de desenhos integrado
- **Mindmap** — Visualização Markmap
- **Agenda/Timeline** — Calendário com vis-timeline
- **Transcrições YouTube** — Busca e indexação de transcrições de vídeo
- **Captura de artigos web** — Extração com Readability

---

## 11. Resumo de Dependências

**Dependências Go diretas** (10):
- `cel.dev/cel-go`, `codeberg.org/readeck/go-readability`, `github.com/JohannesKaufmann/html-to-markdown/v2`, `github.com/a-h/templ`, `github.com/go-chi/chi/v5`, `github.com/horiagug/youtube-transcript-api-go`, `golang.org/x/net`, `gopkg.in/yaml.v3`, `modernc.org/sqlite`

**Dependências npm** (30+):
- Ecossistema TipTap (15+ pacotes), Excalidraw, markmap, vis-timeline, chrono-node, marked, lowlight, PrismJS, React (para markmap), Tailwind CSS 4, esbuild, TypeScript, Playwright

---

## 12. Conclusão

O TON-618 é um projeto **maduro e bem arquitetado**, com:
- **Documentação extensa** (DECISIONS.md com 1300+ linhas)
- **Forte cultura de testes** (60+ arquivos de teste)
- **Clara separação de responsabilidades** (features, camadas, serviços)
- **Ênfase em simplicidade e performance** como sistema PKM em binário único
- **Otimização para low-resource** (Raspberry Pi, 256MB RAM)
- **Auto-hospedagem completa** com Docker

O código prioriza manutenibilidade, com decisões documentadas, testes abrangentes e arquitetura limpa.
