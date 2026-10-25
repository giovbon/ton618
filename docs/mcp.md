# Acesso de IA às notas via MCP (somente leitura)

O TON-618 tem um servidor [MCP](https://modelcontextprotocol.io/) embutido que expõe o acervo
para qualquer cliente de IA local — OpenCode, Claude Desktop, Cursor, Zed, Continue etc. Ele fala
**stdio** (o cliente sobe o processo) e abre o SQLite em **modo somente leitura**: não existe
ferramenta de escrita, rename ou delete.

## Pré-requisitos

- O banco precisa existir: rode o app pelo menos uma vez (`./run.sh`) para criar
  `core/data/ton618.db`.
- Compile o binário se ainda não existir:

  ```sh
  cd core
  go build -tags sqlite_fts5 -o ton618 ./cmd/server
  ```

## Subcomando

```sh
./core/ton618 mcp [--db CAMINHO] [--docs DIR]
```

| flag | padrão |
|---|---|
| `--db` | `DB_PATH`, depois `./data/ton618.db`, depois `<binário>/data/ton618.db` |
| `--docs` | `DOCS_DIR`, depois `<binário>/docs`, depois `./docs` (só contexto; leitura vai no banco) |

O processo **não** migra o schema, não faz backup e não indexa nada. `stdout` é exclusivo do
protocolo JSON-RPC; logs vão para `stderr`.

## Ferramentas expostas

| ferramenta | para quê |
|---|---|
| `search_notes` | busca global (FTS5, sinônimos, acentos) com filtros `tags:` e `tipo:` |
| `get_note` | conteúdo completo de uma nota (com título, tipo, tags, mtime; pagina com `offset`/`max_chars`) |
| `list_notes` | metadados de todas as notas, filtrando por `tipo` e `tag` |
| `query_notes` | executa a [linguagem de consulta](linguagem-de-consulta.md) (`tipo:`, `texto:`, `onde:`, …) |
| `backlinks` | links de entrada e saída da nota |

Caminhos são normalizados: `minha-nota` vira `notes/minha-nota.md`.

## OpenCode (V2)

Adicione ao `opencode.jsonc`:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "ton618": {
        "type": "local",
        "command": [
          "/caminho/para/ton618/core/ton618",
          "mcp",
          "--db",
          "/caminho/para/ton618/core/data/ton618.db"
        ],
        "codemode": false
      }
    }
  }
}
```

Confira a conexão:

```sh
opencode mcp list
```

Sem `--db`, o servidor procura `./data/ton618.db` (diretório atual) e `<binário>/data/ton618.db`.
Se você inicia o OpenCode fora de `core/`, prefira o caminho absoluto.

## Outros clientes

Qualquer cliente MCP com transporte stdio serve. O comando é sempre:

```json
{
  "command": "/caminho/para/ton618",
  "args": ["mcp", "--db", "/caminho/para/core/data/ton618.db"]
}
```

O nome exato do bloco de configuração varia por cliente (Claude Desktop usa
`mcpServers`, por exemplo).

## Limites e segurança

- **Local por design.** Não há transporte HTTP nem autenticação: quem inicia o processo tem acesso
  ao banco. Para IA fora da máquina seria preciso um transporte remoto com token read-only,
  TLS/túnel e rate limit — não implementado.
- **Somente leitura de verdade.** `db.NewReaderStore` abre com `mode=ro`; qualquer escrita é
  rejeitada pelo SQLite, mesmo por bug.
- **FTS não indexa o corpo** de desenhos, mapas mentais e tabelas Tabulator (DECISIONS §14.2).
  Para esses tipos, ache a nota por título/tag com `list_notes`/`search_notes` e leia com
  `get_note`.
- O servidor lê o mesmo arquivo que o app. Com o app rodando, o WAL é lido normalmente; não é
  preciso parar o servidor HTTP.
