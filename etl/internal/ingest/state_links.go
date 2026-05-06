package ingest

import (
	"encoding/json"
	"path/filepath"
	"sync"

	bolt "go.etcd.io/bbolt"
)

// LinkManager gerencia WikiLinks e contagens de backlinks.
type LinkManager struct {
	linkCounts   map[string]int
	linkCountsMu sync.RWMutex
	fileLinks    map[string][]string
	fileLinksMu  sync.RWMutex
	db           *bolt.DB
}

func newLinkManager(db *bolt.DB) *LinkManager {
	return &LinkManager{
		linkCounts: make(map[string]int),
		fileLinks:  make(map[string][]string),
		db:         db,
	}
}

func (lm *LinkManager) UpdateLinkCounts(newCounts map[string]int) {
	lm.linkCountsMu.Lock()
	lm.linkCounts = newCounts
	lm.linkCountsMu.Unlock()

	lm.db.Update(func(tx *bolt.Tx) error {
		tx.DeleteBucket(bucketLinkCounts)
		nb, _ := tx.CreateBucket(bucketLinkCounts)
		for k, v := range newCounts {
			val, _ := json.Marshal(v)
			nb.Put([]byte(k), val)
		}
		return nil
	})
}

func (lm *LinkManager) GetLinkCount(filename string) int {
	lm.linkCountsMu.RLock()
	defer lm.linkCountsMu.RUnlock()

	if count, ok := lm.linkCounts[filename]; ok {
		return count
	}
	base := filepath.Base(filename)
	if count, ok := lm.linkCounts[base]; ok {
		return count
	}
	return 0
}

func (lm *LinkManager) SetFileLinks(filename string, links []string) {
	lm.fileLinksMu.Lock()
	lm.fileLinks[filename] = links
	lm.fileLinksMu.Unlock()

	lm.db.Update(func(tx *bolt.Tx) error {
		val, _ := json.Marshal(links)
		return tx.Bucket(bucketFileLinks).Put([]byte(filename), val)
	})
}

func (lm *LinkManager) GetFileLinks(filename string) []string {
	lm.fileLinksMu.RLock()
	defer lm.fileLinksMu.RUnlock()
	return lm.fileLinks[filename]
}

func (lm *LinkManager) GetAllFileLinks() map[string][]string {
	lm.fileLinksMu.RLock()
	defer lm.fileLinksMu.RUnlock()
	cp := make(map[string][]string)
	for k, v := range lm.fileLinks {
		cp[k] = v
	}
	return cp
}

func (lm *LinkManager) DeleteFileLinks(filename string) {
	lm.fileLinksMu.Lock()
	delete(lm.fileLinks, filename)
	lm.fileLinksMu.Unlock()

	lm.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketFileLinks).Delete([]byte(filename))
	})
}

// setLinkCounts popula o cache (uso interno pelo AppState.Load).
func (lm *LinkManager) setLinkCounts(m map[string]int) {
	lm.linkCountsMu.Lock()
	defer lm.linkCountsMu.Unlock()
	lm.linkCounts = m
}

// setFileLinksMap popula o cache (uso interno pelo AppState.Load).
func (lm *LinkManager) setFileLinksMap(m map[string][]string) {
	lm.fileLinksMu.Lock()
	defer lm.fileLinksMu.Unlock()
	lm.fileLinks = m
}
