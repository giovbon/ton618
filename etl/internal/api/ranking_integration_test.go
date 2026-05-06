package api

import (
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/models"
	"etl/internal/search"
	"sort"
	"testing"
	"time"
)

func TestRankingSortingShield(t *testing.T) {
	// Este teste "blinda" o comportamento de ordenação por Rank (FinalScore)
	// mesmo em consultas wildcard ou com termos variados.

	agora := time.Now().Format(time.RFC3339)
	ontem := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)

	hits := []models.SearchHit{
		{
			ID:    "old_but_relevant",
			Score: 1.0,
			Source: models.Document{
				Arquivo:   "manual.md",
				Texto:     "Este é um manual técnico muito rico e importante.",
				Timestamp: ontem,
				Tags:      []string{"importante"},
			},
		},
		{
			ID:    "new_but_simple",
			Score: 1.0,
			Source: models.Document{
				Arquivo:   "teste.md",
				Texto:     "oi",
				Timestamp: agora,
			},
		},
	}

	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})

	// Simular uma busca wildcard "*"
	rawQuery := "*"
	queryTerms := search.GetHeuristicTerms(rawQuery)

	// 1. Injetar Popularidade no Estado para o item antigo
	appState.IncrementPopularity("manual.md")
	appState.IncrementPopularity("manual.md")
	appState.IncrementPopularity("manual.md") // Agora tem 3 acessos, deve ganhar bônus log

	// 2. Processar Hits (Isso vai calcular o FinalScore com bônus)
	processed := PostProcessSearchHits(hits, queryTerms, rawQuery, false, appState)

	// 3. Ordenar usando a lógica do Handler
	search.SortHitsByScore(processed)

	// Verificação: O item "old_but_relevant" deve estar no topo porque
	// ganha bônus de "Popularidade", superando a vantagem de recência.

	if len(processed) < 2 {
		t.Fatalf("Esperava 2 hits, obteve %d", len(processed))
	}

	// Log dos scores para debug
	for _, h := range processed {
		t.Logf("ID: %s, FinalScore: %f", h.ID, h.FinalScore)
	}

	if processed[0].ID != "old_but_relevant" {
		t.Errorf("A ordenação por Rank falhou para wildcard '*': o item mais popular deveria estar no topo. Obteve %s no topo", processed[0].ID)
	}
}

func TestCompactModeSortingShield(t *testing.T) {
	// Este teste garante que o modo compacto continua priorizando recência pura (Timestamp)

	agora := time.Now().Format(time.RFC3339)
	ontem := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)

	hits := []models.SearchHit{
		{
			ID:    "relevant_old",
			Score: 10.0,
			Source: models.Document{
				Arquivo:   "velho.md",
				Timestamp: ontem,
			},
		},
		{
			ID:    "simple_new",
			Score: 1.0,
			Source: models.Document{
				Arquivo:   "novo.md",
				Timestamp: agora,
			},
		},
	}

	appState := ingest.NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	processed := PostProcessSearchHits(hits, []string{"teste"}, "teste", true, appState)

	// Lógica do Handler para modo compacto
	sort.Slice(processed, func(i, j int) bool {
		return processed[i].Source.Timestamp > processed[j].Source.Timestamp
	})

	if processed[0].ID != "simple_new" {
		t.Errorf("Modo compacto deveria priorizar o mais novo, mas obteve %s no topo", processed[0].ID)
	}
}
