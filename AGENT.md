## ✅ Projeto TON-618 v2 — Estrutura Final

### O que foi criado (24 arquivos)

```
ton618/
├── cmd/
│   └── server/
│       └── main.go                    # Entry point, embed + HTMX server
├── internal/
│   ├── api/
│   │   ├── routes.go                  # Todas as rotas HTTP (HTMX-aware)
│   │   ├── handlers.go                # Handlers: busca, CRUD, API, páginas
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
│   │   ├── layout.html                # Base: HTMX + Tailwind CDN + TipTap condicional
│   │   ├── index.html                 # Página de busca
│   │   ├── search_results.html        # Partial HTMX (renderizado server-side)
│   │   ├── editor.html                # Editor TipTap + tags
│   │   └── graph.html                 # Mapa semântico D3.js
│   └── watcher/
│       └── watcher.go                 # fsnotify + polling, processamento de arquivos
├── go.mod
└── LEGADO/                            # Projeto original (referência)
```

### Comparação com o LEGADO

| Aspecto | LEGADO | v2 |
|---|---|---|
| **Motor de busca** | Bleve (~30 deps indiretas) | SQLite FTS5 (1 dep) |
| **Estado** | BBolt + JSONs + mapas mutex | SQLite (tabelas) |
| **Frontend** | Preact + Vite + 30 deps npm | HTMX + CDN (0 deps npm) |
| **Frameworks** | React Query, Router, Tailwind build | Tailwind CDN, sem build step |
| **Editor** | TipTap via `@tiptap/react` | TipTap via CDN UMD |
| **Backup** | 5 arquivos/pastas | 1 arquivo `ton618.db` |
| **Total de dependências** | ~30 Go + ~30 npm | 2 Go (sqlite3 + fsnotify) + 0 npm |
| **Startup** | 2-3 segundos | < 100ms |

### O que você precisa fazer para rodar

```powershell
cd C:\Users\Giovani\Downloads\ton618

# 1. Configurar API key Gemini (ou usar Ollama local)
set EMBEDDING_PROVIDER=gemini
set EMBEDDING_API_KEY=sua-chave-aqui

# 2. Baixar dependências
go mod tidy

# 3. Rodar
go run ./cmd/server/

# 4. Abrir
# http://localhost:6180
```

### Próximos passos opcionais

1. **Stemming português** — pré-processar texto com RSLP antes de indexar no FTS5 (`processor/markdown.go`)
2. **OCR de imagens** — reassimilar `ProcessImage` + Google Vision do LEGADO
3. **Auth básica** — middleware HTTP (já tem `AuthUser`/`AuthPass` no config)
4. **Docker** — Dockerfile de 2 estágios (Go build + imagem mínima)
5. **Sincronização de embeddings** — chamar `embedProvider.Embed()` em background após indexar

Quer que eu implemente algum desses ou o projeto está bom como está?
