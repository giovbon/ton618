

**TON-618** é o seu sistema pessoal de gestão de conhecimento (PKM) — um "buraco negro" onde você armazena, organiza e busca suas ideias.

## Funcionalidades Principais

O sistema usa três modalidades complementares de pesquisa textual e semântica:

- **Busca de Notas**: Filtro instantâneo no menu focado exclusivamente no nome/título dos arquivos Markdown.
- **Busca Global**: Busca textual de termos no conteúdo interno de todas as notas do sistema.
- **Busca Semântica**: Pesquisa por aproximação conceitual e sentido (IA), lidando com sinônimos e contextos distantes.

## 📁 Tipos de Notas Suportados
No TON-618, suas notas podem ir muito além do texto tradicional. O sistema reconhece e adapta o editor automaticamente de acordo com as seguintes categorias:
- **📄 Nota de Texto (Markdown):** O formato padrão para anotações em prosa, com suporte a links, subtópicos e formatação rica.
- **📊 Planilha (Spreadsheet):** Uma planilha interativa para gerenciar dados em tabelas com suporte a fórmulas matemáticas.
- **🎨 Desenho (Drawing):** Um quadro visual interativo para desenhar esboços, diagramas de blocos e ideias graficamente.
- **📘 Documento Typst:** Um editor acadêmico avançado para compilar relatórios e artigos formatados diretamente em PDF.
- **🧜 Diagrama Mermaid:** Criação de fluxogramas, gráficos e diagramas a partir de código textual declarativo simples.
- **🔱 Mapa Mental (Markmap):** Transforma tópicos estruturados em marcadores de markdown em um mapa mental visual e expansível.
- **🗺️ Mapa Geográfico:** Permite visualizar pontos geográficos, marcar rotas e interagir com localizações geográficas integradas a notas.
- **📒 E-book (EPUB):** Um leitor dedicado de livros digitais em formato `.epub` com visualização de sumário e paginação adaptiva.
- **📕 Documento PDF:** Exibição e leitura direta de arquivos PDF.
- **📦 Anexo ZIP:** Gerenciamento e download de arquivos compactados e anexos salvos na base.

## 📥 Opções de Captura (Web & YouTube)
Você pode salvar conteúdos da internet diretamente na sua base de notas usando o recurso de **Captura**:
- **📰 Artigos da Web:** Insira a URL de qualquer post, notícia ou artigo. O TON-618 remove anúncios, menus e cabeçalhos desnecessários, salvando apenas o corpo principal do texto e imagens em formato Markdown perfeitamente legível.
- **🎥 Vídeos do YouTube:** Insira a URL de um vídeo do YouTube. O sistema recupera o título original e extrai automaticamente a **transcrição completa das legendas faladas** (em português ou inglês), permitindo pesquisar ou ler o conteúdo em texto do vídeo posteriormente.



### 🚧 Task

O sistema rastreia palavras-chave ativas (como `TODO`, `FIX`, `HACK`) em tempo real pelas suas notas e as exibe em um dashboard central. Você pode gerenciar cores e marcadores nas Configurações.

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

## ⚙️ Painel de Configurações
Acesse o painel de configurações clicando no ícone de engrenagem no cabeçalho do sistema. Ele é dividido em abas especializadas para gerenciar seu conhecimento:
- **🗂️ Arquivamento:** Permite arquivar notas em lote filtrando por idade (ex: notas intocadas há 2 anos) ou tags específicas. Também possui o botão **Limpar imagens órfãs** para apagar mídias do disco que não estão mais referenciadas em nenhuma nota.
- **📦 Restaurar:** Lista todos os pacotes de arquivos mortos (`.zip`) salvos anteriormente, permitindo trazê-los de volta à base ativa instantaneamente.
- **💾 Backup:** Oferece duas modalidades de download em formato `.zip`:
  * **Rápido (Apenas Notas):** Baixa um arquivo `.zip` contendo as notas no formato Markdown (`.md` | `.csv` | `.mmd` | `.excalidraw`).
  * **Completo:** Baixa um arquivo `.zip` com tudo: as notas textuais (`.md`) e os arquivos binários em disco (como PDFs e anexos).
- **🔖 Marcadores:** Permite gerenciar as palavras-chave (como `TODO`, `FIX`, `HACK`) ativas para o rastreamento dinâmico de tarefas.
- **📅 Agenda:** Configuração do fuso horário local e coordenadas utilizadas para o cálculo do nascer e pôr do sol nos painéis visuais.
- **🔔 Ntfy:** Permite cadastrar o servidor e tópicos do Ntfy para receber notificações de tarefas agendadas em outros dispositivos.
- **🧠 Semântica:** Opções de gerenciamento da inteligência artificial local:
  * **Modo de Execução:** Alterna entre processamento por CPU (WASM) ou GPU (WebGPU) no navegador.
  * **Sensibilidade da IA (Thresholds):** Sliders para calibrar a nota mínima de corte (similaridade) para a busca semântica global e para as notas recomendadas no rodapé.
  * **Resetar Índice:** Apaga todos os vetores de busca salvos e força a reindexação completa das notas físicas.

## 🗃️ Tabulator


A tela da tabela **Tabulator** no TON-618 (`/database`) possui um interpretador de buscas personalizado muito poderoso implementado em JavaScript. Ele permite desde buscas textuais simples até filtros altamente estruturados usando chaves e valores.

Aqui estão todas as possibilidades e sintaxes que você pode utilizar na caixa de busca do Tabulator:

* **Termos simples:** `git` (encontra qualquer linha que possua "git" em qualquer coluna).
* **Termos compostos (com espaço):** Use aspas simples ou duplas para buscar expressões exatas.
  * Exemplo: `"britpop oasis"` ou `'sam neill'`.
* **Múltiplos termos (Lógica AND):** Se você digitar mais de um termo solto, o sistema exige que a linha satisfaça **todos** eles.
  * Exemplo: `git docker` (encontra linhas que contêm "git" **e** "docker").


Você pode filtrar colunas específicas digitando `nome_da_coluna:valor`. O Tabulator traduz os seguintes aliases amigáveis para as colunas do banco:

* **Título:** `titulo:`, `título:`, `title:`
  * Exemplo: `titulo:Fundamentos` ou `titulo:"Código Limpo"`
* **Tipo da Nota:** `tipo:`, `type:` (filtrando por markdown, mindmap, drawing, spreadsheet, etc.)
  * Exemplo: `tipo:mindmap` ou `tipo:drawing`
* **Data de Modificação:** `data:`, `date:`, `mtime:`, `modificação:`, `modificacao:`
  * Exemplo: `data:2026-07` (encontra notas modificadas em julho de 2026).
* **Tags:** `tag:`, `tags:`
  * Exemplo: `tag:tecnologia`

Se as suas notas possuem propriedades personalizadas no cabeçalho YAML (frontmatter), como `autor`, `status` ou `prioridade`, o Tabulator mapeia essas propriedades em colunas e você pode filtrá-las diretamente!
* Exemplo: `autor:Gisbon`
* Exemplo: `prioridade:alta`
* Exemplo: `status:"em andamento"`

* **Lógica OR para Tags:** Ao filtrar por tags, você pode listar várias separadas por vírgula. O sistema trará notas que tenham **qualquer uma** das tags listadas (lógica OR).
  * Exemplo: `tag:tecnologia,programacao` (encontra notas com a tag `#tecnologia` **ou** `#programacao`).
* **Lógica AND entre Filtros:** Você pode combinar múltiplos filtros estruturados e termos livres na mesma busca. Todos os critérios devem ser satisfeitos simultaneamente.
  * Exemplo: `tipo:mindmap tag:docker "configuração de rede"` 
    *(Trará apenas mapas mentais que tenham a tag #docker e que contenham a frase "configuração de rede" em algum campo).*

### Peso Sináptico e Esquecimento

O TON-618 funciona de forma alinhada à sua memória natural. Ele monitora organicamente o valor de cada nota por meio do **Peso Sináptico**:
- **A Força da Informação (Peso):** Cada nota possui uma força ou relevância. Quanto mais você interage com ela, mais forte e importante ela se torna para o sistema nas pesquisas.
- **Interações Comuns (Abrir/Editar):** Dão um ganho de peso (a informação está fresca na sua mente).
- **Como a nota perde força (Ignorar em buscas):** Se uma nota aparece nos seus resultados de busca mas você passa direto por ela sem abrir, o sistema entende que ela não era relevante para o termo buscado e aplica uma leve penalidade.
- **A Curva de Esquecimento (Hermann Ebbinghaus):** Se você passar mais de 30 dias sem interagir com uma nota, ela sofre um decaimento natural e contínuo.
- **Alertas Dinâmicos (`fria` e `esquecida`):** Caso o peso de uma nota caia muito por falta de uso, o sistema adicionará de forma automática e dinâmica as etiquetas `#fria` ou `#esquecida` nos resultados de busca para te sinalizar que aquele conhecimento está se apagando.