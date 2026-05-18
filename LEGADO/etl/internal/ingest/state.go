package ingest

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"etl/internal/config"
	"etl/internal/models"
	"etl/internal/semantic"

	bolt "go.etcd.io/bbolt"
)

type AppState struct {
	fileModCache   map[string]time.Time
	fileModCacheMu sync.RWMutex
	hashCache      map[string]string
	hashCacheMu    sync.RWMutex
	popularity     map[string]int
	popularityMu   sync.RWMutex
	settings       models.AppSettings
	settingsMu     sync.RWMutex
	fileMetadata   map[string]map[string]interface{}
	fileMetadataMu sync.RWMutex
	db             *bolt.DB

	tags     *TagManager
	links    *LinkManager
	vectors  *VectorManager
	semantic *SemanticManager

	// Cache da funcao de embedding para preservar HTTP client e connection pooling
	embCacheFunc     func(context.Context, string) ([]float32, error)
	embCacheMu       sync.Mutex
	embCacheKey      string
	embCacheProvider semantic.EmbeddingProvider
}

func openDB(dbPath string) (db *bolt.DB, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic ao abrir banco: %v", r)
		}
	}()
	db, err = bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	return
}

func NewAppState(cfg *config.AppConfig) *AppState {
	os.MkdirAll(cfg.StateDir, 0755)
	dbPath := filepath.Join(cfg.StateDir, "state.db")

	db, err := openDB(dbPath)
	if err != nil {
		// Se for apenas timeout, falha sem apagar (provavelmente já aberto)
		if strings.Contains(err.Error(), "timeout") {
			fmt.Fprintf(os.Stderr, "[DB] Erro: Banco de dados ja esta em uso por outro processo: %v\n", err)
		os.Exit(1)
		}

		log.Printf("[DB] Erro ao abrir banco de dados: %v. Tentando recriar...", err)
		os.Remove(dbPath)
		db, err = openDB(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[DB] Erro fatal ao recriar banco de dados de estado: %v\n", err)
		os.Exit(1)
		}
		log.Println("[DB] Banco de dados recriado do zero com sucesso.")
	}

	s := &AppState{
		db:           db,
		fileModCache: make(map[string]time.Time),
		hashCache:    make(map[string]string),
		popularity:   make(map[string]int),
		fileMetadata: make(map[string]map[string]interface{}),
		settings: models.AppSettings{SemanticEnable: true, SemanticStrategy: "whitelist", Language: "pt-BR", EmbeddingDimension: 512},
		tags:         newTagManager(db),
		links:        newLinkManager(db),
		vectors:      newVectorManager(db),
		semantic:     newSemanticManager(db),
	}

	db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{
			bucketFileMods, bucketHashes, bucketPopularity, bucketKnownTags,
			bucketFileTags, bucketLinkCounts, bucketFileLinks, bucketSettings,
			bucketFileMetadata, bucketVectorHashes, bucketNoteVectors,
			bucketNoteProjections, bucketSemanticTopics, bucketFileSemanticLinks,
		}
		for _, b := range buckets {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})

	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNoteVectors)
		if b == nil {
			return nil
		}
		var staleKeys [][]byte
		b.ForEach(func(k, _ []byte) error {
			if !strings.Contains(string(k), "/") {
				staleKeys = append(staleKeys, append([]byte{}, k...))
			}
			return nil
		})
		for _, k := range staleKeys {
			b.Delete(k)
		}
		if len(staleKeys) > 0 {
			log.Printf("[DB] Migração note_vectors: removidas %d chaves obsoletas\n", len(staleKeys))
		}
		return nil
	})

	return s
}

func (s *AppState) IsAlive() bool { return s.db != nil }

func (s *AppState) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *AppState) Save(cfg *config.AppConfig) {}

func (s *AppState) RebuildKnownTagsCache() {
	newKnownTags := make(map[string]bool)
	for _, tags := range s.tags.getFileTagsMap() {
		for _, tag := range tags {
			if tag != "" {
				newKnownTags[tag] = true
			}
		}
	}
	s.tags.setKnownTagsMap(newKnownTags)
	log.Printf("[Sync] Cache de sugestoes de tags reconstruido: %d tags ativas.\n", len(newKnownTags))
}

func (s *AppState) GetAllFileTags() map[string][]string {
	m := s.tags.getFileTagsMap()
	res := make(map[string][]string, len(m))
	for k, v := range m {
		cp := make([]string, len(v))
		copy(cp, v)
		res[k] = cp
	}
	return res
}
