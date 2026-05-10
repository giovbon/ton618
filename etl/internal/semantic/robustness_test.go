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
	embFunc := NewOllamaEmbedding("nomic-embed-text", server.URL, 0)

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

func TestCircuitBreakerAccumulation(t *testing.T) {
	// Verificar que globalCB acumula falhas entre multiplas chamadas
	// (P2.1: circuit breaker global compartilhado)

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Sempre retorna erro para forcar abertura do circuit breaker
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"simulated failure"}`))
	}))
	defer server.Close()

	// Criar duas funcoes de embedding diferentes (simulando chamadas de handlers distintos)
	embFunc1 := NewOllamaEmbedding("test-model", server.URL, 0)
	embFunc2 := NewOllamaEmbedding("test-model", server.URL, 0)

	// Ambas devem compartilhar globalCB e acumular falhas
	for i := 0; i < 3; i++ {
		_, err1 := embFunc1(context.Background(), "texto curto "+string(rune('A'+i)))
		if err1 == nil {
			t.Error("esperava erro na chamada 1")
		}
		_, err2 := embFunc2(context.Background(), "texto curto "+string(rune('Z'-i)))
		if err2 == nil {
			t.Error("esperava erro na chamada 2")
		}
	}

	// Apos 6 falhas acumuladas (3 de cada), o circuit breaker deve estar aberto
	_, err := embFunc1(context.Background(), "mais um texto")
	if err == nil {
		t.Error("esperava erro de circuit open apos 6 falhas acumuladas entre duas funcoes")
	}
	if err != nil && !strings.Contains(err.Error(), "circuit open") {
		t.Errorf("esperava mensagem 'circuit open', obteve: %v", err)
	}

	// Resetar o circuit breaker para nao afetar outros testes
	globalCB.success()
}

func TestMatryoshkaDimension(t *testing.T) {
	// Verifica que o parametro dimension trunca o vetor corretamente (P3.1)
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Retorna vetor de 768 dimensoes (como nomic-embed-text)
		vec := make([]float32, 768)
		for i := range vec {
			vec[i] = float32(i) * 0.001
		}
		resp := struct {
			Embeddings [][]float32 `json:"embeddings"`
		}{
			Embeddings: [][]float32{vec},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Run("Default 512", func(t *testing.T) {
		callCount = 0
		embFunc := NewOllamaEmbedding("test-model", server.URL, 512)
		vec, err := embFunc(context.Background(), "texto de teste")
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if len(vec) != 512 {
			t.Errorf("esperava dimensao 512, obteve %d", len(vec))
		}
	})

	t.Run("Custom 256", func(t *testing.T) {
		callCount = 0
		embFunc := NewOllamaEmbedding("test-model", server.URL, 256)
		vec, err := embFunc(context.Background(), "outro texto")
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if len(vec) != 256 {
			t.Errorf("esperava dimensao 256, obteve %d", len(vec))
		}
	})

	t.Run("Zero usa default 512", func(t *testing.T) {
		callCount = 0
		embFunc := NewOllamaEmbedding("test-model", server.URL, 0)
		vec, err := embFunc(context.Background(), "mais um texto")
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if len(vec) != 512 {
			t.Errorf("esperava dimensao 512 (default), obteve %d", len(vec))
		}
	})

	t.Run("Vector menor que dimension", func(t *testing.T) {
		// Se o modelo retorna vetor menor que a dimensao, nao deve truncar
		callCount = 0
		embFunc := NewOllamaEmbedding("test-model", server.URL, 1000)
		vec, err := embFunc(context.Background(), "texto final")
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if len(vec) != 768 {
			t.Errorf("esperava dimensao original 768 (sem truncamento), obteve %d", len(vec))
		}
	})
}

func TestEmbedBatch(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		var req struct {
			Input []string `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// Verificar que recebeu array de inputs
		if len(req.Input) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Retornar um embedding por input
		embeddings := make([][]float32, len(req.Input))
		for i := range req.Input {
			embeddings[i] = []float32{float32(i) * 0.1, float32(i) * 0.2, 0.5}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"embeddings": embeddings,
		})
	}))
	defer server.Close()

	t.Run("Batch de 3 textos", func(t *testing.T) {
		texts := []string{"texto um", "texto dois", "texto tres"}
		vecs, err := EmbedBatch(context.Background(), server.URL, "test-model", texts, 0)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if len(vecs) != 3 {
			t.Fatalf("esperava 3 vetores, obteve %d", len(vecs))
		}
		for _, text := range texts {
			if _, ok := vecs[text]; !ok {
				t.Errorf("vetor nao encontrado para: %s", text)
			}
		}
	})

	t.Run("Batch com dimensao customizada", func(t *testing.T) {
		texts := []string{"teste"}
		vecs, err := EmbedBatch(context.Background(), server.URL, "test-model", texts, 2)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		vec := vecs["teste"]
		if len(vec) != 2 {
			t.Errorf("esperava dimensao 2, obteve %d", len(vec))
		}
	})

	t.Run("Batch vazio retorna erro", func(t *testing.T) {
		_, err := EmbedBatch(context.Background(), server.URL, "test-model", []string{}, 0)
		if err == nil {
			t.Error("esperava erro para batch vazio")
		}
	})

	t.Run("Normalizacao aplicada no batch", func(t *testing.T) {
		// Verificar que os vetores retornados tem norma ~1.0 (L2)
		texts := []string{"norm-test"}
		vecs, err := EmbedBatch(context.Background(), server.URL, "test-model", texts, 0)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		vec := vecs["norm-test"]
		var sum float64
		for _, v := range vec {
			sum += float64(v) * float64(v)
		}
		norm := sum // ja que normalizamos, sqrt(norm) ≈ 1.0
		if norm < 0.99 || norm > 1.01 {
			t.Errorf("vetor nao normalizado: norma^2 = %f (esperado ~1.0)", norm)
		}
	})
}

func TestNormalizeVector(t *testing.T) {
	t.Run("Vetor unitario permanece unitario", func(t *testing.T) {
		vec := []float32{1.0, 0.0, 0.0}
		normalizeVector(vec)
		if vec[0] != 1.0 || vec[1] != 0.0 || vec[2] != 0.0 {
			t.Errorf("vetor unitario alterado: %v", vec)
		}
	})

	t.Run("Vetor nao-unitario e normalizado", func(t *testing.T) {
		vec := []float32{3.0, 4.0} // norma = 5
		normalizeVector(vec)
		if vec[0] < 0.59 || vec[0] > 0.61 || vec[1] < 0.79 || vec[1] > 0.81 {
			t.Errorf("normalizacao incorreta: %v (esperado ~[0.6, 0.8])", vec)
		}
	})

	t.Run("Vetor zero nao causa NaN", func(t *testing.T) {
		vec := []float32{0.0, 0.0, 0.0}
		normalizeVector(vec) // nao deve panicar nem gerar NaN
		for i, v := range vec {
			if v != 0.0 {
				t.Errorf("vetor zero alterado no indice %d: %f", i, v)
			}
		}
	})

	t.Run("Vetor com valores negativos", func(t *testing.T) {
		vec := []float32{-1.0, -2.0, 2.0, 1.0} // norma = sqrt(1+4+4+1) = sqrt(10) ≈ 3.162
		normalizeVector(vec)
		var sum float64
		for _, v := range vec {
			sum += float64(v) * float64(v)
		}
		if sum < 0.99 || sum > 1.01 {
			t.Errorf("vetor com negativos nao normalizado: norma^2 = %f", sum)
		}
	})
}
