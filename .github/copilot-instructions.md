# Instruções para Agentes de IA — TON-618

## Documentação Obrigatória

Antes de modificar qualquer arquivo, **leia o `DECISIONS.md`** na raiz do projeto. Ele contém:

- Stack principal (Go + chi + SQLite + templ + esbuild)
- Padrões de código (Go, Frontend JS, Testes)
- Decisões arquiteturais sobre embeddings semânticos
- Estrutura do banco de dados e migrações
- Regras da API
- Itens pendentes e observações técnicas

## Regras Gerais

1. **Nunca editar `web/static/` diretamente.** Os arquivos em `static/` são gerados pelo esbuild a partir de `web/src/`. Edite sempre o fonte em `src/` e depois rode `npm run build`.
2. **JSDoc apenas em APIs públicas** (`window.*` ou exports de módulo). Funções internas usam comentários simples.
3. **Testes Go** usam banco real (`newTestStore(t)`), sem mocks. Testes JS usam `node --test` (arquivos `.mjs`).
4. **sqlc** gera código em `internal/core/db/generated/`. Queries vão em `internal/core/db/query.sql`. Rode `sqlc generate` após alterar queries. Queries em tabelas virtuais `note_embeddings` (vec0) não podem ser migradas para sqlc — mantê-las como SQL cru.
5. **CUID2** é implementado localmente em `processor/cuid2.go`.
6. **Mutex para escrita**: `WriteMu sync.Mutex` no `Store` serializa escritas no banco.
