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

// ── Marcadores de TODO (configurados pelo usuário) ──
//
// Os marcadores são digitados no painel de configurações e interpolados no regex
// de extração. Estes testes cobrem os dois defeitos que existiam:
//  1. metacaractere de regex derrubava o regexp.MustCompile com panic — o panic
//     acontecia durante o save da nota (antes do SaveNote), então a nota não era
//     salva e nenhuma outra conseguia ser salva enquanto o marcador existisse;
//  2. metacaractere virava curinga silencioso, criando tarefas que não existem
//     no texto (ex: marcador "." casando qualquer `x: texto`).

func TestExtractTodos_MarcadorComMetacaractereNaoPanica(t *testing.T) {
	mTime := time.Now().UTC()

	// Cada marcador aqui quebraria o regex sem regexp.QuoteMeta ("(" e "[" são
	// sintaxe inválida, "*" e "?" no início são erro de compilação).
	tests := []struct {
		marker  string
		content string
	}{
		{"A(B", "A(B: texto com parêntese"},
		{"X[Y", "X[Y: texto com colchete"},
		{"Z*", "Z*: texto com asterisco"},
		{"W+", "W+: texto com mais"},
		{"V\\", `V\: texto com barra invertida`},
		{"C{2}", "C{2}: texto com chaves"},
		{"$FIM", "$FIM: texto com cifrão"},
	}

	for _, tc := range tests {
		t.Run(tc.marker, func(t *testing.T) {
			// Não deve entrar em panic (era o comportamento antigo).
			todos := ExtractTodos(tc.content, "nota.md", mTime, []string{tc.marker})

			if len(todos) != 1 {
				t.Fatalf("marcador %q: esperava 1 tarefa, got %d (%+v)", tc.marker, len(todos), todos)
			}
			// O tipo é o marcador LITERAL (não a versão interpretada pelo regex).
			if todos[0].Type != strings.ToUpper(tc.marker) {
				t.Errorf("marcador %q: Type = %q, want %q", tc.marker, todos[0].Type, strings.ToUpper(tc.marker))
			}
			if !strings.HasPrefix(todos[0].Text, "texto com") {
				t.Errorf("marcador %q: texto inesperado %q", tc.marker, todos[0].Text)
			}
		})
	}
}

func TestExtractTodos_MarcadorComPontoNaoViraCuringa(t *testing.T) {
	mTime := time.Now().UTC()

	// "." como marcador não pode casar "qualquer coisa: valor" (linha comum de
	// anotação, frontmatter, URL etc).
	todos := ExtractTodos("qualquer coisa: valor\nABC: outro valor", "nota.md", mTime, []string{"."})
	if len(todos) != 0 {
		t.Fatalf("marcador '.' não pode casar linhas arbitrárias, got %+v", todos)
	}

	// O literal "." continua funcionando.
	todos = ExtractTodos(".: ponto literal", "nota.md", mTime, []string{"."})
	if len(todos) != 1 || todos[0].Text != "ponto literal" {
		t.Fatalf("literal '.' deveria casar, got %+v", todos)
	}
}

func TestExtractTodos_SemMarcadoresNaoDetectaNada(t *testing.T) {
	mTime := time.Now().UTC()

	// Slice VAZIO = "nenhum marcador ativo" (usuário desativou todos no painel).
	// Antes o vazio caía nos marcadores padrão e as tarefas continuavam sendo
	// criadas mesmo com TODO/DOING/DONE desativados.
	conteudo := "TODO: nao deve aparecer\nDOING: nem isso\nDONE: nem aquilo\n- [ ] checkbox continua\n"
	todos := ExtractTodos(conteudo, "nota.md", mTime, []string{})

	for _, td := range todos {
		if td.Type != "TASK" {
			t.Errorf("sem marcadores ativos nada deveria ser detectado, veio %+v", td)
		}
	}
	if len(todos) != 1 || todos[0].Type != "TASK" {
		t.Errorf("esperava apenas o checkbox, got %+v", todos)
	}

	// nil = "não informado" → mantém o fallback para os marcadores padrão.
	todos = ExtractTodos("TODO: com defaults", "nota.md", mTime, nil)
	encontrou := false
	for _, td := range todos {
		if td.Type == "TODO" {
			encontrou = true
		}
	}
	if !encontrou {
		t.Errorf("nil deveria usar os marcadores padrão, got %+v", todos)
	}
}

func TestExtractTodos_MarcadoresDuplicadosEVazios(t *testing.T) {
	mTime := time.Now().UTC()

	// Vazio/duplicado não pode gerar alternativa vazia no regex
	// (`TODO||FIXME` casaria uma linha começando com ":").
	todos := ExtractTodos(": linha começando com dois pontos\nTODO: valido", "nota.md", mTime,
		[]string{"TODO", "todo", "  ", "", "TODO"})

	if len(todos) != 1 {
		t.Fatalf("esperava só 1 tarefa (TODO: valido), got %+v", todos)
	}
	if todos[0].Text != "valido" {
		t.Errorf("texto errado: %q", todos[0].Text)
	}
}

func TestIsValidTodoMarkerName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"Simples", "URGENTE", true},
		{"Com número", "BUG2", true},
		{"Com espaço", "FIX ME", true},
		{"Com hífen e underscore", "BUG-FIX_1", true},
		{"Com acento", "AÇÃO", true},
		{"Com ponto", "V1.2", true},
		{"Vazio", "", false},
		{"Só espaços", "   ", false},
		{"Reservado TASK", "TASK", false},
		{"Reservado task minúsculo", "task", false},
		{"Reservado com espaço", " TASK ", false},
		{"Parêntese (quebra regex)", "A(B", false},
		{"Colchete", "A[B", false},
		{"Ponto de interrogação (curinga)", "A?", false},
		{"Ampersand (quebra query string)", "A&B", false},
		{"Dois pontos", "TODO:", false},
		{"Traço do início (viraria opção)", "-TODO", false},
		{"Muito longo (33 chars)", strings.Repeat("A", 33), false},
		{"Limite (32 chars)", strings.Repeat("A", 32), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidTodoMarkerName(tc.input); got != tc.want {
				t.Errorf("IsValidTodoMarkerName(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsValidTodoMarkerColor(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"#3b82f6", true},
		{"#ABCDEF", true},
		{"#000000", true},
		{"", false},
		{"3b82f6", false},
		{"#fff", false},
		{"#3b82f6; background:red", false},
		{"red", false},
		// Espaço em volta é removido (o helper faz TrimSpace) — quem chama já
		// trimou, então aceitar é coerente.
		{"#3b82f6\n", true},
		{"#3b82f6\nred", false},
	}

	for _, tc := range tests {
		if got := IsValidTodoMarkerColor(tc.input); got != tc.want {
			t.Errorf("IsValidTodoMarkerColor(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
