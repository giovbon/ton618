package db

import (
	"math"
	"testing"
)

func TestEncodeDecodeVector(t *testing.T) {
	original := []float32{1.5, -2.5, 3.0, 0.0, -0.5, 100.25, -200.75}

	data := EncodeVector(original)
	if data == nil {
		t.Fatal("EncodeVector retornou nil")
	}
	if len(data) != len(original)*4 {
		t.Fatalf("tamanho incorreto: esperado %d, got %d", len(original)*4, len(data))
	}

	decoded := DecodeVector(data)
	if decoded == nil {
		t.Fatal("DecodeVector retornou nil")
	}
	if len(decoded) != len(original) {
		t.Fatalf("comprimento incorreto: esperado %d, got %d", len(original), len(decoded))
	}

	for i := range original {
		if math.Abs(float64(decoded[i]-original[i])) > 0.0001 {
			t.Fatalf("valor incorreto no indice %d: esperado %f, got %f", i, original[i], decoded[i])
		}
	}
}

func TestDecodeVector_EmptyData(t *testing.T) {
	result := DecodeVector([]byte{})
	if result != nil {
		t.Fatal("DecodeVector de slice vazio deveria retornar nil")
	}
}

func TestDecodeVector_InvalidLength(t *testing.T) {
	result := DecodeVector([]byte{0x00, 0x01, 0x02})
	if result != nil {
		t.Fatal("DecodeVector de slice com tamanho invalido deveria retornar nil")
	}
}

func TestEncodeDecode_RoundTrip(t *testing.T) {
	// Testa com 768 floats (tamanho real do Gemini)
	vec := make([]float32, 768)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}

	data := EncodeVector(vec)
	decoded := DecodeVector(data)

	for i := range vec {
		if math.Abs(float64(decoded[i]-vec[i])) > 0.0001 {
			t.Fatalf("round trip falhou no indice %d", i)
		}
	}
}

func TestSetEmbedding_And_GetEmbeddingCount(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Inicialmente zero
	if c := store.GetEmbeddingCount(); c != 0 {
		t.Fatalf("esperado 0 embeddings, got %d", c)
	}

	// Armazenar um embedding
	vec := []float32{0.1, 0.2, 0.3}
	if err := store.SetEmbedding("doc1", vec, "Teste"); err != nil {
		t.Fatalf("SetEmbedding error: %v", err)
	}

	if c := store.GetEmbeddingCount(); c != 1 {
		t.Fatalf("esperado 1 embedding, got %d", c)
	}

	// Recuperar
	nv, err := store.GetEmbedding("doc1")
	if err != nil {
		t.Fatalf("GetEmbedding error: %v", err)
	}
	if nv == nil {
		t.Fatal("GetEmbedding retornou nil")
	}
	if nv.Title != "Teste" {
		t.Fatalf("esperado titulo 'Teste', got %q", nv.Title)
	}
}

func TestDeleteEmbedding(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	store.SetEmbedding("doc-del", []float32{1.0}, "Delete")
	if c := store.GetEmbeddingCount(); c != 1 {
		t.Fatalf("esperado 1 apos inserir, got %d", c)
	}

	store.DeleteEmbedding("doc-del")
	if c := store.GetEmbeddingCount(); c != 0 {
		t.Fatalf("esperado 0 apos deletar, got %d", c)
	}
}

func TestEmbedding2D(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	store.SetEmbedding("doc2d", []float32{0.5, 0.5}, "2D")
	if err := store.SetEmbedding2D("doc2d", 10.5, -20.3); err != nil {
		t.Fatalf("SetEmbedding2D error: %v", err)
	}

	nv, _ := store.GetEmbedding("doc2d")
	if math.Abs(nv.X-10.5) > 0.0001 || math.Abs(nv.Y+20.3) > 0.0001 {
		t.Fatalf("coordenadas 2D incorretas: %f, %f", nv.X, nv.Y)
	}
}

// Helper para criar store de teste
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}
