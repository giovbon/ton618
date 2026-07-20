# ADR-001: Monorepo Open Core + Comercial

**Data:** 2026-07-20
**Status:** Aceito

## Contexto

O projeto TON-618 começou como um app auto-hospedado. Para comercializá-lo, precisávamos de uma estrutura que:
- Mantivesse o código auto-hospedado publicamente disponível (Docker Hub)
- Permitisse adicionar features pagas sem vazar código comercial
- Facilitasse correções de bug que beneficiassem ambos os produtos

## Decisão

Adotar **monorepo único** com divisão em pastas:

```
ton618/
├── core/              ← Código aberto (auto-hospedado via Docker)
├── desktop/           ← App desktop (Wails) — fechado
└── pkg/commercial/    ← Licenciamento — módulo Go separado
```

Features comerciais são ativadas por **build tag** `commercial`:

```bash
go build -tags commercial -o ton618-pro ./core/cmd/server
```

O `Dockerfile` do core **nunca** usa a tag commercial — a imagem pública contém apenas o código aberto.

## Consequências

**Positivas:**
- ✅ Correções em `core/` beneficiam ambos os produtos automaticamente
- ✅ Não precisa gerenciar múltiplos repositórios
- ✅ Docker Hub recebe apenas o core (sem vazar código comercial)
- ✅ Um `git pull` atualiza tudo

**Negativas:**
- ⚠️ O binário comercial precisa de `go mod replace` para importar o core
- ⚠️ Build tag precisa ser lembrada ao compilar (mitigado pelo `Makefile`)

## Alternativas Consideradas

1. **Repositórios separados** — Rejeitado: correções precisam ser sincronizadas manualmente
2. **Submódulos git** — Rejeitado: complexidade desnecessária
3. **Branch separada** — Rejeitado: branches divergem com o tempo
