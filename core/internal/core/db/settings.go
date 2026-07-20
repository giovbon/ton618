package db

import (
	"database/sql"
	"ton618/core/internal/core/db/generated"
)

// GetSetting retorna o valor de uma configuração.
func (s *Store) GetSetting(key string) (string, error) {
	val, err := s.Q.GetSetting(s.queryCtx(), key)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return val.String, nil
}

// SetSetting atualiza ou insere uma configuração.
func (s *Store) SetSetting(key, value string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.SetSetting(s.queryCtx(), dbgen.SetSettingParams{
		Key:   key,
		Value: sql.NullString{String: value, Valid: true},
	})
}

// HasNotificationBeenSent verifica se uma notificação com o ID já foi enviada.
func (s *Store) HasNotificationBeenSent(id string) (bool, error) {
	count, err := s.Q.HasNotificationBeenSent(s.queryCtx(), id)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RecordNotificationSent registra que uma notificação foi enviada.
func (s *Store) RecordNotificationSent(id, notificationType, sentAt string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.RecordNotificationSent(s.queryCtx(), dbgen.RecordNotificationSentParams{
		ID:     id,
		Type:   sql.NullString{String: notificationType, Valid: true},
		SentAt: sql.NullString{String: sentAt, Valid: true},
	})
}
