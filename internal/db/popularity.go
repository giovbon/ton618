package db

// ---------------------------------------------------------------------------
// popularity
// ---------------------------------------------------------------------------

// GetPopularity returns the access count for a file.
func (s *Store) GetPopularity(arquivo string) int {
	var count int
	s.DB.QueryRow("SELECT count FROM popularity WHERE arquivo = ?", arquivo).Scan(&count)
	return count
}

// IncrementPopularity increases the access count for a file by 1.
func (s *Store) IncrementPopularity(arquivo string) error {
	_, err := s.DB.Exec(`
		INSERT INTO popularity (arquivo, count) VALUES (?, 1)
		ON CONFLICT(arquivo) DO UPDATE SET count = count + 1`, arquivo)
	return err
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
	_, err := s.DB.Exec("DELETE FROM popularity WHERE arquivo = ?", arquivo)
	return err
}
