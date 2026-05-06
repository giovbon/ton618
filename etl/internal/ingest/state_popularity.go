package ingest

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"
)

// Popularity

func (s *AppState) IncrementPopularity(filename string) {
	s.popularityMu.Lock()
	s.popularity[filename]++
	current := s.popularity[filename]
	s.popularityMu.Unlock()

	s.db.Update(func(tx *bolt.Tx) error {
		val, _ := json.Marshal(current)
		return tx.Bucket(bucketPopularity).Put([]byte(filename), val)
	})
}

func (s *AppState) GetPopularity(filename string) int {
	s.popularityMu.RLock()
	defer s.popularityMu.RUnlock()
	return s.popularity[filename]
}

func (s *AppState) GetAllPopularity() map[string]int {
	s.popularityMu.RLock()
	defer s.popularityMu.RUnlock()
	cp := make(map[string]int, len(s.popularity))
	for k, v := range s.popularity {
		cp[k] = v
	}
	return cp
}

// DeletePopularity remove a entrada de popularidade de um arquivo.
func (s *AppState) DeletePopularity(filename string) {
	s.popularityMu.Lock()
	delete(s.popularity, filename)
	s.popularityMu.Unlock()

	s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketPopularity).Delete([]byte(filename))
	})
}
