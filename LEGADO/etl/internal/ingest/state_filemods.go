package ingest

import (
	"time"

	bolt "go.etcd.io/bbolt"
)

// FileModCache

func (s *AppState) GetFileMod(path string) (time.Time, bool) {
	s.fileModCacheMu.RLock()
	defer s.fileModCacheMu.RUnlock()
	mod, exists := s.fileModCache[path]
	return mod, exists
}

func (s *AppState) SetFileMod(path string, mod time.Time) {
	s.fileModCacheMu.Lock()
	s.fileModCache[path] = mod
	s.fileModCacheMu.Unlock()

	s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketFileMods)
		bin, _ := mod.MarshalBinary()
		return b.Put([]byte(path), bin)
	})
}

func (s *AppState) GetFileCreation(path string) (time.Time, bool) {
	s.fileModCacheMu.RLock()
	defer s.fileModCacheMu.RUnlock()
	mod, exists := s.fileModCache[path]
	return mod, exists
}

func (s *AppState) GetAllFileMods() map[string]time.Time {
	s.fileModCacheMu.RLock()
	defer s.fileModCacheMu.RUnlock()
	cp := make(map[string]time.Time)
	for k, v := range s.fileModCache {
		cp[k] = v
	}
	return cp
}

func (s *AppState) DeleteFileMod(path string) {
	s.fileModCacheMu.Lock()
	delete(s.fileModCache, path)
	s.fileModCacheMu.Unlock()

	s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketFileMods).Delete([]byte(path))
	})
}
