# ADR 0001: Stack Principal e Escolha de Tecnologias

* **Status:** Aceito
* **Data:** 2026-09-18

## Contexto e Problema

O TON-618 é um sistema de gerenciamento de conhecimento pessoal e notas em Markdown que exige alta performance na busca, baixa latência de renderização, suporte a busca por texto completo (FTS5) e busca semântica, operando como um executável binário único sem dependências externas complexas.

## Decisão

Adotar uma stack baseada em:

1. **Backend em Go (1.22+ com Chi Router):**
   - Garante binário único de fácil implantação.
   - Excelente performance e suporte nativo à concorrência.

2. **SQLite via `modernc.org/sqlite` sem CGo:**
   - Permite compilação cruzada pura em Go sem necessidade de toolchain C.
   - Modo WAL ativado para alta concorrência em leitura/escrita.

3. **Arquitetura de Front-end com Templ, HTMX e Tailwind CSS:**
   - Interface dinâmica com baixa pegada de JS client-side.
   - Tipagem forte de componentes HTML compilados em Go (`a-h/templ`).

## Consequências

* **Positivas:**
  - Facilidade de implantação e distribuição.
  - Baixa latência de inicialização e resposta de requisições.
  - Segurança de tipos ponta a ponta na renderização de templates.
* **Pontos de Atenção:**
  - Necessidade de rodar `templ generate` ao modificar arquivos `.templ`.
