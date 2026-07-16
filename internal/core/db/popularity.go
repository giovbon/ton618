package db

import (
	"database/sql"
	"math"
	"time"

	"ton618/internal/core/db/generated"
)

// ---------------------------------------------------------------------------
// popularity & synaptic weights (RLHF)
// ---------------------------------------------------------------------------

// GetPopularity returns the access count for a file (Legacy).
func (s *Store) GetPopularity(arquivo string) int {
	count, _ := s.Q.GetPopularity(s.queryCtx(), arquivo)
	return int(count.Int64)
}

// IncrementPopularity increases the access count for a file by 1.
// Agora delegamos para o RLHF
func (s *Store) IncrementPopularity(arquivo string) error {
	return s.ApplyInteractionReward(arquivo, "focus_zoom")
}

// GetAllPopularity returns all popularity records as a map of file -> count.
func (s *Store) GetAllPopularity() (map[string]int, error) {
	rows, err := s.Q.GetAllPopularity(s.queryCtx())
	if err != nil {
		return nil, err
	}

	result := make(map[string]int)
	for _, r := range rows {
		result[r.Arquivo] = int(r.Count.Int64)
	}
	return result, nil
}

// ResetPopularity deletes the popularity record for a file.
func (s *Store) ResetPopularity(arquivo string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.ResetPopularity(s.queryCtx(), arquivo)
}

// ApplyInteractionReward aplica recompensas baseadas em interações implícitas e explícitas (RLHF)
func (s *Store) ApplyInteractionReward(arquivo string, interactionType string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	var reward float64
	switch interactionType {
	case "focus_zoom":           // Abrir/salvar notas, planilhas, desenhos
		reward = 0.10
	case "explicit_upvote":      // Clique manual em "Favoritar" ou "Core"
		reward = 1.00
	case "explicit_downvote":    // Clique manual em "Depreciar" ou "Rascunho"
		reward = -2.00
	case "scroll_past_penalty":  // Notas que estavam acima na busca e foram ignoradas
		reward = -0.05
	default:
		return nil
	}

	return s.Q.ApplyInteractionReward(s.queryCtx(), dbgen.ApplyInteractionRewardParams{
		Arquivo:          arquivo,
		Reward:           reward,
		LastInteractedAt: sql.NullString{String: time.Now().Format(time.RFC3339), Valid: true},
	})
}

// GetSynapticWeight calcula o peso sináptico atual aplicando o Forgetting Curve (decaimento logarítmico)
func (s *Store) GetSynapticWeight(arquivo string) float64 {
	row, err := s.Q.GetSynapticWeight(s.queryCtx(), arquivo)
	if err != nil {
		return 1.0 // Peso neutro padrão
	}

	weight := row.Weight
	lastInteractedStr := row.LastInteractedAt

	if lastInteractedStr == "" {
		return weight
	}

	lastInteracted, err := time.Parse(time.RFC3339, lastInteractedStr)
	if err != nil {
		return weight
	}

	days := time.Since(lastInteracted).Hours() / 24.0

	// Aplica decaimento se não for acessado há mais de 30 dias (curva de esquecimento de Hermann Ebbinghaus)
	if days >= 30 {
		decay := math.Log10(days/30.0 + 1.0)
		weight = math.Max(0.1, weight-decay)
	}

	return weight
}
