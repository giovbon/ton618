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

// GetFileModsPaginated returns a paginated slice of file modification times,
// ordered by mtime descending, along with the total count.
func (s *Store) GetFileModsPaginated(from, size int) (map[string]string, int, error) {
	var total int
	s.DB.QueryRow("SELECT COUNT(*) FROM file_mods").Scan(&total)

	rows, err := s.DB.Query("SELECT arquivo, mtime FROM file_mods ORDER BY mtime DESC LIMIT ? OFFSET ?", size, from)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var arquivo, mtime string
		if err := rows.Scan(&arquivo, &mtime); err != nil {
			continue
		}
		result[arquivo] = mtime
	}
	return result, total, rows.Err()
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

// ---------------------------------------------------------------------------
// settings
// ---------------------------------------------------------------------------

// GetSetting returns the value for a given settings key, or empty string if not set.
func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.DB.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return "", nil
	}
	return value, nil
}

// SetSetting creates or updates a key-value pair in the settings table.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.DB.Exec(
		"INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value,
	)
	return err
}

// DeleteSetting removes a key from the settings table.
func (s *Store) DeleteSetting(key string) error {
	_, err := s.DB.Exec("DELETE FROM settings WHERE key = ?", key)
	return err
}

// GetAllSettings returns all settings as a key-value map.
func (s *Store) GetAllSettings() (map[string]string, error) {
	rows, err := s.DB.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, rows.Err()
}

// ---------------------------------------------------------------------------
// semantic
// ---------------------------------------------------------------------------

// AddSemanticLink creates a semantic relationship between two files.
func (s *Store) AddSemanticLink(fromFile, toFile string) error {
	_, err := s.DB.Exec(
		"INSERT OR IGNORE INTO semantic_links (from_file, to_file) VALUES (?, ?)", fromFile, toFile,
	)
	return err
}

// GetSemanticLinks returns all semantically related files for a given file.
func (s *Store) GetSemanticLinks(fromFile string) ([]string, error) {
	rows, err := s.DB.Query(
		"SELECT to_file FROM semantic_links WHERE from_file = ? ORDER BY to_file", fromFile,
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

// ClearSemanticLinks removes all semantic links originating from a file.
func (s *Store) ClearSemanticLinks(fromFile string) error {
	_, err := s.DB.Exec("DELETE FROM semantic_links WHERE from_file = ?", fromFile)
	return err
}

// ---------------------------------------------------------------------------
// semantic topics
// ---------------------------------------------------------------------------

// AddSemanticTopic inserts a topic (no-op if it already exists).
func (s *Store) AddSemanticTopic(topic string) error {
	_, err := s.DB.Exec("INSERT OR IGNORE INTO semantic_topics (topic) VALUES (?)", topic)
	return err
}

// GetAllSemanticTopics returns all topics.
func (s *Store) GetAllSemanticTopics() ([]string, error) {
	rows, err := s.DB.Query("SELECT topic FROM semantic_topics ORDER BY topic")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		topics = append(topics, t)
	}
	return topics, rows.Err()
}

// DeleteSemanticTopic removes a topic.
func (s *Store) DeleteSemanticTopic(topic string) error {
	_, err := s.DB.Exec("DELETE FROM semantic_topics WHERE topic = ?", topic)
	return err
}
