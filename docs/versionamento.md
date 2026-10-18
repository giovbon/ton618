# Versionamento e Releases — TON-618

Este documento descreve como a versão do projeto é calculada e como publicar
uma release. A referência é o job `bump-version` em
[`.github/workflows/deploy_core.yml`](../.github/workflows/deploy_core.yml).

## Fonte de verdade: as tags git

O projeto **não guarda a versão em nenhum arquivo** (não há `VERSION`, nem
constante no código Go). A versão oficial é sempre a **última tag git** no
formato `vX.Y.Z` (`git describe --tags --abbrev=0`). A tag também é o nome da
imagem publicada no Docker Hub (`giovbon/ton618_core:vX.Y.Z`).

## SemVer a partir dos commits (Conventional Commits)

A cada push na `main`, o workflow lê as mensagens de commit **desde a última
tag** e decide o tipo de bump:

| O que existe nos commits desde a última tag | Bump | Exemplo |
|---|---|---|
| `BREAKING CHANGE` no corpo, ou `tipo!:` no assunto (`feat!:` / `fix(api)!:`) | **MAJOR** | `v3.4.0` → `v4.0.0` |
| Assunto começando com `feat:` | **MINOR** | `v3.4.0` → `v3.5.0` |
| `fix:`, `refactor:`, `perf:` e demais (exceto os "sem release" abaixo) | **PATCH** | `v3.4.0` → `v3.4.1` |
| **Só** `chore:`, `docs:`, `ci:`, `test:`, `style:`, `build:` ou bumps de dependência (`Bump …`) | **nenhum** | não cria tag; só republica `:latest` |
| Nenhum commit desde a última tag | **nenhum** | não cria tag; só republica `:latest` |

A precedência é MAJOR > MINOR > PATCH: basta **um** commit quebrável para subir
o major; na ausência dele, basta **um** `feat:` para subir o minor.

### Quando NÃO há release

Se **todos** os commits desde a última tag forem "não-releasable" — ou seja,
`chore`/`docs`/`ci`/`test`/`style`/`build` ou bumps de dependência
(`Bump …`), ou não houver commit — **nenhuma tag é criada**. O pipeline ainda
roda e republica a imagem apenas como **`:latest`** (sem `vX.Y.Z`), porque o
rebuild é útil (ex.: dependências novas) mas não configura uma versão nova.

Por isso os merges do Dependabot **não geram versão** a cada PR: eles só
atualizam a `:latest`. Uma tag `vX.Y.Z` só nasce quando entra um `feat:`,
`fix:` (ou `refactor:`/`perf:`) ou um `BREAKING CHANGE`.

> Não há mais o antigo "`+1` no patch a cada 30 commits". O tipo é sempre
> derivado do conteúdo dos commits.

## Como escrever os commits

Como o bump depende do texto do commit, siga o padrão:

```
feat: adiciona exportação em PDF
fix: corrige login com cookie expirado
feat(api)!: remove endpoint legado de busca semântica

BREAKING CHANGE: o endpoint /api/search/ask foi removido.
```

- `feat:` → sobe **minor**
- `fix:` (e demais tipos) → sobe **patch**
- `!` ou `BREAKING CHANGE:` → sobe **major**

## Outras formas de versionar

O job `bump-version` também aceita, nesta ordem de precedência:

1. **Tag manual via `workflow_dispatch`** — informe o campo `tag` (ex.:
   `v3.5.0`). Nada é calculado; a tag informada é usada.
2. **Push de uma tag git** (ex.: `git push origin v3.5.0`) — a tag do evento é
   usada como está.

Nesses dois casos o tipo de bump aparece como `manual` no resumo do workflow.

## Fluxo do pipeline

```
push na main / tag
      │
      ▼
┌───────────┐   ┌──────────────┐   ┌──────────────────────┐
│   test    │──▶│ bump-version │──▶│        deploy        │
│ Go + Web  │   │ cria a tag   │   │ build & push Docker   │
└───────────┘   └──────────────┘   └──────────────────────┘
```

O `deploy` só roda se o `test` passar — nenhuma tag é criada para código
quebrado.

## Publicando uma release manualmente

Para cortar uma versão específica fora do fluxo de detecção automática:

```bash
git tag -a v3.4.0 -m "Release v3.4.0"
git push origin v3.4.0

# ou, já com uma tag existente publicada pela Action:
git push origin --tags
```

## Histórico

- **v3.4.0** — marco inicial do versionamento SemVer guiado por Conventional
  Commits. Substitui o esquema anterior (`v2.0.x`), que incrementava o patch a
  cada push e o minor a cada 30 patches, sem considerar o conteúdo dos commits.
