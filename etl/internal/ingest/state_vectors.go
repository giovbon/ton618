package ingest

import (
	"encoding/json"
	"sync"

	bolt "go.etcd.io/bbolt"
)

// VectorManager gerencia vetores de embedding, hashes de vetores e projeções 2D.
type VectorManager struct {
	vectorHashes   map[string]string
	vectorHashesMu sync.RWMutex
	db             *bolt.DB
}

func newVectorManager(db *bolt.DB) *VectorManager {
	return &VectorManager{
		vectorHashes: make(map[string]string),
		db:           db,
	}
}

// Vector Hashes

func (vm *VectorManager) GetVectorHash(id string) (string, bool) {
	vm.vectorHashesMu.RLock()
	defer vm.vectorHashesMu.RUnlock()
	hash, exists := vm.vectorHashes[id]
	return hash, exists
}

func (vm *VectorManager) SetVectorHash(id, hash string) {
	vm.vectorHashesMu.Lock()
	vm.vectorHashes[id] = hash
	vm.vectorHashesMu.Unlock()

	vm.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketVectorHashes).Put([]byte(id), []byte(hash))
	})
}

func (vm *VectorManager) GetVectorHashCount() int {
	vm.vectorHashesMu.RLock()
	defer vm.vectorHashesMu.RUnlock()
	return len(vm.vectorHashes)
}

func (vm *VectorManager) DeleteVectorHash(id string) {
	vm.vectorHashesMu.Lock()
	delete(vm.vectorHashes, id)
	vm.vectorHashesMu.Unlock()

	vm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketVectorHashes)
		if b != nil {
			b.Delete([]byte(id))
		}
		bv := tx.Bucket(bucketNoteVectors)
		if bv != nil {
			bv.Delete([]byte(id))
		}
		return nil
	})
}

// Note Vectors

func (vm *VectorManager) SetNoteVector(id string, vector []float32) {
	vm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNoteVectors)
		if b != nil {
			data, _ := json.Marshal(vector)
			return b.Put([]byte(id), data)
		}
		return nil
	})
}

func (vm *VectorManager) GetAllNoteVectors() map[string][]float32 {
	result := make(map[string][]float32)
	vm.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNoteVectors)
		if b != nil {
			b.ForEach(func(k, v []byte) error {
				var vec []float32
				if err := json.Unmarshal(v, &vec); err == nil {
					result[string(k)] = vec
				}
				return nil
			})
		}
		return nil
	})
	return result
}

func (vm *VectorManager) ClearNoteVectors() error {
	return vm.db.Update(func(tx *bolt.Tx) error {
		tx.DeleteBucket(bucketNoteVectors)
		_, err := tx.CreateBucket(bucketNoteVectors)
		return err
	})
}

// Note Projections (2D PCA/SVD)

func (vm *VectorManager) SetNoteProjections(projections map[string][]float64) {
	vm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNoteProjections)
		if b != nil {
			tx.DeleteBucket(bucketNoteProjections)
			b, _ = tx.CreateBucket(bucketNoteProjections)
			for id, coords := range projections {
				data, _ := json.Marshal(coords)
				b.Put([]byte(id), data)
			}
		}
		return nil
	})
}

func (vm *VectorManager) GetAllNoteProjections() map[string][]float64 {
	result := make(map[string][]float64)
	vm.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNoteProjections)
		if b != nil {
			b.ForEach(func(k, v []byte) error {
				var coords []float64
				if err := json.Unmarshal(v, &coords); err == nil {
					result[string(k)] = coords
				}
				return nil
			})
		}
		return nil
	})
	return result
}

func (vm *VectorManager) ClearNoteProjections() error {
	return vm.db.Update(func(tx *bolt.Tx) error {
		tx.DeleteBucket(bucketNoteProjections)
		_, err := tx.CreateBucket(bucketNoteProjections)
		return err
	})
}

// setVectorHashes popula o cache (uso interno pelo AppState.Load).
func (vm *VectorManager) setVectorHashes(m map[string]string) {
	vm.vectorHashesMu.Lock()
	defer vm.vectorHashesMu.Unlock()
	vm.vectorHashes = m
}
