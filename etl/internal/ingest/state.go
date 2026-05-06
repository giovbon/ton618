package ingest

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"etl/internal/config"
	"etl/internal/models"

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

	tags    *TagManager
	links   *LinkManager
	vectors *VectorManager
}

// openDB abre (ou cria) o banco BBolt no caminho especificado.
// Captura panics internos do bbolt (ex.: freelist corrompida) e os
// converte em erro, permitindo recuperação sem crash.
func openDB(dbPath string) (db *bolt.DB, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic ao abrir banco: %v", r)
		}
	}()
	db, err = bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	return
}

// NewAppState inicializa o estado do aplicativo, abrindo/criando o banco BBolt
// e executando migrações necessárias.
// Se o arquivo state.db estiver corrompido, tenta remover o arquivo corrompido
// e criar um banco novo do zero.
func NewAppState(cfg *config.AppConfig) *AppState {
	os.MkdirAll(cfg.StateDir, 0755)
	dbPath := filepath.Join(cfg.StateDir, "state.db")

	db, err := openDB(dbPath)
	if err != nil {
		log.Printf("[DB] Erro ao abrir banco de dados: %v. Tentando recriar...", err)
		os.Remove(dbPath)
		db, err = openDB(dbPath)
		if err != nil {
			log.Fatalf("[DB] Erro fatal ao recriar banco de dados de estado: %v", err)
		}
		log.Println("[DB] Banco de dados recriado do zero com sucesso.")
	}

	s := &AppState{
		db:           db,
		fileModCache: make(map[string]time.Time),
		hashCache:    make(map[string]string),
		popularity:   make(map[string]int),
		fileMetadata: make(map[string]map[string]interface{}),
		settings:     models.AppSettings{SemanticStrategy: "whitelist", Language: "pt-BR"},
		tags:         newTagManager(db),
		links:        newLinkManager(db),
		vectors:      newVectorManager(db),
	}

	// Inicializa Buckets
	db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{
			bucketFileMods, bucketHashes, bucketPopularity, bucketKnownTags,
			bucketFileTags, bucketLinkCounts, bucketFileLinks, bucketSettings,
			bucketFileMetadata, bucketVectorHashes, bucketNoteVectors,
			bucketNoteProjections,
		}
		for _, b := range buckets {
			_, err := tx.CreateBucketIfNotExists(b)
			if err != nil {
				return err
			}
		}
		return nil
	})

	// Migração: limpar entradas antigas de note_vectors que usavam hash (doc.ID) como chave.
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
			log.Printf("[DB] Migração note_vectors: removidas %d chaves obsoletas (hash → arquivo)\n", len(staleKeys))
		}
		return nil
	})

	return s
}

func (s *AppState) IsAlive() bool {
	return s.db != nil
}

func (s *AppState) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

// Save existe apenas para compatibilidade de assinatura.
// O estado é persistido automaticamente em cada alteração via BBolt.
func (s *AppState) Save(cfg *config.AppConfig) {}

// Load carrega todos os dados do BBolt para os caches em RAM.
func (s *AppState) Load(cfg *config.AppConfig) {
	// 1. Migração Legada: Se existe state.json, migra para o DB
	if _, err := os.Stat(cfg.StateFile); err == nil {
		log.Println("[DB] Detectado state.json legado. Iniciando migração...")
		data, err := os.ReadFile(cfg.StateFile)
		if err == nil {
			var legacy models.SystemState
			if err := json.Unmarshal(data, &legacy); err == nil {
				s.performMigration(legacy)
				os.Rename(cfg.StateFile, cfg.StateFile+".bak")
				log.Println("[DB] Migração concluída. state.json renomeado para .bak")
			}
		}
	}

	// 2. Carregar dados do BBolt para o Cache em RAM
	s.db.View(func(tx *bolt.Tx) error {
		// File Mods
		bMods := tx.Bucket(bucketFileMods)
		bMods.ForEach(func(k, v []byte) error {
			var t time.Time
			t.UnmarshalBinary(v)
			s.fileModCache[string(k)] = t
			return nil
		})

		// Hashes
		bHashes := tx.Bucket(bucketHashes)
		bHashes.ForEach(func(k, v []byte) error {
			s.hashCache[string(k)] = string(v)
			return nil
		})

		// Popularity
		bPop := tx.Bucket(bucketPopularity)
		bPop.ForEach(func(k, v []byte) error {
			var p int
			json.Unmarshal(v, &p)
			s.popularity[string(k)] = p
			return nil
		})

		// File Tags
		bTags := tx.Bucket(bucketFileTags)
		fileTags := make(map[string][]string)
		bTags.ForEach(func(k, v []byte) error {
			var tags []string
			json.Unmarshal(v, &tags)
			fileTags[string(k)] = tags
			return nil
		})
		s.tags.setFileTags(fileTags)

		// Link Counts
		bLinksC := tx.Bucket(bucketLinkCounts)
		linkCounts := make(map[string]int)
		bLinksC.ForEach(func(k, v []byte) error {
			var c int
			json.Unmarshal(v, &c)
			linkCounts[string(k)] = c
			return nil
		})
		s.links.setLinkCounts(linkCounts)

		// File Links
		bLinksF := tx.Bucket(bucketFileLinks)
		fileLinks := make(map[string][]string)
		bLinksF.ForEach(func(k, v []byte) error {
			var links []string
			json.Unmarshal(v, &links)
			fileLinks[string(k)] = links
			return nil
		})
		s.links.setFileLinksMap(fileLinks)

		// File Metadata
		bMeta := tx.Bucket(bucketFileMetadata)
		bMeta.ForEach(func(k, v []byte) error {
			var meta map[string]interface{}
			json.Unmarshal(v, &meta)
			s.fileMetadata[string(k)] = meta
			return nil
		})

		// Settings
		bSettings := tx.Bucket(bucketSettings)
		if bSettings != nil {
			v := bSettings.Get([]byte("current"))
			if v != nil {
				json.Unmarshal(v, &s.settings)
			}
		}

		// Vector Hashes
		bVecHashes := tx.Bucket(bucketVectorHashes)
		if bVecHashes != nil {
			vecHashes := make(map[string]string)
			bVecHashes.ForEach(func(k, v []byte) error {
				vecHashes[string(k)] = string(v)
				return nil
			})
			s.vectors.setVectorHashes(vecHashes)
		}

		return nil
	})

	s.RebuildKnownTagsCache()

	log.Printf("[Init] Estado carregado do BBolt: %d arquivos, %d hashes.\n",
		len(s.fileModCache), len(s.hashCache))
}

func (s *AppState) performMigration(legacy models.SystemState) {
	s.db.Update(func(tx *bolt.Tx) error {
		if legacy.FileModCache != nil {
			b := tx.Bucket(bucketFileMods)
			for k, v := range legacy.FileModCache {
				bin, _ := v.MarshalBinary()
				b.Put([]byte(k), bin)
			}
		}
		if legacy.HashCache != nil {
			b := tx.Bucket(bucketHashes)
			for k, v := range legacy.HashCache {
				b.Put([]byte(k), []byte(v))
			}
		}
		if legacy.Popularity != nil {
			b := tx.Bucket(bucketPopularity)
			for k, v := range legacy.Popularity {
				val, _ := json.Marshal(v)
				b.Put([]byte(k), val)
			}
		}
		if legacy.FileTags != nil {
			b := tx.Bucket(bucketFileTags)
			for k, v := range legacy.FileTags {
				val, _ := json.Marshal(v)
				b.Put([]byte(k), val)
			}
		}
		if legacy.LinkCounts != nil {
			b := tx.Bucket(bucketLinkCounts)
			for k, v := range legacy.LinkCounts {
				val, _ := json.Marshal(v)
				b.Put([]byte(k), val)
			}
		}
		if legacy.FileLinks != nil {
			b := tx.Bucket(bucketFileLinks)
			for k, v := range legacy.FileLinks {
				val, _ := json.Marshal(v)
				b.Put([]byte(k), val)
			}
		}
		bSettings := tx.Bucket(bucketSettings)
		val, _ := json.Marshal(legacy.Settings)
		bSettings.Put([]byte("current"), val)

		return nil
	})
}

// RebuildKnownTagsCache reconstrói o cache de tags conhecidas a partir das tags
// de todos os arquivos. Deve ser chamado após Load e após operações que alterem tags.
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

	log.Printf("[Sync] Cache de sugestões de tags reconstruído: %d tags ativas.\n", len(newKnownTags))
}

// ── LinkCounts (delegação para LinkManager) ──

func (s *AppState) UpdateLinkCounts(newCounts map[string]int) { s.links.UpdateLinkCounts(newCounts) }
func (s *AppState) GetLinkCount(filename string) int          { return s.links.GetLinkCount(filename) }

// ── FileLinks (delegação para LinkManager) ──

func (s *AppState) SetFileLinks(filename string, links []string) {
	s.links.SetFileLinks(filename, links)
}
func (s *AppState) GetFileLinks(filename string) []string { return s.links.GetFileLinks(filename) }
func (s *AppState) GetAllFileLinks() map[string][]string  { return s.links.GetAllFileLinks() }
func (s *AppState) DeleteFileLinks(filename string)       { s.links.DeleteFileLinks(filename) }

// ── Tags (delegação para TagManager) ──

func (s *AppState) AddKnownTag(tag string)                     { s.tags.AddKnownTag(tag) }
func (s *AppState) GetAllKnownTags() []string                  { return s.tags.GetAllKnownTags() }
func (s *AppState) GetKnownTagsCount() int                     { return s.tags.GetKnownTagsCount() }
func (s *AppState) SetFileTags(filename string, tags []string) { s.tags.SetFileTags(filename, tags) }
func (s *AppState) GetFileTags(filename string) []string       { return s.tags.GetFileTags(filename) }
func (s *AppState) HasTags(filename string) bool               { return s.tags.HasTags(filename) }
func (s *AppState) DeleteFileTags(filename string)             { s.tags.DeleteFileTags(filename) }

// ── Vector Hashes (delegação para VectorManager) ──

func (s *AppState) GetVectorHash(id string) (string, bool) { return s.vectors.GetVectorHash(id) }
func (s *AppState) GetVectorHashCount() int                { return s.vectors.GetVectorHashCount() }
func (s *AppState) SetVectorHash(id, hash string)          { s.vectors.SetVectorHash(id, hash) }
func (s *AppState) DeleteVectorHash(id string)             { s.vectors.DeleteVectorHash(id) }

// ── Note Vectors (delegação para VectorManager) ──

func (s *AppState) SetNoteVector(id string, vector []float32) { s.vectors.SetNoteVector(id, vector) }
func (s *AppState) GetAllNoteVectors() map[string][]float32   { return s.vectors.GetAllNoteVectors() }
func (s *AppState) ClearNoteVectors() error                   { return s.vectors.ClearNoteVectors() }

// ── Note Projections (delegação para VectorManager) ──

func (s *AppState) SetNoteProjections(projections map[string][]float64) {
	s.vectors.SetNoteProjections(projections)
}
func (s *AppState) GetAllNoteProjections() map[string][]float64 {
	return s.vectors.GetAllNoteProjections()
}
func (s *AppState) ClearNoteProjections() error { return s.vectors.ClearNoteProjections() }
