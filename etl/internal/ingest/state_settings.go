package ingest

import (
	"encoding/json"

	"etl/internal/models"

	bolt "go.etcd.io/bbolt"
)

// Settings

func (s *AppState) GetSettings() models.AppSettings {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	return s.settings
}

func (s *AppState) SetSettings(settings models.AppSettings) {
	s.settingsMu.Lock()
	s.settings = settings
	s.settingsMu.Unlock()

	s.db.Update(func(tx *bolt.Tx) error {
		val, _ := json.Marshal(settings)
		return tx.Bucket(bucketSettings).Put([]byte("current"), val)
	})
}
