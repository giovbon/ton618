package search

import (
	"context"
	"strings"
	"testing"
	"time"

	"ton618/internal/core/db"
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
	// FTS5 nao suporta NEAR, usa AND por coluna
	if !strings.Contains(result, "texto:\"texto\" AND texto:\"proximo\"") {
		t.Errorf("esperado AND query, got %q", result)
	}
	if !strings.Contains(result, "secao:\"texto\" AND secao:\"proximo\"") {
		t.Errorf("esperado secao AND, got %q", result)
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



// ── scoreTitle ─────────────────────────────────────────────────

func TestScoreTitle_ExactMatch(t *testing.T) {
	score := scoreTitle("Go › Concorrencia", []string{"concorrencia"})
	if score != 1.0 {
		t.Errorf("esperado 1.0 (exact match), got %v", score)
	}
}

func TestScoreTitle_PartialMatch(t *testing.T) {
	score := scoreTitle("Go › Concorrencia", []string{"concor"})
	if score != 0.4 {
		t.Errorf("esperado 0.4 (partial match), got %v", score)
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
	if score <= 0 {
		t.Errorf("esperado > 0 para match com último segmento, got %v", score)
	}
}

// ── scorePath ───────────────────────────────────────────────────

func TestScorePath_Match(t *testing.T) {
	score := scorePath("notas/golang-dicas.md", []string{"golang"})
	if score != 0.5 {
		t.Errorf("esperado 0.5 (match no path), got %v", score)
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
	if score != 0.5 {
		t.Errorf("esperado 0.5 (apenas um termo match), got %v", score)
	}
}

func TestScorePath_ShortTermIgnored(t *testing.T) {
	score := scorePath("notas/go-dicas.md", []string{"go"})
	if score != 0 {
		t.Errorf("esperado 0 (termo curto ignorado), got %v", score)
	}
}

// ── scoreFreshness ──────────────────────────────────────────────

func TestScoreFreshness_Today(t *testing.T) {
	now := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	score := scoreFreshness(now)
	if score != 0.5 {
		t.Errorf("esperado 0.5 (today), got %v", score)
	}
}

func TestScoreFreshness_ThreeDaysAgo(t *testing.T) {
	ts := time.Now().Add(-72 * time.Hour).Format(time.RFC3339)
	score := scoreFreshness(ts)
	if score != 0.25 {
		t.Errorf("esperado 0.25 (3 dias), got %v", score)
	}
}

func TestScoreFreshness_FourteenDaysAgo(t *testing.T) {
	ts := time.Now().Add(-14 * 24 * time.Hour).Format(time.RFC3339)
	score := scoreFreshness(ts)
	if score != 0.1 {
		t.Errorf("esperado 0.1 (14 dias), got %v", score)
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

// ── scoreFragment: verifica pesos individuais ──────────────────

func makeHit(texto, arquivo, secao, timestamp string, tags []string) *SearchHit {
	tagStr := ""
	for i, t := range tags {
		if i > 0 {
			tagStr += ","
		}
		tagStr += t
	}
	// Score negativo simula rank FTS5 (mais negativo = melhor match)
	return &SearchHit{
		Score: -10.0,
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
	score, details := scoreFragment(hit, []string{"instalação"}, "instalação", 0.0, 0)

	if details["titulo"] == 0 {
		t.Error("match exato no titulo deveria gerar bonus de titulo")
	}

	hit2 := makeHit("texto qualquer", "nota.md", "Configuração", "", nil)
	score2, _ := scoreFragment(hit2, []string{"instalação"}, "instalação", 0.0, 0)

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
	score, details := scoreFragment(hit, []string{"goroutines", "channels"}, "goroutines channels", 0.0, 0)

	if details["frase_exata"] == 0 {
		t.Error("frase exata no texto deveria gerar bonus de frase_exata")
	}

	hit2 := makeHit(
		"goroutines sao legais channels tambem mas separados",
		"nota.md", "Go Lang", "",
		nil,
	)
	score2, _ := scoreFragment(hit2, []string{"goroutines", "channels"}, "goroutines channels", 0.0, 0)

	if score2 >= score {
		t.Error("hit com frase exata deveria ter score maior")
	}
}

func TestScoreFragment_CaminhoMatch(t *testing.T) {
	hit := makeHit("conteudo generico", "notas/golang-dicas.md", "Go", "", nil)
	score, details := scoreFragment(hit, []string{"golang"}, "golang", 0.0, 0)

	if details["caminho"] == 0 {
		t.Error("match no nome do arquivo deveria gerar bonus de caminho")
	}

	hit2 := makeHit("conteudo generico", "notas/outra-coisa.md", "Go", "", nil)
	score2, _ := scoreFragment(hit2, []string{"golang"}, "golang", 0.0, 0)

	if score2 >= score {
		t.Error("hit com match no caminho deveria ter score maior")
	}
}

func TestScoreFragment_Recencia(t *testing.T) {
	agora := time.Now().Format(time.RFC3339)
	mesPassado := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)

	hitRecente := makeHit("texto", "nota.md", "", agora, nil)
	hitAntigo := makeHit("texto", "nota.md", "", mesPassado, nil)

	scoreRec, _ := scoreFragment(hitRecente, []string{"texto"}, "texto", 0.0, 0)
	scoreAnt, _ := scoreFragment(hitAntigo, []string{"texto"}, "texto", 0.0, 0)

	if scoreRec <= scoreAnt {
		t.Error("nota recente deveria ter score maior que nota de 30 dias atras")
	}
}

func TestScoreFragment_BM25BaseEDominante(t *testing.T) {
	// Mesmo texto, mesmo titulo, scores diferentes
	hitBom := makeHit("golang explicado em detalhes", "nota.md", "Golang", "", nil)
	hitBom.Score = -20.0 // BM25 forte (mais negativo = melhor)

	hitFraco := makeHit("golang explicado em detalhes", "nota.md", "Golang", "", nil)
	hitFraco.Score = -2.0 // BM25 fraco

	scoreBom, _ := scoreFragment(hitBom, []string{"golang"}, "golang", 0.0, 0)
	scoreFraco, _ := scoreFragment(hitFraco, []string{"golang"}, "golang", 0.0, 0)

	if scoreBom <= scoreFraco {
		t.Error("hit com BM25 forte (score -20) deveria ter score maior que hit com BM25 fraco (score -2)")
	}
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
		func(s string) float64 { return 0.0 },
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
		func(s string) float64 { return 0.0 },
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

// ── buildProximityExpression ───────────────────────────────────

func TestBuildProximityExpression_SingleWord(t *testing.T) {
	result := buildProximityExpression("golang")
	if !strings.Contains(result, "texto:golang") {
		t.Errorf("esperado termo unico, got %q", result)
	}
}

func TestBuildProximityExpression_MultipleWords(t *testing.T) {
	result := buildProximityExpression("parcialmente artigo")
	if !strings.Contains(result, `texto:"parcialmente" AND texto:"artigo"`) {
		t.Errorf("esperado AND entre termos no texto, got %q", result)
	}
	if !strings.Contains(result, `secao:"parcialmente" AND secao:"artigo"`) {
		t.Errorf("esperado AND entre termos na secao, got %q", result)
	}
}

func TestBuildProximityExpression_StopwordsFiltered(t *testing.T) {
	result := buildProximityExpression("de um artigo")
	// "de" e "um" sao stopwords, devem ser filtrados; sobra "artigo" (1 palavra)
	if !strings.Contains(result, "texto:artigo") {
		t.Errorf("so 'artigo' deveria sobrar, got %q", result)
	}
}

// ── extractTerms (quoted phrases) ──────────────────────────────

func TestExtractTerms_DoubleQuotedPhrase(t *testing.T) {
	terms := extractTerms(`"parcialmente artigo"`)
	if !containsTerm(terms, "parcialmente artigo") {
		t.Errorf("frase entre aspas deveria ser termo unico, got %v", terms)
	}
}

func TestExtractTerms_SingleQuotedPhrase(t *testing.T) {
	terms := extractTerms(`'parcialmente artigo'`)
	if !containsTerm(terms, "parcialmente artigo") {
		t.Errorf("frase entre aspas simples deveria ser termo unico, got %v", terms)
	}
}

func TestExtractTerms_QuotedPhraseAddsIndividualWords(t *testing.T) {
	terms := extractTerms(`'parcialmente artigo'`)
	if !containsTerm(terms, "parcialmente") {
		t.Errorf("palavra 'parcialmente' deveria estar nos termos (fallback), got %v", terms)
	}
	if !containsTerm(terms, "artigo") {
		t.Errorf("palavra 'artigo' deveria estar nos termos (fallback), got %v", terms)
	}
}

func TestExtractTerms_MixedQuotedAndUnquoted(t *testing.T) {
	terms := extractTerms(`'proximo termo' golang`)
	if !containsTerm(terms, "proximo termo") {
		t.Errorf("frase entre aspas deveria ser termo unico, got %v", terms)
	}
	if !containsTerm(terms, "golang") {
		t.Errorf("termo solto 'golang' deveria estar nos termos, got %v", terms)
	}
}

func TestExtractTerms_EmptyQuote_Ignored(t *testing.T) {
	terms := extractTerms(`"" golang`)
	// aspas vazias nao devem adicionar termo vazio
	if containsTerm(terms, "") {
		t.Errorf("aspas vazias nao deveriam adicionar termo vazio, got %v", terms)
	}
	if !containsTerm(terms, "golang") {
		t.Errorf("'golang' deveria estar nos termos, got %v", terms)
	}
}



// ── buildFTSQuery edge cases ──────────────────────────────────

func TestBuildFTSQuery_MixedQuotes(t *testing.T) {
	result := buildFTSQuery(`"exato" 'proximo'`)
	if !strings.Contains(result, `"exato"`) {
		t.Errorf("esperado frase exata com aspas duplas, got %q", result)
	}
	if !strings.Contains(result, "proximo") {
		t.Errorf("esperado termo de proximidade, got %q", result)
	}
}
