package search

import (
	"math"
	"reflect"
	"testing"
)

func TestFusionScore(t *testing.T) {
	if got := FusionScore(0, 60); got != 0 {
		t.Errorf("FusionScore(0,60) = %v, want 0", got)
	}
	if got := FusionScore(1, 60); math.Abs(got-1.0/61.0) > 1e-9 {
		t.Errorf("FusionScore(1,60) = %v, want %v", got, 1.0/61.0)
	}
	// k <= 0 usa o default
	if got := FusionScore(2, 0); math.Abs(got-1.0/62.0) > 1e-9 {
		t.Errorf("FusionScore(2,0) = %v, want default 1/62", got)
	}
}

func TestReciprocalRankFusion(t *testing.T) {
	tests := []struct {
		name     string
		fts      map[string]int
		sem      map[string]int
		k        int
		limit    int
		expected []string
	}{
		{
			name:     "Somente no FTS",
			fts:      map[string]int{"a.md": 1, "b.md": 2},
			sem:      nil,
			k:        60,
			limit:    10,
			expected: []string{"a.md", "b.md"},
		},
		{
			name:     "Somente na semantica",
			fts:      nil,
			sem:      map[string]int{"x.md": 1, "y.md": 2},
			k:        60,
			limit:    10,
			expected: []string{"x.md", "y.md"},
		},
		{
			name: "Doc nos dois motores sobe",
			fts:  map[string]int{"comum.md": 5, "solo-fts.md": 1},
			sem:  map[string]int{"comum.md": 2},
			k:    60,
			// comum.md = 1/65 + 1/62 > solo-fts.md = 1/61
			limit:    10,
			expected: []string{"comum.md", "solo-fts.md"},
		},
		{
			name:     "Sem resultados",
			fts:      nil,
			sem:      nil,
			k:        60,
			limit:    10,
			expected: []string{},
		},
		{
			name:     "Limit corta a lista",
			fts:      map[string]int{"a.md": 1, "b.md": 2, "c.md": 3},
			sem:      nil,
			k:        60,
			limit:    2,
			expected: []string{"a.md", "b.md"},
		},
		{
			name:     "k invalido usa default",
			fts:      map[string]int{"a.md": 1},
			sem:      map[string]int{"a.md": 1},
			k:        0,
			limit:    10,
			expected: []string{"a.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReciprocalRankFusion(tt.fts, tt.sem, tt.k, tt.limit)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ReciprocalRankFusion() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReciprocalRankFusion_OrderingStableByBoth(t *testing.T) {
	// Empate de score: doc que aparece nos dois motores vem primeiro.
	fts := map[string]int{"solo.md": 1}
	sem := map[string]int{"solo.md": 1, "outro.md": 1}
	// solo.md: 1/61 + 1/61 = 2/61 ; outro.md: 1/61
	got := ReciprocalRankFusion(fts, sem, 60, 10)
	if len(got) != 2 || got[0] != "solo.md" {
		t.Errorf("esperava solo.md primeiro (nos dois motores), got %v", got)
	}
}

func TestWeightedFusionScore(t *testing.T) {
	if got := WeightedFusionScore(1, 60, 1.0); math.Abs(got-1.0/61.0) > 1e-9 {
		t.Errorf("score 1.0 deve dar contribuição cheia, got %v", got)
	}
	// score 0 (indisponível) = neutro: metade da contribuição pura.
	if got := WeightedFusionScore(1, 60, 0); math.Abs(got-0.5/61.0) > 1e-9 {
		t.Errorf("score 0 deve dar metade (neutro), got %v", got)
	}
	// score fora de [0,1] é limitado.
	if got := WeightedFusionScore(2, 60, 5.0); math.Abs(got-1.0/62.0) > 1e-9 {
		t.Errorf("score >1 deve limitar a 1, got %v", got)
	}
	if got := WeightedFusionScore(2, 60, -3.0); math.Abs(got-0.5/62.0) > 1e-9 {
		t.Errorf("score <0 deve limitar a 0, got %v", got)
	}
}

func TestReciprocalRankFusionWeighted(t *testing.T) {
	t.Run("preserva ordem dentro do mesmo motor", func(t *testing.T) {
		fts := map[string]int{"a.md": 1, "b.md": 2, "c.md": 3}
		ftsScore := map[string]float64{"a.md": 1.0, "b.md": 0.6, "c.md": 0.2}
		got := ReciprocalRankFusionWeighted(fts, nil, ftsScore, nil, 60, 10)
		want := []string{"a.md", "b.md", "c.md"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ordem dentro do FTS mudou indevidamente: got %v, want %v", got, want)
		}
	})

	t.Run("score alto de mesmo rank desempata", func(t *testing.T) {
		// Mesmo rank nos dois motores → a qualidade (score) decide.
		fts := map[string]int{"fraco.md": 2}
		sem := map[string]int{"fraco.md": 1, "forte.md": 1}
		ftsScore := map[string]float64{"fraco.md": 1.0}
		semScore := map[string]float64{"fraco.md": 0.5, "forte.md": 1.0}
		got := ReciprocalRankFusionWeighted(fts, sem, ftsScore, semScore, 60, 10)
		if len(got) != 2 {
			t.Fatalf("esperava 2 docs, got %v", got)
		}
		// fraco.md = 1/62 (score .5→peso .75) + 1/62 (fts score 1)
		// forte.md  = 1/62 (score 1) — fraco nos dois motores deve vencer.
		if got[0] != "fraco.md" {
			t.Errorf("esperava fraco.md (nos dois motores) primeiro, got %v", got)
		}
	})

	t.Run("sem scores usa peso neutro (RRF puro)", func(t *testing.T) {
		fts := map[string]int{"a.md": 1, "b.md": 2}
		got := ReciprocalRankFusionWeighted(fts, nil, nil, nil, 60, 10)
		want := []string{"a.md", "b.md"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("sem scores deveria equivaler ao RRF puro, got %v, want %v", got, want)
		}
	})

	t.Run("sem resultados", func(t *testing.T) {
		got := ReciprocalRankFusionWeighted(nil, nil, nil, nil, 60, 10)
		if len(got) != 0 {
			t.Errorf("esperava lista vazia, got %v", got)
		}
	})

	t.Run("limit corta a lista", func(t *testing.T) {
		fts := map[string]int{"a.md": 1, "b.md": 2, "c.md": 3}
		got := ReciprocalRankFusionWeighted(fts, nil, nil, nil, 60, 2)
		if len(got) != 2 || got[0] != "a.md" || got[1] != "b.md" {
			t.Errorf("limit=2 falhou, got %v", got)
		}
	})

	t.Run("qualidade distingue docs só-semânticos de mesmo rank", func(t *testing.T) {
		sem := map[string]int{"perto.md": 1, "longe.md": 1}
		semScore := map[string]float64{"perto.md": 0.95, "longe.md": 0.62}
		got := ReciprocalRankFusionWeighted(nil, sem, nil, semScore, 60, 10)
		if len(got) != 2 || got[0] != "perto.md" {
			t.Errorf("mesmo rank semântico: similaridade maior deve vir primeiro, got %v", got)
		}
	})

	t.Run("doc nos dois motores supera só-semântico de mesmo rank", func(t *testing.T) {
		sem := map[string]int{"comum.md": 1, "solo-sem.md": 1}
		fts := map[string]int{"comum.md": 30}
		semScore := map[string]float64{"comum.md": 0.5, "solo-sem.md": 1.0}
		ftsScore := map[string]float64{"comum.md": 0.5}
		got := ReciprocalRankFusionWeighted(fts, sem, ftsScore, semScore, 60, 10)
		if len(got) != 2 || got[0] != "comum.md" {
			t.Errorf("doc nos dois motores deveria vencer mesmo com rank FTS alto, got %v", got)
		}
	})
}
