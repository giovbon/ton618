

**TON-618** é o seu sistema pessoal de gestão de conhecimento (PKM) — um "buraco negro" onde você armazena, organiza e busca suas ideias.

## Dependências usadas
- Tip Tap (editor de notas) - https://tiptap.dev/
- jspreadsheet (editor de planilhas) - https://jspreadsheet.com/
- Tabulator (editor de dados tabulares) - https://tabulator.info/
- Excalidraw (editor de desenho) - https://excalidraw.com/
- Typst (compilador e editor de documentos) - https://typst.app/
- Mermaid (editor e renderizador de diagramas) - https://mermaid.js.org/
- CodeJar (editor de código leve) - https://medv.io/codejar/
- vis-timeline (editor e renderizador de diagramas, usado em agenda) - https://visjs.github.io/vis-timeline/
- chrono v2 (Parser de datas, suporte parcial em PT-BR, usado em agenda) - https://github.com/wanasit/chrono
- JSDoc (Tipagem de variáveis no JS) - https://jsdoc.app/
- Markmap (Editor e renderizador de mapas mentais) - https://markmap.js.org/
- Leaflet (Editor e renderizador de mapas) - https://leafletjs.com/

Fórmulas possíveis: https://jspreadsheet.com/docs/formulas/functions


CSP (Content Security Policy - Política de Segurança de Conteúdo) está configurada no servidor backend da aplicação para evitar ataques XSS (Cross-Site Scripting). Por questões de segurança, o navegador é instruído a bloquear conexões (fetch) para qualquer domínio que não esteja explicitamente autorizado na lista de segurança. O que significa que não será possível acessar recursos (como mapas, imagens, etc.) de fontes externas não autorizadas.

## Funcionalidades do Sistema

### 🔍 Busca Global e Local
- **Busca Global**: Pesquisa textual rápida em todos os seus arquivos (Full Text Search - FTS5). O sistema aplica pesos inteligentes nos resultados: **Título** (10x), **Tags** (5x) e **Conteúdo** (1x) para garantir que os itens mais relevantes apareçam primeiro.
- **Listagem Compacta**: Modo de visualização limpo, perfeito para navegar rapidamente por uma lista densa de notas e acessar seus recursos por tipo (Mermaid, Planilha, Typst).

### 🎯 TODOs e Tabulator
- **TODOs Dinâmicos**: O sistema rastreia palavras-chave ativas (como `TODO`, `FIX`, `HACK`) em tempo real pelas suas notas e as exibe em um dashboard central. Você pode gerenciar cores e marcadores nas Configurações.
- **Tabulator**: Visão global de tudo que está armazenado no seu buraco negro de informações, de forma tabular e pesquisável, usando os frontmatter das notas.

### ⚙️ Opções Avançadas e Manutenção
- **Tagueamento e RAKE**: Notas não etiquetadas ou órfãs podem receber sugestões de tags automáticas baseadas em extração de palavras-chave do texto (RAKE).
- **Limpeza Inteligente**: O sistema oferece ferramentas nativas para varrer seu armazenamento e deletar imagens ou anexos órfãos, economizando espaço em disco.
- **Arquivamento e Backup**: Permite filtrar notas por idade e enviá-las para arquivo morto, além de oferecer opções de exportação de Backup rápido (apenas as notas textuais e dados estruturados) ou completo (incluindo PDFs pesados).
- **Stopwords**: Controle sobre as palavras a serem ignoradas nos processos de tagueamento automático e estatísticas.

### 🧠 Decaimento e Peso Sináptico (RLHF)
- O sistema simula uma **curva de esquecimento (Hermann Ebbinghaus)** baseada no tempo em que uma nota fica sem interação e nos seus votos (Favoritar/Depreciar).
- A tabela do banco de dados registra o **"Peso" (Weight / Synaptic Weight)** da nota, indicando a "gravidade" e a relevância real que aquela informação tem para você no longo prazo.
- Interações comuns (abrir a nota) dão pequenos ganhos no peso, enquanto votos explícitos (Favoritar) dão um bônus maior. Notas que aparecem nas buscas mas são sumariamente ignoradas sofrem leves penalidades de pontuação (`scroll_past_penalty`).
- Notas com pouco uso prolongado sofrem um "decaimento logarítmico" passados 30 dias de inatividade. Quando o peso sináptico cai para faixas muito baixas, o sistema as classifica como informações que estão se perdendo, e **acrescenta dinamicamente** as tags `fria` e `esquecida` na visualização dos resultados de pesquisa.

### 📅 Agenda
A biblioteca chrono v2 possui apenas suporte parcial para o português, ela não usa processamento de linguagem natural complexo ou IA para entender o contexto. Em vez disso, ela depende de padrões fixos baseados em expressões regulares (RegEx).
- O que funciona bem:
    - Exemplos: "15 de Janeiro", "20 de Março de 2026", "10 Out".
    - Exemplos: "Segunda", "Terça-feira", "Sexta" ou "Sábado".
    - Exemplos: "Hoje", "Amanhã", "Ontem".
    - Exemplos: "Hoje às 14:30", "Amanhã às 15h", "Meio-dia", "Meia-noite".
    - Horas: 15h30 -> 15:30, 15h -> 15:00
    - Semanas: daqui a X semanas, daqui a uma semana, semana que vem
    - Dias: daqui a X dias, daqui a um dia
    - Meses: daqui a X meses, daqui a um mês, mês que vem
    - Anos: daqui a X anos, daqui a um ano, ano que vem
    - Horas/Minutos Relativos: daqui a X horas, daqui a X minutos