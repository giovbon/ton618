package ingest

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"etl/internal/config"
	"etl/internal/models"

	bolt "go.etcd.io/bbolt"
)

// Load carrega todos os dados do BBolt para os caches em RAM.
func (s *AppState) Load(cfg *config.AppConfig) {
	if _, err := os.Stat(cfg.StateFile); err == nil {
		log.Println("[DB] Detectado state.json legado. Iniciando migracao...")
		data, err := os.ReadFile(cfg.StateFile)
		if err == nil {
			var legacy models.SystemState
			if err := json.Unmarshal(data, &legacy); err == nil {
				s.performMigration(legacy)
				os.Rename(cfg.StateFile, cfg.StateFile+".bak")
				log.Println("[DB] Migracao concluida. state.json renomeado para .bak")
			}
		}
	}

	s.db.View(func(tx *bolt.Tx) error {
		bMods := tx.Bucket(bucketFileMods)
		bMods.ForEach(func(k, v []byte) error {
			var t time.Time
			t.UnmarshalBinary(v)
			s.fileModCache[string(k)] = t
			return nil
		})

		bHashes := tx.Bucket(bucketHashes)
		bHashes.ForEach(func(k, v []byte) error {
			s.hashCache[string(k)] = string(v)
			return nil
		})

		bPop := tx.Bucket(bucketPopularity)
		bPop.ForEach(func(k, v []byte) error {
			var p int
			json.Unmarshal(v, &p)
			s.popularity[string(k)] = p
			return nil
		})

		bTags := tx.Bucket(bucketFileTags)
		fileTags := make(map[string][]string)
		bTags.ForEach(func(k, v []byte) error {
			var tags []string
			json.Unmarshal(v, &tags)
			fileTags[string(k)] = tags
			return nil
		})
		s.tags.setFileTags(fileTags)

		bLinksC := tx.Bucket(bucketLinkCounts)
		linkCounts := make(map[string]int)
		bLinksC.ForEach(func(k, v []byte) error {
			var c int
			json.Unmarshal(v, &c)
			linkCounts[string(k)] = c
			return nil
		})
		s.links.setLinkCounts(linkCounts)

		bLinksF := tx.Bucket(bucketFileLinks)
		fileLinks := make(map[string][]string)
		bLinksF.ForEach(func(k, v []byte) error {
			var links []string
			json.Unmarshal(v, &links)
			fileLinks[string(k)] = links
			return nil
		})
		s.links.setFileLinksMap(fileLinks)

		bMeta := tx.Bucket(bucketFileMetadata)
		bMeta.ForEach(func(k, v []byte) error {
			var meta map[string]interface{}
			json.Unmarshal(v, &meta)
			s.fileMetadata[string(k)] = meta
			return nil
		})

		bSettings := tx.Bucket(bucketSettings)
		if bSettings != nil {
			v := bSettings.Get([]byte("current"))
			if v != nil {
				json.Unmarshal(v, &s.settings)
			}
		}

		bVecHashes := tx.Bucket(bucketVectorHashes)
		if bVecHashes != nil {
			vecHashes := make(map[string]string)
			bVecHashes.ForEach(func(k, v []byte) error {
				vecHashes[string(k)] = string(v)
				return nil
			})
			s.vectors.setVectorHashes(vecHashes)
		}

		bST := tx.Bucket(bucketSemanticTopics)
		if bST != nil {
			topics := make(map[string]bool)
			bST.ForEach(func(k, v []byte) error {
				topics[string(k)] = true
				return nil
			})
			s.semantic.setTopics(topics)
		}

		bSL := tx.Bucket(bucketFileSemanticLinks)
		if bSL != nil {
			links := make(map[string][]string)
			bSL.ForEach(func(k, v []byte) error {
				var l []string
				json.Unmarshal(v, &l)
				links[string(k)] = l
				return nil
			})
			s.semantic.setFileSemanticLinksMap(links)
		}

		// Reconstroi topicos no startup para eliminar orfaos residuais
		s.semantic.RebuildSemanticTopics()

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
}
