package ingest

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"sync"

	bolt "go.etcd.io/bbolt"
)

// NoteVector armazena vetor, titulo e coordenadas 2D cacheadas (t-SNE/PCA).
type NoteVector struct {
	Vector []float32 `json:"v"`
	Title  string    `json:"t,omitempty"`
	X      float64   `json:"x,omitempty"`
	Y      float64   `json:"y,omitempty"`
}

// VectorManager gerencia vetores de embedding, hashes de vetores e projecoes 2D.
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

func (vm *VectorManager) SetNoteVector(id string, vector []float32, title string) {
	vm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNoteVectors)
		if b != nil {
			nv := NoteVector{Vector: vector, Title: title}
			data := encodeNoteVector(nv)
			return b.Put([]byte(id), data)
		}
		return nil
	})
}

func (vm *VectorManager) SetNoteVectors2D(id string, x, y float64) {
	vm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNoteVectors)
		if b == nil {
			return nil
		}
		raw := b.Get([]byte(id))
		if raw == nil {
			return nil
		}
		nv := decodeNoteVector(raw)
		nv.X = x
		nv.Y = y
		return b.Put([]byte(id), encodeNoteVector(nv))
	})
}

func (vm *VectorManager) GetAllNoteVectors() map[string]NoteVector {
	result := make(map[string]NoteVector)
	vm.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNoteVectors)
		if b != nil {
			b.ForEach(func(k, v []byte) error {
				nv := decodeNoteVector(v)
				if len(nv.Vector) > 0 {
					result[string(k)] = nv
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

// Note Projections (obsoleto — coords 2D agora no proprio NoteVector)

func (vm *VectorManager) SetNoteProjections(projections map[string][]float64) {
	vm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNoteProjections)
		if b != nil {
			tx.DeleteBucket(bucketNoteProjections)
			b, _ = tx.CreateBucket(bucketNoteProjections)
			for id, coords := range projections {
				data, _ := json.Marshal(coords)
				b.Put([]byte(id), data)
				vm.updateNoteVectorCoords(tx, id, coords[0], coords[1])
			}
		}
		return nil
	})
}

func (vm *VectorManager) updateNoteVectorCoords(tx *bolt.Tx, id string, x, y float64) {
	bv := tx.Bucket(bucketNoteVectors)
	if bv == nil {
		return
	}
	raw := bv.Get([]byte(id))
	if raw == nil {
		return
	}
	nv := decodeNoteVector(raw)
	if len(nv.Vector) > 0 {
		nv.X = x
		nv.Y = y
		bv.Put([]byte(id), encodeNoteVector(nv))
	}
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

func (vm *VectorManager) DeleteNoteProjection(id string) {
	vm.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNoteProjections)
		if b != nil {
			b.Delete([]byte(id))
		}
		return nil
	})
}

// setVectorHashes popula o cache (uso interno pelo AppState.Load).
func (vm *VectorManager) setVectorHashes(m map[string]string) {
	vm.vectorHashesMu.Lock()
	defer vm.vectorHashesMu.Unlock()
	vm.vectorHashes = m
}

// ── Binary encoding (substitui JSON, mais compacto e rapido) ──

const binMagic = 0xFF

// encodeNoteVector serializa NoteVector em binario.
// Formato: magic(1) + vecLen(4) + floats(vecLen*4) + titleLen(2) + title + x(8) + y(8)
func encodeNoteVector(nv NoteVector) []byte {
	buf := new(bytes.Buffer)

	// Magic byte
	buf.WriteByte(binMagic)

	// Vector length + floats
	binary.Write(buf, binary.LittleEndian, uint32(len(nv.Vector)))
	for _, f := range nv.Vector {
		binary.Write(buf, binary.LittleEndian, f)
	}

	// Title
	titleBytes := []byte(nv.Title)
	binary.Write(buf, binary.LittleEndian, uint16(len(titleBytes)))
	buf.Write(titleBytes)

	// 2D coords
	binary.Write(buf, binary.LittleEndian, nv.X)
	binary.Write(buf, binary.LittleEndian, nv.Y)

	return buf.Bytes()
}

// decodeNoteVector decodifica NoteVector de binario ou JSON (backward compat).
func decodeNoteVector(data []byte) NoteVector {
	if len(data) == 0 {
		return NoteVector{}
	}

	// Detectar formato: binario se primeiro byte == magic, senao JSON
	if data[0] == binMagic {
		return decodeNoteVectorBin(data)
	}
	return decodeNoteVectorJSON(data)
}

func decodeNoteVectorBin(data []byte) NoteVector {
	buf := bytes.NewReader(data)

	// Pular magic byte
	buf.ReadByte()

	// Vector
	var vecLen uint32
	binary.Read(buf, binary.LittleEndian, &vecLen)
	vec := make([]float32, vecLen)
	for i := uint32(0); i < vecLen; i++ {
		var f float32
		binary.Read(buf, binary.LittleEndian, &f)
		vec[i] = f
	}

	// Title
	var titleLen uint16
	binary.Read(buf, binary.LittleEndian, &titleLen)
	titleBytes := make([]byte, titleLen)
	buf.Read(titleBytes)

	// 2D coords
	var x, y float64
	binary.Read(buf, binary.LittleEndian, &x)
	binary.Read(buf, binary.LittleEndian, &y)

	return NoteVector{Vector: vec, Title: string(titleBytes), X: x, Y: y}
}

func decodeNoteVectorJSON(data []byte) NoteVector {
	var nv NoteVector
	if err := json.Unmarshal(data, &nv); err == nil && len(nv.Vector) > 0 {
		return nv
	}
	// Formato antigo: array simples de float32
	var vec []float32
	if err := json.Unmarshal(data, &vec); err == nil && len(vec) > 0 {
		return NoteVector{Vector: vec}
	}
	return NoteVector{}
}
