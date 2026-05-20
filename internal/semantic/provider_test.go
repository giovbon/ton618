package semantic

import (
	"testing"
)

func TestNewProvider_Gemini(t *testing.T) {
	// Com API key, deve criar GeminiProvider
	p := NewProvider("gemini", "fake-key", "gemini-embedding-2", "", "", "", 768)
	if p == nil {
		t.Fatal("provider nao deveria ser nil")
	}
	if p.Dimensions() != 768 {
		t.Fatalf("esperado 768 dimensoes, got %d", p.Dimensions())
	}
}

func TestNewProvider_GeminiSemChave(t *testing.T) {
	// Sem API key, deve cair para Ollama
	p := NewProvider("gemini", "", "gemini-embedding-2", "", "http://localhost:11434", "nomic-embed-text", 768)
	if p == nil {
		t.Fatal("provider nao deveria ser nil mesmo sem chave")
	}
}

func TestNewProvider_Ollama(t *testing.T) {
	p := NewProvider("ollama", "", "", "", "http://ollama:11434", "nomic-embed-text", 768)
	if p == nil {
		t.Fatal("provider ollama nao deveria ser nil")
	}
}

func TestNewProvider_OpenAI(t *testing.T) {
	p := NewProvider("openai", "sk-fake", "text-embedding-3-small", "https://api.openai.com/v1", "", "", 1536)
	if p == nil {
		t.Fatal("provider openai nao deveria ser nil")
	}
	if p.Dimensions() != 1536 {
		t.Fatalf("esperado 1536 dimensoes, got %d", p.Dimensions())
	}
}

func TestCache_SetAndGet(t *testing.T) {
	key := "test-key"
	vec := []float32{0.1, 0.2, 0.3}

	cacheSet(key, vec)
	got, ok := cacheGet(key)
	if !ok {
		t.Fatal("cache miss apos cacheSet")
	}
	if len(got) != 3 || got[0] != 0.1 || got[1] != 0.2 || got[2] != 0.3 {
		t.Fatalf("cache retornou vetor incorreto: %v", got)
	}
}

func TestNormalizeVectorFloat32(t *testing.T) {
	vec := []float32{3.0, 4.0}
	NormalizeVector(vec)

	// Norma deve ser ~1.0 (3^2 + 4^2 = 25, sqrt(25) = 5, divide tudo por 5)
	if vec[0] < 0.59 || vec[0] > 0.61 {
		t.Fatalf("vetor normalizado incorreto: %v", vec)
	}
}
