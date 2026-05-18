package db

import (
	"encoding/binary"
	"math"
)

// NoteVector holds an embedding vector and its associated metadata.
type NoteVector struct {
	Vector []float32
	Title  string
	X, Y   float64
}

// EncodeVector serializes a []float32 slice into a []byte (little-endian).
func EncodeVector(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// DecodeVector deserializes a []byte back into a []float32 slice.
// Returns nil if the data is empty or its length is not a multiple of 4.
func DecodeVector(data []byte) []float32 {
	if len(data) == 0 || len(data)%4 != 0 {
		return nil
	}
	vec := make([]float32, len(data)/4)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return vec
}

// SetEmbedding stores (or replaces) the embedding vector and title for a document.
func (s *Store) SetEmbedding(docID string, vector []float32, title string) error {
	_, err := s.DB.Exec(`
		INSERT OR REPLACE INTO embeddings (doc_id, vector, title, created_at)
		VALUES (?, ?, ?, datetime('now'))`,
		docID, EncodeVector(vector), title,
	)
	return err
}

// SetEmbedding2D updates the 2D projection coordinates for a document embedding.
func (s *Store) SetEmbedding2D(docID string, x, y float64) error {
	_, err := s.DB.Exec(
		"UPDATE embeddings SET x = ?, y = ? WHERE doc_id = ?",
		x, y, docID,
	)
	return err
}

// GetEmbedding returns the embedding for a document, or nil if not found.
func (s *Store) GetEmbedding(docID string) (*NoteVector, error) {
	row := s.DB.QueryRow(
		"SELECT vector, title, x, y FROM embeddings WHERE doc_id = ?", docID)
	var data []byte
	var nv NoteVector
	err := row.Scan(&data, &nv.Title, &nv.X, &nv.Y)
	if err != nil {
		return nil, nil // not found
	}
	nv.Vector = DecodeVector(data)
	return &nv, nil
}

// GetAllEmbeddings returns all stored embeddings keyed by document ID.
func (s *Store) GetAllEmbeddings() (map[string]NoteVector, error) {
	rows, err := s.DB.Query("SELECT doc_id, vector, title, x, y FROM embeddings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]NoteVector)
	for rows.Next() {
		var docID string
		var data []byte
		var nv NoteVector
		if err := rows.Scan(&docID, &data, &nv.Title, &nv.X, &nv.Y); err != nil {
			continue
		}
		nv.Vector = DecodeVector(data)
		result[docID] = nv
	}
	return result, rows.Err()
}

// DeleteEmbedding removes the embedding for a document.
func (s *Store) DeleteEmbedding(docID string) error {
	_, err := s.DB.Exec("DELETE FROM embeddings WHERE doc_id = ?", docID)
	return err
}

// GetEmbeddingCount returns the total number of stored embeddings.
func (s *Store) GetEmbeddingCount() int {
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM embeddings").Scan(&count)
	return count
}

// GetEmbeddingsByFile returns embeddings for all documents belonging to a file.
// This joins embeddings with documents to filter by file path.
func (s *Store) GetEmbeddingsByFile(arquivo string) (map[string]NoteVector, error) {
	rows, err := s.DB.Query(`
		SELECT e.doc_id, e.vector, e.title, e.x, e.y
		FROM embeddings e
		INNER JOIN documents d ON d.id = e.doc_id
		WHERE d.arquivo = ?`, arquivo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]NoteVector)
	for rows.Next() {
		var docID string
		var data []byte
		var nv NoteVector
		if err := rows.Scan(&docID, &data, &nv.Title, &nv.X, &nv.Y); err != nil {
			continue
		}
		nv.Vector = DecodeVector(data)
		result[docID] = nv
	}
	return result, rows.Err()
}
