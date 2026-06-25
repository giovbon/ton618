package db

import (
	"math"
	"time"
)

// ---------------------------------------------------------------------------
// popularity & synaptic weights (RLHF)
// ---------------------------------------------------------------------------

// GetPopularity returns the access count for a file (Legacy).
func (s *Store) GetPopularity(arquivo string) int {
	var count int
	s.DB.QueryRow("SELECT count FROM popularity WHERE arquivo = ?", arquivo).Scan(&count)
	return count
}

// IncrementPopularity increases the access count for a file by 1.
// Agora delegamos para o RLHF
func (s *Store) IncrementPopularity(arquivo string) error {
	return s.ApplyInteractionReward(arquivo, "focus_zoom")
}

// GetAllPopularity returns all popularity records as a map of file -> count.
func (s *Store) GetAllPopularity() (map[string]int, error) {
	rows, err := s.DB.Query("SELECT arquivo, count FROM popularity")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var arquivo string
		var count int
		if err := rows.Scan(&arquivo, &count); err != nil {
			return nil, err
		}
		result[arquivo] = count
	}
	return result, rows.Err()
}

// ResetPopularity deletes the popularity record for a file.
func (s *Store) ResetPopularity(arquivo string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	_, err := s.DB.Exec("DELETE FROM popularity WHERE arquivo = ?", arquivo)
	return err
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

	// Insere se não existir (inicia com 1.0 + recompensa) ou atualiza acumulando o peso
	// Mantemos o campo 'count' incrementando para retrocompatibilidade
	_, err := s.DB.Exec(`
		INSERT INTO popularity (arquivo, count, weight, last_interacted_at) 
		VALUES (?, 1, 1.0 + ?, ?)
		ON CONFLICT(arquivo) DO UPDATE SET 
			count = count + 1,
			weight = MAX(0.1, weight + ?), 
			last_interacted_at = ?`,
		arquivo, reward, time.Now().Format(time.RFC3339),
		reward, time.Now().Format(time.RFC3339),
	)
	return err
}

// GetSynapticWeight calcula o peso sináptico atual aplicando o Forgetting Curve (decaimento logarítmico)
func (s *Store) GetSynapticWeight(arquivo string) float64 {
	var weight float64
	var lastInteractedStr string

	// Usamos COALESCE e valores default caso as colunas weight existam mas estejam NULL ou vazias
	err := s.DB.QueryRow("SELECT COALESCE(weight, 1.0), COALESCE(last_interacted_at, '') FROM popularity WHERE arquivo = ?", arquivo).
		Scan(&weight, &lastInteractedStr)
	
	if err != nil {
		return 1.0 // Peso neutro padrão
	}

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
