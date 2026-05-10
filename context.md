# TON-618 — Contexto do Projeto

**Ultima atualizacao:** 2026-05-10

> ⚠️ **INSTRUCAO AO AGENTE:** Sempre que realizar qualquer alteracao no projeto, ADICIONE UMA ENTRADA no final da secao `## HISTORICO DE ALTERACOES` abaixo.

---

## VISAO GERAL

PKM (Personal Knowledge Management) de alta performance. Stack: Go 1.25 + Preact/Vite 8, Bleve + BBolt, Ollama ou llama-server para embeddings.

---

## COMO RODAR

```bash
./run.sh                    # Dev
docker compose up -d        # Dev com Ollama no Docker
docker compose -f docker-compose.prod.yml up -d  # Producao
```

**Porta:** 6180 | **Credenciais:** admin / ton618_secret

---

## PROVIDERS DE EMBEDDING

| Provider | Env | Host padrão | API |
|----------|-----|-------------|-----|
| Ollama | `EMBEDDING_PROVIDER=ollama` | `http://192.168.15.6:11434` | `/api/embed` |
| llama-server | `EMBEDDING_PROVIDER=llama-server` | `http://llama:8085` | `/v1/embeddings` |

### Modelos suportados

| Modelo | Dimensão | PT-BR | Peso (Q4) | Comando |
|--------|----------|-------|-----------|---------|
| `nomic-embed-text` | 768d | ✅ Bom | ~270MB | `ollama pull nomic-embed-text` |
| `multilingual-e5` | 1024d | 🔝 Ótimo | ~560MB | `ollama pull multilingual-e5` |
| `bge-m3` | 1024d | 🔝 Ótimo | ~1GB+ | `ollama pull bge-m3` |
| `mxbai-embed-large` | 1024d | ⚠️ Inglês | ~335MB | `ollama pull mxbai-embed-large` |

Para trocar de modelo: mude `OLLAMA_MODEL` + ajuste `EmbeddingDimension` nas Settings.

### Persistencia de modelos no Docker

Com `docker compose down` os modelos **NAO sao perdidos** — o volume `./ollama_data` (bind mount) mantem os dados no host. Apenas `docker compose down -v` remove volumes nomeados, mas bind mounts **nunca** sao afetados. O mesmo vale para `./models` com llama-server.

---

## HISTORICO DE ALTERACOES

| Data | Descricao |
|------|-----------|
| 2026-05-10 | **Ciclo 4: Provider Unificado.** `EmbeddingProvider` (ollama/llama-server), factory `NewEmbeddingFunc`/`NewBatchEmbedFunc`, `NewLlamaServerEmbedding` (OpenAI API), `BatchEmbedLlamaServer`. Callers atualizados. `docker-compose.yml` com modelos alternativos + llama-server. |
| 2026-05-10 | **Ciclo 3: Performance & Precisao.** Batch DisjunctionQuery, batch embedding (10/chamada), PCA debounce, normalizacao L2, K-Means++, context timeout. 17/17. |
| 2026-05-10 | **Ciclos 1-2: Embeddings & Grafico.** 11 itens — circuit breaker, PCA, fallback, titulos, projecoes, NoteVector, Matryoshka. |
| 2026-05-09 | Multiplos Provedores Ollama, Visibilidade Mapa, slog. |
| 2026-05-07 | Query Point no mapa. |
| 2026-05-06 | Busca exata, Tabelas TipTap, DocsPanel. |
| 2026-05-05 | WeightsSettings, CodeMirror→TipTap. |
| 2026-05-04 | SPA fallback, Circuit breaker, BBolt testes, state.go fatiado. |
