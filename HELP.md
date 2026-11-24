# Ajuda — TON-618

O **TON-618** é um sistema pessoal de conhecimento que roda na sua máquina. Ele guarda notas,
desenhos, mapas mentais, planilhas, PDFs, EPUBs e anexos em um só lugar — e deixa
você procurar, cruzar e organizar tudo isso sem depender de internet ou de serviços de terceiros.

A ideia central é simples: **você escreve, o app indexa**. A partir daí, você pode buscar por
palavra, filtrar por tag e por tipo, navegar pela hierarquia de notas e organizar o
acervo com o Tabulator.

Esta ajuda foi escrita para quem **usa** o TON-618, não para quem o programa. Ela explica o que
cada recurso faz, quando usar e quais são os detalhes que costumam pegar as pessoas de surpresa.

O **índice** logo abaixo é gerado a partir dos títulos deste documento: clique em qualquer item
para ir direto à seção. Ele se atualiza sozinho — uma seção nova escrita aqui aparece no índice,
sem nenhuma lista paralela para manter.

---

## 1. Primeiros passos

### 1.1 Subir o app

No terminal, a partir da raiz do repositório:

```sh
cd core
./run.sh
```

O script gera os templates e o bundle web, compila o binário, cria as pastas de dados e sobe o
servidor em **http://localhost:6180**.

```sh
./run.sh status   # mostra se está rodando e em qual porta
./run.sh stop     # encerra a instância da porta
```

Se a porta estiver ocupada por uma instância antiga, o próprio `run.sh` avisa e tenta liberá-la.
Evite suspender o processo com `Ctrl+Z`: a porta fica presa e o servidor para de responder. Use
`Ctrl+C` para encerrar ou `./run.sh stop`.

### 1.2 Entrar

O login do TON-618 é feito **apenas por senha** — o usuário interno é sempre `ton618` e não precisa
ser digitado.

| Como você subiu | Senha padrão |
|---|---|
| `./run.sh` | `ton618` (ou o valor de `AUTH_PASS` no `.env`) |
| `docker compose` | `ton618_secret` (ou o valor de `AUTH_PASS` no ambiente) |

A sessão fica guardada no navegador (cookie `ton_auth` + `localStorage`). Para sair, limpe os dados
do site ou use uma janela anônima. Para trocar a senha, defina `AUTH_PASS` antes de subir:

```sh
AUTH_PASS="minha-senha-forte" ./run.sh
```

> **Esqueceu a senha?** Ela não fica gravada no banco: é lida da variável `AUTH_PASS` a cada
> inicialização. Confira o `.env`, o `docker-compose.yml` ou o comando usado para subir.

### 1.3 Onde ficam os dados

Esta é a diferença que mais confunde no começo: **as notas não são arquivos `.md` soltos no disco**.
O conteúdo markdown de cada nota vive na tabela `notes` do SQLite. O que fica em pastas são os
arquivos binários e os pacotes.

| Caminho | O que guarda |
|---|---|
| `core/data/ton618.db` | Banco SQLite: notas, índice de busca, tags, links, configurações |
| `core/docs/notes/` | Reservado para arquivos de nota legados; o conteúdo atual vive no banco |
| `core/docs/pdfs/` | PDFs enviados |
| `core/docs/epubs/` | EPUBs enviados |
| `core/docs/attachments/` | Anexos enviados (empacotados em ZIP) |
| `core/docs/images/` | Imagens coladas ou enviadas pelo editor |
| `core/docs/datasets/` | Arquivos anexados a células de notas Tabulator |
| `core/docs/archives/` | Pacotes de notas arquivadas (arquivo morto) |
| `core/data/backups/` | Cópia automática do banco antes de uma migração (`pre-migration.db`) |
| `core/data/.pre-restore-<data>/` | Estado anterior posto em quarentena por uma restauração de backup |

O banco usa WAL (`ton618.db-wal` e `ton618.db-shm` ao lado). Não apague esses arquivos com o app
rodando. Para copiar o banco com segurança, use o backup dentro do app ou `sqlite3 .backup`
(veja a seção 7.1).

> A pasta `docs/` na **raiz do repositório** é a documentação técnica do projeto. O acervo do app
> usa `core/docs/` e o banco. Não misture as duas.

---


## 2. A tela inicial

A página inicial tem duas ferramentas de busca no topo e a lista de notas logo abaixo.

### 2.1 As duas buscas

| Botão | Nome | O que faz |
|---|---|---|
| Lupa azul | **Busca de Notas** | Filtra pelo **nome do arquivo**. É instantânea e não olha o conteúdo. |
| Lupa verde | **Busca Global** | Procura no **conteúdo** de todas as notas, com trechos destacados e agrupamento por nota. |

A Busca de Notas ignora maiúsculas/minúsculas e faz correspondência parcial: digitar `proj` encontra
`projeto-ton618.md`. Os resultados carregam em páginas de 50, com rolagem infinita.

A Busca Global é a mais poderosa. Ela entende frases entre aspas, filtros de tag e filtros de tipo —
a sintaxe completa está na seção 5.2.

### 2.2 A lista de notas

Abaixo da busca aparece a lista de notas, ordenada da mais recente para a mais antiga. Cada linha
mostra:

- o **ícone do tipo** (nota, desenho, mapa mental, Tabulator, PDF, EPUB, anexo, captura etc.);
- o **nome de exibição** (sem a extensão e sem o prefixo `notes/`);
- as **tags** visíveis (as tags internas de tipo são ocultadas);
- um **contador de filhas** quando a nota é mãe de outras;
- ações de **renomear** (lápis) e **excluir** (lixeira).

Clicar no nome abre a nota no editor adequado ao tipo.

---


## 3. Conceitos que aparecem em todo lugar

### 3.1 Nota

Uma nota é identificada pelo **nome do arquivo**, sempre dentro de `notes/` e com extensão `.md`.
O nome é a identidade: renomear muda a identidade. O app faz a maior parte do trabalho sozinho —
atualiza o `title` do frontmatter, o `pai:` das filhas e os wikilinks/URLs das notas que apontavam
para ela. O que ele não consegue adivinhar são menções em texto corrido (o nome escrito “solto”, sem
`[[ ]]`); essas você ajusta à mão, se importarem.

O **título exibido** é o nome do arquivo sem `notes/` e sem `.md`. O frontmatter pode ter um campo
`title`/`titulo`, que é atualizado automaticamente no rename, mas quem manda na lista é o nome.

### 3.2 Frontmatter

O frontmatter é o bloco YAML no topo da nota, entre duas linhas `---`. Exemplo:

```markdown
---
title: Minha nota
tags: [projetos, ton618]
tipo: nota
pai: Projetos
---
```

O app grava e normaliza alguns campos:

| Campo | Para que serve |
|---|---|
| `title` / `titulo` | Título amigável; atualizado no rename |
| `tags` | Lista de tags (aceita `[a, b]` ou lista em bloco) |
| `tipo` / `type` | Tipo canônico da nota (`nota`, `desenho`, `mapa_mental`, `tabulator`, `calculo`, `pdf`, `anexo`, `artigo`, `captura`) |
| `pai` / `parent` | Nota-mãe na hierarquia (`parent` é aceito por compatibilidade, mas o app grava `pai`) |

Campos personalizados que você inventar (`autor:`, `status:`, `fonte:`) também são indexados e
podem virar **colunas do Acervo** (seção 5.3).

> **Cuidado com YAML:** `pai: [[Projetos]]` sem aspas vira uma lista aninhada em YAML. O app tolera,
> mas o formato recomendado é `pai: Projetos` ou `pai: "[[Projetos]]"`.

### 3.3 Tags

Há duas formas de dar uma tag a uma nota:

1. **No frontmatter:** `tags: [livros, filosofia]`.
2. **No corpo:** escrevendo `#livros` em qualquer lugar do texto.

As duas entram no índice e aparecem como chips na lista. A comparação ignora maiúsculas e acentos
na busca (`tags:saude` encontra `saúde`). Tags internas de editor (`drawing`, `markmap`, `mindmap`,
`tabulator`) não aparecem para o usuário.

Tags são a forma mais rápida de recortar o acervo: use poucas e consistentes. Uma tag nova só para
cada nota não ajuda em nada.

### 3.4 Hierarquia: `pai` e filhas

A hierarquia é declarada no frontmatter, com `pai:`. Ela não é uma pasta: é um relacionamento entre
notas.

```yaml
pai: Projeto TON-618
```

Formas aceitas na leitura:

| Forma | Exemplo |
|---|---|
| Nome simples | `pai: Projetos` |
| Com caminho | `pai: notes/projetos.md` |
| Wikilink | `pai: "[[Projetos]]"` |
| Wikilink com alias | `pai: "[[Projetos\|Projetos pessoais]]"` |
| Com âncora | `pai: "[[Projetos#2026]]"` |

No editor, a barra **NOTA-MÃE** permite escolher a mãe por busca e remover o vínculo. O botão de
**filhas** mostra quantas e quais notas apontam para a atual. Renomear uma nota atualiza o `pai:`
das filhas automaticamente.

PDFs, EPUBs e anexos também podem ter mãe: como não têm frontmatter, o vínculo fica na tabela
`file_metadata`.

### 3.5 Wikilinks e backlinks

Um **wikilink** liga uma nota a outra pelo nome:

```markdown
Veja [[Projetos]] e também [[Projetos|os projetos]].
```

No editor, digite `[[` e o menu de sugestões aparece. Use as setas para navegar e `Enter`/`Tab`
para inserir. Os links são navegáveis: clicar abre a nota de destino; um link para EPUB abre o
leitor, e um link para PDF ou anexo baixa o arquivo.

O **backlink** é o caminho inverso: quem aponta para esta nota. O botão de backlinks no topo do
editor mostra a lista, e a busca global também usa a contagem de citações no ranking.

### 3.6 Tipos de nota

| Tipo (`tipo:`) | Como nasce | Onde edita | Corpo no índice de busca? |
|---|---|---|---|
| `nota` | Criar → + Nota | Editor de texto | Sim |
| `desenho` | Criar → + Desenho | Excalidraw | Não (só metadados) |
| `mapa_mental` | Criar → + Markmap | Markmap | Não (só metadados) |
| `tabulator` | Criar → + Tabulator | Planilha | Sim, linha a linha |
| `calculo` | Criar → + Cálculo | Editor de texto (bloco `calc`) | Sim |
| `pdf` | Captura → Upload PDF | Leitor do navegador | Só metadados |
| `epub` | Captura → Upload EPUB | Leitor embutido | Só metadados |
| `anexo` | Captura → Upload Anexo | Download | Lista de arquivos do ZIP |
| `imagem` | Editor (colar/enviar) | Visualização | Não |
| `artigo` / `captura` | Captura → Web Capture | Editor de texto | Sim |
| `arquivo` | Arquivamento | Pacote em `archives/` | Não |

O tipo pode ser declarado no frontmatter (`tipo: desenho`) ou herdado do caminho (PDF, EPUB, anexo).
A detecção também reconhece formatos legados no conteúdo, mas o caminho recomendado é deixar o app
gravar o `tipo:`.

> **Importante:** o índice de busca não lê o corpo de desenhos e mapas mentais. Para encontrá-los,
> busque pelo título ou pela tag e abra a nota. Já o Tabulator é indexado célula a célula.

### 3.7 O acervo e suas raízes

O TON-618 chama de **raiz** o primeiro segmento do caminho. As raízes conhecidas são:

| Raiz | Conteúdo |
|---|---|
| `notes/` | Notas markdown (conteúdo no banco) |
| `pdfs/` | PDFs |
| `epubs/` | EPUBs |
| `attachments/` | Anexos ZIP |
| `images/` | Imagens do editor |
| `archives/` | Arquivos mortos |
| `datasets/` | Arquivos de células Tabulator (não entram na varredura) |

Essas raízes aparecem na busca global (`tipo:`) e na coluna **Pasta** do Acervo.


---

## 4. Uso diário

### 4.1 Criar uma nota

Clique em **Criar → + Nota**. O editor abre uma nota em branco com um nome gerado automaticamente.
O primeiro passo é dar um nome: edite o campo do título no topo e pressione `Enter`. O nome vira o
identificador da nota.

Para criar os outros tipos, use o mesmo menu:

- **+ Cálculo** — nota com um bloco ` ```calc ` pronto para contas;
- **+ Desenho** — tela do Excalidraw;
- **+ Markmap** — mapa mental a partir de títulos markdown;
- **+ Tabulator** — planilha de dados.

### 4.2 Escrever e formatar

O editor é WYSIWYG (TipTap), mas grava markdown de verdade. Os principais recursos:

- **Menu `/`** — digite `/` no início de uma linha para inserir título, lista, checklist, citação,
  bloco de código, tabela, imagem, linha horizontal e outras estruturas.
- **Menu de seleção** — selecione um texto para ver negrito, itálico, sublinhado, tachado, destaque,
  código e link.
- **Tabelas** — ao clicar numa tabela, aparece um menu para adicionar/remover linhas e colunas,
  mesclar e dividir células.
- **Imagens** — `/` → Imagem envia o arquivo para `docs/images/` e insere a referência.
- **Blocos de código** — `Tab` dentro do bloco insere indentação; o app tenta detectar a linguagem.
- **Calculadora** — `/` → Calculadora insere um bloco de cálculo: escreva `var1 = 10` e, na
  linha de baixo, `var1 + var2` — o resultado aparece **à direita, na mesma linha**, e
  recalcula enquanto você digita (seção 4.12).
- **Sumário (TOC)** — o botão de sumário lista os títulos da nota. No painel do TOC, `Tab` muda o
  nível, apagar a linha remove o título e `Ctrl+clique` (ou `Cmd+clique`) pula até o trecho.
- **Frontmatter** — a seta ao lado do painel abre/fecha o YAML. O painel sugere tags existentes.

### 4.3 Salvamento

O editor salva sozinho **cerca de 2 segundos** depois que você para de digitar (desenhos e mapas
salvam em ~1,5 s). O indicador ao lado do título mostra o estado:

| Indicador | Significado |
|---|---|
| `—` | Nada foi alterado desde que a nota abriu |
| `●` (âmbar) | Há mudanças ainda não gravadas |
| `⟳` (azul) | Gravação em andamento |
| `✓` (verde) | Gravação concluída |

`Ctrl+S` (ou `Cmd+S`) força o salvamento imediato. O botão de salvar também existe ao lado do título.

### 4.4 Organizar

| Ação | Onde |
|---|---|
| **Renomear** | Campo de título no editor, ou lápis na lista lateral |
| **Duplicar** | Botão de duplicar no topo do editor (e no Tabulator) |
| **Excluir** | Lixeira na lista lateral, botão × no Acervo, ou exclusão no editor |
| **Definir mãe** | Barra **NOTA-MÃE** no editor |
| **Ver filhas** | Botão de filhas na barra de hierarquia |
| **Ver backlinks** | Botão de backlinks no topo do editor |

Renomear propaga o novo nome para o `pai:` das filhas e reescreve automaticamente, nas notas que
referenciavam a antiga, os **wikilinks** (`[[Antigo]]`), os **links markdown** e as **URLs** para o
arquivo (`.md`, `.pdf`, `.epub`, `.zip` etc.). O `title` do frontmatter também é atualizado.

O que **não** é reescrito são menções em texto corrido, sem link — se você escreveu o nome “solto”
no meio de uma frase, ele continua como estava.

Excluir é definitivo (não há lixeira de notas). Se quiser guardar sem manter na lista, use
**Arquivar** (seção 4.9).

### 4.5 Nota de cálculo

**Criar → + Cálculo** cria uma nota markdown comum já com a tag interna `calculo` e um bloco
` ```calc ` vazio — o mesmo bloco da calculadora (seção 4.12). É só digitar as expressões: o
resultado aparece à direita, recalculado em tempo real, e o arquivo guarda apenas as expressões.

Use-a para orçamentos, listas de compras, conversões ou qualquer conta recorrente que você queira
manter junto das demais notas. A nota abre no editor de texto normal e pode receber títulos, tags e
texto ao redor do bloco.

### 4.6 Desenho (Excalidraw)

**Criar → + Desenho** abre uma tela cheia do Excalidraw. O desenho é salvo no corpo da nota como
JSON do Excalidraw e ganha a tag interna `drawing`. Você pode desenhar, escrever, importar imagens
e usar a biblioteca de formas do Excalidraw.

O salvamento é automático. Como o corpo não entra no índice de busca, encontre o desenho pelo nome
ou pela tag.

Na barra do título, ao lado de **duplicar**, há dois downloads do desenho atual: o **.excalidraw**
(o JSON nativo, que pode ser aberto no próprio Excalidraw) e o **.png** (imagem na resolução
padrão do Excalidraw). Os dois usam o estado que está no editor, inclusive alterações ainda não
salvas.

### 4.7 Markmap (mapa mental)

**Criar → + Markmap** abre um editor de markdown à esquerda e o mapa à direita. Os títulos viram
nós:

```markdown
# Projeto


## Arquitetura

### Busca


## Diário
```

O mapa é gerado a partir dessa estrutura. Você pode configurar opções no frontmatter:

```yaml
markmap:
  maxWidth: 400
  colorFreezeLevel: 2
```

O zoom e a posição ficam guardados no navegador. Assim como o desenho, o corpo do mapa não entra
no índice de busca.

### 4.8 Tabulator (planilha)

**Criar → + Tabulator** cria uma nota-planilha. Ela é diferente da página **Acervo** (`/database`):
o Tabulator é para você **inserir e manter dados**; o Acervo é para **administrar as notas**.

Recursos:

- adicionar linhas e colunas;
- escolher o tipo da coluna no menu `⋮` do cabeçalho: texto, número, data, sim/não ou arquivo;
- em coluna do tipo arquivo, clicar na célula envia o arquivo para `docs/datasets/` e guarda o
  caminho na célula;
- exportar CSV, duplicar e excluir a tabela;
- o conteúdo é salvo automaticamente e **entra na busca global** (cada linha vira um trecho).

O Tabulator também aceita hierarquia (`pai:`) e aparece no Acervo como qualquer nota.

### 4.9 PDFs, EPUBs, anexos e imagens

Use **Captura** no topo:

| Opção | O que acontece |
|---|---|
| **Upload PDF** | Salva em `docs/pdfs/` e volta para a lista; ao abrir, usa o visualizador do navegador |
| **Upload EPUB** | Salva em `docs/epubs/` e volta para a lista; ao abrir, usa o leitor embutido |
| **Upload Anexo** | Empacota os arquivos em um ZIP em `docs/attachments/` |
| **Web Capture** | Captura um artigo ou a transcrição de um vídeo do YouTube |

O leitor de EPUB tem sumário lateral, navegação por capítulo, barra de progresso, download e
**memória de posição**: ele guarda onde você parou (no servidor e no navegador).

O anexo vira uma nota do tipo `anexo`: o ZIP fica em `attachments/` e a busca indexa o nome do
pacote e a **lista de arquivos** enviados dentro dele. Para abrir, baixe o ZIP.

### 4.10 Web Capture

**Captura → Web Capture** pede uma URL e cria uma nota com:

- o título da página ou do vídeo;
- a fonte (link);
- o conteúdo do artigo convertido para markdown, ou a transcrição do YouTube;
- a tag `captura`;
- a data da captura.

Artigos passam por um extrator de conteúdo (readability), que remove menus, anúncios e barras
laterais. Páginas com paywall, bloqueio de robô ou conteúdo muito curto podem falhar — nesse caso,
o app mostra o erro em vez de criar uma nota vazia.

### 4.11 Arquivar, restaurar e excluir em massa

Abra **Configurações → Arquivamento**:

1. informe uma ou mais tags separadas por vírgula;
2. clique em **Previsualizar Notas**;
3. marque/desmarque as notas encontradas;
4. escolha **Arquivar Selecionadas** ou **Excluir Selecionadas**.

**Arquivar** compacta as notas em um pacote na pasta `archives/` e as remove da lista ativa. O
pacote guarda os caminhos originais. Para voltar, abra **Configurações → Restaurar**, encontre o
pacote e restaure: as notas voltam aos caminhos de origem e reaparecem nas buscas.

**Excluir** remove de vez, sem passar por `archives/`. A confirmação deixa isso explícito.

O botão **Limpar imagens órfãs** remove de `docs/images/` as imagens que não são mais referenciadas
por nenhuma nota.

### 4.12 Calculadora de expressões

`/` → **Calculadora** (ou digite `/calc`) insere um bloco de cálculo — um bloco ` ```calc `
com uma expressão por linha. O resultado aparece **à direita, na mesma linha da expressão** e
é recalculado em tempo real enquanto você digita:

| Você escreve | Você vê à direita |
|---|---|
| `var1 = 10` | `= 10` |
| `var2 = 20` | `= 20` |
| `var1 + var2` | `= 30` |

#### O que você pode escrever

**Números**

| Exemplo | Observação |
|---|---|
| `123` | inteiro |
| `12,5` ou `12.5` | decimal — aceita **vírgula ou ponto** na entrada |
| `.5` | decimal sem zero à esquerda |
| `1e6`, `2E+9` | notação científica |

**Operadores** — `+` `-` `*` `/` `%` (resto/módulo) `^` (potência) e parênteses.

- precedência tradicional: `2 + 3 * 4 = 14`;
- `^` é associativo à direita: `2^3^2 = 512` (= `2^(3^2)`);
- unário mais frouxo que `^`: `-2^2 = -4`;
- `1 / 0` e `1 % 0` param com erro (não retornam Infinity).

**Variáveis** — `nome = expressão` define a variável e mostra o valor na linha. O escopo é o
**bloco**, avaliado de cima para baixo: quem redefine uma variável vale para as linhas
seguintes. Quem usa uma variável antes de defini-la recebe o erro na própria linha. Nomes
aceitam letras com acento (`área = 10`) e maiúsculas/minúsculas são diferentes
(`Total` ≠ `total`).

**Funções** — a maioria tem aridade fixa (chamar com a quantidade errada de valores é erro);
`med` aceita **quantos valores você quiser**:

| Função | Valores | O que faz |
|---|---|---|
| `sqrt(x)` | 1 | raiz quadrada |
| `cbrt(x)` | 1 | raiz cúbica (aceita negativos) |
| `abs(x)` | 1 | valor absoluto |
| `round(x)` | 1 | arredonda para o inteiro mais próximo |
| `floor(x)` | 1 | arredonda para baixo |
| `ceil(x)` | 1 | arredonda para cima |
| `min(a, b)` · `max(a, b)` | 2 | menor / maior valor |
| `pow(a, b)` | 2 | potência (mesmo que `a^b`) |
| `log(x)` | 1 | logaritmo de base 10 |
| `ln(x)` | 1 | logaritmo natural |
| `sin(x)` · `cos(x)` · `tan(x)` | 1 | trigonometria **em radianos** |
| `med(a, b, …)` | 1 ou mais | **média aritmética** de todos os valores dados |

`med` é a única função de aridade livre — passe de 1 a 64 valores, misturando números,
variáveis e expressões:

```calc
nota1 = 8
nota2 = 6
nota3 = 9
med(nota1, nota2, nota3)
```

**Constantes** — `pi` (3,14159…) e `e` (2,71828…). Não são variáveis: `pi = 3` é permitido e
passa a valer só para as linhas de baixo do bloco.

**Comentários e linhas vazias** — linha que começa com `#` é ignorada; linha vazia não mostra
nada. Serve para separar seções dentro do bloco:

```calc
# compras do mês
preco = 12,5
qtd = 4
preco * qtd
```

#### Formato de saída

- inteiro sai sem casas decimais (`sqrt(9)` → `= 3`);
- decimal sai com **vírgula** (pt-BR) e sem separador de milhar, até 12 casas
  (`1 / 8` → `= 0,125`);
- o texto do resultado **seleciona inteiro com um clique** (basta `Ctrl+C` para copiar);
- o resultado nunca é reconvertido em conta: ele é só exibição.

#### Erros

Aparecem em vermelho no fim da própria linha, em português e com a posição quando ajuda:

| Erro | Quando acontece |
|---|---|
| `divisão por zero` / `módulo por zero` | `5 / 0` |
| `"var3" não foi definida — defina antes com var3 = valor` | usar variável antes de definir |
| `função desconhecida "sqr" — disponíveis: sqrt, cbrt, …` | nome de função errado (a lista vem na mensagem) |
| `"sqrt" espera 1 valor, recebeu 2` | aridade errada nas funções fixas |
| `"med" espera ao menos 1 valor, recebeu 0` | `med()` sem valores |
| `resultado inválido (NaN) — confira raízes de negativos e logaritmos` | `sqrt(-4)`, `log(-1)` |
| `resultado fora do intervalo (infinito) — confira divisões e potências` | `ln(0)`, `pow(10, 400)` |
| `falta um operador antes de "2"` | expressão mal formada (`2 2`) |
| `expressão incompleta` / `esperado ")"` | parêntese aberto sem fechar |
| `caractere não permitido "$"` | símbolo fora da gramática (sem strings, sem `;`, sem atribuição composta) |

#### Limites e segurança

- Cálculo **local no navegador**, sem rede e sem `eval` — o motor é um interpretador de uma
  gramática fechada (sem laços, sem I/O, sem funções definidas pelo usuário).
- Limite por linha: 200 símbolos e 32 níveis de aninhamento; por bloco, 1000 linhas;
  no máximo 64 argumentos por chamada. Toda expressão termina — não há como travar o editor.
- **Resultados não são salvos** — a nota guarda só as expressões (o bloco vira ` ```calc `
  no markdown); ao reabrir, tudo é recalculado.


---

## 5. Busca

### 5.1 Busca de Notas (por nome)

Serve para achar uma nota quando você já sabe (ou quase sabe) o nome dela.

- correspondência parcial: `ton` encontra `ton618.md`;
- ignora maiúsculas/minúsculas;
- não olha o conteúdo;
- resultados em páginas de 50, com rolagem infinita.

### 5.2 Busca Global (por conteúdo)

A Busca Global usa o índice FTS5 do SQLite. Ela entende:

| Sintaxe | Efeito |
|---|---|
| `palavra` | Termo livre. Vários termos são combinados com **E** (todos precisam aparecer). |
| `"frase exata"` | Procura a expressão naquela ordem. |
| `'perto disto'` | Procura os termos na mesma seção/trecho (proximidade). |
| `tags:livros` | Só notas com a tag `livros`. Pode repetir para exigir várias. |
| `#livros` | Atalho de hashtag para a tag `livros`. |
| `tipo:pdf` | Só notas do tipo indicado. |
| `type:mapa_mental` | Mesma coisa; `type:` é alias. |
| `tipo:pdf tipo:epub` | Vários tipos viram **OU** (PDF ou EPUB). |

Detalhes que ajudam no dia a dia:

- **Acentos:** a busca normaliza acentos; `filosofia` encontra `filosófia` e vice-versa.
- **Palavras curtas:** termos de uma única letra não ajudam e são ignorados no destaque dos trechos;
  palavras com mais de 2 letras casam também por prefixo (`proj` encontra `projeto`).
- **Stopwords:** palavras como `de`, `a`, `o`, `com`, `para` são ignoradas.
- **Tags e tipos** podem ser combinados com texto livre: `tipo:artigo foco` procura “foco” apenas
  em artigos.
- **Não existe operador de exclusão:** `-palavra` não exclui nada — o hífen é tratado como separador
  e a palavra passa a ser **exigida** junto com as outras.

Os resultados são agrupados por nota. Cada cartão mostra até **5 trechos** e indica quantas
ocorrências existem além disso. O termo exato aparece em azul; variações por prefixo, em roxo.

A busca global tem limite de **30 consultas por minuto**. O campo dispara 300 ms depois que você
para de digitar.

### 5.3 O Acervo (`/database`)

O Acervo é a visão de administração: uma tabela com **todas** as notas e arquivos. Use o ícone de
acervo no topo ou a rota `/database`.

O que dá para fazer:

- **Ver e ordenar** por qualquer coluna; a coluna **Título** mostra o ícone do tipo, um ponto
  âmbar quando o `pai:` não existe (órfã) e o contador de filhas;
- **Editar células**: clique numa célula editável. Título renomeia a nota; Tags atualiza o
  frontmatter; Pai muda a hierarquia (e recarrega a árvore); campos personalizados gravam no
  frontmatter;
- **Excluir** uma linha pelo `×` (com confirmação);
- **Abrir** a nota com duplo clique na linha;
- **Colunas**: mostrar/ocultar colunas e reorganizar por arraste;
- **Expandir/recolher** a árvore inteira;
- **Filtrar hierarquia**: mostrar todas as notas ou só as que participam de alguma relação pai/filha.

A busca do Acervo é **local** (roda no navegador, sobre a tabela já carregada) e aceita:

| Filtro | Exemplo | Observação |
|---|---|---|
| `titulo:` | `titulo:projeto` | Correspondência parcial no título |
| `arquivo:` | `arquivo:notes/2026` | Caminho do arquivo |
| `tipo:` / `type:` | `tipo:pdf` | Normaliza aliases (`markmap` → `mapa_mental`) |
| `tags:` | `tags:livros,filosofia` | Qualquer uma das tags listadas |
| `mtime:` / `data:` | `mtime:2026-10` | Correspondência parcial na data de modificação |
| termo livre | `arquitetura` | Procura em qualquer campo visível |

Combine vários filtros na mesma consulta. Aspas funcionam para valores com espaço:
`titulo:"projeto ton618"`.

O botão ao lado da busca alterna entre dois modos:

- **Pai + irmãos** — mostra o resultado, o caminho de pais e as irmãs (dá contexto);
- **Só resultados** — mostra apenas as notas que casaram.


---

## 6. Backup, restauração e configurações

### 6.1 As abas de Configurações

| Aba | O que faz |
|---|---|
| **Arquivamento** | Filtra por tag, arquiva/exclui em massa e limpa imagens órfãs |
| **Restaurar** | Lista pacotes de `archives/` e devolve as notas aos caminhos originais |
| **Backup** | Baixa backup rápido, completo ou total; restaura um backup total |
| **Rotina** | Liga/desliga o ícone e a página da rotina diária |
| **Colunas** | Quais campos do frontmatter podem virar colunas no Acervo |

### 6.2 Os três backups

| Backup | Inclui | Pode restaurar? |
|---|---|---|
| **Rápido** | Notas (markdown, desenhos, Tabulator). Não inclui arquivos pesados nem `archives/` | Não (só download) |
| **Completo** | Notas + PDFs + anexos + imagens + demais arquivos (exceto `archives/`) | Não (só download) |
| **Total** | `manifest.json` + banco `db/ton618.db` + todo o `docs/` (inclusive `archives/`) | **Sim** |

O backup rápido e o completo são para levar o conteúdo para outro lugar. O **Total** é o único que
preserva configurações e o estado inteiro — e o único que o app consegue
importar de volta.

### 6.3 Restaurar um Backup Total

**Configurações → Backup → Restaurar um Backup Total**:

1. escolha o arquivo `.zip`;
2. confirme (a operação **substitui tudo**);
3. aguarde: o estado atual vai para uma quarentena em `core/data/.pre-restore-<data>/`;
4. o servidor **reinicia sozinho**; espere alguns segundos e recarregue a página.

Se o arquivo não tiver o `manifest.json` ou o banco esperado, a restauração é recusada — isso evita
aplicar um ZIP qualquer por engano.

### 6.4 Colunas

**Configurações → Colunas** controla quais campos do frontmatter aparecem como colunas no Acervo.
Desmarcar um campo o esconde da tabela (o dado continua na nota). A coluna **Título** é especial:
ela sustenta a árvore de hierarquia e por isso permanece visível.


---

## 7. Terminal

### 7.1 Comandos no terminal

Você pode consultar o banco diretamente com `sqlite3`. Isso é útil para scripts,
diagnóstico e buscas que você quer levar para outro lugar.

> **Somente leitura:** use `-readonly` para não correr risco de alterar o banco por acidente.
> Com o app rodando, o WAL é lido normalmente; não é preciso parar o servidor.

#### Preparação

```sh
cd "/caminho/para/ton618"
DB="core/data/ton618.db"

sqlite3 --version   # se não existir: sudo apt install sqlite3
```

#### Notas mais recentes

```sh
sqlite3 -readonly -header -column "$DB" \
  "SELECT filename, mtime FROM notes ORDER BY mtime DESC LIMIT 20;"
```

#### Busca no conteúdo (FTS5), com trecho destacado

```sh
sqlite3 -readonly -header -column "$DB" "
  SELECT arquivo,
         secao,
         snippet(docs_fts, 4, '»', '«', '…', 12) AS trecho,
         bm25(docs_fts) AS score
  FROM docs_fts
  WHERE docs_fts MATCH 'foco*'
  ORDER BY score
  LIMIT 10;"
```

A coluna `4` é `texto` (as colunas do índice são `doc_id`, `tipo`, `arquivo`, `secao`, `texto`,
`tags`, `note_type`).

#### Filtrar por tag

```sh
sqlite3 -readonly -header -column "$DB" "
  SELECT n.filename, n.mtime
  FROM notes n
  JOIN tags t ON t.arquivo = n.filename
  WHERE t.tag = 'livros'
  ORDER BY n.mtime DESC;"
```

#### Contar por tipo de nota

```sh
sqlite3 -readonly -header -column "$DB" "
  SELECT note_type, COUNT(*) AS total
  FROM documents
  GROUP BY note_type
  ORDER BY total DESC;"
```

#### Ler o conteúdo de uma nota

```sh
sqlite3 -readonly "$DB" \
  "SELECT content FROM notes WHERE filename = 'notes/minha-nota.md';"
```

#### Ver backlinks

```sh
sqlite3 -readonly -header -column "$DB" "
  SELECT from_file
  FROM links
  WHERE to_file = 'notes/projetos.md';"
```

#### Atalho com uma função de shell

Se você usa muito o terminal, coloque no seu `~/.bashrc`:

```sh
ton-search() {
  local db="/caminho/para/ton618/core/data/ton618.db"
  sqlite3 -readonly -header -column "$db" "
    SELECT arquivo,
           secao,
           snippet(docs_fts, 4, '»', '«', '…', 12) AS trecho
    FROM docs_fts
    WHERE docs_fts MATCH '$1*'
    ORDER BY bm25(docs_fts)
    LIMIT ${2:-10};"
}
```

Uso: `ton-search filosofia 5`.

#### Backup pelo terminal

```sh
sqlite3 "core/data/ton618.db" ".backup 'ton618-$(date +%F).db'"
```

O `.backup` do SQLite tira um instantâneo consistente mesmo com o app rodando. Para uma cópia
completa (notas + arquivos + configurações), prefira o **Backup Total** dentro do app.

---


## 8. Atalhos e gestos

| Atalho / gesto | Onde | Efeito |
|---|---|---|
| `Ctrl+S` / `Cmd+S` | Editor | Salva imediatamente |
| `/` no início da linha | Editor | Abre o menu de inserção |
| `[[` | Editor | Abre o menu de wikilinks |
| `Tab` | Bloco de código | Insere indentação |
| `Ctrl+clique` numa linha do TOC | Editor | Pula para o título correspondente |
| `Tab` numa linha do TOC | Editor | Muda o nível do título |
| `Esc` | Editor | Fecha menus (slash, wikilink, bolha) |
| Duplo clique numa linha | Acervo | Abre a nota/arquivo |
| Clique numa célula | Acervo | Edita o campo (quando editável) |
| Clique no lápis | Lista lateral | Renomeia a nota |
| Clique na lixeira | Lista lateral | Exclui a nota (com confirmação) |
| Arrastar o cabeçalho | Acervo | Reordena colunas |

---


## 9. Perguntas frequentes e solução de problemas

### Não consigo entrar / a senha não funciona

A senha é lida de `AUTH_PASS` a cada inicialização. Confira o `.env`, o `docker-compose.yml` ou o
comando usado. Se você já tinha uma sessão antiga, limpe o cookie `ton_auth` e o `localStorage` do
site. O usuário digitado é ignorado: só a senha importa.

### A porta 6180 está ocupada

```sh
./run.sh status
./run.sh stop
./run.sh
```

Se o processo estiver **suspenso** (`stat T`, típico de `Ctrl+Z`), a porta fica presa sem responder.
O `status` avisa; o `stop` resolve. Evite `Ctrl+Z`; use `Ctrl+C`.

### Subi o app, mas ele diz que Go não foi encontrado

Instale o Go ou ajuste o `PATH`. O `run.sh` procura em `/usr/local/go/bin`, `/usr/lib/go/bin`,
`~/go/bin`, `/snap/go/current/bin` e em toolchains baixados em `~/go/pkg/mod`. Se nada funcionar,
rode o binário já compilado (`./ton618`) ou use Docker.

### Criei uma nota e ela não aparece na busca

A busca por nome é por arquivo; a global é por conteúdo indexado. Verifique:

1. a nota foi salva? (indicador `saved`);
2. você está no modo de busca certo? (lupa azul x verde);
3. a nota é um desenho/mapa? O corpo desses tipos não é indexado — busque pelo título ou tag;
4. a nota está arquivada? `archives/` não aparece na lista ativa.

### A busca não encontra uma palavra que está na nota

- A busca global casa por **prefixo**, não por lematização: `proj` encontra `projeto`, mas `correr`
  pode não encontrar `corri`, porque o começo da palavra muda.
- Para excluir termos, refine o texto ou combine os filtros disponíveis.

### O `tipo:` da busca não encontra a nota

Use o vocabulário canônico: `nota`, `desenho`, `mapa_mental`, `tabulator`, `calculo`, `pdf`,
`epub`, `anexo`, `artigo`, `captura`, `imagem`, `arquivo`. Aliases como `markmap`, `mindmap`,
`drawing` e `article` são normalizados, mas o valor gravado é o canônico.

### O Acervo mostra uma coluna “Parent” vazia e outra “Pai” preenchida

Notas antigas podem usar `parent:`. O app normaliza para `pai:` na primeira edição da célula. Se
você vir as duas colunas, edite a célula de hierarquia (ou salve a nota) para migrar.

### Editei o `pai:` e a árvore não mudou

A edição de `pai:` recarrega a tabela automaticamente. Se estiver no editor, salve e reabra a nota;
a barra de hierarquia é recalculada na abertura.

### O backup completo não inclui `archives/`

É de propósito. `archives/` é o arquivo morto e só entra no **Backup Total**. Se você precisa
levar os pacotes arquivados, use o Total.

### Restaurei um backup e o servidor caiu

A restauração substitui o banco e os arquivos e reinicia o processo. Aguarde alguns segundos e
recarregue. O estado anterior fica em `core/data/.pre-restore-<data>/` (quarentena) caso algo dê
errado.

### A nota Tabulator abre com “JSON inválido”

O corpo de uma nota Tabulator é JSON. Se você editou o arquivo manualmente e quebrou a estrutura,
o app avisa antes de sobrescrever. Corrija o JSON ou restaure a nota por um backup.

### O login entra em loop

Limpe cookie e `localStorage`, recarregue e entre de novo. Em Docker, confirme que o volume
`./data` está montado e que `AUTH_PASS` é o mesmo do container.

### “Endereço já em uso” ao subir no Docker

Outra instância (ou outro serviço) está na porta 6180. Pare o container antigo ou mude o mapeamento
de portas no `docker-compose.yml`.


---

## 10. Glossário

| Termo | Significado |
|---|---|
| **Acervo** | Conjunto de notas e arquivos, e também a página `/database` que os administra |
| **Backlink** | Link de entrada: uma nota que aponta para a atual |
| **FTS5** | Mecanismo de busca textual do SQLite usado pela busca global |
| **Frontmatter** | Bloco YAML no topo da nota (`--- ... ---`) |
| **Hierarquia** | Relação `pai:`/filhas entre notas |
| **Nó** | Item de um mapa mental (Markmap) |
| **Raiz** | Primeiro segmento do caminho (`notes/`, `pdfs/`, `epubs/`…) |
| **Tabulator** | Tipo de nota que é uma planilha de dados |
| **Tag** | Etiqueta no frontmatter ou `#tag` no corpo |
| **Wikilink** | Link no formato `[[nota]]` |
| **WAL** | *Write-Ahead Log*; modo de journal do SQLite (arquivos `-wal`/`-shm`) |
| **YAML** | Formato do frontmatter |


---

*Esta ajuda acompanha o TON-618 e é lida em `/help`. Se você editar este arquivo, recarregue a
página para ver o resultado.*
