package db

import (
	"encoding/binary"
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

func TestSaveEmbedding_SucessoIndividual(t *testing.T) {
	s := newTestStore(t)
	embedding := make([]float32, EmbeddingDim)
	for i := range embedding {
		embedding[i] = float32(i) * 0.1
	}

	err := s.SaveEmbedding("notes/individual.md#0", embedding)
	if err != nil {
		t.Fatalf("SaveEmbedding individual falhou: %v", err)
	}

	// Verifica via query direta que o blob foi inserido
	var blob []byte
	err = s.DB.QueryRow(`SELECT embedding FROM note_embeddings WHERE chunk_id = ?`, "notes/individual.md#0").Scan(&blob)
	if err != nil {
		t.Fatalf("SELECT falhou: %v", err)
	}
	if len(blob) != EmbeddingDim*4 {
		t.Fatalf("tamanho do blob esperado %d, got %d", EmbeddingDim*4, len(blob))
	}

	// Deserializa e verifica valores
	restored := deserializeEmbedding(blob)
	for i, v := range restored {
		expected := float32(i) * 0.1
		if v != expected {
			t.Fatalf("pos %d: esperado %f, got %f", i, expected, v)
		}
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

// deserializeEmbedding é o inverso de serializeEmbedding — usado apenas em testes.
func deserializeEmbedding(blob []byte) []float32 {
	if len(blob)%4 != 0 {
		return nil
	}
	emb := make([]float32, len(blob)/4)
	for i := range emb {
		bits := binary.LittleEndian.Uint32(blob[i*4 : (i+1)*4])
		emb[i] = math.Float32frombits(bits)
	}
	return emb
}

// ── Round-trip serialize/deserialize ──

func TestSerializeDeserialize_RoundTrip_Zeros(t *testing.T) {
	original := make([]float32, EmbeddingDim)
	blob, err := serializeEmbedding(original)
	if err != nil {
		t.Fatalf("serialize falhou: %v", err)
	}
	restored := deserializeEmbedding(blob)
	for i, v := range restored {
		if v != 0 {
			t.Fatalf("pos %d: esperado 0, got %f", i, v)
		}
	}
}

func TestSerializeDeserialize_RoundTrip_Negativos(t *testing.T) {
	original := make([]float32, EmbeddingDim)
	original[0] = -3.14
	original[100] = -0.001
	original[200] = -1e10
	original[383] = -1.0

	blob, err := serializeEmbedding(original)
	if err != nil {
		t.Fatalf("serialize falhou: %v", err)
	}
	restored := deserializeEmbedding(blob)

	if restored[0] != -3.14 {
		t.Fatalf("pos 0: esperado -3.14, got %f", restored[0])
	}
	if restored[100] != -0.001 {
		t.Fatalf("pos 100: esperado -0.001, got %f", restored[100])
	}
	if restored[200] != -1e10 {
		t.Fatalf("pos 200: esperado -1e10, got %f", restored[200])
	}
	if restored[383] != -1.0 {
		t.Fatalf("pos 383: esperado -1.0, got %f", restored[383])
	}
}

func TestSerializeDeserialize_RoundTrip_Denormalizados(t *testing.T) {
	// Valores float32 denormalizados (muito próximos de zero)
	original := make([]float32, EmbeddingDim)
	original[50] = math.Float32frombits(0x00000001) // menor denormalizado positivo
	original[150] = 1.4e-45                         // menor float32 subnormal
	original[250] = -1.4e-45                        // menor float32 subnormal negativo

	blob, err := serializeEmbedding(original)
	if err != nil {
		t.Fatalf("serialize denormalizados falhou: %v", err)
	}
	restored := deserializeEmbedding(blob)

	if restored[50] != original[50] {
		t.Fatalf("pos 50: round-trip denormalizado falhou: got %x", math.Float32bits(restored[50]))
	}
	if restored[150] != original[150] {
		t.Fatalf("pos 150: round-trip subnormal falhou")
	}
	if restored[250] != original[250] {
		t.Fatalf("pos 250: round-trip subnormal negativo falhou")
	}
}

func TestSerializeDeserialize_RoundTrip_Positivos(t *testing.T) {
	original := make([]float32, EmbeddingDim)
	for i := range original {
		original[i] = float32(i) * 0.05
	}

	blob, err := serializeEmbedding(original)
	if err != nil {
		t.Fatalf("serialize falhou: %v", err)
	}
	restored := deserializeEmbedding(blob)

	for i := range original {
		if original[i] != restored[i] {
			t.Fatalf("pos %d: esperado %f, got %f", i, original[i], restored[i])
		}
	}
}

// ── GetNoteEmbeddings (via DB) ──

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

// ── GetNoteEmbeddings com valores extremos via DB ──

func TestGetNoteEmbeddings_ValoresExtremos(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/extreme_embeddings.md"

	s.SaveNote(filename, "# Extreme", "2024-01-01T00:00:00Z")
	s.SetFileTags(filename, []string{})

	// Cria chunks com valores extremos
	chunk0 := makeChunk(filename, 0, "Zero", 0)
	chunk1P := makeChunk(filename, 1, "Positivo", 3.1415)
	chunk2N := makeChunk(filename, 2, "Negativo", -2.718)
	chunks := []ChunkInfo{chunk0, chunk1P, chunk2N}

	s.SaveNoteChunks(filename, chunks)

	embeddings, err := s.GetNoteEmbeddings(filename)
	if err != nil {
		t.Fatalf("GetNoteEmbeddings falhou: %v", err)
	}
	if len(embeddings) != 3 {
		t.Fatalf("esperado 3 embeddings, got %d", len(embeddings))
	}
	if embeddings[0][0] != 0 {
		t.Fatalf("zero: esperado 0, got %f", embeddings[0][0])
	}
	if embeddings[1][0] != 3.1415 {
		t.Fatalf("positivo: esperado 3.1415, got %f", embeddings[1][0])
	}
	if embeddings[2][0] != -2.718 {
		t.Fatalf("negativo: esperado -2.718, got %f", embeddings[2][0])
	}
}

// TestIsNoteEmbeddableMatchesSQL garante que a lógica de exclusão em Go (isNoteEmbeddable)
// e a lógica de filtragem nas queries SQL (CountEmbeddableNotes e GetPendingEmbeddingNotes)
// estão perfeitamente sincronizadas.
func TestIsNoteEmbeddableMatchesSQL(t *testing.T) {
	s := newTestStore(t)

	// Definimos casos de teste cobrindo os diferentes tipos de notas
	tests := []struct {
		filename string
		content  string
		tags     []string
		expected bool // true se deve ser indexável/embeddable
	}{
		// Indexáveis
		{"notes/normal.md", "# Nota normal", []string{}, true},
		{"notes/com-tag-inutil.md", "# Nota", []string{"estudos"}, true},
		{"notes/typst-nota.md", "# Typst", []string{"typst"}, true},
		{"notes/mindmap-nota.md", "# Mindmap", []string{"mindmap"}, true},
		{"notes/markmap-nota.md", "# Markmap", []string{"markmap"}, true},
		{"notes/youtube-nota.md", "# YouTube", []string{"youtube"}, true},
		{"notes/article-nota.md", "# Artigo", []string{"artigo"}, true},
		{"notes/capture-nota.md", "# Captura", []string{"capture"}, true},

		// Não indexáveis por prefixo de caminho
		{"pdfs/livro.pdf", "# PDF content", []string{}, false},
		{"attachments/imagem.png", "# Anexo", []string{}, false},
		{"archives/velho.md", "# Arquivado", []string{}, false},

		// Não indexáveis por tags
		{"notes/desenho.md", "# Desenho", []string{"drawing"}, false},
		{"notes/planilha.md", "# Planilha", []string{"spreadsheet"}, false},
		{"notes/fluxo.md", "# Mermaid", []string{"mermaid"}, false},
		{"notes/mapa-tag.md", "# Mapa", []string{"map"}, false},
		{"notes/mapa-tag-pt.md", "# Mapa", []string{"mapa"}, false},

		// Nota: Casos com frontmatter (conteúdo) sem tags associadas são considerados
		// embeddable tanto no Go (IsNoteEmbeddable não abre o arquivo por performance)
		// quanto no SQL (não faz busca no texto completo por performance).
		// Na prática, a aplicação sincroniza as tags baseadas no frontmatter ao salvar a nota.
		{"notes/desenho-fm.md", "type: drawing\n# Desenho", []string{}, true},
		{"notes/planilha-fm.md", "type: spreadsheet\n# Planilha", []string{}, true},
		{"notes/fluxo-fm.md", "type: mermaid\n# Mermaid", []string{}, true},
		{"notes/fm-map.md", "type: map\n# Mapa", []string{}, true},
		{"notes/fm-mapa.md", "type: mapa\n# Mapa", []string{}, false},

		// Não indexáveis por heurística de nome de arquivo
		{"notes/mapa-exato.md", "# Mapa", []string{}, false},
		{"notes/mapa.md", "# Mapa", []string{}, false},
	}

	// 1. Salva todas as notas e suas tags no banco de teste
	for _, tc := range tests {
		if err := s.SaveNote(tc.filename, tc.content, "2024-01-01T00:00:00Z"); err != nil {
			t.Fatalf("SaveNote falhou para %s: %v", tc.filename, err)
		}
		if err := s.SetFileTags(tc.filename, tc.tags); err != nil {
			t.Fatalf("SetFileTags falhou para %s: %v", tc.filename, err)
		}
	}

	// 2. Valida a consistência de cada nota individualmente
	for _, tc := range tests {
		// A. Verifica o método Go
		isGoEmbeddable := s.IsNoteEmbeddable(tc.filename, tc.tags)
		if isGoEmbeddable != tc.expected {
			t.Errorf("Go IsNoteEmbeddable discorcorda para %s: esperado %t, got %t", tc.filename, tc.expected, isGoEmbeddable)
		}
	}

	// B. Verifica a contagem do SQL (CountEmbeddableNotes)
	status, err := s.GetEmbeddingStatus()
	if err != nil {
		t.Fatalf("GetEmbeddingStatus falhou: %v", err)
	}

	// Conta quantos no teste deveriam ser indexáveis
	expectedCount := 0
	for _, tc := range tests {
		if tc.expected {
			expectedCount++
		}
	}

	if status.TotalNotes != expectedCount {
		t.Errorf("SQL CountEmbeddableNotes discorda: esperado %d notas indexáveis, total no status foi %d", expectedCount, status.TotalNotes)
	}

	// C. Verifica o conjunto retornado por GetPendingEmbeddingNotes
	pending, err := s.GetPendingEmbeddingNotes(100)
	if err != nil {
		t.Fatalf("GetPendingEmbeddingNotes falhou: %v", err)
	}

	// Cria mapa com os arquivos retornados pelo SQL
	sqlPendingSet := make(map[string]bool)
	for _, p := range pending {
		sqlPendingSet[p.Filename] = true
	}

	// Garante que todas as notas marcadas como embeddable (e apenas elas) estão na lista retornada pelo SQL
	for _, tc := range tests {
		inSQL := sqlPendingSet[tc.filename]
		if tc.expected && !inSQL {
			t.Errorf("SQL falhou em retornar a nota indexável %s nos pendentes", tc.filename)
		}
		if !tc.expected && inSQL {
			t.Errorf("SQL incorretamente retornou a nota não-indexável %s nos pendentes", tc.filename)
		}
	}
}

// TestDeleteNoteCleansEmbeddingsAndOrphanStatus garante que a deleção de nota limpa seus
// chunks/embeddings e que o cálculo de status ignora orfãos pré-existentes.
func TestDeleteNoteCleansEmbeddingsAndOrphanStatus(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/delete_test.md"

	// 1. Cria a nota e salva chunks
	if err := s.SaveNote(filename, "# Nota para deletar", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("SaveNote falhou: %v", err)
	}
	if err := s.SetFileTags(filename, []string{}); err != nil {
		t.Fatalf("SetFileTags falhou: %v", err)
	}

	chunks := []ChunkInfo{
		makeChunk(filename, 0, "Chunk 1", 0.5),
	}
	if err := s.SaveNoteChunks(filename, chunks); err != nil {
		t.Fatalf("SaveNoteChunks falhou: %v", err)
	}

	// Verifica que está tudo indexado
	status, err := s.GetEmbeddingStatus()
	if err != nil {
		t.Fatalf("GetEmbeddingStatus falhou: %v", err)
	}
	if status.TotalNotes != 1 || status.IndexedNotes != 1 {
		t.Fatalf("Status inicial incorreto: Total=%d, Indexed=%d", status.TotalNotes, status.IndexedNotes)
	}

	// 2. Executa a deleção através do DeleteNote
	if err := s.DeleteNote(filename); err != nil {
		t.Fatalf("DeleteNote falhou: %v", err)
	}

	// 3. Garante que os chunks e embeddings foram deletados do banco
	var chunksCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_chunks WHERE filename = ?`, filename).Scan(&chunksCount)
	if chunksCount != 0 {
		t.Errorf("Orfão de note_chunks persistiu: got %d", chunksCount)
	}

	var embCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_embeddings WHERE chunk_id LIKE ?`, filename+`#%`).Scan(&embCount)
	if embCount != 0 {
		t.Errorf("Orfão de note_embeddings persistiu: got %d", embCount)
	}

	// 4. Teste de resiliência: se o órfão de alguma forma existisse (ex: inserido diretamente no SQL),
	// o GetEmbeddingStatus deve ignorar
	filenameOrphan := "notes/orphan.md"
	_, err = s.DB.Exec(`INSERT INTO note_chunks (chunk_id, filename, chunk_index, content, indexed_mtime) VALUES (?, ?, ?, ?, ?)`,
		filenameOrphan+"#0", filenameOrphan, 0, "Conteúdo órfão", "2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("Falha ao forçar órfão no banco de teste: %v", err)
	}

	statusOrphan, err := s.GetEmbeddingStatus()
	if err != nil {
		t.Fatalf("GetEmbeddingStatus falhou: %v", err)
	}

	// Como a nota orphan.md não existe na tabela notes,
	// TotalNotes deve ser 0 e IndexedNotes deve ser 0 (ignorando o órfão)
	if statusOrphan.TotalNotes != 0 {
		t.Errorf("TotalNotes deveria ser 0 (ignorando órfão), got %d", statusOrphan.TotalNotes)
	}
	if statusOrphan.IndexedNotes != 0 {
		t.Errorf("IndexedNotes deveria ser 0 (ignorando órfão), got %d", statusOrphan.IndexedNotes)
	}
}

// TestSaveNoteChunksRaceCondition simula a condição de corrida entre o
// auto-indexador do browser (que gera embeddings em Web Worker) e a
// deleção de uma nota pelo usuário.
//
// Cenário:
//  1. Nota é criada e indexada (chunks/embeddings salvos)
//  2. SearchSimilar retorna a nota normalmente
//  3. Nota é deletada (DeleteAllFileRecords)
//  4. Browser termina de gerar o embedding e chama SaveNoteChunks (atrasado)
//  5. SaveNoteChunks DEVE abortar silenciosamente, sem recriar órfãos
//  6. SearchSimilar NÃO DEVE mais retornar a nota deletada
func TestSaveNoteChunksRaceCondition(t *testing.T) {
	s := newTestStore(t)
	filename := "notes/race_condition_test.md"

	// 1. Cria a nota e salva chunks/embeddings
	if err := s.SaveNote(filename, "# Nota para teste de race condition\nConteudo.", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if err := s.SetFileTags(filename, []string{}); err != nil {
		t.Fatalf("SetFileTags: %v", err)
	}

	chunks := []ChunkInfo{
		makeChunk(filename, 0, "Conteudo do chunk", 0.5),
	}
	if err := s.SaveNoteChunks(filename, chunks); err != nil {
		t.Fatalf("SaveNoteChunks (1ª vez): %v", err)
	}

	// 2. Verifica que a nota aparece na busca semântica
	queryEmb := make([]float32, EmbeddingDim)
	queryEmb[0] = 0.5

	results, err := s.SearchSimilar(queryEmb, 10)
	if err != nil {
		t.Fatalf("SearchSimilar (antes delete): %v", err)
	}
	found := false
	for _, r := range results {
		if r.Filename == filename {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("nota deveria aparecer nos resultados ANTES da deleção")
	}

	// 3. Deleta a nota (simula ação do usuário)
	if err := s.DeleteAllFileRecords(filename); err != nil {
		t.Fatalf("DeleteAllFileRecords: %v", err)
	}

	// Verifica que foi limpo
	var count int
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_chunks WHERE filename = ?`, filename).Scan(&count)
	if count != 0 {
		t.Fatalf("note_chunks deveria estar vazio após delete, got %d", count)
	}
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_embeddings WHERE chunk_id LIKE ?`, filename+`#%`).Scan(&count)
	if count != 0 {
		t.Fatalf("note_embeddings deveria estar vazio após delete, got %d", count)
	}

	// 4. Browser termina de gerar o embedding (atrasado) e tenta salvar
	//    Isso NÃO deve recriar os chunks/embeddings
	err = s.SaveNoteChunks(filename, chunks)
	if err != nil {
		t.Fatalf("SaveNoteChunks (após delete) retornou erro inesperado: %v", err)
	}

	// 5. Verifica que NADA foi recriado
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_chunks WHERE filename = ?`, filename).Scan(&count)
	if count != 0 {
		t.Fatalf("note_chunks foi recriado após deleção! count=%d — SaveNoteChunks deveria ter abortado", count)
	}
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_embeddings WHERE chunk_id LIKE ?`, filename+`#%`).Scan(&count)
	if count != 0 {
		t.Fatalf("note_embeddings foi recriado após deleção! count=%d — SaveNoteChunks deveria ter abortado", count)
	}

	// 6. SearchSimilar NÃO deve mais retornar a nota deletada
	results, err = s.SearchSimilar(queryEmb, 10)
	if err != nil {
		t.Fatalf("SearchSimilar (após delete+save atrasado): %v", err)
	}
	for _, r := range results {
		if r.Filename == filename {
			t.Fatalf("nota deletada NÃO deveria aparecer nos resultados mesmo após SaveNoteChunks tardio! distance=%f", r.Distance)
		}
	}
}

// TestResetAllEmbeddings verifica que ResetAllEmbeddings limpa
// completamente as tabelas note_chunks e note_embeddings.
func TestResetAllEmbeddings(t *testing.T) {
	s := newTestStore(t)

	// Cria algumas notas com chunks/embeddings
	notes := []string{"notes/a.md", "notes/b.md", "notes/c.md"}
	for _, n := range notes {
		s.SaveNote(n, "# Nota "+n, "2024-01-01T00:00:00Z")
		s.SetFileTags(n, []string{})
		s.SaveNoteChunks(n, []ChunkInfo{
			makeChunk(n, 0, "Chunk 0", 0.1),
			makeChunk(n, 1, "Chunk 1", 0.2),
		})
	}

	// Verifica que há dados
	var chunkCount, embCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_chunks`).Scan(&chunkCount)
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_embeddings`).Scan(&embCount)
	if chunkCount == 0 || embCount == 0 {
		t.Fatalf("deveria haver dados antes do reset: chunks=%d embeddings=%d", chunkCount, embCount)
	}

	// Reseta
	if err := s.ResetAllEmbeddings(); err != nil {
		t.Fatalf("ResetAllEmbeddings: %v", err)
	}

	// Verifica que limpou
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_chunks`).Scan(&chunkCount)
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_embeddings`).Scan(&embCount)
	if chunkCount != 0 {
		t.Errorf("note_chunks deveria estar vazio após reset, got %d", chunkCount)
	}
	if embCount != 0 {
		t.Errorf("note_embeddings deveria estar vazio após reset, got %d", embCount)
	}

	// Verifica que notas originais continuam intactas
	for _, n := range notes {
		content, err := s.GetNote(n)
		if err != nil {
			t.Errorf("GetNote(%q) falhou: %v", n, err)
		}
		if content == "" {
			t.Errorf("nota %q deveria continuar existindo após reset", n)
		}
	}

	// Status reflete o reset
	status, _ := s.GetEmbeddingStatus()
	if status.IndexedNotes != 0 {
		t.Errorf("IndexedNotes deveria ser 0 após reset, got %d", status.IndexedNotes)
	}
	if status.PendingNotes != status.TotalNotes {
		t.Errorf("PendingNotes (%d) deveria ser igual a TotalNotes (%d) após reset", status.PendingNotes, status.TotalNotes)
	}
}
