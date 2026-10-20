package search

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/db"
	"ton618/core/internal/processor"
)

// ─────────────────────────────────────────────────────────────────────────────
// Harness de benchmark de desempenho (perf_bench_test.go)
//
// Semeia um vault sintético com N notas realistas (frontmatter + headings +
// wikilinks + TODOs) usando os MESMOS caminhos de código da produção
// (processor.ProcessMarkdownContent + Store.ReplaceFileIndexes + Store.SaveNote)
// e mede os caminhos quentes da busca.
//
// Uso:
//   go test ./internal/search/ -run '^$' -bench . -benchmem
//   go test ./internal/search/ -run '^$' -bench 'VaultScaling/N=3000' -benchmem
// ─────────────────────────────────────────────────────────────────────────────

// perfVocab alimenta as notas sintéticas com vocabulário realista em pt-BR para
// que o tokenizador e o ranker do FTS5 trabalhem com uma distribuição de termos
// parecida com a de produção (palavras repetidas + algumas raras).
var perfVocab = []string{
	"arquitetura", "performance", "banco de dados", "frontend", "backend",
	"segurança", "cache", "índice", "consulta", "latência",
	"memória", "concorrência", "teste", "deploy", "observabilidade",
	"refatoração", "migração", "rollback", "métrica", "gargalo",
}

// perfTags são as tags distribuídas ciclicamente entre as notas.
var perfTags = []string{"projeto", "estudo", "reuniao", "ideia", "referencia"}

// perfSections é o número de seções (headings) por nota sintética.
const perfSections = 6

// perfNoteContent monta o markdown de uma nota sintética.
func perfNoteContent(i int, tag string) string {
	var sb strings.Builder
	sb.WriteString("---\ntags: [")
	sb.WriteString(tag)
	sb.WriteString("]\n---\n\n")
	fmt.Fprintf(&sb, "# Nota %d sobre %s\n\n", i, perfVocab[i%len(perfVocab)])

	for s := 0; s < perfSections; s++ {
		fmt.Fprintf(&sb, "## Seção %d\n\n", s)
		for p := 0; p < 3; p++ {
			fmt.Fprintf(&sb,
				"A %s exige cuidado com %s e com o %s do sistema, "+
					"pois o %s cresce com o volume de dados %d-%d. ",
				perfVocab[(i+s+p)%len(perfVocab)],
				perfVocab[(i+s+p+3)%len(perfVocab)],
				perfVocab[(i+s+p+7)%len(perfVocab)],
				perfVocab[(i+s+p+11)%len(perfVocab)],
				i, s,
			)
		}
		sb.WriteString("\n\n")
		if s%2 == 0 {
			fmt.Fprintf(&sb, "Veja também [[nota-%05d]] para contexto.\n\n", (i+1)%1000)
		}
	}
	fmt.Fprintf(&sb, "- [ ] TODO: revisar %s\n", perfVocab[i%len(perfVocab)])
	return sb.String()
}

// seedPerfVault cria um banco temporário populado com n notas sintéticas.
func seedPerfVault(tb testing.TB, n int) *db.Store {
	tb.Helper()

	store, err := db.NewStore(filepath.Join(tb.TempDir(), "perf.db"))
	if err != nil {
		tb.Fatalf("NewStore: %v", err)
	}
	tb.Cleanup(func() { store.Close() })

	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	start := time.Now()

	for i := 0; i < n; i++ {
		filename := fmt.Sprintf("notes/nota-%05d.md", i)
		modTime := base.Add(time.Duration(i) * time.Minute)
		content := perfNoteContent(i, perfTags[i%len(perfTags)])

		docs, links, tags := processor.ProcessMarkdownContent(
			[]byte(content), filename, modTime, modTime)
		if err := store.ReplaceFileIndexes(ctx, filename, docs, links, tags, nil, modTime); err != nil {
			tb.Fatalf("ReplaceFileIndexes(%s): %v", filename, err)
		}
		if err := store.SaveNote(filename, content, modTime.Format(time.RFC3339)); err != nil {
			tb.Fatalf("SaveNote(%s): %v", filename, err)
		}
	}

	// Estatísticas do planner: sem ANALYZE o SQLite pode escolher planos ruins.
	if _, err := store.DB.Exec("ANALYZE"); err != nil {
		tb.Fatalf("ANALYZE: %v", err)
	}

	tb.Logf("vault semeado: %d notas em %s", n, time.Since(start).Round(time.Millisecond))

	var docs, ftsRows int
	_ = store.DB.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docs)
	_ = store.DB.QueryRow("SELECT COUNT(*) FROM docs_fts").Scan(&ftsRows)
	tb.Logf("  documents=%d docs_fts=%d", docs, ftsRows)

	return store
}

func perfSearchers() (func(string) int, func(string) float64) {
	getBL := func(string) int { return 0 }
	getSW := func(string) float64 { return 1.0 }
	return getBL, getSW
}

// perfQueryCases cobre as formas de query que exercitam caminhos diferentes.
var perfQueryCases = []struct {
	name string
	q    string
}{
	{"TermosMultiplos", "performance do banco de dados"},
	{"TermoUnicoComum", "arquitetura"},
	{"TermoRaro", "observabilidade"},
	{"TagOnly", "#projeto"},
	{"TagFilter", "tags:projeto"},
	{"SemResultado", "zzztermoinesxistentexyz"},
}

// ── Benchmark principal: escalabilidade da busca por tamanho de vault ───────

func BenchmarkSearchVaultScaling(b *testing.B) {
	getBL, getSW := perfSearchers()

	for _, n := range []int{200, 1000, 3000} {
		store := seedPerfVault(b, n)

		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			for _, qc := range perfQueryCases {
				b.Run(qc.name, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if _, err := Search(context.Background(), store, qc.q, 0, 20, getBL, getSW); err != nil {
							b.Fatalf("Search(%q): %v", qc.q, err)
						}
					}
				})
			}
		})
	}
}

// BenchmarkSearchTagOnlyVaultScaling isola o caminho de "busca só por tag",
// que em search.go eleva o LIMIT para 99999 e materializa o corpus inteiro.
func BenchmarkSearchTagOnlyVaultScaling(b *testing.B) {
	getBL, getSW := perfSearchers()

	for _, n := range []int{200, 1000, 3000} {
		store := seedPerfVault(b, n)
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Search(context.Background(), store, "#projeto", 0, 20, getBL, getSW); err != nil {
					b.Fatalf("Search tag-only: %v", err)
				}
			}
		})
	}
}

// ── Benchmarks de funções puras (sem I/O) ───────────────────────────────────

// BenchmarkExtractTerms mede o custo de extractTerms, que recompila um regexp
// a cada chamada e é invocado em loop no gate da busca híbrida.
func BenchmarkExtractTerms(b *testing.B) {
	queries := []string{
		`"termo exato" e outros termos soltos`,
		`performance do banco de dados com muitas palavras aqui dentro`,
		`#tag e "frase com aspas" mais texto`,
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			_ = extractTerms(q)
		}
	}
}

// BenchmarkBuildFTSQuery mede a montagem da query FTS5 (usada em toda busca).
func BenchmarkBuildFTSQuery(b *testing.B) {
	queries := []string{
		"performance do banco de dados",
		"#projeto",
		"tags:projeto",
		`"frase exata" texto`,
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			_ = buildFTSQuery(q)
		}
	}
}

// BenchmarkSearchEmptyQuery_ListAll mede o caminho padrão de abertura da página
// de busca (query vazia), que cai em listAll → GetDocumentsPaginated
// (COUNT + página com "tags NOT LIKE '%drawing%'", ambos full scan).
func BenchmarkSearchEmptyQuery_ListAll(b *testing.B) {
	getBL, getSW := perfSearchers()

	for _, n := range []int{1000, 3000} {
		store := seedPerfVault(b, n)
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Search(context.Background(), store, "", 0, 20, getBL, getSW); err != nil {
					b.Fatalf("Search(vazio): %v", err)
				}
			}
		})
	}
}

// BenchmarkSearchProfile isola uma única forma de query num vault de tamanho
// fixo, para permitir profiling de CPU sem que o custo de seed domine.
//
//	go test ./internal/search/ -run '^$' \
//	  -bench 'BenchmarkSearchProfile/TermosMultiplos' -benchtime 300x \
//	  -cpuprofile /tmp/cpu.prof
//	go tool pprof -top -nodecount=25 /tmp/cpu.prof
func BenchmarkSearchProfile(b *testing.B) {
	store := seedPerfVault(b, 3000)
	getBL, getSW := perfSearchers()

	for _, qc := range perfQueryCases {
		b.Run(qc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Search(context.Background(), store, qc.q, 0, 20, getBL, getSW); err != nil {
					b.Fatalf("Search(%q): %v", qc.q, err)
				}
			}
		})
	}
}
