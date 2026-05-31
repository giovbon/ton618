package db

// ---------------------------------------------------------------------------
// file_mods
// ---------------------------------------------------------------------------

// GetFileMod returns the stored modification time for a file, or empty string if not found.
func (s *Store) GetFileMod(arquivo string) (string, error) {
	var mtime string
	err := s.DB.QueryRow("SELECT mtime FROM file_mods WHERE arquivo = ?", arquivo).Scan(&mtime)
	if err != nil {
		// sql.ErrNoRows is not an error worth surfacing — just return empty.
		return "", nil
	}
	return mtime, nil
}

// SetFileMod inserts or updates the modification time for a file.
func (s *Store) SetFileMod(arquivo, mtime string) error {
	_, err := s.DB.Exec(
		"INSERT OR REPLACE INTO file_mods (arquivo, mtime) VALUES (?, ?)", arquivo, mtime,
	)
	return err
}

// DeleteFileMod removes the modification-time record for a file.
func (s *Store) DeleteFileMod(arquivo string) error {
	_, err := s.DB.Exec("DELETE FROM file_mods WHERE arquivo = ?", arquivo)
	return err
}

// GetAllFileMods returns all stored file modification times as a map.
func (s *Store) GetAllFileMods() (map[string]string, error) {
	rows, err := s.DB.Query("SELECT arquivo, mtime FROM file_mods")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var arquivo, mtime string
		if err := rows.Scan(&arquivo, &mtime); err != nil {
			return nil, err
		}
		result[arquivo] = mtime
	}
	return result, rows.Err()
}
