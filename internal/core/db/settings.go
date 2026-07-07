package db

import (
	"database/sql"
)

// GetSetting retorna o valor de uma configuração.
func (s *Store) GetSetting(key string) (string, error) {
	var val string
	err := s.DB.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

// SetSetting atualiza ou insere uma configuração.
func (s *Store) SetSetting(key, value string) error {
	return s.RunInTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?", key, value, value)
		return err
	})
}

// HasNotificationBeenSent verifica se uma notificação com o ID já foi enviada.
func (s *Store) HasNotificationBeenSent(id string) (bool, error) {
	var count int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM notifications_log WHERE id = ?", id).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RecordNotificationSent registra que uma notificação foi enviada.
func (s *Store) RecordNotificationSent(id, notificationType, sentAt string) error {
	return s.RunInTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO notifications_log (id, type, sent_at) VALUES (?, ?, ?)", id, notificationType, sentAt)
		return err
	})
}
