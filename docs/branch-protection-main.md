# Proteção da branch `main` (PR Checks + deploy)

A proteção de branch é uma **configuração do GitHub** (Settings do repositório), **não** é aplicada pelo código deste repositório. Este doc instrui como exigir que o workflow **PR Checks** passe **antes** do merge — o mesmo check que dá segurança ao **auto-merge do Dependabot** (`.github/dependabot.yml`).

## O status check a exigir

- Workflow: `.github/workflows/pr_checks.yml` (evento `pull_request`)
- Job (nome do check): **`Test (Go + Web)`**

Ele roda `go test ./...`, `gofmt`, `go vet`, `templ generate`, `typecheck` e os testes JS em **todo** PR (inclusive os do Dependabot).

## Aplicar na UI (a forma mais direta)

Repositório → **Settings** → **Branches** → **Add branch protection rule** (ou editar a regra da `main`):

1. **Branch name pattern:** `main`
2. **Require status checks to pass before merging** → **Select checks** → marque **`Test (Go + Web)`**
   - (Opcional) **Require branches to be up to date before merging** — garante que o merge só ocorra com a branch atualizada.
3. **Prevent self-approval** / **Require pull request reviews before merging**: ⚠️ veja o caveat abaixo antes de marcar.
4. **Require linear history** (opcional, para histórico limpo).
5. Salve.

## Aplicar via `gh` (programático)

Requer a API de branch protection. Substitua `OWNER/REPO` (ex.: `giovbon/ton618_core`) e o owner real:

```bash
gh api -X PUT repos/OWNER/REPO/branches/main/protection \
  -H "Accept: application/vnd.github+json" \
  --input - <<'JSON'
{
  "required_status_checks": {
    "strict": true,
    "contexts": ["Test (Go + Web)"]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": {
    "required_approving_review_count": 0
  },
  "restrictions": null
}
JSON
```

O exemplo acima exige o check **sem** revisão obrigatória (count `0`) — ideal para o auto-merge de minors/patch do Dependabot.

## ⚠️ Interação com o auto-merge do Dependabot

- O auto-merge (`.github/dependabot.yml`) só mergeia quando **os checks exigidos passam**.
- Se você **exigir revisão obrigatória** na proteção da `main`, o auto-merge **não** ignora: até minors precisariam de aprovação humana (perde-se o "auto").
- Decisão recomendada por perfil:
  - **Agilidade + auto-merge:** exigir **só o status check** `Test (Go + Web)`, sem revisão obrigatória da branch. (Majors já caem na regra de revisão do Dependabot via `pull_request_review_rule`.)
  - **Mais vigilância:** exigir tambem **1 revisão** obrigatória — aí TODA mudança (inclusive minors) pede aprovação antes de mergear.

## Onde o `main` recebe mudanças

- PRs (com o `PR Checks` exigido) → merge em `main` → dispara `.github/workflows/deploy_core.yml`:
  - job `test` → `bump-version` (auto-tag) → `deploy` (build & push Docker).
- O workflow `pr_checks.yml` cobre o **PR antes do merge**; o `deploy_core.yml` cobre o **pós-merge** (com o mesmo conjunto de testes).
