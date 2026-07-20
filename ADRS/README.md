# Registro de Decisões Arquiteturais (ADR)

Este diretório contém o registro cronológico de decisões arquiteturais significativas.
Cada ADR segue o formato:

```
ADR-{NÚMERO}-{titulo-curto}.md
```

## Estrutura de cada ADR

```markdown
# ADR-{N}: Título

**Data:** YYYY-MM-DD
**Status:** Proposto | Aceito | Depreciado | Substituído

## Contexto
O problema que motivou a decisão.

## Decisão
O que foi decidido e por quê.

## Consequências
Impactos positivos e negativos.

## Alternativas Consideradas
Outras opções que foram avaliadas e descartadas.
```

## Índice

| ADR | Título | Status | Data |
|-----|--------|--------|------|
| [ADR-001](./ADR-001-monorepo-open-core.md) | Monorepo Open Core + Comercial | Aceito | 2026-07-20 |
| [ADR-002](./ADR-002-multi-tenant-sqlite.md) | Multi-Tenant com SQLite isolado | Aceito | 2026-07-20 |
| [ADR-003](./ADR-003-licenciamento-rsa.md) | Licenciamento via RSA-4096 | Aceito | 2026-07-20 |
