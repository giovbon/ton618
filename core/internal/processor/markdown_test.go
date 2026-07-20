package processor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProcessMarkdown_Simples(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "simples.md")
	os.WriteFile(fp, []byte("<p>conteudo normal</p>"), 0644)

	now := time.Now()
	docs, links, tags := ProcessMarkdown(fp, "notes/simples.md", now, now)

	if len(docs) != 1 {
		t.Fatalf("esperado 1 doc, got %d", len(docs))
	}
	d := docs[0]
	if d.Arquivo != "notes/simples.md" {
		t.Errorf("arquivo errado: %q", d.Arquivo)
	}
	if d.Secao != SectionDefault {
		t.Errorf("secao deveria ser Geral, got %q", d.Secao)
	}
	if !strings.Contains(d.Texto, "conteudo normal") {
		t.Errorf("texto nao contem o conteudo: %q", d.Texto)
	}
	if d.ID == "" {
		t.Error("ID vazio")
	}
	if len(links) != 0 {
		t.Errorf("links inesperados: %v", links)
	}
	if len(tags) != 0 {
		t.Errorf("tags inesperadas: %v", tags)
	}
}

func TestProcessMarkdown_ComFrontmatter(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "fm.md")
	content := `---
title: Minha Nota
tags: [golang, programacao]
---
# Seção 1
conteudo aqui
## Sub-seção
mais conteudo`
	os.WriteFile(fp, []byte(content), 0644)

	now := time.Now()
	docs, _, tags := ProcessMarkdown(fp, "notes/fm.md", now, now)

	if len(docs) < 2 {
		t.Fatalf("esperado ao menos 2 fragmentos, got %d", len(docs))
	}
	if len(tags) != 2 {
		t.Fatalf("esperado 2 tags do frontmatter, got %d: %v", len(tags), tags)
	}
	if tags[0] != "golang" || tags[1] != "programacao" {
		t.Errorf("tags erradas: %v", tags)
	}
	// Todos os fragmentos devem ter as tags
	for _, doc := range docs {
		if len(doc.Tags) != 2 {
			t.Errorf("fragmento %q sem tags: %v", doc.Secao, doc.Tags)
		}
	}
}

func TestProcessMarkdown_Hashtags(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "hashtags.md")
	content := "#urgente #programacao\ntexto qualquer"
	os.WriteFile(fp, []byte(content), 0644)

	now := time.Now()
	_, _, tags := ProcessMarkdown(fp, "notes/hashtags.md", now, now)

	if len(tags) != 2 {
		t.Fatalf("esperado 2 hashtags, got %d: %v", len(tags), tags)
	}
	if tags[0] != "urgente" || tags[1] != "programacao" {
		t.Errorf("hashtags erradas: %v", tags)
	}
}

func TestProcessMarkdown_Wikilinks(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "wikilinks.md")
	content := "Veja tambem [[outra-nota]] e [[nota-complexa|titulo custom]]"
	os.WriteFile(fp, []byte(content), 0644)

	now := time.Now()
	_, links, _ := ProcessMarkdown(fp, "notes/wikilinks.md", now, now)

	if len(links) != 2 {
		t.Fatalf("esperado 2 wikilinks, got %d: %v", len(links), links)
	}
	if links[0] != "notes/outra-nota.md" || links[1] != "notes/nota-complexa.md" {
		t.Errorf("wikilinks errados: %v", links)
	}
}

func TestProcessMarkdown_SemHeaders(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "sem-header.md")
	content := "conteudo sem headers\noutra linha"
	os.WriteFile(fp, []byte(content), 0644)

	now := time.Now()
	docs, _, _ := ProcessMarkdown(fp, "notes/sem-header.md", now, now)

	if len(docs) != 1 {
		t.Fatalf("esperado 1 fragmento (Geral), got %d", len(docs))
	}
	if docs[0].Secao != SectionDefault {
		t.Errorf("secao deveria ser Geral, got %q", docs[0].Secao)
	}
}

func TestProcessMarkdown_ComHeaders(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "com-header.md")
	content := "texto antes do header\n# Título\nconteudo da secao\n## Sub-título\nmais conteudo"
	os.WriteFile(fp, []byte(content), 0644)

	now := time.Now()
	docs, _, _ := ProcessMarkdown(fp, "notes/com-header.md", now, now)

	if len(docs) < 2 {
		t.Fatalf("esperado ao menos 2 fragmentos, got %d", len(docs))
	}
	// O primeiro fragmento deve ser "Geral" (texto antes do primeiro header)
	if docs[0].Secao != SectionDefault {
		t.Errorf("primeira secao deveria ser Geral, got %q", docs[0].Secao)
	}
	// Deve ter Título > Sub-título como trail
	hasSub := false
	for _, d := range docs {
		if strings.Contains(d.Secao, "Sub-título") {
			hasSub = true
		}
	}
	if !hasSub {
		t.Error("fragmento com Sub-título nao encontrado")
	}
}

func TestProcessMarkdown_MediaLinks(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "media.md")
	content := "Veja a imagem: ![](/api/file?name=imgs/logo.png)"
	os.WriteFile(fp, []byte(content), 0644)

	now := time.Now()
	_, links, _ := ProcessMarkdown(fp, "notes/media.md", now, now)

	if len(links) != 1 {
		t.Fatalf("esperado 1 media link, got %d", len(links))
	}
	if links[0] != "imgs/logo.png" {
		t.Errorf("media link errado: %q", links[0])
	}
}

func TestProcessMarkdown_HashDeterministico(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "hash.md")
	content := "mesmo conteudo"
	os.WriteFile(fp, []byte(content), 0644)

	now, _ := time.Parse(time.RFC3339, "2025-01-01T12:00:00Z")
	docs1, _, _ := ProcessMarkdown(fp, "notes/hash.md", now, now)
	docs2, _, _ := ProcessMarkdown(fp, "notes/hash.md", now, now)

	if docs1[0].ID != docs2[0].ID {
		t.Errorf("IDs diferentes para mesmo conteudo: %q vs %q", docs1[0].ID, docs2[0].ID)
	}
	if docs1[0].Hash != docs2[0].Hash {
		t.Errorf("hashes diferentes: %q vs %q", docs1[0].Hash, docs2[0].Hash)
	}
}

func TestProcessMarkdown_ArquivoVazio(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "vazio.md")
	os.WriteFile(fp, []byte(""), 0644)

	now := time.Now()
	docs, _, _ := ProcessMarkdown(fp, "notes/vazio.md", now, now)

	if len(docs) != 0 {
		t.Errorf("arquivo vazio deveria ter 0 docs, got %d", len(docs))
	}
}

func TestProcessMarkdown_ArquivoInexistente(t *testing.T) {
	docs, links, tags := ProcessMarkdown("/caminho/inexistente.md", "notes/inexistente.md", time.Now(), time.Now())
	if docs != nil || links != nil || tags != nil {
		t.Error("arquivo inexistente deveria retornar nil")
	}
}

func TestExtractTitle_Normal(t *testing.T) {
	title := ExtractTitle("# Meu Titulo\nconteudo", "notes/titulo.md")
	if title != "Meu Titulo" {
		t.Fatalf("esperado 'Meu Titulo', got %q", title)
	}
}

func TestExtractTitle_SemHeader(t *testing.T) {
	// Sem heading markdown, deve retornar o nome do arquivo sem extensão
	title := ExtractTitle("conteudo direto", "notes/direto.md")
	if title != "direto" {
		t.Fatalf("esperado 'direto' (nome do arquivo), got %q", title)
	}
}

func TestExtractTitle_Vazio(t *testing.T) {
	title := ExtractTitle("", "notes/vazio.md")
	if title != "vazio" {
		t.Fatalf("esperado 'vazio' do filename, got %q", title)
	}
}

func TestHashFunc_Consistente(t *testing.T) {
	h1 := HashFunc("teste")
	h2 := HashFunc("teste")
	if h1 != h2 {
		t.Error("HashFunc nao e consistente")
	}
	if len(h1) != 32 {
		t.Errorf("tamanho do hash deveria ser 32 chars (16 bytes), got %d", len(h1))
	}
}

func TestCalculateHash(t *testing.T) {
	h1 := CalculateHash("secao", "texto", []string{"tag1", "tag2"})
	h2 := CalculateHash("secao", "texto", []string{"tag1", "tag2"})
	if h1 != h2 {
		t.Error("CalculateHash nao e consistente")
	}
	different := CalculateHash("secao", "texto-diferente", []string{"tag1"})
	if h1 == different {
		t.Error("texto diferente deveria gerar hash diferente")
	}
}

func TestProcessMarkdown_TagsFrontmatterMaisHashtags(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "ambos.md")
	content := `---
tags: [python]
---
#django esse texto tem ambas`
	os.WriteFile(fp, []byte(content), 0644)

	now := time.Now()
	_, _, tags := ProcessMarkdown(fp, "notes/ambos.md", now, now)

	// python do frontmatter + django + esse (mas "esse" é stopword? não no código)
	// HashtagRegex captura #django — "django"
	// O código não duplica tags
	if len(tags) < 2 {
		t.Fatalf("esperado ao menos 2 tags (python, django), got %d: %v", len(tags), tags)
	}
	hasPython := false
	hasDjango := false
	for _, tag := range tags {
		if tag == "python" {
			hasPython = true
		}
		if tag == "django" {
			hasDjango = true
		}
	}
	if !hasPython {
		t.Error("tag 'python' do frontmatter nao encontrada")
	}
	if !hasDjango {
		t.Error("hashtag #django nao encontrada")
	}
}

func TestProcessMarkdown_TypstNoHashtags(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "typst_note.md")
	content := `---
type: typst
tags: [relatorio]
---
#set page(paper: "a4")
#let x = 5
#align(center)[
  = Titulo
]
Este e um documento #typst.
`
	os.WriteFile(fp, []byte(content), 0644)

	now := time.Now()
	_, _, tags := ProcessMarkdown(fp, "notes/typst_note.md", now, now)

	// Espera-se apenas a tag "typst" (inserida automaticamente) e "relatorio" (do frontmatter).
	// Não devem ser extraídas diretivas sintáticas do typst como "set", "let", "align", etc.
	expectedTags := map[string]bool{
		"typst":     true,
		"relatorio": true,
	}

	for _, tag := range tags {
		if !expectedTags[tag] {
			t.Errorf("Tag inesperada extraída do documento Typst: %q (não deve extrair diretivas como hashtags)", tag)
		}
	}

	if len(tags) != 2 {
		t.Errorf("Esperado exatamente 2 tags (typst, relatorio), obteve %d: %v", len(tags), tags)
	}
}

func TestExtractTodos_RegexCaching(t *testing.T) {
	// Limpa o cache inicial
	todoRegexMu.Lock()
	cachedTodoRegex = nil
	todoRegexPattern = ""
	todoRegexMu.Unlock()

	// Primeira chamada - deve compilar e fazer cache
	mTime := time.Now()
	todos1 := ExtractTodos("TODO: fazer café", "nota.md", mTime, []string{"TODO", "FIXME"})
	if len(todos1) != 1 || todos1[0].Text != "fazer café" || todos1[0].Type != "TODO" {
		t.Errorf("Erro ao extrair TODO na primeira chamada: %+v", todos1)
	}

	todoRegexMu.RLock()
	firstRegex := cachedTodoRegex
	pattern1 := todoRegexPattern
	todoRegexMu.RUnlock()

	if firstRegex == nil {
		t.Fatal("Esperava que o regex estivesse em cache, mas está nil")
	}
	if pattern1 != "TODO|FIXME" {
		t.Errorf("Pattern errado: %q", pattern1)
	}

	// Segunda chamada com os mesmos marcadores - deve reusar o regex em cache
	todos2 := ExtractTodos("FIXME: consertar bug", "nota.md", mTime, []string{"TODO", "FIXME"})
	if len(todos2) != 1 || todos2[0].Text != "consertar bug" || todos2[0].Type != "FIXME" {
		t.Errorf("Erro ao extrair FIXME na segunda chamada: %+v", todos2)
	}

	todoRegexMu.RLock()
	secondRegex := cachedTodoRegex
	todoRegexMu.RUnlock()

	if firstRegex != secondRegex {
		t.Error("Esperava que o regex cacheado fosse reusado (mesmo ponteiro), mas mudou")
	}

	// Terceira chamada com marcadores diferentes - deve invalidar/recompilar
	todos3 := ExtractTodos("BUG: tela preta", "nota.md", mTime, []string{"BUG"})
	if len(todos3) != 1 || todos3[0].Text != "tela preta" || todos3[0].Type != "BUG" {
		t.Errorf("Erro ao extrair BUG na terceira chamada: %+v", todos3)
	}

	todoRegexMu.RLock()
	thirdRegex := cachedTodoRegex
	pattern3 := todoRegexPattern
	todoRegexMu.RUnlock()

	if thirdRegex == nil {
		t.Fatal("Esperava novo regex compilado em cache")
	}
	if pattern3 != "BUG" {
		t.Errorf("Novo pattern esperado 'BUG', obteve %q", pattern3)
	}
	if thirdRegex == firstRegex {
		t.Error("Esperava novo regex compilado para pattern diferente, mas o ponteiro é o mesmo")
	}
}

func TestExtractTodos_Checkboxes(t *testing.T) {
	content := `
# Tarefas
- [ ] Tarefa pendente 1
* [x] Tarefa concluída 2
- [X] Tarefa concluída 3 com X maiúsculo
* [ ] Tarefa pendente 2
`
	todos := ExtractTodos(content, "tasks.md", time.Now(), nil)
	if len(todos) != 4 {
		t.Fatalf("Esperava 4 tarefas de checkbox, obteve %d", len(todos))
	}

	expected := []struct {
		Text   string
		Status string
	}{
		{"Tarefa pendente 1", "pending"},
		{"Tarefa concluída 2", "completed"},
		{"Tarefa concluída 3 com X maiúsculo", "completed"},
		{"Tarefa pendente 2", "pending"},
	}

	for i, exp := range expected {
		if todos[i].Type != "TASK" {
			t.Errorf("Tarefa %d: esperado Type TASK, got %q", i, todos[i].Type)
		}
		if todos[i].Text != exp.Text {
			t.Errorf("Tarefa %d: esperado Text %q, got %q", i, exp.Text, todos[i].Text)
		}
		if todos[i].Status != exp.Status {
			t.Errorf("Tarefa %d: esperado Status %q, got %q", i, exp.Status, todos[i].Status)
		}
	}
}

func TestProcessMarkdown_TypstHeadings(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "typst_headings.md")
	content := `---
type: typst
---
= Introdução
Este é o começo.
== Subseção A
Detalhes aqui.
`
	os.WriteFile(fp, []byte(content), 0644)

	now := time.Now()
	docs, _, _ := ProcessMarkdown(fp, "notes/typst_headings.md", now, now)

	if len(docs) != 2 {
		t.Fatalf("esperado 2 fragmentos (Introdução e Introdução › Subseção A), got %d", len(docs))
	}

	if docs[0].Secao != "Introdução" {
		t.Errorf("primeira seção errada: %q", docs[0].Secao)
	}
	if !strings.Contains(docs[0].Texto, "Este é o começo.") {
		t.Errorf("texto da primeira seção errado: %q", docs[0].Texto)
	}

	if docs[1].Secao != "Introdução › Subseção A" {
		t.Errorf("segunda seção errada: %q", docs[1].Secao)
	}
	if !strings.Contains(docs[1].Texto, "Detalhes aqui.") {
		t.Errorf("texto da segunda seção errado: %q", docs[1].Texto)
	}
}

func TestExtractTitle_Typst(t *testing.T) {
	content := `---
type: typst
---
= Título Principal Typst
== Subtítulo
`
	title := ExtractTitle(content, "notes/titulo.md")
	if title != "Título Principal Typst" {
		t.Errorf("esperado 'Título Principal Typst', got %q", title)
	}
}

func TestExtractTodos_Typst(t *testing.T) {
	content := `---
type: typst
---
= Seção Typst
TODO: Minha tarefa
`
	todos := ExtractTodos(content, "notes/todos.md", time.Now(), nil)
	if len(todos) != 1 {
		t.Fatalf("esperado 1 todo, got %d", len(todos))
	}
	if todos[0].Section != "Seção Typst" {
		t.Errorf("esperado seção 'Seção Typst', got %q", todos[0].Section)
	}
	if todos[0].Text != "Minha tarefa" {
		t.Errorf("esperado todo text 'Minha tarefa', got %q", todos[0].Text)
	}
}

func TestProcessMarkdown_RegressoesIsolamento(t *testing.T) {
	dir := t.TempDir()

	// 1. Caso Markdown normal com '=' no corpo (não deve gerar seções por '=')
	fpNormal := filepath.Join(dir, "normal.md")
	contentNormal := `# Título MD
Este é um documento MD normal.
a = b
====================
`
	os.WriteFile(fpNormal, []byte(contentNormal), 0644)
	now := time.Now()
	docsNormal, _, _ := ProcessMarkdown(fpNormal, "notes/normal.md", now, now)

	if len(docsNormal) != 1 {
		t.Fatalf("MD normal: esperado 1 fragmento (Título MD), got %d", len(docsNormal))
	}
	if docsNormal[0].Secao != "Título MD" {
		t.Errorf("MD normal: seção incorreta: %q", docsNormal[0].Secao)
	}

	// 2. Caso Typst com '#' no corpo (não deve gerar seções por '#')
	fpTypst := filepath.Join(dir, "typst_iso.md")
	contentTypst := `---
type: typst
---
= Título Typst
#set page(paper: "a4")
#let x = 5
`
	os.WriteFile(fpTypst, []byte(contentTypst), 0644)
	docsTypst, _, tagsTypst := ProcessMarkdown(fpTypst, "notes/typst_iso.md", now, now)

	if len(docsTypst) != 1 {
		t.Fatalf("Typst: esperado 1 fragmento (Título Typst), got %d", len(docsTypst))
	}
	if docsTypst[0].Secao != "Título Typst" {
		t.Errorf("Typst: seção incorreta: %q", docsTypst[0].Secao)
	}

	// Verifica se os comandos Typst '#' não foram extraídos como tags
	for _, tag := range tagsTypst {
		if tag == "set" || tag == "let" || tag == "page" {
			t.Errorf("Typst: tag inválida extraída de diretiva '#': %q", tag)
		}
	}
}

func TestExtractTitle_RegressoesIsolamento(t *testing.T) {
	// Markdown normal com '=' no início de linhas (não deve extrair '=' como título)
	contentNormal := `
= Título MD Falso
# Título MD Real
`
	titleNormal := ExtractTitle(contentNormal, "notes/normal.md")
	if titleNormal != "Título MD Real" {
		t.Errorf("MD normal: esperado 'Título MD Real', got %q", titleNormal)
	}

	// Typst com '#' no início de linhas (não deve extrair '#' como título)
	contentTypst := `---
type: typst
---
#set page(paper: "a4")
= Título Typst Real
`
	titleTypst := ExtractTitle(contentTypst, "notes/typst.md")
	if titleTypst != "Título Typst Real" {
		t.Errorf("Typst: esperado 'Título Typst Real', got %q", titleTypst)
	}
}
