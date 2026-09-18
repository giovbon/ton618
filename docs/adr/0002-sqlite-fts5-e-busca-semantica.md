# ADR 0002: Persistência, FTS5 e Busca Semântica via SQLite

* **Status:** Aceito
* **Data:** 2026-09-18

## Contexto e Problema

Notas em formato Markdown demandam buscas eficientes tanto por termos exatos (FTS5) quanto por similaridade conceitual/semântica (embeddings de linguagem), mantendo a independência de servidores externos de busca ou bancos vetoriais dedicados.

## Decisão

1. **FTS5 Integrado ao SQLite:**
   - Utilizar a extensão nativa FTS5 com o tokenizador `unicode61` configurado para português (`remove_diacritics=1`).
   - Manter tabela virtual `docs_fts` sincronizada com a tabela `documents`.

2. **Busca Híbrida (RRF - Reciprocal Rank Fusion):**
   - Combinar os resultados da busca por texto (FTS5 BM25) e busca semântica (vetorial via `sqlite-vec` / embeddings Transformers.js/Ollama) utilizando a fórmula RRF.

## Consequências

* **Positivas:**
  - Resultados de busca mais relevantes integrando termos exatos e significado.
  - Zero dependência de cluster Elasticsearch ou Pinecone/Qdrant.
* **Pontos de Atenção:**
  - Indexação de notas exige execução em background para não bloquear requisições síncronas de escrita.
