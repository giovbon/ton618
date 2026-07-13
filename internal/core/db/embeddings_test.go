package db

import (
	"fmt"
	"math"
	"testing"
)

// makeChunk é helper para criar um chunk de teste.
func makeChunk(filename string, index int, content string, val float32) ChunkInfo {
	emb := make([]float32, EmbeddingDim)
	emb[0] = val
	return ChunkInfo{
		ChunkID:    fmt.Sprintf("%s#%d", filename, index),
		Filename:   filename,
		ChunkIndex: index,
		Content:    content,
		Embedding:  emb,
	}
}

// ── serializeEmbedding ──────────────────────────────────────────

func TestSerializeEmbedding_DimensaoValida(t *testing.T) {
	embedding := make([]float32, EmbeddingDim)
	for i := range embedding {
		embedding[i] = float32(i) * 0.01
	}

	blob, err := serializeEmbedding(embedding)
	if err != nil {
		t.Fatalf("serializeEmbedding falhou: %v", err)
	}
	if len(blob) != EmbeddingDim*4 {
		t.Fatalf("tamanho esperado %d, got %d", EmbeddingDim*4, len(blob))
	}
}

func TestSerializeEmbedding_TudoZero(t *testing.T) {
	embedding := make([]float32, EmbeddingDim)

	blob, err := serializeEmbedding(embedding)
	if err != nil {
		t.Fatalf("serializeEmbedding com zeros falhou: %v", err)
	}
	if len(blob) != EmbeddingDim*4 {
		t.Fatalf("tamanho esperado %d, got %d", EmbeddingDim*4, len(blob))
	}
	for i, b := range blob {
		if b != 0 {
			t.Fatalf("byte %d esperado 0, got %d", i, b)
		}
	}
}

func TestSerializeEmbedding_NaN(t *testing.T) {
	embedding := make([]float32, EmbeddingDim)
	embedding[100] = float32(math.NaN())

	_, err := serializeEmbedding(embedding)
	if err == nil {
		t.Fatal("esperado erro para embedding com NaN")
	}
}

func TestSerializeEmbedding_Inf(t *testing.T) {
	embedding := make([]float32, EmbeddingDim)
	embedding[200] = float32(math.Inf(1))

	_, err := serializeEmbedding(embedding)
	if err == nil {
		t.Fatal("esperado erro para embedding com +Inf")
	}
}

func TestSerializeEmbedding_NegInf(t *testing.T) {
	embedding := make([]float32, EmbeddingDim)
	embedding[300] = float32(math.Inf(-1))

	_, err := serializeEmbedding(embedding)
	if err == nil {
		t.Fatal("esperado erro para embedding com -Inf")
	}
}

// ── SaveEmbedding (chunk individual) ────────────────────────────

func TestSaveEmbedding_DimensaoInvalida(t *testing.T) {
	s := newTestStore(t)
	err := s.SaveEmbedding("notes/test.md#0", []float32{1.0, 2.0})
	if err == nil {
		t.Fatal("esperado erro para embedding com dimensao invalida")
	}
}

func TestSaveEmbedding_NaN(t *testing.T) {
	s := newTestStore(t)
	embedding := make([]float32, EmbeddingDim)
	embedding[0] = float32(math.NaN())
	err := s.SaveEmbedding("notes/test.md#0", embedding)
	if err == nil {
		t.Fatal("esperado erro para embedding com NaN")
	}
}

// ── SaveNoteChunks / DeleteEmbedding ────────────────────────────

func TestSaveNoteChunks_AndDelete(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/chunk_test.md"

	// Cria nota no banco
	err := s.SaveNote(filename, "# Teste\n\nParágrafo 1\n\nParágrafo 2", "2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("SaveNote falhou: %v", err)
	}
	err = s.SetFileTags(filename, []string{})
	if err != nil {
		t.Fatalf("SetFileTags falhou: %v", err)
	}

	// Salva 2 chunks
	chunks := []ChunkInfo{
		makeChunk(filename, 0, "Parágrafo 1", 0.5),
		makeChunk(filename, 1, "Parágrafo 2", 0.7),
	}
	err = s.SaveNoteChunks(filename, chunks)
	if err != nil {
		t.Fatalf("SaveNoteChunks falhou: %v", err)
	}

	// Verifica que existe (pelo filename)
	if !s.HasEmbedding(filename) {
		t.Fatal("HasEmbedding deveria retornar true apos save")
	}

	// Deleta
	err = s.DeleteEmbedding(filename)
	if err != nil {
		t.Fatalf("DeleteEmbedding falhou: %v", err)
	}

	// Verifica que nao existe mais
	if s.HasEmbedding(filename) {
		t.Fatal("HasEmbedding deveria retornar false apos delete")
	}
}

func TestSaveNoteChunks_Upsert(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/upsert_test.md"

	err := s.SaveNote(filename, "# Upsert", "2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("SaveNote falhou: %v", err)
	}
	err = s.SetFileTags(filename, []string{})
	if err != nil {
		t.Fatalf("SetFileTags falhou: %v", err)
	}

	// Primeiro save (1 chunk)
	chunks1 := []ChunkInfo{
		makeChunk(filename, 0, "Versão 1", 1.0),
	}
	err = s.SaveNoteChunks(filename, chunks1)
	if err != nil {
		t.Fatalf("primeiro SaveNoteChunks falhou: %v", err)
	}

	// Segundo save com chunk diferente (upsert)
	chunks2 := []ChunkInfo{
		makeChunk(filename, 0, "Versão 2", 2.0),
	}
	err = s.SaveNoteChunks(filename, chunks2)
	if err != nil {
		t.Fatalf("segundo SaveNoteChunks (upsert) falhou: %v", err)
	}

	// Deve continuar existindo
	if !s.HasEmbedding(filename) {
		t.Fatal("HasEmbedding deveria retornar true apos upsert")
	}
}

func TestSaveNoteChunks_MultiplosChunks(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/multi_chunk.md"

	s.SaveNote(filename, "# Multi\n\nChunk A\n\nChunk B\n\nChunk C", "2024-01-01T00:00:00Z")
	s.SetFileTags(filename, []string{})

	chunks := []ChunkInfo{
		makeChunk(filename, 0, "Chunk A", 0.1),
		makeChunk(filename, 1, "Chunk B", 0.2),
		makeChunk(filename, 2, "Chunk C", 0.3),
	}
	err := s.SaveNoteChunks(filename, chunks)
	if err != nil {
		t.Fatalf("SaveNoteChunks falhou: %v", err)
	}

	if !s.HasEmbedding(filename) {
		t.Fatal("HasEmbedding deveria retornar true")
	}

	// Verifica contagem de chunks via query direta
	var count int
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_chunks WHERE filename = ?`, filename).Scan(&count)
	if count != 3 {
		t.Fatalf("esperado 3 chunks, got %d", count)
	}

	var embCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_embeddings WHERE chunk_id LIKE ?`, filename+`#%`).Scan(&embCount)
	if embCount != 3 {
		t.Fatalf("esperado 3 embeddings, got %d", embCount)
	}
}

// ── HasEmbedding ────────────────────────────────────────────────

func TestHasEmbedding_Inexistente(t *testing.T) {
	s := newTestStore(t)
	if s.HasEmbedding("notes/nao_existe.md") {
		t.Fatal("HasEmbedding deveria retornar false para nota inexistente")
	}
}

// ── GetEmbeddingStatus ──────────────────────────────────────────

func TestGetEmbeddingStatus_Vazio(t *testing.T) {
	s := newTestStore(t)

	status, err := s.GetEmbeddingStatus()
	if err != nil {
		t.Fatalf("GetEmbeddingStatus falhou: %v", err)
	}
	if status.TotalNotes != 0 {
		t.Fatalf("TotalNotes esperado 0, got %d", status.TotalNotes)
	}
	if status.IndexedNotes != 0 {
		t.Fatalf("IndexedNotes esperado 0, got %d", status.IndexedNotes)
	}
	if status.PendingNotes != 0 {
		t.Fatalf("PendingNotes esperado 0, got %d", status.PendingNotes)
	}
}

func TestGetEmbeddingStatus_ComNotas(t *testing.T) {
	s := newTestStore(t)

	// Cria algumas notas embeddable
	for i, name := range []string{"notes/a.md", "notes/b.md", "notes/c.md"} {
		err := s.SaveNote(name, "# Nota "+name, "2024-01-01T00:00:00Z")
		if err != nil {
			t.Fatalf("SaveNote %d falhou: %v", i, err)
		}
		err = s.SetFileTags(name, []string{})
		if err != nil {
			t.Fatalf("SetFileTags %d falhou: %v", i, err)
		}
	}

	status, err := s.GetEmbeddingStatus()
	if err != nil {
		t.Fatalf("GetEmbeddingStatus falhou: %v", err)
	}
	if status.TotalNotes != 3 {
		t.Fatalf("TotalNotes esperado 3, got %d", status.TotalNotes)
	}
	if status.PendingNotes != 3 {
		t.Fatalf("PendingNotes esperado 3, got %d", status.PendingNotes)
	}
	if status.IndexedNotes != 0 {
		t.Fatalf("IndexedNotes esperado 0, got %d", status.IndexedNotes)
	}

	// Indexa uma nota (1 chunk)
	err = s.SaveNoteChunks("notes/a.md", []ChunkInfo{
		makeChunk("notes/a.md", 0, "Nota A", 0.5),
	})
	if err != nil {
		t.Fatalf("SaveNoteChunks falhou: %v", err)
	}

	status, err = s.GetEmbeddingStatus()
	if err != nil {
		t.Fatalf("GetEmbeddingStatus falhou: %v", err)
	}
	if status.IndexedNotes != 1 {
		t.Fatalf("IndexedNotes esperado 1, got %d", status.IndexedNotes)
	}
	if status.PendingNotes != 2 {
		t.Fatalf("PendingNotes esperado 2, got %d", status.PendingNotes)
	}
}

// ── SearchSimilar ───────────────────────────────────────────────

func TestSearchSimilar_ComChunks(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/search_test.md"

	s.SaveNote(filename, "# Busca\n\nPrimeiro parágrafo\n\nSegundo parágrafo", "2024-01-01T00:00:00Z")
	s.SetFileTags(filename, []string{})

	// Indexa com 2 chunks
	chunks := []ChunkInfo{
		makeChunk(filename, 0, "Primeiro parágrafo", 1.0),
		makeChunk(filename, 1, "Segundo parágrafo", 0.8),
	}
	err := s.SaveNoteChunks(filename, chunks)
	if err != nil {
		t.Fatalf("SaveNoteChunks falhou: %v", err)
	}

	// Busca pelo embedding similar ao primeiro chunk
	queryEmb := make([]float32, EmbeddingDim)
	queryEmb[0] = 1.0

	results, err := s.SearchSimilar(queryEmb, 10)
	if err != nil {
		t.Fatalf("SearchSimilar falhou: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("esperado 1 resultado, got %d", len(results))
	}
	if results[0].Filename != filename {
		t.Fatalf("esperado filename %s, got %s", filename, results[0].Filename)
	}
}

func TestSearchSimilar_DiferentesNotas(t *testing.T) {
	s := newTestStore(t)

	for _, name := range []string{"notes/a.md", "notes/b.md"} {
		s.SaveNote(name, "# Nota", "2024-01-01T00:00:00Z")
		s.SetFileTags(name, []string{})
	}

	// Insere chunks para ambas as notas
	s.SaveNoteChunks("notes/a.md", []ChunkInfo{
		makeChunk("notes/a.md", 0, "Nota A", 1.0),
	})
	s.SaveNoteChunks("notes/b.md", []ChunkInfo{
		makeChunk("notes/b.md", 0, "Nota B", 2.0),
	})

	queryEmb := make([]float32, EmbeddingDim)
	queryEmb[0] = 1.0

	results, err := s.SearchSimilar(queryEmb, 10)
	if err != nil {
		t.Fatalf("SearchSimilar falhou: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("esperado 2 resultados, got %d", len(results))
	}
}

// ── GetPendingEmbeddingNotes ────────────────────────────────────

func TestGetPendingEmbeddingNotes_Vazio(t *testing.T) {
	s := newTestStore(t)

	pending, err := s.GetPendingEmbeddingNotes(10)
	if err != nil {
		t.Fatalf("GetPendingEmbeddingNotes falhou: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("esperado 0 pendentes, got %d", len(pending))
	}
}

func TestGetPendingEmbeddingNotes_ComPendentes(t *testing.T) {
	s := newTestStore(t)

	for i, name := range []string{"notes/p1.md", "notes/p2.md", "notes/p3.md", "notes/p4.md", "notes/p5.md"} {
		err := s.SaveNote(name, "# Conteudo "+name, "2024-01-01T00:00:00Z")
		if err != nil {
			t.Fatalf("SaveNote %d falhou: %v", i, err)
		}
		err = s.SetFileTags(name, []string{})
		if err != nil {
			t.Fatalf("SetFileTags %d falhou: %v", i, err)
		}
	}

	// Indexa 2 (com chunks)
	s.SaveNoteChunks("notes/p1.md", []ChunkInfo{makeChunk("notes/p1.md", 0, "P1", 0.5)})
	s.SaveNoteChunks("notes/p2.md", []ChunkInfo{makeChunk("notes/p2.md", 0, "P2", 0.5)})

	pending, err := s.GetPendingEmbeddingNotes(10)
	if err != nil {
		t.Fatalf("GetPendingEmbeddingNotes falhou: %v", err)
	}
	if len(pending) != 3 {
		t.Fatalf("esperado 3 pendentes, got %d", len(pending))
	}
}

func TestGetPendingEmbeddingNotes_RespeitaLimite(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("notes/lim_%d.md", i)
		s.SaveNote(name, "# Nota", "2024-01-01T00:00:00Z")
		s.SetFileTags(name, []string{})
	}

	pending, err := s.GetPendingEmbeddingNotes(3)
	if err != nil {
		t.Fatalf("GetPendingEmbeddingNotes falhou: %v", err)
	}
	if len(pending) > 3 {
		t.Fatalf("esperado no maximo 3 pendentes, got %d", len(pending))
	}
}

func TestGetPendingEmbeddingNotes_ComStale(t *testing.T) {
	s := newTestStore(t)

	// Cria 2 notas
	s.SaveNote("notes/a.md", "# Conteudo A", "2024-01-01T00:00:00Z")
	s.SetFileTags("notes/a.md", []string{})
	s.SaveNote("notes/b.md", "# Conteudo B", "2024-01-01T00:00:00Z")
	s.SetFileTags("notes/b.md", []string{})

	// Indexa ambas
	s.SaveNoteChunks("notes/a.md", []ChunkInfo{makeChunk("notes/a.md", 0, "A", 0.5)})
	s.SaveNoteChunks("notes/b.md", []ChunkInfo{makeChunk("notes/b.md", 0, "B", 0.5)})

	// status: Total = 2, Indexed = 2, Pending = 0, Stale = 0
	status, _ := s.GetEmbeddingStatus()
	if status.PendingNotes != 0 {
		t.Fatalf("esperado 0 pendentes inicialmente, got %d", status.PendingNotes)
	}

	// Altera mtime de notes/a.md no banco sem reindexar para torná-la desatualizada (stale)
	s.SaveNote("notes/a.md", "# Conteudo A Modificado", "2024-01-02T00:00:00Z")

	// status: Total = 2, Indexed = 2, Pending = 1 (por ser stale), Stale = 1
	status, _ = s.GetEmbeddingStatus()
	if status.StaleNotes != 1 {
		t.Fatalf("esperado 1 stale, got %d", status.StaleNotes)
	}
	if status.PendingNotes != 1 {
		t.Fatalf("esperado 1 pendente (stale), got %d", status.PendingNotes)
	}

	// Deve retornar notes/a.md como pendente
	pending, err := s.GetPendingEmbeddingNotes(10)
	if err != nil {
		t.Fatalf("GetPendingEmbeddingNotes falhou: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("esperado 1 pendente, got %d", len(pending))
	}
	if pending[0].Filename != "notes/a.md" {
		t.Fatalf("esperado notes/a.md, got %s", pending[0].Filename)
	}
}

func TestSaveNoteChunks_ChangeChunkCount(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/chunk_count_test.md"

	s.SaveNote(filename, "# Nota", "2024-01-01T00:00:00Z")
	s.SetFileTags(filename, []string{})

	// 1. Salva 3 chunks
	chunks := []ChunkInfo{
		makeChunk(filename, 0, "A", 0.1),
		makeChunk(filename, 1, "B", 0.2),
		makeChunk(filename, 2, "C", 0.3),
	}
	s.SaveNoteChunks(filename, chunks)

	var count int
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_chunks WHERE filename = ?`, filename).Scan(&count)
	if count != 3 {
		t.Fatalf("esperado 3 chunks, got %d", count)
	}

	// 2. Modifica a nota e envia apenas 1 chunk (simulando texto apagado)
	chunksNovo := []ChunkInfo{
		makeChunk(filename, 0, "A Modificado", 0.5),
	}
	s.SaveNoteChunks(filename, chunksNovo)

	s.DB.QueryRow(`SELECT COUNT(*) FROM note_chunks WHERE filename = ?`, filename).Scan(&count)
	if count != 1 {
		t.Fatalf("esperado 1 chunk apos atualizacao reduzida, got %d. O banco deve apagar os antigos.", count)
	}

	// Garante que a tabela de embeddings também limpou os órfãos
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_embeddings WHERE chunk_id LIKE ?`, filename+`#%`).Scan(&count)
	if count != 1 {
		t.Fatalf("esperado 1 embedding orfao apagado, got %d", count)
	}
}

func TestSearchSimilar_ChunkGrouping(t *testing.T) {
	s := newTestStore(t)

	filenameA := "notes/longa_a.md"
	s.SaveNote(filenameA, "# Nota Longa A", "2024-01-01T00:00:00Z")
	s.SetFileTags(filenameA, []string{})
	
	filenameB := "notes/curta_b.md"
	s.SaveNote(filenameB, "# Nota Curta B", "2024-01-01T00:00:00Z")
	s.SetFileTags(filenameB, []string{})

	// Nota longa tem 4 chunks. O último chunk tem um embedding muito específico (1.0).
	chunksA := []ChunkInfo{
		makeChunk(filenameA, 0, "Intro", 0.1),
		makeChunk(filenameA, 1, "Meio 1", 0.1),
		makeChunk(filenameA, 2, "Meio 2", 0.1),
		makeChunk(filenameA, 3, "Conclusao Escondida", 1.0), // <- Alvo!
	}
	s.SaveNoteChunks(filenameA, chunksA)

	// Nota B tem 1 chunk.
	chunksB := []ChunkInfo{
		makeChunk(filenameB, 0, "Nada a ver", -1.0),
	}
	s.SaveNoteChunks(filenameB, chunksB)

	// Busca: queryEmb perfeito para o chunk 3 da Nota A
	queryEmb := make([]float32, EmbeddingDim)
	queryEmb[0] = 1.0

	results, err := s.SearchSimilar(queryEmb, 10)
	if err != nil {
		t.Fatalf("SearchSimilar falhou: %v", err)
	}

	// O retorno da busca deve agrupar (GROUP BY filename) e retornar o arquivo com a melhor distância
	if len(results) != 2 {
		t.Fatalf("esperado 2 arquivos retornados no total, got %d", len(results))
	}
	if results[0].Filename != filenameA {
		t.Fatalf("esperado que o filenameA estivesse no topo pois seu 4º chunk e o mais proximo. Got: %s", results[0].Filename)
	}
}

func TestGetNoteEmbeddings(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/get_embeddings_test.md"

	s.SaveNote(filename, "# Nota", "2024-01-01T00:00:00Z")
	s.SetFileTags(filename, []string{})

	// Salva 2 chunks com valores específicos
	chunks := []ChunkInfo{
		makeChunk(filename, 0, "Chunk 1", 12.34),
		makeChunk(filename, 1, "Chunk 2", 56.78),
	}
	err := s.SaveNoteChunks(filename, chunks)
	if err != nil {
		t.Fatalf("SaveNoteChunks falhou: %v", err)
	}

	embeddings, err := s.GetNoteEmbeddings(filename)
	if err != nil {
		t.Fatalf("GetNoteEmbeddings falhou: %v", err)
	}

	if len(embeddings) != 2 {
		t.Fatalf("esperado 2 embeddings, got %d", len(embeddings))
	}

	// Verifica se os valores float32 foram restaurados corretamente
	if embeddings[0][0] != 12.34 {
		t.Fatalf("esperado 12.34 no index 0, got %f", embeddings[0][0])
	}
	if embeddings[1][0] != 56.78 {
		t.Fatalf("esperado 56.78 no index 0, got %f", embeddings[1][0])
	}
}
