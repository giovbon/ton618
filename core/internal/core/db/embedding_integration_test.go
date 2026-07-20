package db

import (
	"context"
	"testing"
	"time"
)

// ── Testes de Integração: NoteService + Embeddings ──
//
// Estes testes verificam o fluxo completo de indexação semântica:
//   1. Salvar nota → mtime atualizado no banco
//   2. Salvar nota → detectada como pendente de embedding
//   3. Salvar chunks → nota aparece como indexada
//   4. Modificar nota → chunks ficam stale → detectada como pendente novamente
//   5. Nota não-indexável (drawing) → chunks são limpos

func TestEmbeddingIntegration_SaveNoteAtualizaMtime(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/integration_mtime.md"

	// Salva nota
	err := s.SaveNote(filename, "# Nota de teste\n\nConteúdo inicial", "2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	s.SetFileTags(filename, []string{})

	// Verifica mtime
	mtime, err := s.GetNoteMtime(filename)
	if err != nil {
		t.Fatalf("GetNoteMtime: %v", err)
	}
	if mtime != "2024-01-01T00:00:00Z" {
		t.Fatalf("mtime esperado 2024-01-01T00:00:00Z, got %q", mtime)
	}
}

func TestEmbeddingIntegration_NotaSalvaApareceComoPendente(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/integration_pendente.md"

	// Cria nota sem embedding
	s.SaveNote(filename, "# Pendente\n\nPrecisa de embedding", "2024-01-01T00:00:00Z")
	s.SetFileTags(filename, []string{})

	// Verifica que aparece como pendente
	pending, err := s.GetPendingEmbeddingNotes(10)
	if err != nil {
		t.Fatalf("GetPendingEmbeddingNotes: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("esperado 1 pendente, got %d", len(pending))
	}
	if pending[0].Filename != filename {
		t.Fatalf("esperado filename %q, got %q", filename, pending[0].Filename)
	}
	if pending[0].Content == "" {
		t.Fatal("conteúdo não deveria estar vazio")
	}
}

func TestEmbeddingIntegration_NotaComTagNaoIndexavel(t *testing.T) {
	s := newTestStore(t)

	// Cria notas de tipos diferentes
	tests := []struct {
		filename string
		tags     []string
		wantPend bool // se deve aparecer como pendente
		desc     string
	}{
		{"notes/markdown.md", []string{}, true, "markdown puro é indexável"},
		{"notes/typst.md", []string{"typst"}, true, "typst é indexável"},
		{"notes/mindmap.md", []string{"markmap"}, true, "mindmap é indexável"},
		{"notes/drawing.md", []string{"drawing"}, false, "drawing não é indexável"},
		{"notes/spreadsheet.md", []string{"spreadsheet"}, false, "spreadsheet não é indexável"},
		{"notes/mermaid.md", []string{"mermaid"}, false, "mermaid não é indexável"},
		{"notes/artigo.md", []string{"artigo"}, true, "artigo é indexável"},
		{"pdfs/doc.pdf", []string{}, false, "pdfs/ não é indexável"},
		{"archives/backup.zip", []string{}, false, "archives/ não é indexável"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			s.SaveNote(tt.filename, "# Conteúdo de "+tt.filename, "2024-01-01T00:00:00Z")
			s.SetFileTags(tt.filename, tt.tags)

			pending, _ := s.GetPendingEmbeddingNotes(100)
			found := false
			for _, p := range pending {
				if p.Filename == tt.filename {
					found = true
					break
				}
			}
			if found != tt.wantPend {
				t.Errorf("filename=%q tags=%v: esperado pendente=%v, got %v",
					tt.filename, tt.tags, tt.wantPend, found)
			}

			// Cleanup for next subtest
			s.DeleteAllFileRecords(tt.filename)
		})
	}
}

func TestEmbeddingIntegration_ChunkSalvoNotaIndexada(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/integration_indexada.md"

	s.SaveNote(filename, "# Indexada\n\nConteúdo", "2024-01-01T00:00:00Z")
	s.SetFileTags(filename, []string{})

	// Antes de salvar chunks: não indexada
	if s.HasEmbedding(filename) {
		t.Fatal("não deveria ter embedding antes de salvar chunks")
	}

	// Salva chunks
	chunks := []ChunkInfo{
		makeChunk(filename, 0, "Chunk 0", 0.5),
	}
	err := s.SaveNoteChunks(filename, chunks)
	if err != nil {
		t.Fatalf("SaveNoteChunks: %v", err)
	}

	// Depois: indexada
	if !s.HasEmbedding(filename) {
		t.Fatal("deveria ter embedding após salvar chunks")
	}

	// Status reflete a indexação
	status, _ := s.GetEmbeddingStatus()
	if status.IndexedNotes < 1 {
		t.Fatalf("IndexedNotes esperado >=1, got %d", status.IndexedNotes)
	}
	if status.PendingNotes > 0 {
		t.Fatalf("PendingNotes esperado 0 após indexar única nota, got %d", status.PendingNotes)
	}
}

func TestEmbeddingIntegration_StaleDetection(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/integration_stale.md"

	// 1. Cria nota e indexa
	s.SaveNote(filename, "# Versão 1", "2024-01-01T00:00:00Z")
	s.SetFileTags(filename, []string{})

	chunks := []ChunkInfo{
		makeChunk(filename, 0, "Versão 1", 0.5),
	}
	s.SaveNoteChunks(filename, chunks)

	// Status: 0 pendentes
	status, _ := s.GetEmbeddingStatus()
	if status.PendingNotes != 0 {
		t.Fatalf("esperado 0 pendentes após indexar, got %d", status.PendingNotes)
	}

	// 2. Modifica a nota (novo mtime) sem reindexar
	s.SaveNote(filename, "# Versão 2 - modificada", "2024-01-02T00:00:00Z")

	// Status: 1 pendente (stale)
	status, _ = s.GetEmbeddingStatus()
	if status.PendingNotes != 1 {
		t.Fatalf("esperado 1 pendente (stale), got %d", status.PendingNotes)
	}
	if status.StaleNotes != 1 {
		t.Fatalf("esperado 1 stale, got %d", status.StaleNotes)
	}

	// 3. GetPendingEmbeddingNotes inclui a nota stale
	pending, err := s.GetPendingEmbeddingNotes(10)
	if err != nil {
		t.Fatalf("GetPendingEmbeddingNotes: %v", err)
	}
	found := false
	for _, p := range pending {
		if p.Filename == filename {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("nota stale deveria aparecer como pendente")
	}
}

func TestEmbeddingIntegration_NotaViraNaoIndexavel_LimpaChunks(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/integration_cleanup.md"

	// 1. Cria nota markdown e indexa
	s.SaveNote(filename, "# Antiga\n\nEra markdown", "2024-01-01T00:00:00Z")
	s.SetFileTags(filename, []string{})

	chunks := []ChunkInfo{
		makeChunk(filename, 0, "Era markdown", 0.5),
	}
	s.SaveNoteChunks(filename, chunks)

	if !s.HasEmbedding(filename) {
		t.Fatal("deveria ter embedding inicialmente")
	}

	// 2. Salva novamente como non-embeddable (drawing) via ReplaceFileIndexes
	//    Isso simula o que o NoteService.processAndSave faz
	s.SaveNote(filename, "{}", "2024-01-02T00:00:00Z")
	s.ReplaceFileIndexes(context.Background(), filename, nil, nil, []string{"drawing"}, nil, time.Now())

	// 3. Chunks devem ter sido limpos
	if s.HasEmbedding(filename) {
		t.Fatal("embedding deveria ter sido limpo após nota virar non-embeddable")
	}

	// 4. Não aparece como pendente
	pending, _ := s.GetPendingEmbeddingNotes(10)
	for _, p := range pending {
		if p.Filename == filename {
			t.Fatal("nota drawing não deveria aparecer como pendente")
		}
	}
}

func TestEmbeddingIntegration_MultiplosChunksEDeducacao(t *testing.T) {
	s := newTestStore(t)

	// Cria duas notas com múltiplos chunks
	s.SaveNote("notes/a.md", "# Nota A\n\nConteúdo A", "2024-01-01T00:00:00Z")
	s.SetFileTags("notes/a.md", []string{})
	s.SaveNote("notes/b.md", "# Nota B\n\nConteúdo B", "2024-01-01T00:00:00Z")
	s.SetFileTags("notes/b.md", []string{})

	s.SaveNoteChunks("notes/a.md", []ChunkInfo{
		makeChunk("notes/a.md", 0, "A1", 1.0),
		makeChunk("notes/a.md", 1, "A2", 0.9),
	})
	s.SaveNoteChunks("notes/b.md", []ChunkInfo{
		makeChunk("notes/b.md", 0, "B1", 2.0),
	})

	// SearchSimilar deve retornar ambas, com a mais similar primeiro
	queryEmb := make([]float32, EmbeddingDim)
	queryEmb[0] = 1.0

	results, err := s.SearchSimilar(queryEmb, 10)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("esperado 2 resultados, got %d", len(results))
	}

	// A (distância 0.9 do chunk mais próximo) deve vir antes de B (2.0)
	if results[0].Filename != "notes/a.md" {
		t.Fatalf("esperado notes/a.md primeiro, got %q", results[0].Filename)
	}
}

func TestEmbeddingIntegration_ReindexacaoAposModificacao(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/integration_reindex.md"

	// 1. Cria e indexa com 3 chunks
	s.SaveNote(filename, "# Original\n\nA\n\nB\n\nC", "2024-01-01T00:00:00Z")
	s.SetFileTags(filename, []string{})

	s.SaveNoteChunks(filename, []ChunkInfo{
		makeChunk(filename, 0, "A", 0.1),
		makeChunk(filename, 1, "B", 0.2),
		makeChunk(filename, 2, "C", 0.3),
	})

	// Verifica 3 chunks
	var count int
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_chunks WHERE filename = ?`, filename).Scan(&count)
	if count != 3 {
		t.Fatalf("esperado 3 chunks, got %d", count)
	}

	// 2. Reindexa com apenas 1 chunk (simulando edição que reduziu conteúdo)
	s.SaveNoteChunks(filename, []ChunkInfo{
		makeChunk(filename, 0, "Apenas A", 0.5),
	})

	// Verifica que os chunks antigos foram substituídos
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_chunks WHERE filename = ?`, filename).Scan(&count)
	if count != 1 {
		t.Fatalf("esperado 1 chunk após reindexação, got %d", count)
	}

	// Embeddings órfãos também foram limpos
	var embCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_embeddings WHERE chunk_id LIKE ?`, filename+`#%`).Scan(&embCount)
	if embCount != 1 {
		t.Fatalf("esperado 1 embedding após reindexação, got %d", embCount)
	}

	// Busca ainda funciona (retorna a nota)
	queryEmb := make([]float32, EmbeddingDim)
	queryEmb[0] = 0.5
	results, err := s.SearchSimilar(queryEmb, 10)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("esperado 1 resultado, got %d", len(results))
	}
}

func TestEmbeddingIntegration_DeleteAllFileRecordsLimpaEmbebeddings(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/integration_delete.md"

	s.SaveNote(filename, "# Deletar\n\nConteúdo", "2024-01-01T00:00:00Z")
	s.SetFileTags(filename, []string{})
	s.SaveNoteChunks(filename, []ChunkInfo{
		makeChunk(filename, 0, "Conteúdo", 0.5),
	})

	if !s.HasEmbedding(filename) {
		t.Fatal("deveria ter embedding antes do delete")
	}

	// DeleteAllFileRecords
	s.DeleteAllFileRecords(filename)

	if s.HasEmbedding(filename) {
		t.Fatal("embedding deveria ter sido deletado")
	}

	// Status reflete a remoção
	status, _ := s.GetEmbeddingStatus()
	if status.TotalNotes != 0 {
		t.Fatalf("esperado 0 notas após delete, got %d", status.TotalNotes)
	}
}

func TestEmbeddingIntegration_GetEmbeddingStatusComNotasMistas(t *testing.T) {
	s := newTestStore(t)

	// Cria notas de tipos variados
	notes := []struct {
		name string
		tags []string
	}{
		{"notes/md1.md", []string{}},
		{"notes/md2.md", []string{}},
		{"notes/typst.md", []string{"typst"}},
		{"notes/drawing.md", []string{"drawing"}},
		{"notes/mermaid.md", []string{"mermaid"}},
		{"notes/artigo.md", []string{"artigo"}},
		{"pdfs/doc.pdf", []string{}},
	}

	for _, n := range notes {
		s.SaveNote(n.name, "# Conteúdo", "2024-01-01T00:00:00Z")
		s.SetFileTags(n.name, n.tags)
	}

	// Indexa apenas 2 notas
	s.SaveNoteChunks("notes/md1.md", []ChunkInfo{makeChunk("notes/md1.md", 0, "MD1", 0.5)})
	s.SaveNoteChunks("notes/artigo.md", []ChunkInfo{makeChunk("notes/artigo.md", 0, "Artigo", 0.5)})

	status, err := s.GetEmbeddingStatus()
	if err != nil {
		t.Fatalf("GetEmbeddingStatus: %v", err)
	}

	// Total = 4 (md1, md2, typst, artigo) — drawing, mermaid, pdfs/ são excluídos
	if status.TotalNotes != 4 {
		t.Fatalf("TotalNotes esperado 4, got %d", status.TotalNotes)
	}
	// Indexados = 2
	if status.IndexedNotes != 2 {
		t.Fatalf("IndexedNotes esperado 2, got %d", status.IndexedNotes)
	}
	// Pendentes = Total - Indexados = 2
	if status.PendingNotes != 2 {
		t.Fatalf("PendingNotes esperado 2, got %d", status.PendingNotes)
	}
}

func TestEmbeddingIntegration_GetNoteEmbeddings(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/integration_get_emb.md"

	s.SaveNote(filename, "# Get Embeddings", "2024-01-01T00:00:00Z")
	s.SetFileTags(filename, []string{})

	chunks := []ChunkInfo{
		makeChunk(filename, 0, "Chunk 0", 0.1),
		makeChunk(filename, 1, "Chunk 1", 0.2),
	}
	s.SaveNoteChunks(filename, chunks)

	embeddings, err := s.GetNoteEmbeddings(filename)
	if err != nil {
		t.Fatalf("GetNoteEmbeddings: %v", err)
	}
	if len(embeddings) != 2 {
		t.Fatalf("esperado 2 embeddings, got %d", len(embeddings))
	}

	// Verifica valores
	if embeddings[0][0] != 0.1 {
		t.Fatalf("embedding[0][0] esperado 0.1, got %f", embeddings[0][0])
	}
	if embeddings[1][0] != 0.2 {
		t.Fatalf("embedding[1][0] esperado 0.2, got %f", embeddings[1][0])
	}
}

func TestEmbeddingIntegration_NotaSemConteudoNaoApareceComoPendente(t *testing.T) {
	s := newTestStore(t)

	// Nota com conteúdo vazio
	s.SaveNote("notes/vazia.md", "", "2024-01-01T00:00:00Z")
	s.SetFileTags("notes/vazia.md", []string{})

	pending, _ := s.GetPendingEmbeddingNotes(10)
	for _, p := range pending {
		if p.Filename == "notes/vazia.md" {
			t.Fatal("nota com conteúdo vazio não deveria aparecer como pendente")
		}
	}
}

func TestEmbeddingIntegration_GetEmbeddingStatusStaleEDeletado(t *testing.T) {
	s := newTestStore(t)

	// Cria e indexa nota
	s.SaveNote("notes/stale.md", "# Stale", "2024-01-01T00:00:00Z")
	s.SetFileTags("notes/stale.md", []string{})
	s.SaveNoteChunks("notes/stale.md", []ChunkInfo{makeChunk("notes/stale.md", 0, "Stale", 0.5)})

	status, _ := s.GetEmbeddingStatus()
	if status.StaleNotes != 0 {
		t.Fatalf("esperado 0 stale, got %d", status.StaleNotes)
	}

	// Modifica a nota
	s.SaveNote("notes/stale.md", "# Stale Modificado", "2024-01-02T00:00:00Z")

	status, _ = s.GetEmbeddingStatus()
	if status.StaleNotes != 1 {
		t.Fatalf("esperado 1 stale após modificação, got %d", status.StaleNotes)
	}
	if status.PendingNotes < 1 {
		t.Fatalf("esperado pelo menos 1 pendente (stale), got %d", status.PendingNotes)
	}

	// Deleta a nota
	s.DeleteAllFileRecords("notes/stale.md")

	status, _ = s.GetEmbeddingStatus()
	if status.StaleNotes != 0 {
		t.Fatalf("esperado 0 stale após delete, got %d", status.StaleNotes)
	}
}
