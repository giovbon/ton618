package db

import (
	"encoding/binary"
	"math"
	"strings"
)

// NoteVector holds an embedding vector and its associated metadata.
type NoteVector struct {
	Vector []float32
	Title  string
	X, Y   float64
}

// Embedding2D holds a light 2D projection point for graph rendering (no vector blob).
type Embedding2D struct {
	DocID   string
	Title   string
	Arquivo string
	X, Y    float64
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

// DeleteEmbedding removes the embedding for a document by its doc_id (hash).
func (s *Store) DeleteEmbedding(docID string) error {
	_, err := s.DB.Exec("DELETE FROM embeddings WHERE doc_id = ?", docID)
	return err
}

// DeleteEmbeddingsByFile removes all embeddings for documents belonging to a file.
// Usa JOIN com a tabela documents para mapear arquivo -> doc_ids.
func (s *Store) DeleteEmbeddingsByFile(arquivo string) error {
	_, err := s.DB.Exec(`
		DELETE FROM embeddings WHERE doc_id IN (
			SELECT id FROM documents WHERE arquivo = ?
		)
	`, arquivo)
	return err
}

// DeleteOrphanedEmbeddings removes embeddings whose documents no longer exist.
func (s *Store) DeleteOrphanedEmbeddings() (int64, error) {
	res, err := s.DB.Exec(`
		DELETE FROM embeddings WHERE doc_id NOT IN (
			SELECT id FROM documents
		)
	`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// HasFileEmbedding returns true if any document belonging to a file has an embedding stored.
func (s *Store) HasFileEmbedding(arquivo string) bool {
	var count int
	s.DB.QueryRow(`
		SELECT COUNT(*) FROM embeddings e
		INNER JOIN documents d ON d.id = e.doc_id
		WHERE d.arquivo = ?`, arquivo).Scan(&count)
	return count > 0
}

// GetEmbeddingCount returns the total number of stored embeddings.
func (s *Store) GetEmbeddingCount() int {
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM embeddings").Scan(&count)
	return count
}

// GetEmbeddings2DForGraph returns embeddings with 2D coords joined with document info,
// without loading the full vector BLOB. Limited and randomized for graph display.
func (s *Store) GetEmbeddings2DForGraph(limit int) ([]Embedding2D, error) {
	rows, err := s.DB.Query(`
		SELECT e.doc_id, e.title, e.x, e.y, COALESCE(d.arquivo, '')
		FROM embeddings e
		LEFT JOIN documents d ON d.id = e.doc_id
		WHERE e.x != 0 OR e.y != 0
		ORDER BY RANDOM()
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Embedding2D
	for rows.Next() {
		var e Embedding2D
		if err := rows.Scan(&e.DocID, &e.Title, &e.X, &e.Y, &e.Arquivo); err != nil {
			continue
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// GetEmbeddings2DWithVectors returns embeddings that have vectors but no 2D coords yet,
// so they can be projected later. Returns a map keyed by doc_id.
func (s *Store) GetEmbeddings2DWithVectors(limit int) (map[string]NoteVector, error) {
	rows, err := s.DB.Query(`
		SELECT e.doc_id, e.vector, e.title, e.x, e.y, COALESCE(d.arquivo, '')
		FROM embeddings e
		LEFT JOIN documents d ON d.id = e.doc_id
		WHERE (e.x = 0 AND e.y = 0) AND e.vector IS NOT NULL
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]NoteVector)
	for rows.Next() {
		var docID, arquivo string
		var data []byte
		var nv NoteVector
		if err := rows.Scan(&docID, &data, &nv.Title, &nv.X, &nv.Y, &arquivo); err != nil {
			continue
		}
		nv.Vector = DecodeVector(data)
		result[docID] = nv
	}
	return result, rows.Err()
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

// GetEmbeddingsByDocIDs returns embeddings for a list of document IDs in a single query.
// Muito mais eficiente que N chamadas individuais a GetEmbedding.
func (s *Store) GetEmbeddingsByDocIDs(docIDs []string) (map[string]NoteVector, error) {
	if len(docIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(docIDs))
	args := make([]any, len(docIDs))
	for i, id := range docIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := "SELECT doc_id, vector, title, x, y FROM embeddings WHERE doc_id IN (" +
		strings.Join(placeholders, ",") + ")"

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]NoteVector, len(docIDs))
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
