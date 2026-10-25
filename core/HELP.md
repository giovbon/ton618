# Ajuda — TON-618

O **TON-618** é um sistema pessoal de conhecimento que roda na sua máquina. Ele guarda notas,
desenhos, mapas mentais, planilhas, PDFs, EPUBs, anexos e compromissos em um só lugar — e deixa
você procurar, cruzar e organizar tudo isso sem depender de internet ou de serviços de terceiros.

A ideia central é simples: **você escreve, o app indexa**. A partir daí, você pode buscar por
palavra, filtrar por tag e por tipo, navegar pela hierarquia de notas, montar páginas que se
atualizam sozinhas e até deixar uma IA local consultar o acervo em modo somente leitura.

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
| `core/data/ton618.db` | Banco SQLite: notas, índice de busca, tags, links, tarefas, agenda, configurações |
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
(veja a seção 10.2).

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
| `tipo` / `type` | Tipo canônico da nota (`nota`, `desenho`, `mapa_mental`, `tabulator`, `semanal`, `pdf`, `anexo`, `artigo`, `captura`) |
| `pai` / `parent` | Nota-mãe na hierarquia (`parent` é aceito por compatibilidade, mas o app grava `pai`) |

Campos personalizados que você inventar (`autor:`, `status:`, `fonte:`) também são indexados e
podem virar **colunas do Acervo** (seção 5.3) e **campos de consulta** pela linguagem de consulta.

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
| `semanal` | Criar → + Semana | Editor de texto | Sim |
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

Essas raízes aparecem na linguagem de consulta (`pasta:`), na busca global (`tipo:`) e na coluna
**Pasta** do Acervo.


---

## 4. Uso diário

### 4.1 Criar uma nota

Clique em **Criar → + Nota**. O editor abre uma nota em branco com um nome gerado automaticamente.
O primeiro passo é dar um nome: edite o campo do título no topo e pressione `Enter`. O nome vira o
identificador da nota.

Para criar os outros tipos, use o mesmo menu:

- **+ Semana** — nota semanal da semana ISO atual;
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

### 4.5 Nota semanal

**Criar → + Semana** cria (ou abre, se já existir) a nota da semana ISO atual, no formato
`notes/2026-S38.md`. A nota nasce em branco, de propósito: o tipo `semanal` vem do nome do arquivo,
então não é preciso tag nem frontmatter. Clicar de novo na mesma semana abre a mesma nota — não
duplica.

Use-a como diário semanal, registro de decisões ou ponto de partida para revisões.

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

Na mesma aba há o **Auto-Tag**: regras do tipo “30 dias → `#inativo`”. A aplicação é **manual**
(botão **Aplicar Tags Agora**). O critério é a última interação com a nota; notas nunca abertas
usam a data de modificação. A tag é removida na próxima aplicação se a nota voltou a ser usada.

O botão **Limpar imagens órfãs** remove de `docs/images/` as imagens que não são mais referenciadas
por nenhuma nota.


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
- **Sinônimos:** quando a consulta tem **uma única palavra de conteúdo**, o app expande com o mapa
  embutido e com os sinônimos que você cadastrou em **Configurações → Sinônimos**. Em consultas com
  várias palavras, não há expansão (para não perder precisão).
- **Tags e tipos** podem ser combinados com texto livre: `tipo:artigo foco` procura “foco” apenas
  em artigos.
- **Não existe operador de exclusão:** `-palavra` não exclui nada — o hífen é tratado como separador
  e a palavra passa a ser **exigida** junto com as outras. Para excluir, use a linguagem de consulta
  com `onde: '!(...)'`.

Os resultados são agrupados por nota. Cada cartão mostra até **5 trechos** e indica quantas
ocorrências existem além disso. O termo exato aparece em azul; variações e sinônimos, em roxo.

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

## 6. Tarefas

O TON-618 extrai tarefas do texto das notas. Há dois mecanismos diferentes, e é importante não
confundi-los.

### 6.1 Marcadores

Uma linha que começa com um marcador seguido de dois-pontos vira uma tarefa:

```markdown
TODO: publicar o artigo
DOING: revisar a introdução
DONE: escolher o título
```

- O marcador é **case-insensitive** (`todo:` também funciona).
- A seção (`## Título`) onde a linha está é guardada junto.
- Os marcadores padrão são `TODO`, `DOING` e `DONE`. Outros (como `FIXME` ou `REVISAR`) só são
  detectados depois que você os cria em **Configurações → Marcadores**.
- Só marcadores **ativos** são detectados.

### 6.2 Checkboxes

Checkboxes markdown também são reconhecidos:

```markdown
- [ ] comprar café
- [x] pagar a conta
```

Eles têm tipo interno `TASK` e status `pending`/`completed`. **Eles não aparecem na página Task**
nem no badge do cabeçalho — a página Task é só de marcadores. Use checkboxes para listas dentro de
uma nota e marcadores para tarefas que você quer acompanhar no painel global. A linguagem de
consulta (`tipo: tarefas`) enxerga os dois.

### 6.3 A página Task

Abra **Task** no topo. Você encontra:

- filtros por marcador: um botão **Todos** e um botão por marcador ativo;
- filtros de status: **Pendentes** (padrão) e **Completos**;
- busca por texto, arquivo ou seção;
- agrupamento por arquivo e por seção, com contagem;
- **Alternar Expansão** para abrir/fechar todos os grupos.

O número no ícone **Task** é o badge: conta as tarefas dos marcadores que têm a opção **Contar**
ligada. Os três marcadores padrão (`TODO`, `DOING`, `DONE`) nascem contando — inclusive `DONE`.
Se você usa o badge como “o que ainda falta”, desligue **Contar** em `DONE` na aba **Marcadores**.

### 6.4 Configurar marcadores

**Configurações → Marcadores** permite:

- adicionar um marcador (até 32 caracteres; letras, números, espaço, `_`, `-`, `.`);
- escolher a cor;
- ativar/desativar;
- decidir se ele conta no badge;
- reordenar;
- restaurar o padrão.

O nome `TASK` é reservado para checkboxes e não pode ser criado como marcador.

Ao salvar uma nota, o app reprocessa os marcadores daquele arquivo. Se você mudou a configuração
de marcadores, abra e salve as notas afetadas para que os novos marcadores sejam detectados — a
mudança de configuração sozinha não reindexa o acervo.


---

## 7. Agenda

A Agenda guarda **compromissos**, não notas. Ela fica em `/agenda` e é independente do acervo.

### 7.1 Criar um compromisso

No campo **Novo compromisso...**, escreva em linguagem natural:

```text
Reunião com a equipe amanhã às 15h
Dentista na próxima terça às 9h
Publicar o texto sexta que vem
```

Enquanto você digita, o app mostra a data reconhecida (por exemplo, “Data reconhecida: 26/10/2026
15:00”). Se não reconhecer nada, avisa em cinza. O reconhecimento é feito localmente, sem internet,
com o analisador de datas em português.

Clique em salvar. O compromisso aparece:

- na **linha do tempo** no topo (com as cores do dia);
- na **árvore** por ano/semana, com contagem;
- com ações de editar e excluir.

O botão **Limpar** remove compromissos anteriores a 7 dias.

### 7.2 Links e tags na descrição

A descrição aceita:

- `[[Nome da nota]]` — vira link para a nota;
- `#tag` — vira um chip de tag.

Isso permite amarrar um compromisso a um projeto ou a um contexto.

### 7.3 Notificações (Ntfy)

**Configurações → Ntfy** envia notificações por push via [ntfy](https://ntfy.sh):

| Campo | O que é |
|---|---|
| URL do servidor | `https://ntfy.sh` ou seu servidor |
| Tópico | o nome do tópico que você assina no app ntfy |
| Usuário / Senha | opcionais, para servidor protegido |

Com o Ntfy configurado, o app envia:

- **lembretes de compromissos** — X horas antes do evento (padrão 24 h, configurável em
  **Configurações → Agenda**);
- **resumo semanal** — aos domingos, com a semana seguinte.

O fuso usado nos cálculos é o de **Configurações → Agenda** (padrão `America/Sao_Paulo`). Ele também
define o nascer/pôr do sol mostrado no gráfico da linha do tempo.

### 7.4 Rotina

A **Rotina** é a linha do tempo das atividades do **seu dia** — o equivalente pessoal da Agenda, sem
convidados nem “responsável”: o TON-618 é de uso individual e toda atividade pertence ao dono do cofre.

Abra pelo ícone **Rotina** no cabeçalho, ao lado de **Agenda** (ou pela URL `/rotina`).

| Parte | O que mostra |
|---|---|
| **Resumo do dia** | Minutos **ocupados**, **livres** e a barra de ocupação sobre 24 h |
| **Linha do tempo** | Gráfico de Gantt (Mermaid) com uma barra por atividade |
| **Atividades** | Lista em ordem de horário, com editar/excluir |
| **Nova atividade** | Início, fim e descrição; salva e atualiza tudo na hora |

Detalhes que valem saber:

- O tempo **ocupado** é a **união** dos intervalos: duas atividades que se sobrepõem contam uma vez
  só, e o app avisa quando há sobreposição (sem somar o tempo duas vezes).
- **Ampliar** abre a linha do tempo do **dia inteiro**; no cartão, telas pequenas compactam o eixo
  para o intervalo real das atividades (com folga de 2 h, mínimo de 8 h).
- As setas ‹ › e o seletor de data navegam entre os dias; **Hoje** volta para a data atual.
- A Rotina pode ser **desligada** em **Configurações → Rotina**: o ícone some do cabeçalho e
  `/rotina` redireciona para a home. Nenhuma atividade é apagada — ao religar, tudo volta.

---




## 8. Linguagem de consulta: páginas vivas

Esta é a parte que transforma o TON-618 de “bloco de notas” em **painel pessoal**. Você escreve uma
pergunta dentro de uma nota e o app mostra a resposta, recalculada a cada abertura.

### 8.1 O que é (e o que não é)

Uma consulta é um bloco de código com a linguagem `consulta`:

````markdown
```consulta
tipo: notas
tags: [livros]
onde: '!("lido" in n.tags)'
ordenar: mtime asc
limite: 10
mostrar: lista
```
````

| É | Não é |
|---|---|
| Declarativa: você descreve o que quer | Programação imperativa (sem `for`, sem variáveis, sem atribuição) |
| Somente leitura | Nunca cria, altera, move ou apaga nada |
| Executada no servidor | Não roda no navegador, não baixa nada, não usa internet |
| Determinística | Mesma entrada, mesma saída (o “agora” é injetado) |
| Efêmera | O resultado **não é gravado** na nota, só exibido |

> **Por que o resultado não é salvo?** Se fosse, o app reindexaria a nota, o resultado mudaria e
> isso entraria em loop. O texto do bloco é a pergunta; a resposta existe só na tela.

### 8.2 Onde escrever

Em qualquer nota markdown, num bloco ` ```consulta `. Regras:

- o idioma `consulta` é obrigatório; um bloco comum (ou `sql`, `go` etc.) continua sendo código;
- pode haver **até 10 blocos por nota**, cada um independente;
- a ordem dos cartões no painel segue a ordem dos blocos no texto;
- o conteúdo é **YAML**: `chave: valor`, uma por linha. Listas podem ser `[a, b]` ou em bloco.

### 8.3 Onde ver o resultado

No editor, **abaixo do texto**, aparece o painel **Consultas**, com um cartão por bloco:

| Elemento | Significado |
|---|---|
| Ícone + contagem | Tipo da consulta e quantos itens voltaram |
| Botão de atualizar | Recalcula só aquele bloco |
| Corpo | O resultado, conforme `mostrar:` |
| Rodapé `mostrando N de M` | O resultado foi truncado por `limite:` |
| Faixa de erro | Mensagem em português, dentro do cartão |

O painel é atualizado quando você salva a nota e quando clica em atualizar. Um erro em uma consulta
**nunca** derruba a nota: ele aparece no cartão.

### 8.4 As chaves do bloco

| Chave | Obrigatória | Valores | Padrão |
|---|---|---|---|
| `tipo` | **sim** | `notas` · `tarefas` · `hierarquia` | — |
| `onde` | não | expressão CEL | vazio = todos |
| `ordenar` | não | `<campo>` + `asc`/`desc` | depende do tipo |
| `limite` | não | inteiro de `1` a `200` | `20` |
| `mostrar` | não | `lista` · `tabela` · `contagem` · `cartoes` · `arvore` | `lista` |
| `colunas` | não | lista de campos (só com `mostrar: tabela`) | `[titulo, tags, mtime]` |

Atalhos de índice (cada um usa uma consulta já indexada):

| Chave | Tipo | Valores | Efeito |
|---|---|---|---|
| `tags` | `notas` | lista de tags | A nota precisa ter **todas** |
| `sem-tags` | `notas` | lista de tags | A nota **não pode ter nenhuma** |
| `texto` | `notas` | string | Busca FTS5 no conteúdo |
| `pasta` | `notas` | `notes`, `pdfs`, `epubs`, `attachments`, `images`, `archives` | Restringe a raiz |
| `marcador` | `tarefas` | lista de marcadores | Filtra pelo tipo da tarefa |
| `estado` | `tarefas` | `pendente`, `concluida`, `todas` | Filtra pelo status |
| `de` | `hierarquia` | nome/caminho de nota | Raiz da árvore (padrão: a nota atual) |
| `janela` | `agenda` | `hoje`, `semana`, `mes` | **Reservado** (ainda não suportado) |

> **Regra mental:** tudo o que tem índice vira atalho; o resto vai no `onde:`. Se a informação não
> está no índice (por exemplo, `n.citada_por >= 3`), use CEL sobre os candidatos que os atalhos já
> reduziram.

Chave desconhecida **é erro** (não é ignorada em silêncio): o cartão lista as chaves válidas.

### 8.5 `tipo: notas`

É o tipo principal: consulta o acervo de arquivos.

- Atalhos: `tags`, `sem-tags`, `texto`, `pasta`.
- Campos de `ordenar`/`colunas`: os de `n` (seção 8.9).
- `mostrar:`: todos, exceto `arvore`.

````markdown
```consulta
tipo: notas
tags: [projeto, ativo]
onde: n.filhas.size() == 0
ordenar: citada_por desc
limite: 15
mostrar: tabela
colunas: [titulo, tags, citada_por, interacao]
```
````

Semântica dos atalhos:

- `tags: [a, b]` é **E** lógico. Para **OU**, use `onde: "a" in n.tags || "b" in n.tags`.
- `sem-tags: [rascunho]` exclui notas com `rascunho`.
- `texto: "guerra"` usa FTS5 e pode ser combinado com `onde:`.
- `pasta: pdfs` limita àquela raiz.

### 8.6 `tipo: tarefas`

Consulta a tabela de tarefas (a mesma que alimenta a página Task e o badge).

- Atalhos: `marcador`, `estado`.
- Campos: `t.*` (seção 8.9).
- `mostrar:`: `lista`, `tabela`, `contagem`.

````markdown
```consulta
tipo: tarefas
marcador: [TODO]
estado: pendente
onde: t.texto.contains("publicar")
ordenar: criado asc
limite: 50
mostrar: lista
```
````

| Chave | Valores | Padrão | Observação |
|---|---|---|---|
| `marcador` | `[TODO]`, `[FIXME]`, `[REVISAR]`… | todos | Casa com o tipo da tarefa |
| `estado` | `pendente` · `concluida` · `todas` | `pendente` | `concluida` corresponde a checkboxes marcados |

> Checkboxes `- [ ]` entram aqui com tipo `TASK`; por isso `estado: pendente` inclui os marcadores
> pendentes e os checkboxes abertos. Para ver só marcadores, use `marcador:`.

### 8.7 `tipo: hierarquia`

Percorre a árvore **pai → filhas** declarada no frontmatter.

- Atalhos: `de` (raiz).
- `mostrar:`: `lista`, `arvore`, `contagem`.

````markdown
```consulta
tipo: hierarquia
de: Projeto TON-618
onde: n.mtime > timestamp("2026-09-01T00:00:00Z")
mostrar: arvore
```
````

Sem `de:`, a raiz é **a própria nota onde o bloco está escrito** — ideal para um índice de filhos.
A profundidade é limitada (padrão: 3 níveis) e `arvore` mostra a indentação com `└`.

### 8.8 Expressões (`onde:`) — CEL

O `onde:` é avaliado por **CEL** (*Common Expression Language*), a mesma linguagem de regras do
Kubernetes. É uma linguagem de expressões: cada linha produz `true` ou `false`.

#### Operadores e literais

| Categoria | Operadores / literais |
|---|---|
| Lógicos | `&&` (E) · `\|\|` (OU) · `!` (NÃO) |
| Comparação | `==` · `!=` · `<` · `<=` · `>` · `>=` |
| Aritmética | `+` · `-` · `*` · `/` · `%` |
| Agrupamento | `( … )` |
| Literais | `42` · `3.14` · `"texto"` · `'texto'` · `true` / `false` · `null` |
| Listas | `["a", "b"]` · `[]` · `in` (`"a" in n.tags`) |
| Mapa | `{"a": 1}` · `n.mapa["chave"]` |

A precedência é a usual: `!` > aritmética > comparação > `&&` > `||`. Na dúvida, use parênteses.

#### Macros

| Macro | Assinatura | Para que serve |
|---|---|---|
| `exists` | `lista.exists(x, condicao)` | Existe algum item que satisfaz |
| `all` | `lista.all(x, condicao)` | Todos satisfazem |
| `exists_one` | `lista.exists_one(x, condicao)` | Exatamente um satisfaz |
| `map` | `lista.map(x, expressao)` | Transforma a lista |
| `filter` | `lista.filter(x, condicao)` | Filtra a lista |
| `has` | `has(mapa.chave)` | Testa se o campo existe |
| `size` | `lista.size()` / `"txt".size()` | Quantidade / comprimento |

```cel
// alguma tag começa com "livro"?
n.tags.exists(t, t.startsWith("livro"))

// todas as tags são curtas (menos de 12 caracteres)?
n.tags.all(t, t.size() < 12)

// quantas tags começam com "proj"?
n.tags.filter(t, t.startsWith("proj")).size() >= 2
```

#### Funções de texto

| Função | Exemplo | Observação |
|---|---|---|
| `contains` | `n.titulo.contains("ATA")` | Sensível a maiúsculas |
| `startsWith` | `n.caminho.startsWith("notes/2026")` | |
| `endsWith` | `n.titulo.endsWith("pdf")` | |
| `matches` | `n.caminho.matches("^notes/2026-\\d\\d")` | Regex RE2 (Go), sem backtracking |
| `size` | `n.titulo.size() > 60` | Comprimento em caracteres |

Para busca sem acento ou sem diferenciar maiúsculas, prefira o atalho `texto:` (FTS5) em vez de
`matches`/`contains`: as funções de texto do CEL são sensíveis à caixa.

#### Tempo e datas

| Recurso | Exemplo | Significado |
|---|---|---|
| `agora` | `agora` | Instante da execução (injetado, determinístico) |
| `duration("2160h")` | `duration("720h")` | Duração (sufixos `h`, `m`, `s`) |
| `timestamp(...)` | `timestamp("2026-09-01T00:00:00Z")` | Instante fixo em RFC 3339 |
| `dias(ts)` | `dias(n.mtime)` | Dias inteiros até `agora` (função do app) |
| `horas(ts)` | `horas(n.interacao)` | Horas até `agora` (função do app) |
| `-` entre timestamps | `agora - n.mtime` | Duração |

```cel
// não mexo há mais de 90 dias
dias(n.mtime) > 90

// editada nas últimas 2 semanas
agora - n.mtime < duration("336h")

// nunca interagi
n.interacao == null

// "esfriou": interagi antes, mas não nos últimos 30 dias
n.interacao != null && dias(n.interacao) > 30
```

#### Erros de expressão

| Situação | Mensagem no cartão |
|---|---|
| Sintaxe | `onde: <input>:1:12: Syntax error: …` (mensagem do CEL, com linha e coluna) |
| Campo inexistente | `onde: campo desconhecido "velocidade" — campos de n: ...` |
| Tipo incompatível | `onde: tipo incompatível — no such overload: _>_` |
| Expressão cara | `onde: expressão excedeu o limite de custo (simplifique a expressão)` |
| Demorou demais | `onde: tempo esgotado (200 ms)` |
| Regex inválida | `onde: error parsing regexp: …` |
| Resultado não booleano | `onde: a expressão precisa resultar em verdadeiro ou falso` |

Tudo aparece dentro do cartão, com a posição quando o CEL a informa. Nunca é um erro 500.

### 8.9 Variáveis e campos

| Variável | Tipo | Existe em |
|---|---|---|
| `n` | nota / arquivo | `tipo: notas`, `tipo: hierarquia` |
| `t` | tarefa | `tipo: tarefas` |
| `a` | compromisso | `tipo: agenda` (reservado) |
| `agora` | timestamp | todos |

**Campos de `n`:**

| Campo | Tipo | Significado | Custo |
|---|---|---|---|
| `n.caminho` | string | Caminho relativo (`notes/pessoal/x.md`) | índice |
| `n.titulo` | string | Nome de exibição (sem extensão) | índice |
| `n.pasta` | string | Raiz (`notes`, `pdfs`, `epubs`, `attachments`, `images`, `archives`) | índice |
| `n.classe` | string | Tipo do item (veja abaixo) | índice |
| `n.tags` | lista de string | Tags | índice |
| `n.mtime` | timestamp | Última modificação | índice |
| `n.pai` | string | Valor de `pai:` (`""` se não tiver) | índice |
| `n.pai_resolvido` | bool | O `pai:` aponta para uma nota existente | agregado |
| `n.filhas` | lista de string | Nomes-base das filhas diretas | agregado |
| `n.citada_por` | int | Quantos backlinks apontam para ela | agregado |
| `n.tarefas_abertas` | int | Tarefas pendentes dentro dela | agregado |
| `n.popularidade` | int | Contador de interações | índice |
| `n.interacao` | timestamp ou `null` | Última interação no app | índice |

> **Não existe “última abertura”.** O banco registra a **última interação**
> (`popularity.last_interacted_at`), que também muda ao editar. É por isso que o campo se chama
> `n.interacao` e não `n.abertura`.

Valores de `n.classe`: `nota` (markdown), `desenho`, `markmap`, `youtube`, `artigo`, `captura`,
`semanal`, `tabulator`, `pdf`, `epub`, `anexo`, `imagem`, `arquivo`.

**Campos de `t`:**

| Campo | Tipo | Significado |
|---|---|---|
| `t.texto` | string | Texto da tarefa |
| `t.marcador` | string | `TODO`, `FIXME`, `REVISAR`… |
| `t.estado` | string | `pendente` ou `concluida` |
| `t.arquivo` | string | Caminho da nota onde ela vive |
| `t.nota_titulo` | string | Título dessa nota |
| `t.secao` | string | Seção (`##`) onde a tarefa está |
| `t.linha` | int | Linha no arquivo |
| `t.criado` | timestamp | Quando foi detectada |

### 8.10 Ordenação e limite

```yaml
ordenar: mtime desc
ordenar: titulo
limite: 25
```

| Regra | Detalhe |
|---|---|
| Direção natural | Campos de texto (`titulo`, `caminho`, `arquivo`, `pasta`, `classe`, `texto`, `marcador`, `estado`, `secao`, `pai`) vão de A a Z; datas, números e listas vão do maior para o menor |
| `asc` / `desc` | O sufixo explícito vence a direção natural |
| Um campo só | O v1 ordena por um campo; `mtime desc, titulo asc` não é suportado |
| Listas | `filhas` e `tags` ordenam pelo tamanho da lista |
| `limite` | `1` a `200`; padrão `20`; acima de 200 é rejeitado |
| Truncamento | O cartão mostra `mostrando 20 de 137` |
| Desempate | Sempre `caminho asc`, para o resultado ser estável |

### 8.11 Apresentações (`mostrar:`)

#### `lista` (padrão)

Lista de links, com ícone do tipo e data relativa. Permitido em `notas`, `tarefas` e `hierarquia`.

```text
• 📄 Livros por ler            (editado há 2 dias)
• 📄 Fila do mês               (editado há 3 semanas)
```

#### `tabela`

Tabela compacta; `colunas:` escolhe e ordena as colunas.

````markdown
```consulta
tipo: notas
tags: [livros]
ordenar: citada_por desc
mostrar: tabela
colunas: [titulo, tags, interacao_ha_dias, citada_por]
```
````

| Título | Tags | Interação há (dias) | Citações |
|---|---|---|---|
| A História Secreta do Ocidente | `livros, história` | 12 | 5 |
| Tecnofeudalismo | `livros, economia` | — | 3 |

- `interacao_ha_dias` = `dias(n.interacao)` (vazio quando nunca interagida).
- Colunas válidas: `arquivo`, `caminho`, `citada_por`, `classe`, `criado`, `estado`, `filhas`,
  `interacao`, `interacao_ha_dias`, `linha`, `marcador`, `modif_ha_dias`, `mtime`, `n_tags`,
  `nota_titulo`, `pai`, `pai_resolvido`, `pasta`, `popularidade`, `secao`, `tags`,
  `tarefas_abertas`, `texto`, `titulo`.
- `colunas:` só vale com `mostrar: tabela`. Coluna desconhecida é erro com a lista das válidas.

#### `contagem`

Um número e um rótulo, sem lista. Ideal para dashboards.

```text
12 notas
```

#### `cartoes`

Grade com título + trecho. Como o trecho vem do índice FTS5, combina com `texto:`.

````markdown
```consulta
tipo: notas
texto: "foco profundo"
mostrar: cartoes
limite: 6
```
````

#### `arvore`

Só em `tipo: hierarquia`: a subárvore com indentação.

```text
Projeto TON-618
├─ Arquitetura
│  └─ Busca
└─ Diário
   └─ 2026-09-30
```

### 8.12 Limites e segurança

| Guarda | Valor | Por quê |
|---|---|---|
| Somente leitura | por construção | O motor só recebe uma interface de leitura |
| Nada é persistido | sempre | Evita loop de escrita |
| Blocos por nota | 10 | Uma página não vira um dump |
| Candidatos pré-filtrados | 500 | Resultado sempre pequeno |
| `limite:` | máximo 200 | Resultado sempre pequeno para a tela |
| Tempo por bloco | 200 ms | A nota abre rápido mesmo com expressão ruim |
| Custo da expressão | limite do CEL | Impede expressão quadrática/exponencial |
| Cache | 5 s + invalidação em escrita | Consistência sem custo |
| Sem rede, sem I/O, sem `exec` | por linguagem | CEL não tem I/O |
| `agora` injetado | sempre | Determinismo |

Quando há mais de 500 candidatos, o cartão avisa `avaliadas as 500 mais recentes de 1.204`.
Quando a expressão é patológica, aparece uma mensagem de limite — não um travamento.

### 8.13 Receitas prontas

**1. Fila de leitura (livros que ainda não terminei)**

````markdown
```consulta
tipo: notas
tags: [livros]
onde: '!("lido" in n.tags)'
ordenar: mtime asc
limite: 10
mostrar: lista
```
````

**2. Total de notas do acervo**

````markdown
```consulta
tipo: notas
mostrar: contagem
```
````

**3. Esfriando: não interajo há mais de 60 dias**

````markdown
```consulta
tipo: notas
onde: n.interacao != null && dias(n.interacao) > 60
ordenar: interacao asc
mostrar: tabela
colunas: [titulo, tags, interacao_ha_dias]
```
````

**4. Capturas não processadas**

````markdown
```consulta
tipo: notas
onde: n.classe == "captura" && !("processado" in n.tags)
ordenar: mtime asc
mostrar: lista
```
````

**5. Órfãs: têm `pai:` mas o pai não existe**

````markdown
```consulta
tipo: notas
onde: n.pai != "" && !n.pai_resolvido
mostrar: tabela
colunas: [titulo, pai]
```
````

**6. Notas-mãe mais carregadas**

````markdown
```consulta
tipo: notas
onde: n.filhas.size() >= 5
ordenar: filhas desc
mostrar: tabela
colunas: [titulo, filhas, citada_por]
```
````

**7. Sem etiqueta nenhuma (limpeza)**

````markdown
```consulta
tipo: notas
pasta: notes
onde: n.tags.size() == 0 && n.classe == "nota"
ordenar: mtime desc
limite: 30
mostrar: lista
```
````

**8. Mais citadas do acervo**

````markdown
```consulta
tipo: notas
onde: n.citada_por >= 3
ordenar: citada_por desc
limite: 10
mostrar: tabela
colunas: [titulo, citada_por, tags]
```
````

**9. Notas semanais recentes**

````markdown
```consulta
tipo: notas
onde: n.titulo.matches("^2026-S")
ordenar: titulo desc
limite: 8
mostrar: lista
```
````

**10. Tarefas pendentes de um projeto**

````markdown
```consulta
tipo: tarefas
marcador: [TODO]
estado: pendente
onde: t.arquivo.contains("projeto-ton618")
mostrar: lista
```
````

**11. Checkboxes concluídos em notas editadas nesta semana**

````markdown
```consulta
tipo: tarefas
estado: concluida
onde: agora - t.criado < duration("168h")
mostrar: contagem
```
````

> `t.criado` é o momento em que a tarefa foi **detectada** na última indexação (normalmente a
> modificação da nota), não a data em que você a concluiu. O app não guarda um “concluído em”.

**12. Índice dos filhos desta nota**

````markdown
```consulta
tipo: hierarquia
mostrar: arvore
```
````

**13. Trechos de busca em cartões**

````markdown
```consulta
tipo: notas
texto: "foco"
onde: n.classe == "artigo"
mostrar: cartoes
limite: 6
```
````

**14. Desenhos/mapas abandonados**

````markdown
```consulta
tipo: notas
onde: (n.classe == "desenho" || n.classe == "markmap") && dias(n.mtime) > 180
ordenar: mtime asc
mostrar: lista
```
````

**15. Painel de manutenção da semana**

````markdown
```consulta
tipo: notas
onde: agora - n.mtime < duration("168h")
ordenar: mtime desc
limite: 20
mostrar: tabela
colunas: [titulo, tags, modif_ha_dias]
```
````

### 8.14 Armadilhas do YAML

O bloco é YAML. Três regras resolvem quase todos os problemas:

| Situação | Como escrever |
|---|---|
| A expressão tem `#` ou `: ` | Ponha entre aspas: `onde: 'n.titulo.contains("#1")'` |
| Lista de tags | `tags: [a, b]` ou em bloco com `- a` |
| String que é só número (`"2026"`) | Use aspas: `onde: n.titulo == "2026"` |
| Regex com `\` | Barra dupla: `n.caminho.matches("^notes/\\d{4}")` |
| A expressão começa com `!` ou `[` | Ponha entre aspas: `onde: '!(...)'` ou `onde: '["a","b"].exists(t, t in n.tags)'` |

Erros de YAML aparecem como `bloco: YAML inválido (linha 4) — …`.

### 8.15 O que ainda não existe

| Recurso | Situação | Motivo |
|---|---|---|
| `similar:` (semântico no bloco) | Fora de escopo | Dependia do índice de embeddings, removido |
| `tipo: agenda` | v2 | Precisa fixar os campos sobre o modelo real de compromissos |
| `n.tamanho`, `n.palavras`, `n.peso` | v2 | Custam I/O por linha; o motor v1 lê só o banco |
| `janela:` | v2 | Só faz sentido com `tipo: agenda` |
| Resultado dentro do texto | Planejado | Exige um modo leitura / renderizador no servidor |
| `{{ }}` inline | Planejado | Depende do item acima |
| Várias ordenações | Fora do v1 | Simplicidade do contrato |
| Ações (mover, aplicar tag) | Fora | A linguagem é somente leitura por decisão |
| `for`, variáveis, funções próprias | Nunca | Seria programação; CEL já cobre filtro e cálculo |
| Busca no conteúdo de PDF/EPUB | Não | Só metadados e texto indexado |
| Consultas remotas/rede | Nunca | O app é local e offline |


---

## 9. Backup, restauração e configurações

### 9.1 As abas de Configurações

| Aba | O que faz |
|---|---|
| **Arquivamento** | Filtra por tag, arquiva/exclui em massa, Auto-Tag e limpeza de imagens órfãs |
| **Restaurar** | Lista pacotes de `archives/` e devolve as notas aos caminhos originais |
| **Backup** | Baixa backup rápido, completo ou total; restaura um backup total |
| **Marcadores** | Configura os marcadores de tarefa (cor, atividade, contagem no badge) |
| **Agenda** | Fuso horário e antecedência do lembrete |
| **Rotina** | Liga/desliga o ícone e a página da rotina diária |
| **Ntfy** | Servidor, tópico e credenciais das notificações |
| **Colunas** | Quais campos do frontmatter podem virar colunas no Acervo |
| **Sinônimos** | Regras `termo → sinônimos` usadas na busca |

### 9.2 Os três backups

| Backup | Inclui | Pode restaurar? |
|---|---|---|
| **Rápido** | Notas (markdown, desenhos, Tabulator). Não inclui arquivos pesados nem `archives/` | Não (só download) |
| **Completo** | Notas + PDFs + anexos + imagens + demais arquivos (exceto `archives/`) | Não (só download) |
| **Total** | `manifest.json` + banco `db/ton618.db` + todo o `docs/` (inclusive `archives/`) | **Sim** |

O backup rápido e o completo são para levar o conteúdo para outro lugar. O **Total** é o único que
preserva configurações, agenda, marcadores e o estado inteiro — e o único que o app consegue
importar de volta.

### 9.3 Restaurar um Backup Total

**Configurações → Backup → Restaurar um Backup Total**:

1. escolha o arquivo `.zip`;
2. confirme (a operação **substitui tudo**);
3. aguarde: o estado atual vai para uma quarentena em `core/data/.pre-restore-<data>/`;
4. o servidor **reinicia sozinho**; espere alguns segundos e recarregue a página.

Se o arquivo não tiver o `manifest.json` ou o banco esperado, a restauração é recusada — isso evita
aplicar um ZIP qualquer por engano.

### 9.4 Sinônimos

A busca global expande sinônimos quando a consulta tem **uma única palavra de conteúdo**. O mapa
embutido já cobre termos comuns; o que você salva em **Configurações → Sinônimos** é somado e tem
prioridade.

Exemplo: `pc → computador, máquina` faz `pc` encontrar notas que só falam “computador”.

### 9.5 Colunas

**Configurações → Colunas** controla quais campos do frontmatter aparecem como colunas no Acervo.
Desmarcar um campo o esconde da tabela (o dado continua na nota). A coluna **Título** é especial:
ela sustenta a árvore de hierarquia e por isso permanece visível.


---

## 10. IA e terminal

### 10.1 MCP: acesso de IA ao acervo (somente leitura)

O TON-618 tem um servidor [MCP](https://modelcontextprotocol.io/) embutido. Ele permite que um
cliente de IA local — OpenCode, Claude Desktop, Cursor, Zed, Continue etc. — **leia** o acervo:
buscar, abrir notas, listar metadados, executar consultas e ver backlinks.

O servidor fala **stdio** (o cliente sobe o processo) e abre o SQLite em **modo somente leitura**.
Não existe ferramenta de escrita, rename ou delete. Quem inicia o processo tem acesso ao banco —
por isso ele é local por design, sem transporte HTTP e sem autenticação.

#### Pré-requisitos

O banco precisa existir (rode o app pelo menos uma vez). Compile o binário se ainda não existir:

```sh
cd core
go build -tags sqlite_fts5 -o ton618 ./cmd/server
```

#### Subir o servidor MCP

```sh
./core/ton618 mcp [--db CAMINHO] [--docs DIR]
```

| Flag | Padrão |
|---|---|
| `--db` | `DB_PATH`, depois `./data/ton618.db`, depois `<binário>/data/ton618.db` |
| `--docs` | `DOCS_DIR`, depois `<binário>/docs`, depois `./docs` (só contexto; a leitura vai no banco) |

O processo **não** migra o schema, não faz backup e não indexa nada. `stdout` é exclusivo do
protocolo JSON-RPC; logs vão para `stderr`.

#### As ferramentas

| Ferramenta | Para quê | Parâmetros |
|---|---|---|
| `search_notes` | Busca global (FTS5, sinônimos, acentos) com filtros `tags:` e `tipo:` | `query`, `limit` (1–50, padrão 10) |
| `get_note` | Conteúdo completo de uma nota, com título, tipo, tags e mtime | `file`, `offset`, `max_chars` (padrão 50000, máx. 200000) |
| `list_notes` | Metadados de todas as notas | `tipo`, `tag`, `ordenar` (`recentes`, `antigas`, `nome`), `limit` (1–200) |
| `query_notes` | Executa a linguagem de consulta | `consulta` (YAML), `nota` (raiz, para hierarquia) |
| `backlinks` | Links de entrada e saída de uma nota | `file` |

Caminhos são normalizados: `minha-nota` vira `notes/minha-nota.md`. O mesmo cuidado do app se
aplica: o FTS não indexa o corpo de desenhos, mapas e Tabulators — para esses, ache pelo título/tag
e leia com `get_note`.

#### Configurar no OpenCode

Adicione ao `opencode.json` (ou `opencode.jsonc`):

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "ton618": {
        "type": "local",
        "command": [
          "/caminho/para/ton618/core/ton618",
          "mcp",
          "--db",
          "/caminho/para/ton618/core/data/ton618.db"
        ],
        "codemode": false
      }
    }
  }
}
```

Confira a conexão:

```sh
opencode mcp list
```

Você deve ver `✓ ton618 connected`. Sem `--db`, o servidor procura `./data/ton618.db` no diretório
atual e depois `<binário>/data/ton618.db`. Se você inicia o OpenCode fora de `core/`, use caminhos
absolutos.

#### Outros clientes

Qualquer cliente MCP com transporte stdio serve. O comando é sempre:

```json
{
  "command": "/caminho/para/ton618",
  "args": ["mcp", "--db", "/caminho/para/core/data/ton618.db"]
}
```

O nome do bloco varia por cliente (Claude Desktop usa `mcpServers`).

#### Exemplos de pedidos para a IA

- “Procure no meu acervo tudo sobre atenção e me dê um resumo com as fontes.”
- “Liste minhas notas de livros que ainda não têm a tag `lido`.”
- “Quais tarefas pendentes mencionam o projeto TON-618?”
- “Leia a nota `arquitetura` e proponha um índice melhor.”
- “Quais notas apontam para `busca`?”
- “Execute uma consulta de notas com texto `foco` e mostre em tabela.”

A IA não consegue alterar nada. Se ela sugerir mudanças, você aplica manualmente no app.

### 10.2 Comandos no terminal

Além do MCP, você pode consultar o banco diretamente com `sqlite3`. Isso é útil para scripts,
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

#### Listar tarefas pendentes

```sh
sqlite3 -readonly -header -column "$DB" "
  SELECT file, type, text, section, line
  FROM todos
  WHERE status = 'pending'
  ORDER BY file, line;"
```

> `DONE:` é um marcador como outro qualquer; ele não muda o `status` para `completed`. Quem tem
> status `completed` são os checkboxes `- [x]`. Ajuste o `WHERE` conforme o que você quer ver.

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


## 11. Atalhos e gestos

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


## 12. Perguntas frequentes e solução de problemas

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
- Para excluir termos, não use `-`; use a linguagem de consulta com `onde: '!(...)'`.

### O `tipo:` da busca não encontra a nota

Use o vocabulário canônico: `nota`, `desenho`, `mapa_mental`, `tabulator`, `semanal`, `pdf`,
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

### As consultas não atualizam

Salve a nota ou clique no botão de atualizar do cartão. Resultados têm cache de 5 segundos e são
invalidados em qualquer escrita. Se a expressão estiver errada, o erro aparece no próprio cartão.

### O login entra em loop

Limpe cookie e `localStorage`, recarregue e entre de novo. Em Docker, confirme que o volume
`./data` está montado e que `AUTH_PASS` é o mesmo do container.

### “Endereço já em uso” ao subir no Docker

Outra instância (ou outro serviço) está na porta 6180. Pare o container antigo ou mude o mapeamento
de portas no `docker-compose.yml`.


---

## 13. Glossário

| Termo | Significado |
|---|---|
| **Acervo** | Conjunto de notas e arquivos, e também a página `/database` que os administra |
| **Backlink** | Link de entrada: uma nota que aponta para a atual |
| **Bloco de consulta** | Trecho ` ```consulta ` dentro de uma nota, com uma pergunta em YAML |
| **CEL** | *Common Expression Language*; a linguagem usada no `onde:` |
| **FTS5** | Mecanismo de busca textual do SQLite usado pela busca global |
| **Frontmatter** | Bloco YAML no topo da nota (`--- ... ---`) |
| **Hierarquia** | Relação `pai:`/filhas entre notas |
| **Marcador** | Palavra (`TODO`, `DOING`, `DONE`…) que transforma uma linha em tarefa |
| **MCP** | *Model Context Protocol*; o servidor que expõe o acervo para IA local |
| **Nó** | Item de um mapa mental (Markmap) |
| **Raiz** | Primeiro segmento do caminho (`notes/`, `pdfs/`, `epubs/`…) |
| **Tabulator** | Tipo de nota que é uma planilha de dados |
| **Tag** | Etiqueta no frontmatter ou `#tag` no corpo |
| **Wikilink** | Link no formato `[[nota]]` |
| **WAL** | *Write-Ahead Log*; modo de journal do SQLite (arquivos `-wal`/`-shm`) |
| **YAML** | Formato do frontmatter e dos blocos de consulta |


---

## Apêndice — cola rápida da linguagem de consulta

```text
tipo: notas | tarefas | hierarquia            (obrigatório)
onde: <expressão CEL>                         (opcional)
ordenar: campo [asc|desc]
limite: 1..200                                (padrão 20)
mostrar: lista | tabela | contagem | cartoes | arvore
colunas: [campo, ...]                         (só com tabela)

tags: [a, b]        sem-tags: [c]      texto: "fts5"
pasta: notes        marcador: [TODO]   estado: pendente|concluida|todas
de: Nome da nota
```

```text
Operadores:  && || !  == != < <= > >=  + - * / %  in  ( )
Macros:      exists  all  exists_one  map  filter  has  size
Texto:       contains startsWith endsWith matches size
Tempo:       agora  duration("720h")  timestamp("2026-01-01T00:00:00Z")
App:         dias(ts)  horas(ts)
n:           caminho titulo pasta classe tags mtime pai pai_resolvido filhas
             citada_por tarefas_abertas popularidade interacao
t:           texto marcador estado arquivo nota_titulo secao linha criado
```

---

*Esta ajuda acompanha o TON-618 e é lida em `/help`. Se você editar este arquivo, recarregue a
página para ver o resultado.*
