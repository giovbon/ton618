# 🌌 TON-618 v2 — Motor de Busca Personal Knowledge Management

**TON-618 v2** é um motor de busca pessoal (PKM) que indexa arquivos Markdown, combina busca textual **FTS5** com **embeddings semânticos** e entrega resultados via um frontend **HTMX + Tailwind CDN** — tudo sem dependências npm e com inicialização em **< 100ms**.

| Aspecto | Stack |
|---|---|
| **Linguagem** | Go 1.24+ |
| **Busca textual** | SQLite FTS5 |
| **Embeddings** | Google Gemini / Ollama / OpenAI |
| **Frontend** | HTMX + Tailwind CDN + TipTap (editor) + D3.js (grafo) |
| **Banco** | SQLite (`mattn/go-sqlite3`) |
| **Monitoramento** | `fsnotify` + polling |
| **Dependências Go** | 3 (sqlite3, fsnotify, yaml.v3) |
| **Dependências npm** | 0 |

---

## 📋 Pré-requisitos

| Ferramenta | Versão | Notas |
|---|---|---|
| **Go** | 1.24+ | [Download](https://go.dev/dl/) |
| **GCC** | qualquer | Necessário para **CGO** (usado pelo `go-sqlite3`) |
| **Git** | qualquer | Para clonar o repositório |
| **Docker + Buildx** | recente | *Opcional* — para construir imagens multi-arch |
| **API Key Google Gemini** | grátis | Ou [Ollama](https://ollama.com) rodando localmente |

### Verificando pré-requisitos

```ton618plus/terminal.txt#L1-5
go version          # → go1.24.1 linux/amd64
gcc --version       # → gcc (GCC) 14.2.0
git --version       # → git 2.45.0
```

> ⚠️ **CGO é obrigatório.** O pacote `mattn/go-sqlite3` é uma biblioteca C vinculada via CGO. Sem GCC, a compilação falhará com erro do linker.

> ⚠️ **Build tag `sqlite_fts5`.** O FTS5 (Full-Text Search) não vem habilitado no SQLite de todos os sistemas.
> A tag `-tags sqlite_fts5` faz o driver embarcar uma versão do SQLite com FTS5 compilado.
> Use-a **sempre** nos comandos `go run`, `go build` e `go test`.

---

## 🚀 Começando

### 1. Clone e entre no diretório

```ton618plus/terminal.sh#L1-2
git clone https://github.com/giovbon/ton618plus.git
cd ton618plus
```

### 2. Configure as variáveis de ambiente (`.env`)

Crie um arquivo `.env` na raiz do projeto:

```ton618plus/.env#L1-12
EMBEDDING_PROVIDER=gemini
EMBEDDING_API_KEY=sua-chave-aqui
EMBEDDING_MODEL=text-embedding-004
DOCS_DIR=./docs
DB_PATH=./data/ton618.db
PORT=6180
AUTH_USER=admin
AUTH_PASS=ton618
OLLAMA_HOST=http://localhost:11434
OLLAMA_MODEL=nomic-embed-text
POLL_INTERVAL_SEC=30
EMBEDDING_DIM=768
```

#### Variáveis disponíveis

| Variável | Padrão | Descrição |
|---|---|---|
| `EMBEDDING_PROVIDER` | `gemini` | Provedor de embedding: `gemini`, `ollama` ou `openai` |
| `EMBEDDING_API_KEY` | — | Chave da API (obrigatório para Gemini/OpenAI) |
| `EMBEDDING_MODEL` | `text-embedding-004` | Modelo de embedding (ex.: `text-embedding-3-small` para OpenAI) |
| `EMBEDDING_DIM` | `768` | Dimensionalidade dos vetores de embedding |
| `DOCS_DIR` | `./docs` | Diretório com seus arquivos `.md` |
| `DB_PATH` | `./data/ton618.db` | Caminho do banco SQLite |
| `STATE_DIR` | `./data` | Diretório para estado interno |
| `PORT` | `6180` | Porta do servidor HTTP |
| `AUTH_USER` | `admin` | Usuário da autenticação básica HTTP |
| `AUTH_PASS` | `ton618` | Senha da autenticação básica HTTP |
| `OLLAMA_HOST` | `http://localhost:11434` | URL do servidor Ollama |
| `OLLAMA_MODEL` | `nomic-embed-text` | Modelo usado no Ollama |
| `POLL_INTERVAL_SEC` | `30` | Intervalo de polling do watcher (em segundos) |

### 3. Baixe as dependências

```ton618plus/terminal.sh#L1-1
go mod tidy
```

### 4. Crie as pastas de dados

```ton618plus/terminal.sh#L1-1
mkdir -p docs data
```

### 5. Rode o servidor

```ton618plus/terminal.sh#L1-1
go run -tags sqlite_fts5 ./cmd/server/
```

Você verá algo como:

```
time=... level=INFO msg="TON-618 iniciando..."
time=... level=INFO msg="Banco SQLite pronto"
time=... level=INFO msg="Provedor de embedding" provider=gemini
time=... level=INFO msg="Templates carregados"
time=... level=INFO msg="Indexação inicial..."
time=... level=INFO msg="Indexação inicial concluída"
time=... level=INFO msg="Servidor HTTP rodando" addr="http://localhost:6180"
```

### 6. Acesse

Abra no navegador: **[http://localhost:6180](http://localhost:6180)**

---

## 🐚 Scripts

### `run.sh` — Inicialização rápida

Script que compila o binário otimizado (com `-ldflags="-s -w"`), carrega o `.env` automaticamente e sobe o servidor com logging.

```ton618plus/terminal.sh#L1-2
chmod +x run.sh
./run.sh
```

### `deploy.sh` — Build multi-arch + Docker Hub

Faz o build e push de imagens multi-arquitetura (AMD64 + ARM64) para o Docker Hub.

```ton618plus/terminal.sh#L1-2
chmod +x deploy.sh
./deploy.sh v1.0.0 2
```

| Modo | Descrição |
|---|---|
| `1` | Apenas **ARM64** (rápido, Raspberry Pi / Apple Silicon) |
| `2` | **Multi-Arch** AMD64 + ARM64 (padrão) |

A imagem é publicada como `giovbon/ton618plus:<tag>`.

---

## 🐳 Docker

### Construir imagem local

```ton618plus/terminal.sh#L1-1
docker build -t ton618 .
```

### Rodar container

```ton618plus/terminal.sh#L1-1
docker run -d \
  -p 6180:6180 \
  -v ./docs:/app/docs \
  -v ./data:/app/data \
  --env-file .env \
  --name ton618 \
  ton618
```

### Docker Compose

```ton618plus/terminal.sh#L1-1
docker compose up -d
```

O `docker-compose.yml` já expõe a porta `6180`, monta os volumes `./docs` e `./data`, e carrega as variáveis do `.env`.

**Certifique-se de criar o `.env` antes:**

```ton618plus/.env#L1-2
EMBEDDING_PROVIDER=gemini
EMBEDDING_API_KEY=sua-chave-aqui
```

### Healthcheck

O container expõe um healthcheck via `/api/health`. Você pode testar com:

```ton618plus/terminal.sh#L1-1
curl http://localhost:6180/api/health
```

---

## 🏗️ Estrutura do Projeto

```
ton618/                          # Módulo Go (import path: "ton618")
├── cmd/
│   └── server/
│       └── main.go              # Entry point — embed FS, servidor HTTP, graceful shutdown
├── internal/
│   ├── api/
│   │   ├── routes.go            # Registro de todas as rotas HTTP (HTMX-aware)
│   │   ├── handlers.go          # Handlers: busca, CRUD, API, páginas
│   │   ├── middleware.go        # Middleware: recovery, logging, basic auth
│   │   └── render.go            # Server-side rendering com html/template
│   ├── config/
│   │   └── config.go            # Config por env vars + criação de diretórios
│   ├── db/
│   │   ├── db.go                # Conexão SQLite + schema (9 tabelas + FTS5)
│   │   ├── documents.go         # CRUD de documentos
│   │   ├── fts.go               # Busca FTS5 + fallback LIKE
│   │   ├── state.go             # Popularidade, tags, links, file_mods, settings
│   │   └── vectors.go           # Embeddings (vector BLOB + encode/decode)
│   ├── processor/
│   │   └── markdown.go          # Parse: YAML frontmatter, hashtags, wikilinks, headers
│   ├── search/
│   │   ├── search.go            # Motor: FTS5 → LIKE fallback → re-rank → sort
│   │   └── ranker.go            # Scoring com pesos (título, tag, frase, frescor, path)
│   ├── semantic/
│   │   ├── provider.go          # Interface Embedder + factory + cache LRU
│   │   ├── gemini.go            # Google Gemini Embeddings API
│   │   ├── ollama.go            # Ollama local embeddings
│   │   └── openai.go            # OpenAI Embeddings API
│   ├── template/
│   │   ├── layout.html          # Base: HTMX + Tailwind CDN + Alpine.js + TipTap condicional
│   │   ├── index.html           # Página de busca principal
│   │   ├── search_results.html  # Partial HTMX (renderizado server-side)
│   │   ├── editor.html          # Editor rich text TipTap + gerenciamento de tags
│   │   └── graph.html           # Mapa semântico interativo (D3.js force graph)
│   └── watcher/
│       └── watcher.go           # fsnotify + polling, processamento de arquivos
├── go.mod                       # Módulo: ton618, Go 1.24.1
├── go.sum
├── Dockerfile                   # Multi-stage: golang:1.24-alpine → alpine:3.21
├── docker-compose.yml           # Serviço + volumes + healthcheck
├── run.sh                       # Script de inicialização rápida
├── deploy.sh                    # Build multi-arch + push Docker Hub
├── .gitignore                   # Ignora .env, data/, docs/, binário
├── AGENT.md                     # Documentação interna do desenvolvedor
├── README.md                    # ← Você está aqui
└── LEGADO/                      # Projeto original (referência histórica)
```

### Comparação com o LEGADO

| Aspecto | LEGADO | v2 |
|---|---|---|
| **Motor de busca** | Bleve (~30 deps indiretas) | SQLite FTS5 (1 dep) |
| **Estado** | BBolt + JSONs + mapas mutex | SQLite (tabelas relacionais) |
| **Frontend** | Preact + Vite + 30 deps npm | HTMX + CDN (0 deps npm) |
| **Frameworks** | React Query, Router, Tailwind build | Tailwind CDN, sem build step |
| **Editor** | TipTap via `@tiptap/react` | TipTap via CDN UMD |
| **Backup** | 5 arquivos/pastas | 1 arquivo `ton618.db` |
| **Dependências totais** | ~30 Go + ~30 npm | 3 Go + 0 npm |
| **Startup** | 2–3 segundos | < 100ms |

---

## ⚙️ Configuração de Embeddings

### Google Gemini (recomendado — grátis)

1. Acesse [aistudio.google.com/apikey](https://aistudio.google.com/apikey)
2. Crie uma **API Key** (gratuita, com cotas generosas)
3. Coloque no `.env`:

```ton618plus/.env#L1-2
EMBEDDING_PROVIDER=gemini
EMBEDDING_API_KEY=AIzaSySuaChaveAqui
```

O modelo padrão é `text-embedding-004` com 768 dimensões.

### Ollama (local, sem internet)

```ton618plus/terminal.sh#L1-2
ollama pull nomic-embed-text
ollama serve
```

No `.env`:

```ton618plus/.env#L1-3
EMBEDDING_PROVIDER=ollama
OLLAMA_HOST=http://localhost:11434
OLLAMA_MODEL=nomic-embed-text
```

### OpenAI

No `.env`:

```ton618plus/.env#L1-3
EMBEDDING_PROVIDER=openai
EMBEDDING_API_KEY=sk-sua-chave-aqui
EMBEDDING_MODEL=text-embedding-3-small
```

> 💡 **Dica:** A interface `Embedder` em `internal/semantic/provider.go` usa um cache LRU para evitar re-embedding de textos já processados. Você pode implementar novos provedores basta implementar a interface.

---

## 🔍 Como funciona

1. **Watcher** (`internal/watcher/watcher.go`) monitora a pasta `DOCS_DIR` com `fsnotify` + polling a cada `POLL_INTERVAL_SEC` segundos.

2. **Processor** (`internal/processor/markdown.go`) faz o parse de cada arquivo `.md`: extrai YAML frontmatter, hashtags (`#tag`), wikilinks (`[[link]]`), headers e o texto puro.

3. **Indexação**: O documento é inserido no SQLite com **FTS5** (Full-Text Search) para busca textual rápida.

4. **Embeddings**: O texto é enviado ao provedor configurado (Gemini / Ollama / OpenAI) que retorna um vetor numérico. Esse vetor é armazenado como BLOB no banco para busca semântica.

5. **Busca**: O motor (`internal/search/search.go`) executa:
   - Primeiro: busca全文 via FTS5
   - Fallback: `LIKE` para consultas parciais
   - Re-ranking: algoritmo em `internal/search/ranker.go` com pesos para título, tags, correspondência de frase, frescor (data de modificação) e path do arquivo

6. **Frontend**: HTMX faz requisições ao servidor, que renderiza HTML parcial (server-side rendering) e devolve ao navegador — sem JavaScript complexo.

---

## 🧪 Endpoints da API

| Método | Rota | Descrição |
|---|---|---|
| `GET` | `/` | Página inicial (busca) |
| `GET` | `/api/search?q=...` | Busca textual + semântica (HTMX partial) |
| `GET` | `/api/document/{id}` | Detalhes de um documento |
| `GET` | `/api/health` | Healthcheck |
| `GET` | `/editor` | Editor TipTap |
| `GET` | `/graph` | Mapa semântico D3.js |
| `POST` | `/api/document` | Criar novo documento |
| `PUT` | `/api/document/{id}` | Atualizar documento |
| `DELETE` | `/api/document/{id}` | Deletar documento |

---

## 🛠️ Desenvolvimento

### Compilação otimizada

```ton618plus/terminal.sh#L1-1
go build -tags sqlite_fts5 -ldflags="-s -w" -o ton618 ./cmd/server/
```

### Testes

```ton618plus/terminal.sh#L1-1
go test ./...
```

### Lint

```ton618plus/terminal.sh#L1-1
go vet ./...
```

---

## 📄 Licença

**MIT** — veja o arquivo [LICENSE](LICENSE) para detalhes.

---

<p align="center">
  🌌 <strong>TON-618 v2</strong> — Porque sua base de conhecimento merece um buraco negro de busca. 🔭
</p>
