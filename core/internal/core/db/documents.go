package db

import (
	"database/sql"
	"strings"
	"ton618/core/internal/core/db/generated"
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

// fromDBGen converts a dbgen.Document to a db.Document
func fromDBGen(d dbgen.Document) Document {
	return Document{
		ID:        d.ID,
		Tipo:      d.Tipo.String,
		Arquivo:   d.Arquivo.String,
		Secao:     d.Secao.String,
		Texto:     d.Texto.String,
		Tags:      d.Tags.String,
		Pagina:    int(d.Pagina.Int64),
		Ordem:     int(d.Ordem.Int64),
		Timestamp: d.Timestamp.String,
		CreatedAt: d.CreatedAt.String,
		Hash:      d.Hash.String,
	}
}

// InsertDocument inserts a new document or replaces an existing one with the same ID.
func (s *Store) InsertDocument(doc Document) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.InsertDocument(s.queryCtx(), dbgen.InsertDocumentParams{
		ID:        doc.ID,
		Tipo:      sql.NullString{String: doc.Tipo, Valid: true},
		Arquivo:   sql.NullString{String: doc.Arquivo, Valid: true},
		Secao:     sql.NullString{String: doc.Secao, Valid: true},
		Texto:     sql.NullString{String: doc.Texto, Valid: true},
		Tags:      sql.NullString{String: doc.Tags, Valid: true},
		Pagina:    sql.NullInt64{Int64: int64(doc.Pagina), Valid: true},
		Ordem:     sql.NullInt64{Int64: int64(doc.Ordem), Valid: true},
		Timestamp: sql.NullString{String: doc.Timestamp, Valid: true},
		CreatedAt: sql.NullString{String: doc.CreatedAt, Valid: true},
		Hash:      sql.NullString{String: doc.Hash, Valid: true},
	})
}

// DeleteDocument removes a single document by ID.
func (s *Store) DeleteDocument(id string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.DeleteDocument(s.queryCtx(), id)
}

// DeleteDocumentsByFile removes all documents associated with a given file path.
func (s *Store) DeleteDocumentsByFile(arquivo string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.DeleteDocumentsByFile(s.queryCtx(), sql.NullString{String: arquivo, Valid: true})
}

// GetDocument returns a single document by ID, or nil if not found.
func (s *Store) GetDocument(id string) (*Document, error) {
	row, err := s.Q.GetDocument(s.queryCtx(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	doc := fromDBGen(row)
	return &doc, nil
}

// BatchGetTimestamps returns a map of doc ID to timestamp for all given doc IDs.
// Executa uma única query SQL com IN clause em vez de N queries individuais.
func (s *Store) BatchGetTimestamps(docIDs []string) (map[string]string, error) {
	if len(docIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(docIDs))
	args := make([]any, len(docIDs))
	for i, id := range docIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := "SELECT id, timestamp FROM documents WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string, len(docIDs))
	for rows.Next() {
		var id, ts string
		if err := rows.Scan(&id, &ts); err != nil {
			return nil, err
		}
		result[id] = ts
	}
	return result, rows.Err()
}

// GetDocumentsByFile returns all documents belonging to a file, ordered by position.
func (s *Store) GetDocumentsByFile(arquivo string) ([]Document, error) {
	rows, err := s.Q.GetDocumentsByFile(s.queryCtx(), sql.NullString{String: arquivo, Valid: true})
	if err != nil {
		return nil, err
	}
	var docs []Document
	for _, r := range rows {
		docs = append(docs, fromDBGen(r))
	}
	return docs, nil
}

// GetAllDocumentsByFile returns all documents grouped by their file path.
func (s *Store) GetAllDocumentsByFile() (map[string][]Document, error) {
	rows, err := s.Q.GetAllDocumentsByFile(s.queryCtx())
	if err != nil {
		return nil, err
	}
	result := make(map[string][]Document)
	for _, r := range rows {
		doc := fromDBGen(r)
		result[doc.Arquivo] = append(result[doc.Arquivo], doc)
	}
	return result, nil
}

// GetAllDocuments returns every document in the database.
func (s *Store) GetAllDocuments() ([]Document, error) {
	rows, err := s.Q.GetAllDocuments(s.queryCtx())
	if err != nil {
		return nil, err
	}
	var docs []Document
	for _, r := range rows {
		docs = append(docs, fromDBGen(r))
	}
	return docs, nil
}

// GetDocumentsPaginated returns a page of documents, along with the total count.
func (s *Store) GetDocumentsPaginated(from, size int) ([]Document, int, error) {
	total, err := s.Q.CountDocumentsWithoutDrawing(s.queryCtx())
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.Q.GetDocumentsPaginated(s.queryCtx(), dbgen.GetDocumentsPaginatedParams{
		Limit:  int64(size),
		Offset: int64(from),
	})
	if err != nil {
		return nil, int(total), err
	}
	var docs []Document
	for _, r := range rows {
		docs = append(docs, fromDBGen(r))
	}
	return docs, int(total), nil
}

// GetDocumentCount returns the total number of documents in the database.
func (s *Store) GetDocumentCount() int {
	count, _ := s.Q.GetDocumentCount(s.queryCtx())
	return int(count)
}

// GetDistinctFiles returns all unique file paths that have documents.
func (s *Store) GetDistinctFiles() ([]string, error) {
	rows, err := s.Q.GetDistinctFiles(s.queryCtx())
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range rows {
		files = append(files, f.String)
	}
	return files, nil
}

// SearchDocumentText returns the count of documents whose texto column contains the given substring.
// Usado para verificar se uma imagem ainda é referenciada por alguma nota.
func (s *Store) SearchDocumentText(substring string) (int, error) {
	count, err := s.Q.SearchDocumentText(s.queryCtx(), sql.NullString{String: "%" + substring + "%", Valid: true})
	return int(count), err
}
