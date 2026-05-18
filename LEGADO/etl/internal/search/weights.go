package search

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type RankingWeights struct {
	BaseMultiplier     float64 `json:"base_multiplier"`
	BoostTitleExact    float64 `json:"boost_title_exact"`
	BoostTitlePartial  float64 `json:"boost_title_partial"`
	BoostPathContext   float64 `json:"boost_path_context"`
	BoostPhrase        float64 `json:"boost_phrase"`
	BoostFreshnessMax  float64 `json:"boost_freshness_max"`
	BoostTechnical     float64 `json:"boost_technical"`
	BoostLinkAuthority float64 `json:"boost_link_authority"`
}

var (
	currentWeights = GetDefaultWeights()
	weightsMu      sync.RWMutex
	weightsFile    string
)

func GetDefaultWeights() RankingWeights {
	return RankingWeights{
		BaseMultiplier:     1.0,
		BoostTitleExact:    1.0,
		BoostTitlePartial:  0.4,
		BoostPathContext:   0.5,
		BoostPhrase:        1.2, // +120% (Multiplicador)
		BoostFreshnessMax:  0.5, // Aumentado para dar mais peso a notas novas
		BoostTechnical:     0.5,
		BoostLinkAuthority: 1.5,
	}
}

func InitializeWeights(stateDir string) {
	weightsFile = filepath.Join(stateDir, "weights.json")
	if _, err := os.Stat(weightsFile); os.IsNotExist(err) {
		currentWeights = GetDefaultWeights()
		SaveWeights(currentWeights)
	} else {
		data, _ := os.ReadFile(weightsFile)
		json.Unmarshal(data, &currentWeights)
	}
}

func GetWeights() RankingWeights {
	weightsMu.RLock()
	defer weightsMu.RUnlock()
	return currentWeights
}

func SaveWeights(w RankingWeights) error {
	weightsMu.Lock()
	defer weightsMu.Unlock()
	currentWeights = w
	data, _ := json.MarshalIndent(w, "", "  ")
	return os.WriteFile(weightsFile, data, 0644)
}
