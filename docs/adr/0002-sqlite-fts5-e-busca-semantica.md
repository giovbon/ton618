# ADR 0002: Persistência, FTS5 e Busca Semântica via SQLite

* **Status:** ~~Aceito~~ → **SUBSTITUÍDO em 16/10/2026**
* **Data:** 2026-09-18
* **Substituído por:** remoção do índice semântico e da busca híbrida (ver `DECISIONS.md` §13).
  A parte de **persistência e FTS5 continua válida**; tudo que dependia de embeddings
  (sqlite-vec/`vec0`, Transformers.js, RRF) foi removido do app em 16/10/2026.

## Contexto e Problema

Notas em formato Markdown demandam buscas eficientes tanto por termos exatos (FTS5) quanto por similaridade conceitual/semântica (embeddings de linguagem), mantendo a independência de servidores externos de busca ou bancos vetoriais dedicados.

## Decisão

1. **FTS5 Integrado ao SQLite:**
   - Utilizar a extensão nativa FTS5 com o tokenizador `unicode61` configurado para português (`remove_diacritics=1`).
   - Manter tabela virtual `docs_fts` sincronizada com a tabela `documents`.

2. **Busca Híbrida (RRF - Reciprocal Rank Fusion):** ❌ **REMOVIDO em 16/10/2026**
   - ~~Combinar os resultados da busca por texto (FTS5 BM25) e busca semântica (vetorial via `sqlite-vec` / embeddings Transformers.js/Ollama) utilizando a fórmula RRF.~~
   - A fusão dependia do motor vetorial, que não existe mais. O app hoje tem busca de nomes
     (`LIKE`) e busca global (FTS5). Ver `DECISIONS.md` §13.

## Consequências

* **Positivas:**
  - Resultados de busca mais relevantes integrando termos exatos e significado.
  - Zero dependência de cluster Elasticsearch ou Pinecone/Qdrant.
* **Pontos de Atenção:**
  - Indexação de notas exige execução em background para não bloquear requisições síncronas de escrita.
