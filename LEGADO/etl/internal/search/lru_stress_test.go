package search

import (
	"etl/internal/models"
	"fmt"
	"testing"
)

func TestLRUStressEviction(t *testing.T) {
	ClearCache()
	maxEntries = 5 // Reduzir para o teste ser rápido

	// 1. Encher o cache
	for i := 0; i < 5; i++ {
		SetCachedResult([]byte(fmt.Sprintf("query-%d", i)), models.SearchResults{})
	}

	// Ordem interna (do mais antigo ao mais novo): 0, 1, 2, 3, 4

	// 2. Acessar o "query-0" (ele deve ir para o final da fila de expulsão)
	_, found := GetCachedResult([]byte("query-0"))
	if !found {
		t.Fatal("query-0 deveria estar no cache")
	}

	// Ordem esperada após promoção: 1, 2, 3, 4, 0

	// 3. Adicionar um novo item (deve expulsar o "query-1" que agora é o mais antigo)
	SetCachedResult([]byte("query-new"), models.SearchResults{})

	// 4. Verificar se query-1 foi expulso
	_, found = GetCachedResult([]byte("query-1"))
	if found {
		t.Error("query-1 deveria ter sido expulso (era o mais antigo após a promoção do 0)")
	}

	// 5. Verificar se query-0 ainda está lá (foi salvo pela promoção)
	_, found = GetCachedResult([]byte("query-0"))
	if !found {
		t.Error("query-0 deveria ter sobrevivido devido à política LRU")
	}

	// Resetar maxEntries para o padrão do sistema após o teste
	maxEntries = 15
	t.Log("LRU Stress test passed: hot items are protected from eviction.")
}
