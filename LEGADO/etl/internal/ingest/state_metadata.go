package ingest

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"
)

// File Metadata

func (s *AppState) SetFileMetadata(filename string, meta map[string]interface{}) {
	s.fileMetadataMu.Lock()
	s.fileMetadata[filename] = meta
	s.fileMetadataMu.Unlock()

	s.db.Update(func(tx *bolt.Tx) error {
		val, _ := json.Marshal(meta)
		return tx.Bucket(bucketFileMetadata).Put([]byte(filename), val)
	})
}

func (s *AppState) GetFileMetadata(filename string) map[string]interface{} {
	s.fileMetadataMu.RLock()
	defer s.fileMetadataMu.RUnlock()
	return s.fileMetadata[filename]
}

func (s *AppState) DeleteFileMetadata(filename string) {
	s.fileMetadataMu.Lock()
	delete(s.fileMetadata, filename)
	s.fileMetadataMu.Unlock()

	s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketFileMetadata).Delete([]byte(filename))
	})
}

func (s *AppState) GetAllFileMetadata() map[string]map[string]interface{} {
	s.fileMetadataMu.RLock()
	defer s.fileMetadataMu.RUnlock()
	cp := make(map[string]map[string]interface{})
	for k, v := range s.fileMetadata {
		cp[k] = v
	}
	return cp
}
