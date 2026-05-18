package semantic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := newCircuitBreaker()

	// Estado inicial: fechado
	if !cb.allow() {
		t.Error("should allow in closed state")
	}

	// 3 falhas consecutivas devem abrir
	cb.failure()
	cb.failure()
	cb.failure()

	if cb.allow() {
		t.Error("should block after 3 failures")
	}
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	cb := newCircuitBreaker()
	cb.openTimeout = 50 * time.Millisecond

	// Abrir o circuito
	cb.failure()
	cb.failure()
	cb.failure()

	if cb.allow() {
		t.Error("should block immediately after opening")
	}

	// Esperar o timeout
	time.Sleep(60 * time.Millisecond)

	// Deve permitir uma tentativa (half-open)
	if !cb.allow() {
		t.Error("should allow in half-open state after timeout")
	}

	// Sucesso deve fechar
	cb.success()
	if !cb.allow() {
		t.Error("should allow in closed state after success")
	}
}

func TestCircuitBreaker_HalfOpen_Failure_Reopens(t *testing.T) {
	cb := newCircuitBreaker()
	cb.openTimeout = 50 * time.Millisecond

	// Abrir
	for i := 0; i < 3; i++ {
		cb.failure()
	}

	// Esperar half-open
	time.Sleep(60 * time.Millisecond)
	if !cb.allow() {
		t.Fatal("should allow in half-open")
	}

	// Falha no half-open deve reabrir
	cb.failure()
	if cb.allow() {
		t.Error("should block after half-open failure")
	}
}

func TestNormalizeVectorUnit(t *testing.T) {
	vec := []float32{3.0, 4.0} // length = 5
	normalizeVector(vec)

	// After normalization, magnitude should be 1.0
	var mag float64
	for _, v := range vec {
		mag += float64(v) * float64(v)
	}
	if mag < 0.99 || mag > 1.01 {
		t.Errorf("normalized vector magnitude should be ~1.0, got %f", mag)
	}
}

func TestNormalizeVector_ZeroVectorUnit(t *testing.T) {
	vec := []float32{0.0, 0.0, 0.0}
	normalizeVector(vec)
	// Should not panic and should remain zeros
	for i, v := range vec {
		if v != 0 {
			t.Errorf("zero vector should remain zero, index %d is %f", i, v)
		}
	}
}

func TestQueryCache_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "embeddings_cache.json")

	// Inicializa
	InitCache(cachePath)

	// Adiciona entrada manualmente
	queryCacheMu.Lock()
	queryCache["test-key"] = []float32{1.0, 2.0}
	queryCacheKeys = []string{"test-key"}
	queryCacheMu.Unlock()

	// Salva
	SaveCache()

	// Verifica que o arquivo existe
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("cache file should exist: %v", err)
	}

	var loaded struct {
		Cache map[string][]float32 `json:"cache"`
		Keys  []string             `json:"keys"`
	}
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("cache file should be valid JSON: %v", err)
	}
	if loaded.Cache["test-key"] == nil {
		t.Error("cache should contain test-key")
	}
}

func TestQueryCache_LoadPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "embeddings_cache.json")

	// Cria cache diretamente no disco
	initial := map[string]interface{}{
		"cache": map[string][]float32{"loaded-key": {3.0, 4.0}},
		"keys":  []string{"loaded-key"},
	}
	data, _ := json.Marshal(initial)
	os.WriteFile(cachePath, data, 0644)

	// Reseta o estado global para teste
	queryCacheMu.Lock()
	queryCache = make(map[string][]float32)
	queryCacheKeys = nil
	queryCacheMu.Unlock()

	// Carrega
	InitCache(cachePath)

	queryCacheMu.RLock()
	val, exists := queryCache["loaded-key"]
	queryCacheMu.RUnlock()

	if !exists {
		t.Error("should load cache from disk")
	}
	if len(val) != 2 || val[0] != 3.0 || val[1] != 4.0 {
		t.Errorf("loaded value mismatch: %v", val)
	}
}

func TestOllamaEmbedding_ShortText(t *testing.T) {
	// Mock server Ollama
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := map[string]interface{}{
			"embeddings": [][]float32{{0.1, 0.2, 0.3}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Reseta o globalCB entre testes
	globalCB = newCircuitBreaker()
	globalCB.threshold = 100 // evita abertura acidental

	embedFn := NewOllamaEmbedding("test-model", server.URL, 3)
	vec, err := embedFn(context.Background(), "hello")
	if err != nil {
		t.Fatalf("embedding should succeed: %v", err)
	}
	if len(vec) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(vec))
	}
}

func TestOllamaEmbedding_CacheHit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := map[string]interface{}{
			"embeddings": [][]float32{{0.5, 0.6}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Reseta cache
	queryCacheMu.Lock()
	queryCache = make(map[string][]float32)
	queryCacheKeys = nil
	queryCacheLimit = 200
	queryCacheMu.Unlock()

	globalCB = newCircuitBreaker()
	globalCB.threshold = 100

	embedFn := NewOllamaEmbedding("cache-model", server.URL, 2)

	// Primeira chamada: vai ao servidor
	_, err := embedFn(context.Background(), "cache-test")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("first call should hit server, got %d calls", callCount)
	}

	// Segunda chamada: deve vir do cache
	_, err = embedFn(context.Background(), "cache-test")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("second call should use cache, got %d calls", callCount)
	}
}

func TestOllamaEmbedding_ServerError_ShortText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	globalCB = newCircuitBreaker()
	globalCB.threshold = 100

	embedFn := NewOllamaEmbedding("error-model", server.URL, 3)
	_, err := embedFn(context.Background(), "too")
	if err == nil {
		t.Error("should fail for text < 3 chars")
	}

	_, err2 := embedFn(context.Background(), "valid text")
	if err2 == nil {
		t.Error("should fail for server error")
	}
}

func TestPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Ollama is running")
	}))
	defer server.Close()

	if err := Ping(server.URL); err != nil {
		t.Errorf("ping should succeed: %v", err)
	}
}

func TestPing_Offline(t *testing.T) {
	if err := Ping("http://127.0.0.1:19999"); err == nil {
		t.Error("ping should fail for offline server")
	}
}
