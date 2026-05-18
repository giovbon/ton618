package ingest

import (
	"encoding/json"
	"log"
	"sort"
	"sync"

	bolt "go.etcd.io/bbolt"
)

type SemanticManager struct {
	topics            map[string]bool
	topicsMu          sync.RWMutex
	fileSemanticLinks map[string][]string
	fileLinksMu       sync.RWMutex
	db                *bolt.DB
}

func newSemanticManager(db *bolt.DB) *SemanticManager {
	return &SemanticManager{
		topics:            make(map[string]bool),
		fileSemanticLinks: make(map[string][]string),
		db:                db,
	}
}

func (sm *SemanticManager) SetFileSemanticLinks(filename string, links []string) {
	sm.fileLinksMu.Lock()
	sm.fileSemanticLinks[filename] = links
	sm.fileLinksMu.Unlock()

	// Rebuild completo para remover topicos que nao sao mais referenciados.
	// Isso garante que ao sobrescrever links de um arquivo, topicos antigos
	// que sumiram nao permanecam no cache.
	sm.RebuildSemanticTopics()
}

func (sm *SemanticManager) GetFileSemanticLinks(filename string) []string {
	sm.fileLinksMu.RLock()
	defer sm.fileLinksMu.RUnlock()
	return sm.fileSemanticLinks[filename]
}

func (sm *SemanticManager) GetAllFileSemanticLinks() map[string][]string {
	sm.fileLinksMu.RLock()
	defer sm.fileLinksMu.RUnlock()
	cp := make(map[string][]string, len(sm.fileSemanticLinks))
	for k, v := range sm.fileSemanticLinks {
		cp[k] = v
	}
	return cp
}

func (sm *SemanticManager) GetAllSemanticTopics() []string {
	sm.topicsMu.RLock()
	defer sm.topicsMu.RUnlock()
	topics := make([]string, 0, len(sm.topics))
	for t := range sm.topics {
		topics = append(topics, t)
	}
	sort.Strings(topics)
	return topics
}

// DeleteFileSemanticLinks remove os links de um arquivo e limpa topicos orfaos.
// Usa RebuildSemanticTopics (O(N+M)) em vez de scan O(N*M) por topico.
func (sm *SemanticManager) DeleteFileSemanticLinks(filename string) {
	sm.fileLinksMu.Lock()
	delete(sm.fileSemanticLinks, filename)
	sm.fileLinksMu.Unlock()

	sm.RebuildSemanticTopics()
}

// setTopics popula o cache (uso interno pelo AppState.Load).
func (sm *SemanticManager) setTopics(m map[string]bool) {
	sm.topicsMu.Lock()
	defer sm.topicsMu.Unlock()
	sm.topics = m
}

// setFileSemanticLinksMap popula o cache (uso interno pelo AppState.Load).
func (sm *SemanticManager) setFileSemanticLinksMap(m map[string][]string) {
	sm.fileLinksMu.Lock()
	defer sm.fileLinksMu.Unlock()
	sm.fileSemanticLinks = m
}

// RebuildSemanticTopics recria a lista de topicos baseado estritamente nos links ativos.
// O(N+M) — chamado no startup via Load() e apos cada DeleteFileSemanticLinks.
func (sm *SemanticManager) RebuildSemanticTopics() {
	validTopics := make(map[string]bool)

	sm.fileLinksMu.RLock()
	cp := make(map[string][]string, len(sm.fileSemanticLinks))
	for k, v := range sm.fileSemanticLinks {
		cp[k] = v
		for _, link := range v {
			validTopics[link] = true
		}
	}
	sm.fileLinksMu.RUnlock()

	sm.topicsMu.Lock()
	sm.topics = validTopics
	sm.topicsMu.Unlock()

	sm.db.Update(func(tx *bolt.Tx) error {
		// Persiste links de arquivo
		tx.DeleteBucket(bucketFileSemanticLinks)
		bFL, _ := tx.CreateBucketIfNotExists(bucketFileSemanticLinks)
		for filename, links := range cp {
			val, _ := json.Marshal(links)
			bFL.Put([]byte(filename), val)
		}

		// Persiste topicos
		tx.DeleteBucket(bucketSemanticTopics)
		bT, _ := tx.CreateBucketIfNotExists(bucketSemanticTopics)
		for topic := range validTopics {
			bT.Put([]byte(topic), []byte("true"))
		}
		return nil
	})
	log.Printf("[SemanticManager] Topicos reconstruidos: %d ativos\n", len(validTopics))
}
