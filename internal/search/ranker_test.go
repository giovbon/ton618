package search

import (
	"testing"
	"time"

	"ton618/internal/db"
)

func TestScoreFragment_Base(t *testing.T) {
	hit := &SearchHit{
		Doc: db.Document{
			Texto:   "golang é uma linguagem de programação compilada",
			Arquivo: "notes/golang.md",
			Secao:   "📄 Golang",
		},
		Score: -1.0,
	}
	score, _ := scoreFragment(hit, []string{"golang"}, "golang", 0.0, 0)
	if score <= 0 {
		t.Fatalf("score deveria ser > 0, got %f", score)
	}
}

func TestScoreTitle_Partial(t *testing.T) {
	b := scoreTitle("📄 Aprendendo Programacao", []string{"programa"})
	if b <= 0 {
		t.Fatalf("esperado boost > 0 para match parcial, got %f", b)
	}
}

func TestScoreTitle_SemMatch(t *testing.T) {
	b := scoreTitle("📄 Matematica", []string{"fisica"})
	if b != 0 {
		t.Fatalf("esperado 0 sem match, got %f", b)
	}
}

func TestScoreTitle_StopwordIgnorada(t *testing.T) {
	b := scoreTitle("📄 O Artigo Definido", []string{"o"})
	if b != 0 {
		t.Fatalf("stopword nao deveria dar boost, got %f", b)
	}
}

func TestScoreTitle_TermoCurtoIgnorado(t *testing.T) {
	b := scoreTitle("📄 Curso de Go", []string{"go"})
	if b != 0 {
		t.Fatalf("termo com menos de 3 chars nao deveria dar boost, got %f", b)
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
	if f >= 0.5 {
		t.Fatalf("documento da semana deveria ter boost menor que o maximo (0.5)")
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
	score, details := scoreFragment(hit, []string{"texto"}, "texto", 0.0, 0)
	if score < 0 {
		t.Fatalf("score nao pode ser negativo, got %f", score)
	}
	for name, val := range details {
		if val < 0 {
			t.Fatalf("detail %q nao pode ser negativo, got %f", name, val)
		}
	}
}
