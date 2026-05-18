# TON-618 — Guia de Uso

O TON-618 é um gerenciador de conhecimento pessoal focado em **capturar e buscar sem atrito**.
Tudo o que você adiciona é indexado instantaneamente e recuperável por busca full-text.

---

## 🔍 Busca

Digite qualquer termo na barra de pesquisa. O motor Bleve busca em **todo o conteúdo** das suas notas.

| Botão | Função |
|-------|--------|
| :icon-compact: | Alterna entre **Modo Compacto** (notas inteiras) e **Modo Fragmentos** (trechos com score) |
| :icon-map: | Abre o **Mapa Semântico** — visualização 2D das suas notas |

### Busca Global

| Tipo | Sintaxe | O que faz |
|------|---------|-----------|
| **Simples** | `termo` | Busca por aproximação no texto e título |
| **Exata** | `"frase exata"` | Busca o termo exatamente como escrito |
| **Obrigatória** | `+importante` | A nota **tem que ter** esse termo |
| **Exclusão** | `-rascunho` | **Esconde** notas com esse termo |
| **Tag** | `#ideia` | Filtra por hashtag |
| **Semântica** | (ativar no :icon-settings:) | Busca por sentido, não por palavra exata |

### Super Busca (Dataview)

Quando a consulta começa com `TABLE` ou `LIST`, os resultados viram uma tabela dinâmica baseada no Frontmatter das suas notas.

Sintaxe: `COMANDO [campos] [FROM fonte] [WHERE condições] [SORT campo ORDEM]`

| Elemento | Exemplo | Descrição |
|----------|---------|-----------|
| **TABLE** | `TABLE status, prioridade` | Exibe campos do frontmatter em tabela |
| **LIST** | `LIST` | Lista simples com nomes de arquivos |
| **FROM** | `FROM #projeto` | Filtra por tag ou pasta (`FROM notes/`) |
| **WHERE** | `WHERE status == "fazendo"` | Filtra com `==`, `!=`, `>`, `<`, `AND` |
| **SORT** | `SORT file.mtime DESC` | Ordena `ASC` ou `DESC` |
| **count()** | `TABLE count(file.name) FROM #trabalho` | Conta itens |

**Metadados automáticos:** `file.name`, `file.mtime`, `file.size`, `file.path`

**Exemplos práticos:**

```
TABLE prioridade, status FROM #projeto WHERE status != "concluído" SORT prioridade ASC
TABLE file.mtime SORT file.mtime DESC
LIST FROM notes/
TABLE count(file.name) FROM #projeto
```

---

## ✏️ Editor

Clique em qualquer nota para abrir o editor WYSIWYG. A formatação é aplicada **inline**.

### Atalhos do Editor

| Atalho | Ação |
|--------|------|
| `Ctrl+S` | Salvar nota |
| `Ctrl+B` | **Negrito** |
| `Ctrl+I` | *Itálico* |
| `Ctrl+K` | Criar [[WikiLink]] |
| `Ctrl+D` | Selecionar próxima ocorrência |
| `Tab` | Indentar |
| `/` | Abrir menu de Slash Commands |

### Slash Commands (`/`)

Digite `/` no início de uma linha: `/h1`, `/h2`, `/h3`, `/bullet`, `/ordered`, `/quote`, `/code`, `/divider`, `/image`, `/table`

---

## 🔗 WikiLinks

Use `[[Nome da Nota]]` para criar links entre notas. O autocomplete sugere notas existentes enquanto você digita.

- `Ctrl+K` insere a sintaxe `[[ ]]` automaticamente
- Links para notas inexistentes abrem uma nota nova ao clicar
- Ao deletar uma nota, anexos vinculados são removidos em cascata

---

## 🏷️ Links Semânticos

Diferente dos links comuns, os **Links Semânticos** servem para marcar conceitos, tópicos ou entidades importantes que conectam suas ideias no **Mapa Estruturado**.

Use a sintaxe `@[Nome do Tópico]` para criar um link semântico.

- **Conexão de Ideias:** Notas que compartilham o mesmo `@[Conceito]` aparecem conectadas no grafo.
- **Diferenciação:** Use para tópicos que não são necessariamente arquivos físicos, mas temas que atravessam várias notas.
- **Navegação:** Clicar em um link semântico no editor abre o **Mapa Estruturado** focado naquele tópico.

---

## 🌌 Mapa Semântico

Visualiza suas notas como **estrelas num céu 2D**. Notas similares aparecem próximas automaticamente via IA.

- Ative pelo botão :icon-map: na barra de pesquisa.
- Notas com conteúdo parecido orbitam os mesmos clusters.
- Scroll para zoom, arraste para navegar.
- Clique na estrela para abrir a nota.

---

## 🕸️ Mapa Estruturado

O **Mapa Estruturado** exibe as conexões explícitas que você criou através de **Links Semânticos** (`@[...]`).

- Ative pelo botão de camadas no cabeçalho.
- Exibe tópicos como nós e notas como conexões.
- **Arraste e Solte:** Organize manualmente a posição dos nós (as posições são salvas automaticamente).
- **Interação:** Clique nos tópicos para navegar entre as notas relacionadas.
- Útil para criar mapas mentais ou ontologias personalizadas do seu conhecimento.

---

## 📤 Upload de Arquivos

| Botão | Função | Formatos |
|-------|--------|----------|
| :icon-note: | **Nova Nota do Dia** — cria nota com a data atual | `.md` |
| :icon-link: | **Capturar Link** — extrai conteúdo de qualquer URL | URL |
| :icon-camera: | **OCR de Imagem** — extrai texto da imagem e cria nota | `.png`, `.jpg` |
| :icon-pdf: | **Upload PDF** — extrai texto e cria nota vinculada | `.pdf` |
| :icon-bundle: | **Bundle Zip** — envia múltiplos arquivos de uma vez | `.zip` |

---

## 🏷️ Tags

Gerencie tags no frontmatter das notas:

- Adicione no editor: `tags: [importante, projeto-x]`
- Filtre por tag na busca: `#importante`
- Autocomplete ao digitar `#`
- Tags são indexadas e pesquisáveis

---

## ⚙️ Configurações

Acesse pelo botão :icon-settings: no painel superior.

**Aba Pesos:** Ajuste o ranking de busca — **α (Alpha)** controla o peso semântico vs lexical, **Recência** dá prioridade a notas novas, **Autoridade** valoriza notas com muitos backlinks.

**Aba Visão:** Configure o motor de IA — ativar/desativar busca semântica, ajustar threshold de similaridade.

**Aba Backup:** Baixar `.zip` com todos os seus documentos, ver tamanho antes de baixar.

**Aba Manutenção:** Limpeza de referências órfãs, remoção de conteúdo não utilizado.

---

## 🔄 Sincronização

O TON-618 monitora automaticamente a pasta `docs/`:

- Arquivos novos → indexados em segundos
- Edições → detectadas e reindexadas
- Deleções → removidas do índice

Use :icon-sync: para forçar uma sincronização manual imediata.

---

## ⌨️ Atalhos Globais

| Atalho | Ação |
|--------|------|
| `Digitar` | Foco na barra de busca |
| `Esc` | Fechar editor / modal |
| `Enter` (na busca) | Executar pesquisa |

---

## 💡 Dicas

1. Use :icon-note: para criar uma nota com a data de hoje
2. Clique :icon-sync: antes de sair para garantir que tudo foi indexado
3. Use `tags: [projeto, urgente]` no topo da nota
4. [[WikiLinks]] conectam notas relacionadas para navegação rápida
5. Use `@[Conceito]` para criar conexões explícitas no **Mapa Estruturado**
6. Use a aba Backup no :icon-settings: para baixar seus dados regularmente
