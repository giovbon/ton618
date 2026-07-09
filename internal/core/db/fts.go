package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// FTSResult represents a single result row from a full-text search query.
type FTSResult struct {
	DocID   string
	Tipo    string
	Arquivo string
	Secao   string
	Texto   string
	Tags    string
	Rank    float64
	Snippet string
}

// IndexFTS inserts or updates a document in the FTS5 index.
// It first deletes any existing entry for the same doc_id to avoid duplicates.
//
// ATENÇÃO: este método adquire WriteMu de forma independente. Não chame de dentro
// de outro método que já segure WriteMu (ex: ReplaceFileIndexes), pois causará deadlock.
// Dentro de transações, use tx.Exec diretamente em vez de chamar este método.
func (s *Store) IndexFTS(docID, tipo, arquivo, secao, texto, tags string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	s.DB.Exec("DELETE FROM docs_fts WHERE doc_id = ?", docID)
	_, err := s.DB.Exec(`
		INSERT INTO docs_fts (doc_id, tipo, arquivo, secao, texto, tags)
		VALUES (?, ?, ?, ?, ?, ?)`,
		docID, tipo, arquivo, secao, texto, tags,
	)
	return err
}

// DeleteFTS removes a single document from the FTS5 index by doc_id.
func (s *Store) DeleteFTS(docID string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	_, err := s.DB.Exec("DELETE FROM docs_fts WHERE doc_id = ?", docID)
	return err
}

// DeleteFTSByFile removes all FTS5 entries for a given file path.
func (s *Store) DeleteFTSByFile(arquivo string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	_, err := s.DB.Exec("DELETE FROM docs_fts WHERE arquivo = ?", arquivo)
	return err
}

// SearchFTS performs a full-text search using FTS5.
// Supports operators: +term (mandatory), -term (exclude), "exact phrase", term* (prefix).
// Pass an empty query or "*" to return all documents.
func (s *Store) SearchFTS(query string, from, size int) ([]FTSResult, int, error) {
	if query == "" || query == "*" {
		query = ""
	}

	// Count total
	var total int
	var countErr error
	if query == "" {
		countErr = s.DB.QueryRow("SELECT COUNT(*) FROM docs_fts WHERE tags NOT LIKE '%drawing%'").Scan(&total)
	} else {
		countErr = s.DB.QueryRow(
			"SELECT COUNT(*) FROM docs_fts WHERE docs_fts MATCH ? AND tags NOT LIKE '%drawing%'",
			query,
		).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, fmt.Errorf("fts count: %w", countErr)
	}

	var rows *sql.Rows
	var err error
	if query == "" {
		rows, err = s.DB.Query(`
			SELECT doc_id, tipo, arquivo, secao, texto, tags, 0.0 as rank, '' as snippet_text
			FROM docs_fts
			WHERE tags NOT LIKE '%drawing%'
			ORDER BY rowid DESC
			LIMIT ? OFFSET ?`, size, from)
	} else {
		rows, err = s.DB.Query(`
			SELECT doc_id, tipo, arquivo, secao, texto, tags, rank, snippet(docs_fts, -1, '<b>', '</b>', '...', 64) as snippet_text
			FROM docs_fts
			WHERE docs_fts MATCH ? AND tags NOT LIKE '%drawing%'
			ORDER BY rank
			LIMIT ? OFFSET ?`, query, size, from)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var results []FTSResult
	for rows.Next() {
		var r FTSResult
		if err := rows.Scan(&r.DocID, &r.Tipo, &r.Arquivo, &r.Secao, &r.Texto, &r.Tags, &r.Rank, &r.Snippet); err != nil {
			return nil, 0, err
		}
		results = append(results, r)
	}

	return results, total, rows.Err()
}

// SearchFTSLike is a fallback search using LIKE for fuzzy/wildcard patterns
// that FTS5 does not handle natively (e.g., mid-word substrings).
func (s *Store) SearchFTSLike(term string, from, size int) ([]FTSResult, int, error) {
	pattern := "%" + strings.ToLower(term) + "%"

	var total int
	s.DB.QueryRow(`
		SELECT COUNT(*) FROM documents
		WHERE (LOWER(texto) LIKE ? OR LOWER(secao) LIKE ? OR LOWER(arquivo) LIKE ?) AND tags NOT LIKE '%drawing%'`,
		pattern, pattern, pattern,
	).Scan(&total)

	rows, err := s.DB.Query(`
		SELECT id, tipo, arquivo, secao, texto, tags, 0.0 as rank, '' as snippet_text
		FROM documents
		WHERE (LOWER(texto) LIKE ? OR LOWER(secao) LIKE ? OR LOWER(arquivo) LIKE ?) AND tags NOT LIKE '%drawing%'
		LIMIT ? OFFSET ?`,
		pattern, pattern, pattern, size, from)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []FTSResult
	for rows.Next() {
		var r FTSResult
		if err := rows.Scan(&r.DocID, &r.Tipo, &r.Arquivo, &r.Secao, &r.Texto, &r.Tags, &r.Rank, &r.Snippet); err != nil {
			return nil, 0, err
		}
		results = append(results, r)
	}
	return results, total, rows.Err()
}
