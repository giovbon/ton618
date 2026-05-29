package db

// ---------------------------------------------------------------------------
// tags
// ---------------------------------------------------------------------------

// SetFileTags replaces the entire set of tags for a file atomically.
func (s *Store) SetFileTags(arquivo string, tags []string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM tags WHERE arquivo = ?", arquivo); err != nil {
		return err
	}

	stmt, err := tx.Prepare("INSERT INTO tags (arquivo, tag) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, tag := range tags {
		if _, err := stmt.Exec(arquivo, tag); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetFileTags returns all tags associated with a file.
func (s *Store) GetFileTags(arquivo string) ([]string, error) {
	rows, err := s.DB.Query("SELECT tag FROM tags WHERE arquivo = ? ORDER BY tag", arquivo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// GetAllTags returns every distinct tag present in the database.
func (s *Store) GetAllTags() ([]string, error) {
	rows, err := s.DB.Query("SELECT DISTINCT tag FROM tags ORDER BY tag")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// GetFilesByTag returns all file paths that have a specific tag.
func (s *Store) GetFilesByTag(tag string) ([]string, error) {
	rows, err := s.DB.Query("SELECT arquivo FROM tags WHERE tag = ? ORDER BY arquivo", tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// AddTagToFile adds a single tag to a file (no-op if already present).
func (s *Store) AddTagToFile(arquivo, tag string) error {
	_, err := s.DB.Exec(
		"INSERT OR IGNORE INTO tags (arquivo, tag) VALUES (?, ?)", arquivo, tag,
	)
	return err
}

// RemoveTagFromFile removes a single tag from a file.
func (s *Store) RemoveTagFromFile(arquivo, tag string) error {
	_, err := s.DB.Exec("DELETE FROM tags WHERE arquivo = ? AND tag = ?", arquivo, tag)
	return err
}
