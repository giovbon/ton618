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
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	_, err := s.DB.Exec(
		"INSERT OR REPLACE INTO file_mods (arquivo, mtime) VALUES (?, ?)", arquivo, mtime,
	)
	return err
}

// DeleteFileMod removes the modification-time record for a file.
func (s *Store) DeleteFileMod(arquivo string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
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

// FileModTag represents a file's modification time and its associated tags (comma separated).
type FileModTag struct {
	Arquivo string
	Mtime   string
	Tags    string
}

// GetFilesModsAndTags returns all files with their mtime and concatenated tags in a single query.
func (s *Store) GetFilesModsAndTags() ([]FileModTag, error) {
	query := `
		SELECT f.arquivo, f.mtime, IFNULL(GROUP_CONCAT(t.tag, ','), '') as tags
		FROM file_mods f
		LEFT JOIN tags t ON f.arquivo = t.arquivo
		GROUP BY f.arquivo, f.mtime
	`
	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []FileModTag
	for rows.Next() {
		var item FileModTag
		if err := rows.Scan(&item.Arquivo, &item.Mtime, &item.Tags); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
