# Ajuda — TON-618

Sistema de Gestão de Conhecimento (PKM) auto-hospedado. Este documento é o **help do sistema**: todo o conteúdo escrito aqui é renderizado como a página de ajuda (ícone **docs** no topo). Edite este arquivo para atualizar a documentação.

---

## Visão geral

O TON-618 guarda suas notas em Markdown (com frontmatter YAML), organiza hierarquias pai/filho, mantém uma agenda, lista de tarefas e um banco de dados pesquisável — tudo em um único binário, sem depender de serviços externos. A interface é renderizada no servidor (HTMX + Templ), com foco em funcionar **offline** e em máquinas de baixo recurso.

> **Atalho:** a maioria das ações tem um ícone no topo (barra de navegação). Passe o mouse sobre um ícone para ver a descrição.

---

## Notas

- **Criar:** use **`Criar → + Nota`** no topo. Uma nota nova nasce em `notes/`.
- **Editar:** clique no título na barra lateral ou na tabela. O editor é em Markdown (WYSIWYG) com suporte a tabelas, listas de tarefas (`[ ]`), blocos de código e imagens.
- **Frontmatter:** o bloco no topo (`--- ... ---`) pode conter metadados como `pai`, `tags`, `type`, `markmap`, e qualquer campo extra — campos do frontmatter podem virar **colunas** na tabela (veja Configurações → *Colunas*).
- **Hierarquia:** declare `pai: nome-da-nota` no frontmatter para aninhar. Notas podem ter **uma** mãe e **várias** filhas. A barra lateral e a tabela exibem essa árvore.

## Busca

- Use a busca na barra lateral (busca local) ou a **busca global**.
- A busca é **insensível a acento** e **insensível a maiúsculas/minúsculas**, com sufixo de prefixo (`corp*` casa `corporação`).
- **Sinônimos:** ao buscar **uma única palavra**, ela é expandida com os sinônimos cadastrados (Configurações → *Sinônimos*). Notas que contêm o termo exato aparecem **antes** das que só casam com sinônimo.
- **Frase exata:** use aspas — `"linguagem de programação"`. Frases entre aspas não expandem sinônimos (modo exato).
- **Proximidade:** use aspas simples — `'termos proximos'`.
- **Filtro por tag:** `tags:nome` ou `#tag`.
- **Multi-palavra:** termos soltos são unidos por **AND** (todas as palavras devem aparecer).

## Painel de consultas (linguagem de consulta)

Blocos `consulta` dentro de notas executam uma consulta dinâmica na própria nota. Exemplo:

```consulta
tipo: notas
tags: [livros]
onde: n.interacao == null || dias(n.interacao) > 90
ordenar: mtime desc
limite: 10
mostrar: lista
```

- Os resultados são renderizados **dentro da nota** e atualizados ao salvar.
- O motor é **somente-leitura** e determinístico.

## Tarefas (Task) e marcadores

- O editor reconhece marcadores customizados (TODO, DOING, DONE, etc.) e checkboxes `[ ]`.
- O badge no topo mostra a contagem de tarefas pendentes.
- Os marcadores podem ser ajustados em **Configurações → Marcadores**.

## Agenda

- Crie compromissos em **Agenda** (`/agenda`).
- Visualize por **timeline** ou **árvore**.
- Configure o **fuso horário** e o **lead time de notificação** (horas antes do evento) em Configurações → Agenda. Lembretes via **Ntfy** (Configurações → Ntfy).

## Tabela (Tabulator)

- Em **Tabulator** (`/database`), todas as notas/arquivos aparecem como uma tabela em árvore.
- **Colunas:** use o seletor **Colunas** (canto da tabela) para ocultar/mostrar. Em **Configurações → Colunas**, você define **quais campos do frontmatter podem virar coluna**.
- Clique em uma célula editável para alterar na hora (título, tags, tipo, pai, campos).
- **Duplo clique** em uma linha abre a nota.

## Desenhos e Mapas Mentais

- **+ Desenho** abre o editor Excalidraw (integrado).
- **+ Markmap** abre um mapa mental; o frontmatter `type: markmap` e a opção `markmap.maxWidth` controlam a renderização.

## Captura e importação

- **Upload PDF / EPUB / Anexo** importa arquivos físicos.
- **Web Capture** extrai o conteúdo de uma URL (Readability) e o salva como nota.
- **Transcrição de YouTube** via frontmatter de tipo adequado.

## Configurações (ícone engrenagem)

| Aba | O que controla |
|---|---|
| **Arquivamento** | Filtrar e arquivar/excluir notas em massa; **Auto-Tag** por inatividade |
| **Restaurar** | Restaurar arquivos mortos/arquivados |
| **Backup** | Backup rápido/completo e restauração |
| **Marcadores** | Marcadores de tarefas customizados |
| **Agenda** | Fuso horário e lead time de notificação |
| **Ntfy** | Servidor, tópico e credenciais de notificação push |
| **Colunas** | Quais campos do frontmatter viram coluna na tabela |
| **Sinônimos** | Expansão de sinônimos para a busca de 1 palavra |

## Backup e restauração

- **Backup rápido:** zera as notas/arquivos (sem o banco).
- **Backup total:** exporta o banco SQLite **e** os arquivos (com manifest + checksum).
- **Restauração:** leia o backup e confirme. O estado atual é movido para uma pasta de quarentena antes de restaurar.
- **Integridade:** o banco é **snapshotado antes de qualquer migration** (`<dir-do-banco>/backups/pre-migration.db`) — se uma migration falhar, você pode restaurar desse arquivo.

## Segurança

- Autenticação **Basic Auth** (senha). Defina `AUTH_PASS` no ambiente.
- Headers de segurança (CSP, X-Frame-Options, etc.) e **rate-limiting** em busca pesada.
- O motor de consulta é somente-leitura, sem caminhos de escrita.
