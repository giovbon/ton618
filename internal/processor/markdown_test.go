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
