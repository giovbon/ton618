package semantic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChunkByChars(t *testing.T) {
	text := "abcdefghij" // 10 chars
	size := 3
	chunks := chunkByChars(text, size)

	expected := []string{"abc", "def", "ghi", "j"}
	if len(chunks) != len(expected) {
		t.Fatalf("esperava %d chunks, obteve %d", len(expected), len(chunks))
	}

	for i, c := range chunks {
		if c != expected[i] {
			t.Errorf("chunk %d: esperava %s, obteve %s", i, expected[i], c)
		}
	}
}

func TestOllamaEmbeddingRobustness(t *testing.T) {
	// Mock server para simular o Ollama
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		var req struct {
			Input string `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// Simular erro de contexto se o input for grande demais (como acontecia antes)
		if len(req.Input) > 2000 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"the input length exceeds the context length"}`))
			return
		}

		// Resposta padrão de sucesso
		resp := struct {
			Embeddings [][]float32 `json:"embeddings"`
		}{
			Embeddings: [][]float32{{0.1, 0.2, 0.3}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Criar o embedding func apontando para o mock server
	embFunc := NewOllamaEmbedding("nomic-embed-text", server.URL)

	t.Run("Texto Pequeno", func(t *testing.T) {
		callCount = 0
		_, err := embFunc(context.Background(), "olá mundo")
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if callCount != 1 {
			t.Errorf("esperava 1 chamada ao Ollama, obteve %d", callCount)
		}
	})

	t.Run("Texto Grande (Gatilho de Chunking)", func(t *testing.T) {
		callCount = 0
		// Criar um texto de 3500 chars (deve gerar 4 chunks de 1000 + média)
		largeText := strings.Repeat("a", 3500)

		vec, err := embFunc(context.Background(), largeText)
		if err != nil {
			t.Fatalf("deveria ter processado com sucesso via chunking, mas deu erro: %v", err)
		}

		if len(vec) == 0 {
			t.Error("vetor retornado está vazio")
		}

		// Como o limite de chunking é 1000, 3500 chars devem gerar 4 chamadas
		if callCount != 4 {
			t.Errorf("esperava 4 chamadas (chunks), obteve %d. O chunking falhou ou não foi acionado.", callCount)
		}
	})
}
