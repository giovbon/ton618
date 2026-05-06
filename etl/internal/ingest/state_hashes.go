package ingest

import (
	bolt "go.etcd.io/bbolt"
)

// HashCache

func (s *AppState) GetHash(id string) (string, bool) {
	s.hashCacheMu.RLock()
	defer s.hashCacheMu.RUnlock()
	hash, exists := s.hashCache[id]
	return hash, exists
}

func (s *AppState) SetHash(id, hash string) {
	s.hashCacheMu.Lock()
	s.hashCache[id] = hash
	s.hashCacheMu.Unlock()

	s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketHashes).Put([]byte(id), []byte(hash))
	})
}

func (s *AppState) DeleteHashesByIDs(ids []string) {
	s.hashCacheMu.Lock()
	for _, id := range ids {
		delete(s.hashCache, id)
	}
	s.hashCacheMu.Unlock()

	s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHashes)
		for _, id := range ids {
			b.Delete([]byte(id))
		}
		return nil
	})
}
