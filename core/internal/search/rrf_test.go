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
