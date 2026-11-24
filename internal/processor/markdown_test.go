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

// TestProcessMarkdown_TypeMarkmapGaranteTagCanonica garante que o frontmatter
// "type: markmap" (ou "type: mindmap") adiciona a tag canônica "markmap" às
// tags do arquivo. Sem ela, o ícone da nota cai para "nota comum" na sidebar
// (DetectNoteType depende de tags + caminho, nunca do conteúdo).
func TestProcessMarkdown_TypeMarkmapGaranteTagCanonica(t *testing.T) {
	now := time.Now()

	for _, typeStr := range []string{"markmap", "mindmap"} {
		content := "---\ntype: " + typeStr + "\n---\n# Titulo\n- Topico 1\n- Topico 2"
		_, _, tags := ProcessMarkdownContent([]byte(content), "notes/meu-mapa.md", now, now)

		found := false
		for _, tag := range tags {
			if tag == "markmap" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("type: %s — esperado tag canônica \"markmap\" nas tags, got %v", typeStr, tags)
		}
	}
}

// O tipo do frontmatter é normalizado uma única vez, aceitando `tipo:` (chave
// canônica gravada pelo app) e `type:` (legado), em qualquer caixa. Antes,
// `type: Drawing` não limpava o JSON do Excalidraw e `tipo: tabulator` não
// disparava a indexação célula a célula.
func TestProcessMarkdownContent_FrontmatterTipoNormalizado(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name           string
		fm             string
		body           string
		wantTag        string
		wantDocs       bool
		wantDefaultSec bool
	}{
		{"type Drawing", "type: Drawing", `{"elements":[]}`, "drawing", false, false},
		{"tipo desenho", "tipo: desenho", `{"elements":[]}`, "drawing", false, false},
		{"type TABULATOR", "type: TABULATOR", tabulatorSample, "tabulator", true, false},
		{"tipo tabulator", "tipo: tabulator", tabulatorSample, "tabulator", true, false},
		{"tipo mapa mental", "tipo: mapa mental", "# Titulo\n- Topico", "markmap", true, false},
		{"tipo desconhecido", "tipo: foo", "conteudo comum", "", true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := "---\n" + tc.fm + "\n---\n" + tc.body
			docs, _, tags := ProcessMarkdownContent([]byte(content), "notes/teste.md", now, now)

			if tc.wantTag != "" {
				found := false
				for _, tag := range tags {
					if tag == tc.wantTag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("esperado tag canônica %q, got %v", tc.wantTag, tags)
				}
			}
			if tc.wantDocs && len(docs) == 0 {
				t.Error("esperado documentos indexados, got 0")
			}
			if !tc.wantDocs && len(docs) != 0 {
				t.Errorf("esperado 0 documentos (corpo limpo), got %d", len(docs))
			}
			for _, d := range docs {
				if d.Secao == SectionDefault && !tc.wantDefaultSec {
					t.Errorf("JSON/corpo especial não deveria ser indexado como markdown (seção %q)", d.Secao)
				}
			}
		})
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
}

func TestExtractTitle_RegressoesIsolamento(t *testing.T) {
	// Markdown normal com '=' no início de linhas (não deve extrair '=' com o título)
	contentNormal := `
= Título MD Falso
# Título MD Real
`
	titleNormal := ExtractTitle(contentNormal, "notes/normal.md")
	if titleNormal != "Título MD Real" {
		t.Errorf("MD normal: esperado 'Título MD Real', got %q", titleNormal)
	}
}
