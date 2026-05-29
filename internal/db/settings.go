package db

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
