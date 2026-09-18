# ADR 0003: Templating Compilado com Templ e HTMX

* **Status:** Aceito
* **Data:** 2026-09-18

## Contexto e Problema

Substituir o modelo tradicional de Single Page Application (SPA) pesada em JavaScript por uma arquitetura mais simples e performática impulsionada pelo servidor (Server-Driven UI), mantendo alta reatividade na interface.

## Decisão

1. **`a-h/templ` para UI no Go:**
   - Compilar templates `.templ` diretamente em código Go tipo-seguro.
   - Detectar erros de sintaxe e tipos na compilação, ao invés de em tempo de execução.

2. **HTMX para Atualizações Parciais da DOM:**
   - Utilizar atributos `hx-get`, `hx-post`, `hx-target`, `hx-swap` para chamadas assíncronas.
   - Utilizar **Out-of-Band Swaps (`hx-swap-oob`)** para atualizar regiões independentes da tela (ex: contador de tarefas no cabeçalho) em uma única resposta HTTP.

## Consequências

* **Positivas:**
  - Redução massiva do pacote JS enviado ao cliente.
  - Renderização ultra-rápida no servidor com validações do compilador Go.
* **Pontos de Atenção:**
  - Exige manter a sincronia entre a contagem de badges e as ações de banco de dados nos handlers HTMX.
