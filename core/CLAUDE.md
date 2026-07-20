# TON-618 — Regras para Agentes de IA

## Leitura Obrigatória

Antes de modificar qualquer arquivo, **leia o `DECISIONS.md`** na raiz do projeto.

## Regras Gerais

1. **Nunca editar `web/static/` diretamente.** Edite sempre em `web/src/` e depois rode `npm run build`.
2. **JSDoc apenas em APIs públicas** (`window.*` ou exports de módulo). Funções internas usam comentários simples.
3. **Testes Go** usam banco real (`newTestStore(t)`), sem mocks. Testes JS usam `node --test` (arquivos `.mjs`).
4. **sqlc** gera código em `internal/core/db/generated/`. Queries vão em `internal/core/db/query.sql`. Rode `sqlc generate` após alterar queries. Tabelas virtuais `note_embeddings` (vec0) ficam como SQL cru.
5. **CUID2** implementado localmente em `processor/cuid2.go`.
6. **Mutex para escrita**: `WriteMu sync.Mutex` no `Store` serializa escritas no banco.
7. **DECISIONS.md** contém stack, padrões, decisões arquiteturais e pendências — consulte sempre.
