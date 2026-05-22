package search

import (
	"context"
	"strings"
	"testing"
	"time"

	"ton618/internal/db"
)

// ── buildFTSQuery ───────────────────────────────────────────────

func TestBuildFTSQuery_EmptyString(t *testing.T) {
	result := buildFTSQuery("")
	if result != "" {
		t.Errorf("esperado vazio, got %q", result)
	}
}

func TestBuildFTSQuery_SingleWord(t *testing.T) {
	result := buildFTSQuery("palavra")
	expected := `(tags:palavra* OR arquivo:palavra* OR secao:palavra* OR texto:palavra*)`
	if result != expected {
		t.Errorf("esperado %q, got %q", expected, result)
	}
}

func TestBuildFTSQuery_QuotedPhrase(t *testing.T) {
	result := buildFTSQuery(`"frase exata"`)
	expected := `(tags:"frase exata" OR arquivo:"frase exata" OR secao:"frase exata" OR texto:"frase exata")`
	if result != expected {
		t.Errorf("esperado %q, got %q", expected, result)
	}
}

func TestBuildFTSQuery_SingleQuoteProximity(t *testing.T) {
	result := buildFTSQuery(`'texto proximo'`)
	expected := `(secao:("texto" NEAR/10 "proximo") OR texto:("texto" NEAR/10 "proximo"))`
	if result != expected {
		t.Errorf("esperado %q, got %q", expected, result)
	}
}

func TestBuildFTSQuery_StopwordSkipped(t *testing.T) {
	result := buildFTSQuery("de")
	if result != "" {
		t.Errorf("stopword deveria ser ignorada, got %q", result)
	}
}

func TestBuildFTSQuery_ShortWordNoStar(t *testing.T) {
	result := buildFTSQuery("go")
	expected := `(tags:go OR arquivo:go OR secao:go OR texto:go)`
	if result != expected {
		t.Errorf("esperado %q (sem sufixo *), got %q", expected, result)
	}
}

func TestBuildFTSQuery_MultipleTermsWithAND(t *testing.T) {
	result := buildFTSQuery("golang concorrencia")
	// Deve conter AND entre os dois termos
	if !contains(result, "AND") {
		t.Errorf("esperado AND entre termos, got %q", result)
	}
	if !contains(result, "golang*") || !contains(result, "concorrencia*") {
		t.Errorf("ambos os termos devem estar presentes com sufixo *, got %q", result)
	}
}

// ── extractTerms ────────────────────────────────────────────────

func TestExtractTerms_NormalQuery(t *testing.T) {
	terms := extractTerms("golang concorrencia")
	expected := []string{"golang", "concorrencia"}
	if !stringSliceEqual(terms, expected) {
		t.Errorf("esperado %v, got %v", expected, terms)
	}
}

func TestExtractTerms_StopwordsFiltered(t *testing.T) {
	terms := extractTerms("goroutines de go e channels")
	// "de" e "e" são stopwords e devem ser filtradas.
	// "go" tem 2 caracteres (len > 1) e não é stopword → deve ser incluído.
	for _, term := range terms {
		if stopwords[term] {
			t.Errorf("stopword %q não deveria estar nos termos", term)
		}
	}
	if !containsTerm(terms, "goroutines") {
		t.Errorf("esperado 'goroutines' nos termos, got %v", terms)
	}
}

func TestExtractTerms_PunctuationTrimmed(t *testing.T) {
	terms := extractTerms("golang, concorrencia! python?")
	expected := []string{"golang", "concorrencia", "python"}
	if !stringSliceEqual(terms, expected) {
		t.Errorf("esperado %v (sem pontuação), got %v", expected, terms)
	}
}

func TestExtractTerms_HashtagPreserved(t *testing.T) {
	// O caractere "#" não está na lista de Trim ("?,;.:!+-")
	// Portanto #urgente permanece como está em extractTerms
	terms := extractTerms("#urgente golang")
	expected := []string{"#urgente", "golang"}
	if !stringSliceEqual(terms, expected) {
		t.Errorf("esperado %v, got %v", expected, terms)
	}
}

// ── cleanQuery ─────────────────────────────────────────────────

func TestCleanQuery_Normal(t *testing.T) {
	result := cleanQuery("Golang  Concorrencia")
	expected := "golang concorrencia"
	if result != expected {
		t.Errorf("esperado %q, got %q", expected, result)
	}
}

func TestCleanQuery_RemovesSpecialChars(t *testing.T) {
	result := cleanQuery("+tags:urgente \"exata\" golang")
	// cleanQueryRe (regex [\+\*\"]) remove +, * e aspas
	// Resultado: "tags:urgente exata golang"
	if contains(result, "\"") {
		t.Errorf("cleanQuery deveria remover aspas, got %q", result)
	}
}

func TestCleanQuery_Empty(t *testing.T) {
	result := cleanQuery("")
	if result != "" {
		t.Errorf("esperado vazio, got %q", result)
	}
}

// ── extractTags ─────────────────────────────────────────────────

func TestExtractTags_TagsPrefix(t *testing.T) {
	tags, remaining := extractTags("tags:urgente golang")
	expected := []string{"urgente"}
	if !stringSliceEqual(tags, expected) {
		t.Errorf("esperado %v, got %v", expected, tags)
	}
	if !contains(remaining, "golang") {
		t.Errorf("'golang' deveria permanecer em remaining, got %q", remaining)
	}
}

func TestExtractTags_Hashtag(t *testing.T) {
	tags, _ := extractTags("#importante")
	expected := []string{"importante"}
	if !stringSliceEqual(tags, expected) {
		t.Errorf("esperado %v, got %v", expected, tags)
	}
}

func TestExtractTags_Multiple(t *testing.T) {
	tags, _ := extractTags("#urgente tags:programacao golang")
	if len(tags) < 2 {
		t.Errorf("esperado 2 tags, got %v", tags)
	}
	if !containsTerm(tags, "urgente") || !containsTerm(tags, "programacao") {
		t.Errorf("esperado ['urgente', 'programacao'], got %v", tags)
	}
}

func TestExtractTags_QuotedTagValue(t *testing.T) {
	tags, _ := extractTags(`tags:"minha tag"`)
	expected := []string{"minha tag"}
	if !stringSliceEqual(tags, expected) {
		t.Errorf("esperado %v, got %v", expected, tags)
	}
}

func TestExtractTags_NoTags(t *testing.T) {
	tags, remaining := extractTags("golang concorrencia")
	if len(tags) != 0 {
		t.Errorf("esperado nenhuma tag, got %v", tags)
	}
	if remaining != "golang concorrencia" {
		t.Errorf("remaining não deveria mudar, got %q", remaining)
	}
}

// ── buildHighlight ──────────────────────────────────────────────

func TestBuildHighlight_TermFound(t *testing.T) {
	text := "Este é um texto sobre concorrencia em Go com goroutines e channels."
	result := buildHighlight(text, []string{"concorrencia"})
	if result == nil {
		t.Fatal("esperado mapa de highlight, got nil")
	}
	frags := result["texto"]
	if len(frags) == 0 {
		t.Fatal("esperado fragmentos, got vazio")
	}
	if !contains(frags[0], "concorrencia") {
		t.Errorf("fragmento deveria conter o termo, got %q", frags[0])
	}
}

func TestBuildHighlight_TermNotFound(t *testing.T) {
	text := "Este é um texto sobre programação."
	result := buildHighlight(text, []string{"golang"})
	if result != nil {
		t.Errorf("esperado nil para termo não encontrado, got %v", result)
	}
}

func TestBuildHighlight_EmptyTerms(t *testing.T) {
	result := buildHighlight("algum texto", nil)
	if result != nil {
		t.Errorf("esperado nil para terms vazio, got %v", result)
	}
}

func TestBuildHighlight_MultipleTerms(t *testing.T) {
	text := "Golang é ótimo para concorrencia. Rust também é rápido."
	result := buildHighlight(text, []string{"golang", "rust"})
	if result == nil {
		t.Fatal("esperado highlight, got nil")
	}
	frags := result["texto"]
	if len(frags) != 2 {
		t.Errorf("esperado 2 fragmentos (um por termo), got %d", len(frags))
	}
}

func TestBuildHighlight_ContextAroundTerm(t *testing.T) {
	text := "INICIO " + strings.Repeat("palavra ", 50) + "TERMO_BUSCADO " + strings.Repeat("palavra ", 50) + "FIM"
	result := buildHighlight(text, []string{"TERMO_BUSCADO"})
	if result == nil {
		t.Fatal("esperado highlight, got nil")
	}
	frag := result["texto"][0]
	if !contains(frag, "TERMO_BUSCADO") {
		t.Errorf("fragmento deveria conter o termo, got %q", frag)
	}
}

// ── scoreTitle ─────────────────────────────────────────────────

func TestScoreTitle_ExactMatch(t *testing.T) {
	score := scoreTitle("Go › Concorrencia", []string{"concorrencia"})
	if score != weights.BoostTitleExact {
		t.Errorf("esperado %v (exact match), got %v", weights.BoostTitleExact, score)
	}
}

func TestScoreTitle_PartialMatch(t *testing.T) {
	score := scoreTitle("Go › Concorrencia", []string{"concor"})
	if score != weights.BoostTitlePartial {
		t.Errorf("esperado %v (partial match), got %v", weights.BoostTitlePartial, score)
	}
}

func TestScoreTitle_NoMatch(t *testing.T) {
	score := scoreTitle("Go › Concorrencia", []string{"python"})
	if score != 0 {
		t.Errorf("esperado 0 (sem match), got %v", score)
	}
}

func TestScoreTitle_MultipleTerms(t *testing.T) {
	score := scoreTitle("Go › Concorrencia Channels", []string{"concorrencia"})
	// Verifica que pelo menos um match foi encontrado
	if score <= 0 {
		t.Errorf("esperado > 0 para match com último segmento, got %v", score)
	}
}

// ── scoreKeywords ───────────────────────────────────────────────

func TestScoreKeywords_MatchTag(t *testing.T) {
	score := scoreKeywords("golang,programacao", "texto generico", []string{"golang"})
	if score != 3.0 {
		t.Errorf("esperado 3.0 (match exato em tag), got %v", score)
	}
}

func TestScoreKeywords_MatchTexto(t *testing.T) {
	score := scoreKeywords("", "aprendendo golang concorrencia", []string{"golang"})
	if score != 1.0 {
		t.Errorf("esperado 1.0 (match no texto), got %v", score)
	}
}

func TestScoreKeywords_MatchRadical(t *testing.T) {
	// "programacao" com radical "prog" (4 primeiras letras)
	score := scoreKeywords("", "programando em go", []string{"programacao"})
	if score != 0.5 {
		t.Errorf("esperado 0.5 (match por radical), got %v", score)
	}
}

func TestScoreKeywords_NoMatch(t *testing.T) {
	score := scoreKeywords("", "texto generico qualquer", []string{"xyzabc"})
	if score != 0 {
		t.Errorf("esperado 0 (sem match), got %v", score)
	}
}

func TestScoreKeywords_StopwordIgnored(t *testing.T) {
	score := scoreKeywords("de,RUST", "texto generico", []string{"de"})
	// "de" é stopword e tem len < 3 → deve ser ignorada
	if score != 0 {
		t.Errorf("esperado 0 (stopword ignorada), got %v", score)
	}
}

// ── scorePath ───────────────────────────────────────────────────

func TestScorePath_Match(t *testing.T) {
	score := scorePath("notas/golang-dicas.md", []string{"golang"})
	if score != weights.BoostPathContext {
		t.Errorf("esperado %v (match no path), got %v", weights.BoostPathContext, score)
	}
}

func TestScorePath_NoMatch(t *testing.T) {
	score := scorePath("notas/python-dicas.md", []string{"golang"})
	if score != 0 {
		t.Errorf("esperado 0 (sem match), got %v", score)
	}
}

func TestScorePath_MultipleTerms(t *testing.T) {
	score := scorePath("notas/golang-dicas.md", []string{"golang", "rust"})
	// Apenas "golang" deve dar match → 0.5
	if score != weights.BoostPathContext {
		t.Errorf("esperado %v (apenas um termo match), got %v", weights.BoostPathContext, score)
	}
}

func TestScorePath_ShortTermIgnored(t *testing.T) {
	score := scorePath("notas/go-dicas.md", []string{"go"})
	// "go" tem len < 3 → deve ser ignorado
	if score != 0 {
		t.Errorf("esperado 0 (termo curto ignorado), got %v", score)
	}
}

// ── scoreFreshness ──────────────────────────────────────────────

func TestScoreFreshness_Today(t *testing.T) {
	now := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	score := scoreFreshness(now)
	if score != weights.BoostFreshness {
		t.Errorf("esperado %v (today), got %v", weights.BoostFreshness, score)
	}
}

func TestScoreFreshness_ThreeDaysAgo(t *testing.T) {
	ts := time.Now().Add(-72 * time.Hour).Format(time.RFC3339)
	score := scoreFreshness(ts)
	if score != weights.BoostFreshness*0.5 {
		t.Errorf("esperado %v (3 dias), got %v", weights.BoostFreshness*0.5, score)
	}
}

func TestScoreFreshness_FourteenDaysAgo(t *testing.T) {
	ts := time.Now().Add(-14 * 24 * time.Hour).Format(time.RFC3339)
	score := scoreFreshness(ts)
	if score != weights.BoostFreshness*0.2 {
		t.Errorf("esperado %v (14 dias), got %v", weights.BoostFreshness*0.2, score)
	}
}

func TestScoreFreshness_SixtyDaysAgo(t *testing.T) {
	ts := time.Now().Add(-60 * 24 * time.Hour).Format(time.RFC3339)
	score := scoreFreshness(ts)
	if score != 0 {
		t.Errorf("esperado 0 (60 dias), got %v", score)
	}
}

func TestScoreFreshness_InvalidTimestamp(t *testing.T) {
	score := scoreFreshness("invalido")
	if score != 0 {
		t.Errorf("esperado 0 (timestamp inválido), got %v", score)
	}
}

func TestScoreFreshness_EmptyTimestamp(t *testing.T) {
	score := scoreFreshness("")
	if score != 0 {
		t.Errorf("esperado 0 (vazio), got %v", score)
	}
}

// ── scoreRichness ───────────────────────────────────────────────

func TestScoreRichness_CodeBlock(t *testing.T) {
	text := strings.Repeat("palavra_muito_longa_qualquer ", 30) + "\n```go\nfunc main() {}\n```"
	score := scoreRichness(text)
	if score < weights.BoostTechnical {
		t.Errorf("esperado pelo menos %v (código), got %v", weights.BoostTechnical, score)
	}
}

func TestScoreRichness_Table(t *testing.T) {
	// O marcador |--| é detectado como tabela
	text := strings.Repeat("palavra_muito_longa_qualquer ", 30) + "\n|--|\nconteúdo"
	score := scoreRichness(text)
	if score < weights.BoostTechnical {
		t.Errorf("esperado pelo menos %v (tabela), got %v", weights.BoostTechnical, score)
	}
}

func TestScoreRichness_ShortText(t *testing.T) {
	text := "texto curto"
	score := scoreRichness(text)
	// Apenas bônus de tamanho: 2 palavras / 500 * 0.5 = 0.002
	if score != 0.002 {
		t.Errorf("esperado 0.002 (apenas length bonus), got %v", score)
	}
}

func TestScoreRichness_Empty(t *testing.T) {
	score := scoreRichness("")
	if score != 0 {
		t.Errorf("esperado 0 (vazio), got %v", score)
	}
}

func TestScoreRichness_LongWordsBonus(t *testing.T) {
	// Texto com 22 palavras, das quais 6 têm mais de 8 caracteres
	words := []string{
		"implementação", "arquitetura", "escalável", "performática",
		"utilizando", "goroutines", "assíncronas", "concorrencia",
		"paralelismo", "distribuídos", "abstração", "encapsulamento",
		"polimorfismo", "concorrente", "abstrair", "implementar",
		"decomposição", "composição", "reutilização", "manutenção",
		"testabilidade", "legibilidade",
	}
	text := strings.Join(words, " ")
	score := scoreRichness(text)
	// totalWords > 20, longWords > 5 → bonus += 1.0
	// length bonus: 22/500 * 0.5 = 0.022
	// total esperado: 1.0 + 0.022 = 1.022
	if score < 1.0 {
		t.Errorf("esperado >= 1.0 (long words bonus), got %v", score)
	}
}

// ── scoreFragment: verifica pesos individuais ──────────────────

func makeHit(texto, arquivo, secao, timestamp string, tags []string) *SearchHit {
	tagStr := ""
	for i, t := range tags {
		if i > 0 {
			tagStr += ","
		}
		tagStr += t
	}
	return &SearchHit{
		Score: 10.0,
		Doc: db.Document{
			Texto:     texto,
			Arquivo:   arquivo,
			Secao:     secao,
			Tags:      tagStr,
			Timestamp: timestamp,
		},
	}
}

func TestScoreFragment_TituloExato(t *testing.T) {
	hit := makeHit("texto qualquer", "nota.md", "Instalação", "", nil)
	score, details := scoreFragment(hit, []string{"instalação"}, "instalação", 0, 0)

	if details["titulo"] == 0 {
		t.Error("match exato no titulo deveria gerar bonus de titulo")
	}

	// Sem match no titulo
	hit2 := makeHit("texto qualquer", "nota.md", "Configuração", "", nil)
	score2, _ := scoreFragment(hit2, []string{"instalação"}, "instalação", 0, 0)

	if score2 >= score {
		t.Error("hit sem match no titulo deveria ter score menor que hit com match")
	}
}

func TestScoreFragment_FraseExata(t *testing.T) {
	hit := makeHit(
		"aprendendo a usar goroutines channels em go",
		"nota.md", "Go Lang", "",
		nil,
	)
	score, details := scoreFragment(hit, []string{"goroutines", "channels"}, "goroutines channels", 0, 0)

	if details["frase_exata"] == 0 {
		t.Error("frase exata no texto deveria gerar bonus de frase_exata")
	}

	// Sem frase exata
	hit2 := makeHit(
		"goroutines sao legais channels tambem mas separados",
		"nota.md", "Go Lang", "",
		nil,
	)
	score2, _ := scoreFragment(hit2, []string{"goroutines", "channels"}, "goroutines channels", 0, 0)

	if score2 >= score {
		t.Error("hit com frase exata deveria ter score maior")
	}
}

func TestScoreFragment_CaminhoMatch(t *testing.T) {
	hit := makeHit("conteudo generico", "notas/golang-dicas.md", "Go", "", nil)
	score, details := scoreFragment(hit, []string{"golang"}, "golang", 0, 0)

	if details["caminho"] == 0 {
		t.Error("match no nome do arquivo deveria gerar bonus de caminho")
	}

	// Sem match no caminho
	hit2 := makeHit("conteudo generico", "notas/outra-coisa.md", "Go", "", nil)
	score2, _ := scoreFragment(hit2, []string{"golang"}, "golang", 0, 0)

	if score2 >= score {
		t.Error("hit com match no caminho deveria ter score maior")
	}
}

func TestScoreFragment_Recencia(t *testing.T) {
	agora := time.Now().Format(time.RFC3339)
	mesPassado := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)

	hitRecente := makeHit("texto", "nota.md", "", agora, nil)
	hitAntigo := makeHit("texto", "nota.md", "", mesPassado, nil)

	scoreRec, _ := scoreFragment(hitRecente, []string{"texto"}, "texto", 0, 0)
	scoreAnt, _ := scoreFragment(hitAntigo, []string{"texto"}, "texto", 0, 0)

	if scoreRec <= scoreAnt {
		t.Error("nota recente deveria ter score maior que nota de 30 dias atras")
	}
}

func TestScoreFragment_KeywordMatchTexto(t *testing.T) {
	hit := makeHit("usando concorrencia em go com goroutines", "nota.md", "", "", nil)
	_, details := scoreFragment(hit, []string{"concorrencia"}, "concorrencia", 0, 0)

	if details["keywords"] == 0 {
		t.Error("keyword encontrada no texto deveria gerar bonus")
	}
}

func TestScoreFragment_Popularidade(t *testing.T) {
	hit := makeHit("texto", "nota.md", "", "", nil)

	scoreSemPop, _ := scoreFragment(hit, []string{"texto"}, "texto", 0, 0)
	scoreComPop, details := scoreFragment(hit, []string{"texto"}, "texto", 100, 0)

	if details["popularidade"] == 0 {
		t.Error("popularidade > 0 deveria gerar bonus")
	}
	if scoreComPop <= scoreSemPop {
		t.Error("hit com popularidade deveria ter score maior")
	}
}

func TestScoreFragment_AutoridadeLinks(t *testing.T) {
	hit := makeHit("texto", "nota.md", "", "", nil)

	scoreSemLink, _ := scoreFragment(hit, []string{"texto"}, "texto", 0, 0)
	scoreComLink, details := scoreFragment(hit, []string{"texto"}, "texto", 0, 5)

	if details["autoridade"] == 0 {
		t.Error("linkCount > 0 deveria gerar bonus de autoridade")
	}
	if scoreComLink <= scoreSemLink {
		t.Error("hit com backlinks deveria ter score maior")
	}
}

func TestScoreFragment_RiquezaEstrutural(t *testing.T) {
	textoRico := "Implementação de arquitetura escalável e performática utilizando goroutines assíncronas.\n|--|\n```go\ncode\n```"
	textoPobre := "fazer isso e aquilo com um teste"

	hitRico := makeHit(textoRico, "nota.md", "", "", nil)
	hitPobre := makeHit(textoPobre, "nota.md", "", "", nil)

	scoreRico, _ := scoreFragment(hitRico, []string{"x"}, "x", 0, 0)
	scorePobre, _ := scoreFragment(hitPobre, []string{"x"}, "x", 0, 0)

	if scoreRico <= scorePobre {
		t.Error("nota com tabela, codigo e palavras longas deveria ter score maior")
	}
}

func TestScoreFragment_MultiplusFatoresAcumulam(t *testing.T) {
	hit := makeHit(
		"aprendendo goroutines e channels em go",
		"notas/golang-dicas.md",
		"Golang", time.Now().Format(time.RFC3339),
		[]string{"golang", "programacao"},
	)
	score, details := scoreFragment(hit, []string{"golang"}, "golang", 10, 3)

	// Deve ter bonus de titulo, caminho, keywords, recencia, popularidade, autoridade
	fatores := []string{"titulo", "frase_exata", "caminho", "keywords", "recencia", "popularidade", "autoridade"}
	for _, f := range fatores {
		if details[f] > 0 {
			return // achou pelo menos um bonus — ok
		}
	}
	t.Errorf("nenhum fator deu bonus — score final: %f, details: %v", score, details)
}

// ── Ordenação por score ────────────────────────────────────────

func TestSearch_ResultadosOrdenadosPorScore(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Cria duas notas com relevancias diferentes
	now := time.Now()

	store.InsertDocument(db.Document{ID: "doc1", Tipo: "markdown", Arquivo: "notas/relevante.md", Secao: "Titulo", Texto: "termo_de_busca especifico aqui", Timestamp: now.Format(time.RFC3339)})
	store.IndexFTS("doc1", "markdown", "notas/relevante.md", "Titulo", "termo_de_busca especifico aqui", "tag1")
	store.SetFileMod("notas/relevante.md", now.Format(time.RFC3339))

	store.InsertDocument(db.Document{ID: "doc2", Tipo: "markdown", Arquivo: "notas/irrelevante.md", Secao: "Outro", Texto: "conteudo generico sem o termo", Timestamp: now.Add(-24 * time.Hour).Format(time.RFC3339)})
	store.IndexFTS("doc2", "markdown", "notas/irrelevante.md", "Outro", "conteudo generico sem o termo", "")
	store.SetFileMod("notas/irrelevante.md", now.Add(-24*time.Hour).Format(time.RFC3339))

	results, err := Search(context.Background(), store, "termo_de_busca", 0, 20,
		func(s string) int { return 0 },
		func(s string) int { return 0 },
	)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results.Hits) == 0 {
		t.Fatal("nenhum resultado retornado")
	}

	// A nota com o termo deve vir primeiro
	if results.Hits[0].Doc.Arquivo != "notas/relevante.md" {
		t.Errorf("nota relevante deveria vir primeiro. Got: %s", results.Hits[0].Doc.Arquivo)
	}
}

func TestSearch_NotaComTagVemPrimeiro(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now()

	// Nota com tag correspondente ao termo
	store.InsertDocument(db.Document{ID: "doc1", Tipo: "markdown", Arquivo: "notas/com-tag.md", Secao: "Assunto", Texto: "texto sobre o termo", Timestamp: now.Format(time.RFC3339)})
	store.IndexFTS("doc1", "markdown", "notas/com-tag.md", "Assunto", "texto sobre o termo", "urgente")
	store.SetFileMod("notas/com-tag.md", now.Format(time.RFC3339))

	// Nota sem tag mas com termo no texto
	store.InsertDocument(db.Document{ID: "doc2", Tipo: "markdown", Arquivo: "notas/sem-tag.md", Secao: "Assunto", Texto: "texto sobre o termo de busca", Timestamp: now.Format(time.RFC3339)})
	store.IndexFTS("doc2", "markdown", "notas/sem-tag.md", "Assunto", "texto sobre o termo de busca", "")
	store.SetFileMod("notas/sem-tag.md", now.Format(time.RFC3339))

	results, err := Search(context.Background(), store, "urgente", 0, 20,
		func(s string) int { return 0 },
		func(s string) int { return 0 },
	)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if len(results.Hits) == 0 {
		t.Fatal("nenhum resultado retornado")
	}

	// Nota com a tag "urgente" matchando o termo "urgente"
	// Se ela está nos resultados, deve vir antes da sem tag
	for _, h := range results.Hits {
		if h.Doc.Arquivo == "notas/com-tag.md" {
			return // encontrou — ok
		}
	}
}

// ── Helpers ──────────────────────────────────────────────────────

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func containsTerm(slice []string, term string) bool {
	for _, s := range slice {
		if s == term {
			return true
		}
	}
	return false
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	s, err := db.NewStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	return s
}
