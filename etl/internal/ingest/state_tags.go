package ingest

import (
	"encoding/json"
	"sync"

	bolt "go.etcd.io/bbolt"
)

// TagManager gerencia tags de arquivos e o cache de tags conhecidas.
type TagManager struct {
	knownTags   map[string]bool
	knownTagsMu sync.RWMutex
	fileTags    map[string][]string
	fileTagsMu  sync.RWMutex
	db          *bolt.DB
}

func newTagManager(db *bolt.DB) *TagManager {
	return &TagManager{
		knownTags: make(map[string]bool),
		fileTags:  make(map[string][]string),
		db:        db,
	}
}

// --- Known Tags ---

func (tm *TagManager) AddKnownTag(tag string) {
	tm.knownTagsMu.Lock()
	defer tm.knownTagsMu.Unlock()
	tm.knownTags[tag] = true
}

func (tm *TagManager) GetAllKnownTags() []string {
	tm.knownTagsMu.RLock()
	defer tm.knownTagsMu.RUnlock()
	var tags []string
	for k := range tm.knownTags {
		tags = append(tags, k)
	}
	return tags
}

func (tm *TagManager) GetKnownTagsCount() int {
	tm.knownTagsMu.RLock()
	defer tm.knownTagsMu.RUnlock()
	return len(tm.knownTags)
}

// --- File Tags ---

func (tm *TagManager) SetFileTags(filename string, tags []string) {
	tm.fileTagsMu.Lock()
	tm.fileTags[filename] = tags
	tm.fileTagsMu.Unlock()

	tm.db.Update(func(tx *bolt.Tx) error {
		val, _ := json.Marshal(tags)
		return tx.Bucket(bucketFileTags).Put([]byte(filename), val)
	})
}

func (tm *TagManager) GetFileTags(filename string) []string {
	tm.fileTagsMu.RLock()
	defer tm.fileTagsMu.RUnlock()
	tags, exists := tm.fileTags[filename]
	if !exists {
		return nil
	}
	cp := make([]string, len(tags))
	copy(cp, tags)
	return cp
}

func (tm *TagManager) HasTags(filename string) bool {
	tm.fileTagsMu.RLock()
	defer tm.fileTagsMu.RUnlock()
	_, exists := tm.fileTags[filename]
	return exists
}

func (tm *TagManager) DeleteFileTags(filename string) {
	tm.fileTagsMu.Lock()
	delete(tm.fileTags, filename)
	tm.fileTagsMu.Unlock()

	tm.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketFileTags).Delete([]byte(filename))
	})
}

// getFileTagsMap retorna referência interna para RebuildKnownTagsCache (uso interno pelo AppState.Load).
func (tm *TagManager) getFileTagsMap() map[string][]string {
	tm.fileTagsMu.RLock()
	defer tm.fileTagsMu.RUnlock()
	return tm.fileTags
}

// setKnownTagsMap substitui o mapa de known tags (uso interno pelo AppState.Load e RebuildKnownTagsCache).
func (tm *TagManager) setKnownTagsMap(m map[string]bool) {
	tm.knownTagsMu.Lock()
	defer tm.knownTagsMu.Unlock()
	tm.knownTags = m
}

// setFileTags popula o cache de fileTags (uso interno pelo AppState.Load).
func (tm *TagManager) setFileTags(m map[string][]string) {
	tm.fileTagsMu.Lock()
	defer tm.fileTagsMu.Unlock()
	tm.fileTags = m
}
