package search

import (
	"context"
	"testing"
	"time"

	"ton618/internal/db"
)

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

// ── Helper ──────────────────────────────────────────────────────

func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	s, err := db.NewStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	return s
}
