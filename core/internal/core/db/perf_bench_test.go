package db

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Benchmarks de desempenho da camada de banco (perf_bench_test.go)
//
// Mede os caminhos de banco que são chamados em request ou em polling:
// status de embeddings, full scans por LIKE, tabela virtual vec0, e o padrão
// N+1 do rename de notas.
//
// Uso:
//   go test ./internal/core/db/ -run '^$' -bench . -benchmem
// ─────────────────────────────────────────────────────────────────────────────

// seedDBPerf cria um store com nNotes notas, cada uma com chunksPerNote chunks.
// Se withEmbeddings, cada chunk recebe um vetor aleatório determinístico de 384 dims.
func seedDBPerf(tb testing.TB, nNotes, chunksPerNote int, withEmbeddings bool) *Store {
	tb.Helper()

	s, err := NewStore(filepath.Join(tb.TempDir(), "perf.db"))
	if err != nil {
		tb.Fatalf("NewStore: %v", err)
	}
	tb.Cleanup(func() { s.Close() })

	rng := rand.New(rand.NewSource(42)) //nolint:gosec // determinismo no benchmark
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Texto com termos repetidos para o FTS5 ter o que ranquear.
	body := strings.Repeat("conteudo relevante para busca textual de performance ", 20)

	for i := 0; i < nNotes; i++ {
		filename := fmt.Sprintf("notes/nota-%05d.md", i)
		mtime := base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339)

		if err := s.SaveNote(filename, body, mtime); err != nil {
			tb.Fatalf("SaveNote(%s): %v", filename, err)
		}
		if err := s.SetFileTags(filename, []string{"projeto"}); err != nil {
			tb.Fatalf("SetFileTags(%s): %v", filename, err)
		}

		if chunksPerNote > 0 {
			chunks := make([]ChunkInfo, 0, chunksPerNote)
			for c := 0; c < chunksPerNote; c++ {
				ci := ChunkInfo{
					ChunkID:    fmt.Sprintf("%s#%d", filename, c),
					Filename:   filename,
					ChunkIndex: c,
					Content:    "chunk de conteudo da nota para indexacao semantica",
				}
				if withEmbeddings {
					vec := make([]float32, EmbeddingDim)
					for d := range vec {
						vec[d] = rng.Float32()*2 - 1
					}
					ci.Embedding = vec
				}
				chunks = append(chunks, ci)
			}
			if err := s.SaveNoteChunks(filename, chunks); err != nil {
				tb.Fatalf("SaveNoteChunks(%s): %v", filename, err)
			}
		}
	}

	if _, err := s.DB.Exec("ANALYZE"); err != nil {
		tb.Fatalf("ANALYZE: %v", err)
	}
	return s
}

// ── Status de embeddings (rota /api/embeddings/status, pollada pelo browser) ─

func BenchmarkGetEmbeddingStatusScaling(b *testing.B) {
	for _, n := range []int{200, 1000, 3000} {
		s := seedDBPerf(b, n, 5, true)
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := s.GetEmbeddingStatus(); err != nil {
					b.Fatalf("GetEmbeddingStatus: %v", err)
				}
			}
		})
	}
}

func BenchmarkGetPendingEmbeddingNotes(b *testing.B) {
	s := seedDBPerf(b, 3000, 0, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.GetPendingEmbeddingNotes(10); err != nil {
			b.Fatalf("GetPendingEmbeddingNotes: %v", err)
		}
	}
}

// BenchmarkSearchSimilarWithConsensus mede o KNN (usado pela semântica pura e
// pela híbrida) para diferentes `limit` — a híbrida chama com limit=engineN.
func BenchmarkSearchSimilarWithConsensus(b *testing.B) {
	s := seedDBPerf(b, 1000, 10, true)
	emb := make([]float32, EmbeddingDim)
	for i := range emb {
		emb[i] = 0.1
	}
	for _, limit := range []int{15, 30, 200} {
		b.Run(fmt.Sprintf("limit=%d", limit), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := s.SearchSimilarWithConsensus(context.Background(), emb, limit, math.MaxFloat64); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ── varredura por prefixo em tabela virtual vec0 vs. índice real ─────────────

func BenchmarkVec0ChunkLookupVsIndex(b *testing.B) {
	const nNotes, chunksPerNote = 1000, 10
	s := seedDBPerf(b, nNotes, chunksPerNote, true)
	target := fmt.Sprintf("notes/nota-%05d.md", nNotes/2)
	like := target + "#%"

	b.Run("Vec0_LIKE_prefixo", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var n int
			if err := s.DB.QueryRow(
				"SELECT COUNT(*) FROM note_embeddings WHERE chunk_id LIKE ?", like).Scan(&n); err != nil {
				b.Fatalf("like: %v", err)
			}
		}
	})

	b.Run("NoteChunks_indice_por_filename", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var n int
			if err := s.DB.QueryRow(
				"SELECT COUNT(*) FROM note_chunks WHERE filename = ?", target).Scan(&n); err != nil {
				b.Fatalf("index: %v", err)
			}
		}
	})
}

// BenchmarkCollectChunkIDsByFilename mede a alternativa indexada ao LIKE:
// obter os chunk_ids via note_chunks (índice) para depois deletar por igualdade.
func BenchmarkCollectChunkIDsByFilename(b *testing.B) {
	s := seedDBPerf(b, 1000, 10, true)
	target := "notes/nota-00500.md"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := s.DB.Query("SELECT chunk_id FROM note_chunks WHERE filename = ?", target)
		if err != nil {
			b.Fatalf("query: %v", err)
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				b.Fatalf("scan: %v", err)
			}
			ids = append(ids, id)
		}
		rows.Close()
		if len(ids) == 0 {
			b.Fatal("esperado pelo menos 1 chunk_id")
		}
	}
}

// ── Full scans por LIKE (mesma forma usada na produção) ─────────────────────

func BenchmarkGetNotesNeedingMarkmapTag(b *testing.B) {
	s := seedDBPerf(b, 3000, 0, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.GetNotesNeedingMarkmapTag(); err != nil {
			b.Fatalf("GetNotesNeedingMarkmapTag: %v", err)
		}
	}
}

func BenchmarkGetAllNotesContent(b *testing.B) {
	s := seedDBPerf(b, 3000, 0, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.GetAllNotesContent(); err != nil {
			b.Fatalf("GetAllNotesContent: %v", err)
		}
	}
}

// BenchmarkDeleteEmbeddingsForFile compara a limpeza de embeddings por LIKE
// (full scan na vec0 — comportamento antigo) com a nova via índice
// (chunk_ids de note_chunks + DELETE por igualdade). Ambas rodam numa
// transação revertida para não alterar o dataset.
func BenchmarkDeleteEmbeddingsForFile(b *testing.B) {
	const nNotes, chunksPerNote = 1000, 10
	s := seedDBPerf(b, nNotes, chunksPerNote, true)
	target := "notes/nota-00500.md"

	b.Run("Antigo_LIKE_prefixo", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tx, err := s.DB.Begin()
			if err != nil {
				b.Fatal(err)
			}
			if _, err := tx.Exec("DELETE FROM note_embeddings WHERE chunk_id LIKE ?", target+"#%"); err != nil {
				tx.Rollback()
				b.Fatal(err)
			}
			tx.Rollback()
		}
	})

	b.Run("Novo_indice_por_id", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tx, err := s.DB.Begin()
			if err != nil {
				b.Fatal(err)
			}
			if err := deleteEmbeddingsForFile(tx, target); err != nil {
				tx.Rollback()
				b.Fatal(err)
			}
			tx.Rollback()
		}
	})
}

// ── Padrão N+1 do rename (UpdateBacklinksOnRename) ──────────────────────────
//
// UpdateBacklinksOnRename() carrega TODAS as notas como candidatas e faz
// GetNote + (se mudou) Save para cada uma. Este benchmark mede a parte de
// leitura desse padrão: GetAllNotes + 1 GetNote por nota.

func BenchmarkBacklinkRenameScan(b *testing.B) {
	s := seedDBPerf(b, 1000, 0, false)

	b.Run("Atual_GetNote_por_nota", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			all, err := s.GetAllNotes()
			if err != nil {
				b.Fatalf("GetAllNotes: %v", err)
			}
			for filename := range all {
				if _, err := s.GetNote(filename); err != nil {
					b.Fatalf("GetNote(%s): %v", filename, err)
				}
			}
		}
	})

	b.Run("Alternativa_GetAllNotesContent", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := s.GetAllNotesContent(); err != nil {
				b.Fatalf("GetAllNotesContent: %v", err)
			}
		}
	})
}

// ── FTS5: isolar o custo do COUNT(*) vs. a página de resultados ─────────────
//
// SearchFTSWithContext() executa SEMPRE duas queries: um COUNT(*) de todos os
// matches (com "tags NOT LIKE '%drawing%'") e depois a página com LIMIT.
// Estes benchmarks separam as duas para medir quanto o COUNT custa.

func seedFTSPerf(tb testing.TB, nDocs int) *Store {
	tb.Helper()

	s, err := NewStore(filepath.Join(tb.TempDir(), "fts.db"))
	if err != nil {
		tb.Fatalf("NewStore: %v", err)
	}
	tb.Cleanup(func() { s.Close() })

	tx, err := s.DB.Begin()
	if err != nil {
		tb.Fatalf("begin: %v", err)
	}
	stmt, err := tx.Prepare(
		`INSERT INTO docs_fts (doc_id, tipo, arquivo, secao, texto, tags) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tb.Fatalf("prepare: %v", err)
	}
	defer stmt.Close()

	// "performance" aparece em ~metade dos documentos (termo comum);
	// "zzztermoinesxistentexyz" não aparece em nenhum.
	common := strings.Repeat("performance relevante para busca textual ", 12)
	rare := strings.Repeat("conteudo generico sem o termo alvo ", 12)

	for i := 0; i < nDocs; i++ {
		texto := rare
		if i%2 == 0 {
			texto = common
		}
		if _, err := stmt.Exec(
			fmt.Sprintf("doc-%07d", i), "markdown",
			fmt.Sprintf("notes/nota-%05d.md", i/7), "Geral", texto, "projeto",
		); err != nil {
			tb.Fatalf("insert: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		tb.Fatalf("commit: %v", err)
	}
	if _, err := s.DB.Exec("ANALYZE"); err != nil {
		tb.Fatalf("ANALYZE: %v", err)
	}
	return s
}

func BenchmarkFTSCountVsPage(b *testing.B) {
	const nDocs = 21000 // ~ equals 3.000 notas × 7 seções
	s := seedFTSPerf(b, nDocs)

	cases := []struct {
		name  string
		query string
	}{
		{"TermoComum", "performance"},
		{"TermoAusente", "zzztermoinesxistentexyz"},
	}

	for _, c := range cases {
		b.Run(c.name+"/Count_total", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var n int
				if err := s.DB.QueryRow(
					"SELECT COUNT(*) FROM docs_fts WHERE docs_fts MATCH ? AND tags NOT LIKE '%drawing%'",
					c.query).Scan(&n); err != nil {
					b.Fatalf("count: %v", err)
				}
			}
		})

		b.Run(c.name+"/Pagina_LIMIT20", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				rows, err := s.DB.Query(`
					SELECT doc_id, tipo, arquivo, secao, texto, tags, rank,
					       snippet(docs_fts, 4, '__HL_START__', '__HL_END__', '...', 64)
					FROM docs_fts
					WHERE docs_fts MATCH ? AND tags NOT LIKE '%drawing%'
					ORDER BY rank LIMIT 20`, c.query)
				if err != nil {
					b.Fatalf("page: %v", err)
				}
				for rows.Next() {
					var a, bb, cc, dd, ee, ff, gg string
					var rank float64
					if err := rows.Scan(&a, &bb, &cc, &dd, &ee, &ff, &rank, &gg); err != nil {
						rows.Close()
						b.Fatalf("scan: %v", err)
					}
				}
				rows.Close()
			}
		})
	}

	b.Run("SemQuery/Count_tabela_inteira", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var n int
			if err := s.DB.QueryRow(
				"SELECT COUNT(*) FROM docs_fts WHERE tags NOT LIKE '%drawing%'").Scan(&n); err != nil {
				b.Fatalf("count all: %v", err)
			}
		}
	})
}
