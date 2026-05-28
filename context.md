# TON-618 v2 — Contexto do Projeto

**Ultima atualizacao:** 2026-05-28

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
       ├── 1. Cria registro mínimo no FTS5 (apenas título, SEM texto extraído)
       ├── 2. Redireciona para pagina inicial (modo compacto)
       └── NOTA: Não extrai mais o texto completo do PDF
```

### Processamento (internal/processor/pdf.go)

| Etapa | Detalhes |
|---|---|
| **Extracao de texto** | ❌ **DESATIVADA** — não extrai mais o texto do PDF |
| **Armazenamento** | `doc.Texto` = `""` (vazio) — apenas título/nome indexado no FTS5 |
| **Tags** | `["pdf"]` — identificador de tipo |

### Comportamento atual

Desde 2026-05-28, **PDFs não têm mais o texto completo extraído** para evitar poluir a busca global com conteúdo de livros inteiros. O que muda:

- **Busca compacta**: PDF continua aparecendo normalmente (por nome de arquivo)
- **Busca global**: PDF pode ser encontrado pelo **título** e **nome do arquivo** (colunas `secao` e `arquivo` com peso 10× e 20× no FTS5)
- **Embedding**: não é gerado embedding (texto vazio)
- **Mapa semântico**: PDF continua aparecendo como nó normal com ícone 📕

> ⚠️ Se precisar do texto completo indexado para um PDF específico, use o toggle de embedding na interface ou converta o PDF para markdown manualmente.

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

### 2026-05-25 — Cluster high-D + t-SNE total + heatmap (substitui Voronoi)
- **Problema:** PCA + Voronoi nos centroides não separava notas por assunto. Voronoi criava células artificiais sem significado semântico.
- **Clustering high-D (768D):** `index.ClusterHighD()` faz k-means diretamente nos vetores de embedding originais, não nas coordenadas 2D projetadas. Agrupa por similaridade semântica real.
- **t-SNE total no startup:** `QueueFullReproject()` força t-SNE em TODAS as embeddings após indexação inicial, e sempre que o endpoint `/api/graph/project` é chamado (em vez de PCA).
- **Heatmap de densidade:** Substitui o Voronoi por acúmulo de gradientes radiais gaussianos — mostra concentrações naturais de notas sem fronteiras artificiais.
- **`GetAllFileEmbeddings()`:** Nova query com JOIN que carrega embeddings + arquivo em 1 consulta (não N+1).
- D3.js removido do mapa (não é mais necessário).

### 2026-05-28 — Arquivamento de notas (substitui exclusão em massa)
- **Nova funcionalidade:** Notas podem ser arquivadas em ZIP e restauradas depois
- **Aba "Arquivamento"** substitui "Exclusão" como tab principal: preview com checkboxes, selecionar/desselecionar todos, "Arquivar Selecionadas" (zipa e move para `docs/archives/`) e "Excluir Selecionadas" (permanente)
- **Aba "Restaurar"** lista archives disponíveis com botão "Restaurar" que extrai o ZIP de volta para os diretórios originais e reindexa
- **Endpoint `POST /api/bulk-archive`:** recebe lista de arquivos, cria ZIP em `docs/archives/`, remove do DB e filesystem
- **Endpoint `GET /api/archives`:** lista archives com metadados (arquivos, tamanho, data)
- **Endpoint `POST /api/archive/restore`:** extrai ZIP, reindexa via watcher, remove o archive
- **`HandleBulkDelete`** atualizado para aceitar `files[]` explícito (seleção via checkbox)
- Diretório `archives/` adicionado ao `config.EnsureDirs()` e `watcher.MonitoredSubDirs`
- Archives são indexados no FTS5 (pesquisáveis) com lista de arquivos que contêm
- Interface: checkboxes, contador de seleção, botões com número de itens selecionados

### 2026-05-28 — PDFs: desativada extração de texto completo
- `ProcessPDF` (`internal/processor/pdf.go`) modificado: **não extrai mais o texto completo** do PDF
- Agora cria apenas um documento stub com `Texto: ""` — o PDF é pesquisável apenas por título (coluna `secao`) e nome de arquivo (coluna `arquivo`)
- Motivo: livros grandes em PDF poluíam a busca global com conteúdo irrelevante
- Documento stub mantém o PDF visível na busca compacta e no mapa semântico
- Dependência `github.com/ledongthuc/pdf` removida do código (ainda em `go.mod`/`go.sum`)
- Testes atualizados em `internal/processor/pdf_test.go` para refletir novo comportamento
- `context.md` atualizado com a nova documentação
