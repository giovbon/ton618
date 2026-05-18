package db

import (
	"database/sql"
	"strings"
)

// Document represents a single document/chunk stored in the database.
type Document struct {
	ID         string
	Tipo       string
	Arquivo    string
	Secao      string
	Texto      string
	Tags       string
	Pagina     int
	Ordem      int
	Timestamp  string
	CreatedAt  string
	Hash       string
	VectorHash string
	IsIndexing bool
	IsNoEmbed  bool
}

// TagsToSlice converts a comma-separated tag string to a slice.
func TagsToSlice(tags string) []string {
	if tags == "" {
		return nil
	}
	return strings.Split(tags, ",")
}

// SliceToTags joins a tag slice into a comma-separated string.
func SliceToTags(tags []string) string {
	return strings.Join(tags, ",")
}

// InsertDocument inserts a new document or replaces an existing one with the same ID.
func (s *Store) InsertDocument(doc Document) error {
	_, err := s.DB.Exec(`
		INSERT OR REPLACE INTO documents
		(id, tipo, arquivo, secao, texto, tags, pagina, ordem, timestamp, created_at, hash, vector_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.ID, doc.Tipo, doc.Arquivo, doc.Secao, doc.Texto, doc.Tags,
		doc.Pagina, doc.Ordem, doc.Timestamp, doc.CreatedAt, doc.Hash, doc.VectorHash,
	)
	return err
}

// DeleteDocument removes a single document by ID.
func (s *Store) DeleteDocument(id string) error {
	_, err := s.DB.Exec("DELETE FROM documents WHERE id = ?", id)
	return err
}

// DeleteDocumentsByFile removes all documents associated with a given file path.
func (s *Store) DeleteDocumentsByFile(arquivo string) error {
	_, err := s.DB.Exec("DELETE FROM documents WHERE arquivo = ?", arquivo)
	return err
}

// GetDocument returns a single document by ID, or nil if not found.
func (s *Store) GetDocument(id string) (*Document, error) {
	row := s.DB.QueryRow(`
		SELECT id, tipo, arquivo, secao, texto, tags, pagina, ordem, timestamp, created_at, hash, vector_hash
		FROM documents WHERE id = ?`, id)
	var doc Document
	err := row.Scan(&doc.ID, &doc.Tipo, &doc.Arquivo, &doc.Secao, &doc.Texto,
		&doc.Tags, &doc.Pagina, &doc.Ordem, &doc.Timestamp, &doc.CreatedAt, &doc.Hash, &doc.VectorHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// GetDocumentsByFile returns all documents belonging to a file, ordered by position.
func (s *Store) GetDocumentsByFile(arquivo string) ([]Document, error) {
	rows, err := s.DB.Query(`
		SELECT id, tipo, arquivo, secao, texto, tags, pagina, ordem, timestamp, created_at, hash, vector_hash
		FROM documents WHERE arquivo = ? ORDER BY ordem ASC`, arquivo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var doc Document
		if err := rows.Scan(&doc.ID, &doc.Tipo, &doc.Arquivo, &doc.Secao, &doc.Texto,
			&doc.Tags, &doc.Pagina, &doc.Ordem, &doc.Timestamp, &doc.CreatedAt, &doc.Hash, &doc.VectorHash); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// GetAllDocumentsByFile returns all documents grouped by their file path.
func (s *Store) GetAllDocumentsByFile() (map[string][]Document, error) {
	rows, err := s.DB.Query(`
		SELECT id, tipo, arquivo, secao, texto, tags, pagina, ordem, timestamp, created_at, hash, vector_hash
		FROM documents ORDER BY arquivo, ordem ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]Document)
	for rows.Next() {
		var doc Document
		if err := rows.Scan(&doc.ID, &doc.Tipo, &doc.Arquivo, &doc.Secao, &doc.Texto,
			&doc.Tags, &doc.Pagina, &doc.Ordem, &doc.Timestamp, &doc.CreatedAt, &doc.Hash, &doc.VectorHash); err != nil {
			return nil, err
		}
		result[doc.Arquivo] = append(result[doc.Arquivo], doc)
	}
	return result, rows.Err()
}

// GetAllDocuments returns every document in the database.
func (s *Store) GetAllDocuments() ([]Document, error) {
	rows, err := s.DB.Query(`
		SELECT id, tipo, arquivo, secao, texto, tags, pagina, ordem, timestamp, created_at, hash, vector_hash
		FROM documents ORDER BY arquivo, ordem ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var doc Document
		if err := rows.Scan(&doc.ID, &doc.Tipo, &doc.Arquivo, &doc.Secao, &doc.Texto,
			&doc.Tags, &doc.Pagina, &doc.Ordem, &doc.Timestamp, &doc.CreatedAt, &doc.Hash, &doc.VectorHash); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// GetDocumentCount returns the total number of documents in the database.
func (s *Store) GetDocumentCount() int {
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM documents").Scan(&count)
	return count
}

// GetDistinctFiles returns all unique file paths that have documents.
func (s *Store) GetDistinctFiles() ([]string, error) {
	rows, err := s.DB.Query("SELECT DISTINCT arquivo FROM documents ORDER BY arquivo")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}
