# TON-618 v2 — Contexto do Projeto

**Ultima atualizacao:** 2026-05-11

> ⚠️ **INSTRUCAO AO AGENTE:** Sempre que realizar qualquer alteracao no projeto, ADICIONE UMA ENTRADA no final da secao `## HISTORICO DE ALTERACOES` abaixo.

---

## Suporte a PDFs

### Upload

O upload de PDFs e feito atraves do link **📕 PDF** na barra superior, ao lado de "+ Nova".

```
Usuario clica em "📕 PDF" na navbar
       │
       ▼
Seleciona arquivo .pdf
       │
       ▼
Upload via POST /upload
       │
       ▼
Salvo em docs/pdfs/nome-do-arquivo.pdf
       │
       ▼
ProcessFile → ProcessPDF()
       │
       ├── 1. Extrai texto de TODAS as paginas
       ├── 2. Cria documento FTS5 com o texto COMPLETO
       ├── 3. Gera embedding do texto COMPLETO (sempre, independente de tags)
       ├── 4. Extrai top 30 keywords para boost de relevancia na busca
       └── 5. Redireciona para pagina inicial (modo compacto)
```

### Processamento (internal/processor/pdf.go)

| Etapa | Detalhes |
|---|---|
| **Biblioteca** | `github.com/ledongthuc/pdf` — puro Go, sem CGO |
| **Extracao de texto** | Todas as paginas, texto puro (plain text) |
| **Keywords** | Top 30 termos mais frequentes apos remover stopwords (NLTK pt + en, ~310 palavras) |
| **Armazenamento** | `doc.Texto` = texto completo \| `doc.Tags` = 30 keywords (para peso no FTS5) |

### Embedding

Diferente das notas markdown (que seguem a tag `embed` ou a flag `EMBEDDING_ALL`), **PDFs sao SEMPRE embedados** automaticamente, desde que haja um provider de embedding configurado.

O embedding usa o texto completo do PDF (`secao + texto`), nao apenas as keywords.

### Busca (FTS5)

O texto completo do PDF e indexado no FTS5, tornando-o buscavel pela busca global. As 30 keywords ficam na coluna `tags` do indice FTS5 com **peso 50×**, dando um boost de relevancia quando o termo pesquisado esta entre as palavras mais frequentes do PDF.

### Mapa Semantico

PDFs aparecem como nos normais no grafo (bolinhas coloridas), com as seguintes diferencas:
- **Texto do no**: prefixo `📕` antes do nome do arquivo (ex: `📕 relatorio`)
- **Cor do texto**: laranja (`#fb923c`) em vez de cinza
- **Clique**: abre o PDF em nova aba via `/file?name=pdfs/...` (em vez do editor)

### Interface

- **Navbar**: link `📕 PDF` entre "+ Nova" e "Mapa"
- **Modo compacto**: PDFs aparecem com `📕` e link direto para o arquivo (abre em nova aba)
- **Busca global**: PDFs mostram `📕` no tipo e link laranja direto para o arquivo
- **Sem #tags visiveis**: as keywords extraidas NAO aparecem como hashtags na interface (apenas para peso no FTS5)

### Armazenamento

Os arquivos PDF ficam em `docs/pdfs/`. O diretorio e criado automaticamente no startup e monitorado pelo watcher (fsnotify + polling).

---

## Modelos suportados (embedding)

| Provider | Variavel | Modelo | Dimensao |
|---|---|---|---|
| **Gemini** (padrao) | `EMBEDDING_PROVIDER=gemini` | `gemini-embedding-2` | 768 |
| **Ollama** | `EMBEDDING_PROVIDER=ollama` | `nomic-embed-text` | 768 |
| **OpenAI** | `EMBEDDING_PROVIDER=openai` | `text-embedding-3-small` | 768 |

---

## HISTORICO DE ALTERACOES

### 2026-05-11 — Suporte a PDFs
- Criado `internal/processor/pdf.go` com `ProcessPDF()` e `ExtractKeywords()`
- Adicionado `github.com/ledongthuc/pdf` como dependencia
- Watcher: monitora `pdfs/`, processa arquivos `.pdf`, faz embedding automatico
- Upload: salva em `pdfs/`, redireciona para home
- Navbar: link `📕 PDF` ao lado de "+ Nova"
- Mapa semantico: nos de PDF com prefixo 📕 e cor laranja, clique abre o arquivo
- Busca: texto completo indexado, keywords com peso 50× no FTS5
- Interface: PDFs aparecem no modo compacto e busca global sem #tags visiveis
- Stopwords: listas completas do NLTK (131 pt + 179 en)

### 2026-05-24 — Frontmatter indexado na busca global
- Todo campo do frontmatter (exceto `tags`) é serializado no formato `chave: valor` e prefixado ao texto do primeiro documento FTS
- Permite buscar por `status: draft`, `author: joao`, `category: programacao` etc. usando a busca global
- Zero schema change: só prefixa o texto, FTS5 indexa naturalmente
- Alterado em `internal/processor/markdown.go`: coleta campos do YAML em `metaParts` e prefixa em `docs[0].Texto`

### 2026-05-24 — Rebalanceamento dos pesos de ranking
- `scoreFragment` reescrito: BM25 base positivo, bônus relativos ao BM25 (não absolutos)
- Removidos: `scoreKeywords` (tags já tem 50× no FTS5), `scoreRichness` (favorecia notas longas), radical match (falso positivo)
- Popularidade e backlinks movidos do re-ranker (sinais globais, não relacionados à consulta)
- Cap: score máximo 5× o BM25 base
- `RankWeights` simplificado de 8 campos para 1 (só BoostFreshness)
- Alterado em `internal/index/ranker.go` e `internal/index/search.go`

### 2026-05-24 — Reestruturação de pacotes
- `internal/search/` + `internal/semantic/` → `internal/index/` (busca textual + embeddings + ranking unificados)
- `internal/capture/` → `internal/api/handlers_capture.go` (não precisava de pacote separado)
- `internal/api/handlers.go` dividido em 5 arquivos:
  - `handlers.go`: páginas + API endpoints pequenos (~170 linhas)
  - `handlers_file.go`: CRUD de arquivos (~466 linhas)
  - `handlers_search.go`: busca + exclusão em massa (~409 linhas)
  - `handlers_graph.go`: mapa semântico (~434 linhas)
  - `handlers_capture.go`: captura de URLs (~297 linhas)
- Nenhum arquivo do pacote `api` passa de 470 linhas
- Import paths atualizados: `ton618/internal/semantic` → `ton618/internal/index`, `ton618/internal/search` → `ton618/internal/index`, `ton618/internal/capture` removido
