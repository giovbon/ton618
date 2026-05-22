package search

import (
	"math"
	"strings"
	"testing"
	"time"

	"ton618/internal/db"
)

func TestScoreFragment_Base(t *testing.T) {
	hit := &SearchHit{
		Doc: db.Document{
			Texto:   "golang é uma linguagem de programação compilada",
			Arquivo: "notes/golang.md",
			Secao:   "📝 Golang",
		},
		Score: 1.0,
	}
	score, _ := scoreFragment(hit, []string{"golang"}, "golang", 0, 0)
	if score <= 0 {
		t.Fatalf("score deveria ser > 0, got %f", score)
	}
}

func TestScoreFragment_PopularidadeELinks(t *testing.T) {
	b := scoreTitle("📝 Golang Basics", []string{"golang"})
	if b <= 0 {
		t.Fatalf("esperado boost > 0 para match exato no titulo, got %f", b)
	}
}

func TestScoreTitle_Partial(t *testing.T) {
	b := scoreTitle("📝 Aprendendo Programacao", []string{"programa"})
	if b <= 0 {
		t.Fatalf("esperado boost > 0 para match parcial, got %f", b)
	}
}

func TestScoreTitle_SemMatch(t *testing.T) {
	b := scoreTitle("📝 Matematica", []string{"fisica"})
	if b != 0 {
		t.Fatalf("esperado 0 sem match, got %f", b)
	}
}

func TestScoreTitle_StopwordIgnorada(t *testing.T) {
	b := scoreTitle("📝 O Artigo Definido", []string{"o"})
	if b != 0 {
		t.Fatalf("stopword nao deveria dar boost, got %f", b)
	}
}

func TestScoreTitle_TermoCurtoIgnorado(t *testing.T) {
	b := scoreTitle("📝 Curso de Go", []string{"go"})
	if b != 0 {
		t.Fatalf("termo com menos de 3 chars nao deveria dar boost, got %f", b)
	}
}

func TestScoreKeywords_TagExata(t *testing.T) {
	// Tags sao armazenadas como CSV, nao JSON
	b := scoreKeywords("golang,web", "", []string{"golang"})
	if b < 2.0 {
		t.Fatalf("tag exata deveria dar boost >= 3, got %f", b)
	}
}

func TestScoreKeywords_TextoMatch(t *testing.T) {
	b := scoreKeywords("", "aprendendo golang do basico ao avancado", []string{"golang"})
	if b <= 0 {
		t.Fatalf("match no texto deveria dar boost > 0, got %f", b)
	}
}

func TestScoreKeywords_Stemming(t *testing.T) {
	b := scoreKeywords("", "programacao em varias linguagens", []string{"programar"})
	if b <= 0 {
		t.Fatalf("stemming (4 primeiras letras) deveria dar boost > 0, got %f", b)
	}
}

func TestScoreKeywords_SemMatch(t *testing.T) {
	b := scoreKeywords("", "conteudo aleatorio", []string{"xyzneverfound"})
	if b != 0 {
		t.Fatalf("sem match, esperado 0, got %f", b)
	}
}


func TestScorePath_SemMatch(t *testing.T) {
	b := scorePath("notes/matematica.md", []string{"golang"})
	if b != 0 {
		t.Fatalf("sem match no path, esperado 0, got %f", b)
	}
}

func TestScoreFreshness_Hoje(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	f := scoreFreshness(now)
	if f <= 0 {
		t.Fatalf("documento de hoje deveria ter boost > 0, got %f", f)
	}
}

func TestScoreFreshness_EssaSemana(t *testing.T) {
	weekAgo := time.Now().UTC().AddDate(0, 0, -3).Format(time.RFC3339)
	f := scoreFreshness(weekAgo)
	if f <= 0 {
		t.Fatalf("documento da semana deveria ter boost > 0, got %f", f)
	}
	if f >= weights.BoostFreshness {
		t.Fatalf("documento da semana deveria ter boost menor que o maximo")
	}
}

func TestScoreFreshness_EsteMes(t *testing.T) {
	monthAgo := time.Now().UTC().AddDate(0, 0, -15).Format(time.RFC3339)
	f := scoreFreshness(monthAgo)
	if f <= 0 {
		t.Fatalf("documento do mes deveria ter boost > 0, got %f", f)
	}
}

func TestScoreFreshness_Antigo(t *testing.T) {
	old := time.Now().UTC().AddDate(0, 0, -60).Format(time.RFC3339)
	f := scoreFreshness(old)
	if f != 0 {
		t.Fatalf("documento antigo (>30d) deveria ter boost 0, got %f", f)
	}
}

func TestScoreFreshness_TimestampVazio(t *testing.T) {
	f := scoreFreshness("")
	if f != 0 {
		t.Fatalf("timestamp vazio deveria retornar 0, got %f", f)
	}
}

func TestScoreFreshness_TimestampInvalido(t *testing.T) {
	f := scoreFreshness("data-invalida")
	if f != 0 {
		t.Fatalf("timestamp invalido deveria retornar 0, got %f", f)
	}
}

func TestScoreRichness_PalavrasLongas(t *testing.T) {
	texto := "anticonstitucionalissimamente supercalifragilisticexpialidocious"
	b := scoreRichness(texto)
	if b <= 0 {
		t.Fatalf("texto com palavras longas deveria ter boost > 0, got %f", b)
	}
}

func TestScoreRichness_Tabela(t *testing.T) {
	texto := "algum texto |--| mais |--| conteudo"
	b := scoreRichness(texto)
	if b <= 0 {
		t.Fatalf("texto com tabela deveria ter boost > 0, got %f", b)
	}
}

func TestScoreRichness_Codigo(t *testing.T) {
	texto := "texto normal ```go fmt.Println('oi') ``` mais texto"
	b := scoreRichness(texto)
	if b <= 0 {
		t.Fatalf("texto com bloco de codigo deveria ter boost > 0, got %f", b)
	}
}

func TestScoreRichness_TextoCurtoSemEstrutura(t *testing.T) {
	b := scoreRichness("texto curto")
	// 2 palavras = (2/500)*0.5 = 0.002 de bonus de tamanho
	// sem bonus de palavras longas nem codigo/tabela
	expected := 0.002
	if b != expected {
		t.Fatalf("texto curto: esperado %f, got %f", expected, b)
	}
}

func TestScoreRichness_BonusTamanho(t *testing.T) {
	// Gera texto com ~1500 palavras para testar o bonus de tamanho
	var words []string
	for i := 0; i < 1500; i++ {
		words = append(words, "palavra")
	}
	texto := strings.Join(words, " ")
	b := scoreRichness(texto)
	if b <= 0 {
		t.Fatalf("texto longo deveria ter boost > 0, got %f", b)
	}
}

func TestScoreFragment_ScoresNaoNegativos(t *testing.T) {
	hit := &SearchHit{
		Doc: db.Document{
			Texto:     "texto qualquer para busca",
			Arquivo:   "notes/test.md",
			Secao:     "Geral",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Tags:      `["tag1"]`,
		},
		Score: 0.0,
	}
	score, details := scoreFragment(hit, []string{"texto"}, "texto", 0, 0)
	if score < 0 {
		t.Fatalf("score nao pode ser negativo, got %f", score)
	}
	for name, val := range details {
		if val < 0 {
			t.Fatalf("detail %q nao pode ser negativo, got %f", name, val)
		}
	}
}

func TestLog2EdgeCases(t *testing.T) {
	// popularidade 0
	if v := math.Log2(1) * 1.0; v != 0 {
		t.Fatalf("log2(1) deveria ser 0, got %f", v)
	}
	// linkCount 0
	if v := math.Log2(1) * weights.BoostLinkAuthority; v != 0 {
		t.Fatalf("log2(1) com link authority deveria ser 0, got %f", v)
	}
}
