# Glossário de Componentes — TON-618

Este glossário define nomes consistentes para cada componente do sistema.
Sirva-se dele para nomear arquivos, funções, pacotes e commits.

---

## 🏗️ Camadas de Sistema

| Nome | Onde está | O que faz |
|------|-----------|-----------|
| **Core** | `core/` | Código base compartilhado entre auto-hospedado e comercial |
| **Server** | `core/cmd/server/` | Binário principal (chi + SQLite + templ) |
| **Frontend Web** | `core/web/` | Interface browser (TipTap, HTMX, Alpine, Tailwind) |
| **Desktop** | `desktop/` | App desktop (Wails — Go + WebView) |
| **Commercial** | `pkg/commercial/` | Licenciamento RSA (build tag `commercial`) |
| **Docs** | `docs/`, `core/docs/` | Documentos de usuário (PDFs, arquivos) |
| **Notes** | `documents/`, `core/documents/` | Notas markdown dos usuários |

## 🧱 Módulos Internos (Go)

| Nome | Package | Responsabilidade |
|------|---------|-----------------|
| **Config** | `core/internal/core/config` | Carrega configurações de ambiente + YAML |
| **Store** | `core/internal/core/db` | Banco SQLite (WAL, migrações, queries) |
| **Notes** | `core/internal/features/notes` | CRUD de notas, editores, upload |
| **Search** | `core/internal/features/search` | Busca textual (FTS5) |
| **Embeddings** | `core/internal/features/embeddings` | Busca semântica (sqlite-vec) |
| **System** | `core/internal/features/system` | Login, settings, docs, database |
| **Appointments** | `core/internal/features/appointments` | Agenda (vis-timeline) |
| **Todos** | `core/internal/features/todos` | Tarefas (kanban) |
| **Watcher** | `core/internal/watcher` | Observa arquivos no disco |
| **Middleware** | `core/internal/middleware` | Auth, CSP, rate-limit, tenant |
| **Processor** | `core/internal/processor` | IDs (CUID2), markdown, PDF |
| **Ranker** | `core/internal/search` | Ranqueamento de resultados de busca |

## 🖥️ Frontend (JavaScript)

| Nome | Arquivo fonte | Compilado para | Função |
|------|---------------|----------------|--------|
| **TipTap editor** | `web/src/editor.js` | `static/editor.js` | Config das extensões do editor |
| **Editor init** | `web/src/editor-init.js` | `static/editor-init.js` | Inicialização do editor |
| **Editor common** | `-` | `static/editor-common.js` | Helpers compartilhados (save, rename) |
| **Spreadsheet** | `web/src/spreadsheet.js` | `static/spreadsheet.js` | Planilhas (jspreadsheet) |
| **Drawing** | `web/src/drawing.jsx` | `static/drawing.js` | Desenhos (Excalidraw) |
| **Mindmap** | `web/src/mindmap.js` | `static/mindmap.js` | Mapas mentais (Markmap) |
| **Map** | `web/src/map.js` | `static/map.js` | Mapas (Leaflet) |
| **Semantic** | `web/src/semantic.js` | `static/semantic.js` | Busca semântica (Transformers.js) |
| **Semantic worker** | `web/src/semantic-worker.js` | `static/semantic-worker.js` | Web Worker de embeddings ONNX |
| **Database** | `web/src/database.js` | `static/database.js` | Tabela de notas (Tabulator) |
| **Agenda** | `web/static/js/agenda/` | (já em static) | Timeline de eventos |

## 📡 API (Rotas)

| Prefixo | Formato | Exemplo |
|---------|---------|---------|
| `/api/notes` | JSON | `/api/notes/database` |
| `/api/embeddings` | JSON | `/api/embeddings/search` |
| `/api/settings` | JSON | `/api/settings/semantic-thresholds` |
| `/file/*` | Redirect/Form | `/file/save`, `/file/rename` |
| `/editor` | HTML | `/editor?file=notes/exemplo.md` |
| `/spreadsheet` | HTML | `/spreadsheet?file=notes/planilha.md` |
| `/drawing` | HTML | `/drawing?file=notes/desenho.md` |
| `/typst` | HTML | `/typst?file=notes/doc.typ` |
| `/mermaid` | HTML | `/mermaid?file=notes/diagrama.md` |
| `/mindmap` | HTML | `/mindmap?file=notes/mapa-mental.md` |
| `/map` | HTML | `/map?file=notes/mapa-local.md` |
| `/database` | HTML | `/database` (Tabulator) |
| `/agenda` | HTML | `/agenda` (Calendário) |
| `/epub/reader` | HTML | `/epub/reader?file=epubs/livro.epub` |

## 🗄️ Banco de Dados (SQLite)

| Tabela | Tipo | Finalidade |
|--------|------|------------|
| `notes` | Row | Conteúdo markdown + metadados |
| `note_chunks` | Row | Pedaços de texto para embedding |
| `note_embeddings` | Virtual (vec0) | Vetores FLOAT[384] para busca semântica |
| `documents` | Row | Fragmentos indexados (FTS5) |
| `docs_fts` | Virtual (FTS5) | Índice de busca textual |
| `tags` | Row | Tags por arquivo |
| `links` | Row | Wikilinks entre notas |
| `popularity` | Row | Score de popularidade |
| `schema_versions` | Row | Controle de migrações |
| `settings` | Row | Configurações do usuário |

## 🔧 Infraestrutura

| Nome | Tecnologia | Função |
|------|-----------|--------|
| **Docker image core** | `core/Dockerfile` | Imagem pública (Docker Hub) |
| **Docker image commercial** | `core/Dockerfile` + BUILD_TAGS | Imagem comercial |
| **Cloud deploy** | `docker-compose.cloud.yml` | Deploy SaaS multi-tenant |
| **Workflow** | `.github/workflows/deploy_core.yml` | CI/CD GitHub Actions |

## 🧪 Padrões de Teste

| Tipo | Onde | Framework |
|------|------|-----------|
| Testes Go | Junto do código (`package db`) | `testing` padrão |
| Testes JS | `web/tests/*.spec.ts` | Playwright |
| Testes unit JS | `web/tests/*.unit.cjs` | `node --test` |
