package db

// ---------------------------------------------------------------------------
// links
// ---------------------------------------------------------------------------

// AddLink creates a directed link from one file to another.
func (s *Store) AddLink(fromFile, toFile string) error {
	_, err := s.DB.Exec(
		"INSERT OR IGNORE INTO links (from_file, to_file) VALUES (?, ?)", fromFile, toFile,
	)
	return err
}

// RemoveLink deletes a directed link between two files.
func (s *Store) RemoveLink(fromFile, toFile string) error {
	_, err := s.DB.Exec(
		"DELETE FROM links WHERE from_file = ? AND to_file = ?", fromFile, toFile,
	)
	return err
}

// GetLinks returns all outbound links from a file.
func (s *Store) GetLinks(fromFile string) ([]string, error) {
	rows, err := s.DB.Query(
		"SELECT to_file FROM links WHERE from_file = ? ORDER BY to_file", fromFile,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []string
	for rows.Next() {
		var to string
		if err := rows.Scan(&to); err != nil {
			return nil, err
		}
		links = append(links, to)
	}
	return links, rows.Err()
}

// GetLinkCount returns the number of outbound links from a file.
func (s *Store) GetLinkCount(fromFile string) int {
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM links WHERE from_file = ?", fromFile).Scan(&count)
	return count
}

// GetBacklinks returns all files that link to the given file.
func (s *Store) GetBacklinks(toFile string) ([]string, error) {
	rows, err := s.DB.Query(
		"SELECT from_file FROM links WHERE to_file = ? ORDER BY from_file", toFile,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []string
	for rows.Next() {
		var from string
		if err := rows.Scan(&from); err != nil {
			return nil, err
		}
		links = append(links, from)
	}
	return links, rows.Err()
}

// GetBacklinkCount returns the number of files that link to the given file.
func (s *Store) GetBacklinkCount(toFile string) int {
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM links WHERE to_file = ?", toFile).Scan(&count)
	return count
}

// GetAllLinks returns all links as a map of from_file -> []to_file.
func (s *Store) GetAllLinks() (map[string][]string, error) {
	rows, err := s.DB.Query("SELECT from_file, to_file FROM links ORDER BY from_file, to_file")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var from, to string
		if err := rows.Scan(&from, &to); err != nil {
			return nil, err
		}
		result[from] = append(result[from], to)
	}
	return result, rows.Err()
}

// ClearLinks removes all links originating from a file.
func (s *Store) ClearLinks(fromFile string) error {
	_, err := s.DB.Exec("DELETE FROM links WHERE from_file = ?", fromFile)
	return err
}
