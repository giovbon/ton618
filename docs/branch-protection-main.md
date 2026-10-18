# Proteção da branch `main` (PR Checks + deploy)

A proteção de branch é uma **configuração do GitHub** (Settings do repositório), **não** é aplicada pelo código deste repositório. Este doc instrui como exigir que o workflow **PR Checks** (`Test (Go + Web)`) passe **antes** do merge — o mesmo check que dá segurança ao **auto-merge do Dependabot** (que é configurado na UI: Settings → Dependabot → Automations).

## O status check a exigir

- Workflow: `.github/workflows/pr_checks.yml` (evento `pull_request`)
- Job (nome do check): **`Test (Go + Web)`**

Ele roda `go test ./...`, `gofmt`, `go vet`, `templ generate`, `typecheck` e os testes JS em **todo** PR (inclusive os do Dependabot).

## Aplicar na UI (a forma mais direta)

Repositório → **Settings** → **Branches** → **Add branch protection rule** (ou editar a regra da `main`):

1. **Branch name pattern:** `main`
2. **Require status checks to pass before merging** → **Select checks** → marque **`Test (Go + Web)`**
   - (Opcional) **Require branches to be up to date before merging** — garante que o merge só ocorra com a branch atualizada.
3. **Require pull request reviews before merging**: ⚠️ veja o caveat abaixo antes de marcar.
   - Nota: **não existe um checkbox "Prevent self-approval"** — impedir que o autor aprove o próprio PR é **comportamento padrão** do GitHub (não há o que ativar).
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

## ⚠️ Auto-merge do Dependabot (NÃO é via `dependabot.yml`)

> As chaves `auto_merge` / `pull_request_review_rule` **não existem** no schema do `dependabot.yml` — usá-las quebra a validação do Dependabot. O auto-merge nativo é **configurado na UI do GitHub**:

**GitHub → Settings → General → Pull Requests → "Allow auto-merge":** habilitar (condição para qualquer auto-merge).

Depois, escolha UMA das formas de ativar o auto-merge nos PRs do Dependabot:

**(a) Workflow versionado (recomendado)** — `.github/workflows/dependabot_auto_merge.yml`: detecta PR do Dependabot e roda `gh pr merge --auto --merge` para minors+patch, esperando os checks passar. É versionável e não depende de aba da UI.

**(b) Nativo/Dependabot (se disponível)** — a aba/opção nativa, quando existe, está sob as configurações do Dependabot; a localização variou entre versões do GitHub e não é estável. O workflow em (a) é o caminho confiável e documentado.

Em ambos: os PRs qualificados só mergeiam **depois que o "PR Checks" (`Test (Go + Web)`) passa** — desde que a proteção da `main` **não** exija revisão obrigatória que bloqueie o merge. Para restringir a `main`, a proteção de branch é o mecanismo.

Interação com a proteção da branch:
- Se você **exigir revisão obrigatória** na `main`, o auto-merge **não** ignora: até minors precisariam de aprovação humana (perde-se o "auto").
- Decisão recomendada por perfil:
  - **Agilidade + auto-merge:** exigir **só o status check** `Test (Go + Web)`, sem revisão obrigatória. Minors/patch auto-mergeiam; majors exigem review (via `reviewers` do Dependabot).
  - **Mais vigilância:** exigir **1 revisão** obrigatória — aí TODA mudança (inclusive minors) pede aprovação antes de mergear.

## Onde o `main` recebe mudanças

- PRs (com o `PR Checks` exigido) → merge em `main` → dispara `.github/workflows/deploy_core.yml`:
  - job `test` → `bump-version` (auto-tag) → `deploy` (build & push Docker).
- O workflow `pr_checks.yml` cobre o **PR antes do merge**; o `deploy_core.yml` cobre o **pós-merge** (com o mesmo conjunto de testes).
