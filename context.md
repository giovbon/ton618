# TON-618 — Contexto do Projeto

**Ultima atualizacao:** 2026-05-11

> ⚠️ **INSTRUCAO AO AGENTE:** Sempre que realizar qualquer alteracao no projeto, ADICIONE UMA ENTRADA no final da secao `## HISTORICO DE ALTERACOES` abaixo.

---


## Modelos suportados

| Modelo | Dimensao | PT-BR | Peso | Comando |
|--------|----------|-------|------|---------|
| `nomic-embed-text` | 768d | ✅ Bom | ~270MB | `ollama pull nomic-embed-text` |
| `multilingual-e5` | 1024d | 🔝 Otimo | ~560MB | `ollama pull multilingual-e5` |
| `bge-m3` | 1024d | 🔝 Otimo | ~1GB+ | `ollama pull bge-m3` |
| `mxbai-embed-large` | 1024d | ⚠️ Ingles | ~335MB | `ollama pull mxbai-embed-large` |

Troque `OLLAMA_MODEL` + ajuste `EmbeddingDimension` nas Settings.

---

## Estado dos testes

| Pacote | Cobertura | Status |
|--------|-----------|--------|
| `config` | 100% | 🟢 |
| `search` | 91.2% | 🟢 |
| `clustering` | 76.7% | 🟢 (era 15.7%) |
| `query` | 76.4% | 🟢 |
| `utils` | 65.7% | 🟡 |
| `semantic` | 63.3% | 🟡 |
| `api` | 52.1% | 🟡 |
| `ingest` | - | ❌ state_settings_test.go quebrado |

---

## HISTORICO DE ALTERACOES

| Data | Descricao |
|------|-----------|
| 2026-05-10 | **t-SNE + BestK + cache 2D + binario.** `tsne.go` substitui PCA (separacao superior). `BestK` adaptativo por silhouette. Coords 2D cacheadas no `NoteVector` (mapa instantaneo em chamadas subsequentes). `encodeNoteVector`/`decodeNoteVector` binario (magic byte 0xFF, backward compat JSON). `state.go` fatiado em `state_load.go` + `state_delegation.go` (de 370 para 130 linhas). 7 novos testes: t-SNE, BestK, silhouette, binary round-trip. Cobertura clustering 15.7% → 76.7%. |
| 2026-05-10 | **Revertido Ciclo 4** (llama-server/provider unificado removido). Mantido apenas Ollama. |
| 2026-05-10 | **Ciclo 3: Performance & Precisao.** Batch DisjunctionQuery, batch embedding (10/chamada), PCA debounce, normalizacao L2, K-Means++, context timeout. |
| 2026-05-10 | **Ciclos 1-2: Embeddings & Grafico.** Circuit breaker, PCA max dim, zero leitura disco, titulos piggyback, projecoes background, globalCB, chunk warning, extractTitle, NoteVector, DeleteNoteProjection, Matryoshka. |
| 2026-05-09 | Multiplos Provedores Ollama, Visibilidade Mapa, slog. |
| 2026-05-07 | Query Point no mapa. |
| 2026-05-06 | Busca exata, Tabelas TipTap, DocsPanel. |
| 2026-05-11 | **Grafo Estruturado + Links Semânticos.** Implementação do `ManualSemanticMap` (D3 Canvas) com arraste, zoom e quebra de linha inteligente. Extração via regex `@\[...\]` no Go (`processor.go`) com limpeza de HTML e proteção multi-linha. Persistência em BBolt (`semantic_topics`, `file_semantic_links`). Editor TipTap atualizado com `SemanticLinkMark` (não-inclusivo) e `HashtagMark`. |
| 2026-05-10 | **WeightsSettings, CodeMirror→TipTap.** |
| 2026-05-04 | SPA fallback, Circuit breaker, BBolt testes, state.go fatiado. |
