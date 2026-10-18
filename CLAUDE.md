# Instruções para Agentes de IA — TON-618

- Documentação Obrigatória: Antes de modificar qualquer arquivo, **leia o `/docs/DECISIONS.md`**
- SEMPRE que corrigir algum bug existente importante para o funcionamento estável da aplicação, crie testes para evitar regressões.
- A aplicação é desenvolvida, quando no windows, com base no linux, geralmente ubuntu, via recurso de wsl... sempre dê prederência a esse ambiente.
- O projeto usa atualmente 3 tipos de subagentes do opencode: @planner, @bugfixer ou @feature-builder.