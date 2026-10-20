package processor

import (
	"strings"
	"testing"
	"time"
)

// BenchmarkProcessMarkdownContent mede a performance de parsing, extração de frontmatter,
// wikilinks, tags e marcadores de TODOs de um documento Markdown típico.
func BenchmarkProcessMarkdownContent(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("---\ntitle: Nota de Teste Benchmark\ntags: [go, performance, benchmark, sqlite]\n---\n\n")
	sb.WriteString("# Arquitetura e Desempenho do Sistema\n\n")
	sb.WriteString("A performance do banco de dados SQLite exige atenção constante com indexação e FTS5. ")
	sb.WriteString("Veja mais em [[nota-arquitetura]] e [[nota-performance]].\n\n")
	sb.WriteString("## Tarefas Pendentes\n\n")
	sb.WriteString("- [ ] TODO: Otimizar query de busca semântica\n")
	sb.WriteString("- [ ] URGENTE: Corrigir leak de memória no watcher\n")
	sb.WriteString("- [x] DONE: Implementar testes de benchmark\n\n")
	sb.WriteString("## Seção de Conteúdo Extenso\n\n")
	for i := 0; i < 50; i++ {
		sb.WriteString("Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore.\n")
	}

	content := []byte(sb.String())
	filename := "notes/benchmark-note.md"
	now := time.Now()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		docs, links, tags := ProcessMarkdownContent(content, filename, now, now)
		if len(docs) == 0 {
			b.Fatal("Esperava documentos processados")
		}
		_ = links
		_ = tags
	}
}

// BenchmarkGenerateCUID2 mede a geração de IDs únicos CUID2.
func BenchmarkGenerateCUID2(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = GenerateCUID2()
	}
}
