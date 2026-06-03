
---
<!--
## 📖 Índice

1. [Visão Geral](#-visão-geral)
2. [Notas Markdown](#-notas-markdown)
3. [Frontmatter YAML](#-frontmatter-yaml)
4. [Captura de Conteúdo](#-captura-de-conteúdo)
5. [Busca e Filtros](#-busca-e-filtros)
6. [Tags e Hashtags](#-tags-e-hashtags)
7. [Links entre Notas (Wikilinks)](#-links-entre-notas-wikilinks)
8. [Mapa Semântico](#-mapa-semântico)
9. [Editor Rich Text](#-editor-rich-text)
10. [Configurações](#-configurações)
11. [Teclas de Atalho](#-teclas-de-atalho)
12. [Desabilitando Keywords](#-desabilitando-keywords)
-->

# 🌌 Bem-vindo ao **TON-618** v2

**TON-618** é o seu sistema pessoal de gestão de conhecimento (PKM) — um "buraco negro" onde você armazena, organiza e busca suas notas, artigos, PDFs e ideias.

## Visão Geral

### O que o TON-618 faz?

- 📝 **Cria e edita notas** em Markdown com editor rich text (TipTap)
- 🔍 **Busca inteligente** combinando busca textual (FTS5) com palavras-chave extraídas
- 🏷️ **Organiza com tags** — via frontmatter YAML ou hashtags no texto
- 🔗 **Conecta notas** com wikilinks `[[link]]`
- 🌐 **Captura artigos da web** e transcrições do YouTube
- 📕 **Indexa PDFs** para busca
- 💾 **Backup completo** em ZIP

---

## Notas Markdown

Todas as notas são arquivos `.md` salvos na pasta `docs/notes/`.

### Criar uma nova nota

Clique em **📄 NOTA** no cabeçalho. Uma nova nota será criada com um nome único.

### Salvar

Clique no ícone **💾** no editor ou use `Ctrl+S` (`Cmd+S` no Mac).

### Estrutura básica

```markdown
---
title: Minha Nota
tags: [tag1, tag2]
created: 2025-01-01
---

# Conteúdo da nota

Escreva seu texto em **Markdown**.

Use #hashtags e [[wikilinks]] para conectar ideias.
```

## Frontmatter YAML

O frontmatter é o bloco `---` no início do arquivo. Suporta qualquer propriedade YAML.

## Captura de Conteúdo

### Artigos da Web

Clique em **🌐 CAPTURA** no cabeçalho, cole a URL de um artigo, e o TON-618:

1. Baixa o artigo com **Readability** (modo leitura)
2. Converte para Markdown
3. Cria uma nova nota com o conteúdo

### YouTube

Cole a URL de um vídeo do YouTube. O sistema:

1. Extrai o título do vídeo
2. Baixa a **transcrição completa**
3. Salva como nota markdown com tag `#youtube`

---

## Busca e Filtros

### Modo Notas (padrão)

Filtra pelo **nome do arquivo** e **palavras-chave** extraídas (RAKE).

Use `#tags` para filtrar por tag específica:
```
#javascript #react
```

### Modo Busca

Alterna para **busca* com FTS5, buscando no conteúdo completo das notas.

### Dicas de busca

- `"frase exata"` — busca entre aspas
- `tags:javascript` — filtrar por tag
- `#react` — hashtag na barra de busca

---

## Tags e Hashtags

### Frontmatter

```yaml
tags: [javascript, react, tutorial]
```

### Hashtags no texto

Use `#tag` no corpo da nota para criar tags automaticamente:

```markdown
Neste #tutorial vamos aprender #react.
```

As tags aparecem na listagem de notas e podem ser usadas para filtrar. Tags são salvas em minúsculas.

---

## Configurações

Clique em **⚙️** no cabeçalho para abrir as configurações:

| Aba | Função |
|---|---|
| 🗂️ **Arquivamento** | Arquivar ou excluir notas em lote |
| 📦 **Restaurar** | Restaurar arquivos ZIP de notas arquivadas |
| 💾 **Backup** | Baixar ZIP completo de todas as notas |
| 🚫 **Stopwords** | Gerenciar stopwords personalizadas (RAKE) |

---

## Teclas de Atalho

| Atalho | Ação |
|---|---|
| `Ctrl+S` | Salvar nota |
| `Ctrl+B` | Negrito (no editor) |
| `Ctrl+I` | Itálico (no editor) |
| `Ctrl+K` | Inserir link |
| `Enter` | No frontmatter, adiciona nova linha |

---

## Desabilitando Keywords

Por padrão, o TON-618 extrai automaticamente as **1|3|5 palavras-chave mais relevantes** de cada nota (de acordo com o tamanho da nota) usando o algoritmo **RAKE**.

Você pode **desabilitar** essa extração de duas formas:

### 1. Propriedade no frontmatter

Adicione `no_keywords: true` ao frontmatter YAML:

```yaml
---
title: Rascunho aleatório
tags: [rascunho]
no_keywords: true
---
```

### 2. Tag especial

Adicione a tag `no-keywords` às tags:

```yaml
---
tags: [temporario, no-keywords]
---
```

Útil para notas de rascunho, listas de compras, ou qualquer nota onde as keywords não são relevantes para busca.
