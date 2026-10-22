package db

import (
	"context"
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

// ftsCountCap limita o COUNT(*) de resultados de busca.
//
// Sem teto, o FTS5 precisa visitar TODOS os matches só para contá-los (mesmo
// custo da página com ORDER BY rank) — medido: ~28ms por busca num termo que
// casa 50% de 21k docs. Com o teto, a contagem é exata para bases pequenas e
// para cedo em bases grandes. A UI exibe "~total" e o sentinela de paginação
// (from+size < total) segue correto até o teto.
const ftsCountCap = 1000

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
	// Coluna note_type (migração v24): derivada das tags persistidas + caminho
	// — mesma regra de DeriveNoteType, sem mudar a assinatura (os ~30 call
	// sites não se importam com o valor; para notas o processo principal é o
	// ReplaceFileIndexes, que recebe o valor content-aware do processor).
	noteType := DeriveNoteType(tags, arquivo)
	_, err := s.DB.Exec(`
		INSERT INTO docs_fts (doc_id, tipo, arquivo, secao, texto, tags, note_type)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		docID, tipo, arquivo, secao, texto, tags, noteType,
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

// noteTypeExclusions monta o trecho WHERE de exclusões das buscas, considerando
// o filtro por tipo (`tipo:` — coluna note_type, migração v24). Sem filtro, a
// listagem geral esconde desenhos/PDFs/anexos (não têm texto útil); com filtro
// explícito essas exclusões mascarariam os resultados (ex: `tipo:pdf` jamais
// casaria), então só permanece a de `deletar`. Os placeholders de note_type
// gerados aqui recebem os valores via noteTypeArgs.
func noteTypeExclusions(noteTypes []string) string {
	const deletarOnly = "tags NOT LIKE '%deletar%'"
	if len(noteTypes) == 0 {
		return deletarOnly + " AND tags NOT LIKE '%drawing%' AND arquivo NOT LIKE 'pdfs/%' AND arquivo NOT LIKE '%.pdf' AND arquivo NOT LIKE 'attachments/%'"
	}
	placeholders := make([]string, len(noteTypes))
	for i := range noteTypes {
		placeholders[i] = "note_type = ?"
	}
	return deletarOnly + " AND (" + strings.Join(placeholders, " OR ") + ")"
}

// noteTypeArgs concatena os argumentos SQL de uma query com filtro por tipo:
// prefix (args antes dos placeholders de note_type) + valores do filtro +
// tail (LIMIT/OFFSET etc).
func noteTypeArgs(prefix []interface{}, noteTypes []string, tail ...interface{}) []interface{} {
	args := make([]interface{}, 0, len(prefix)+len(noteTypes)+len(tail))
	args = append(args, prefix...)
	for _, nt := range noteTypes {
		args = append(args, nt)
	}
	args = append(args, tail...)
	return args
}

// SearchFTS performs a full-text search using FTS5.
// Supports operators: +term (mandatory), -term (exclude), "exact phrase", term* (prefix).
// Pass an empty query or "*" to return all documents.
func (s *Store) SearchFTS(query string, from, size int) ([]FTSResult, int, error) {
	return s.SearchFTSWithContext(s.queryCtx(), query, from, size, nil)
}

// SearchFTSWithContext é como SearchFTS, mas respeita o contexto (timeout/cancelamento).
//
// noteTypes restringe a busca aos documentos daqueles tipos (vocabulário do
// campo `tipo:`: mapa_mental, anexo, nota, ...). Com filtro de tipo ativo as
// exclusões duras de listagem geral (desenhos, PDFs, anexos) são relaxadas —
// senão `tipo:pdf` nunca casaria — mantendo apenas a exclusão de `deletar`.
// Os valores são aplicados também em SQL, como segunda linha de defesa além do
// predicado note_type:"x" já embutido na query MATCH pelo search.
func (s *Store) SearchFTSWithContext(ctx context.Context, query string, from, size int, noteTypes []string) ([]FTSResult, int, error) {
	if query == "" || query == "*" {
		query = ""
	}

	exclusions := noteTypeExclusions(noteTypes)

	// Count total (limitado por ftsCountCap — ver constante). Conta ARQUIVOS
	// distintos (notas), não fragmentos: um card por nota nos resultados, então
	// "X resultados" tem que contar cards — uma nota Tabulator com 30 linhas
	// que casando é 1 resultado, não 30.
	var total int
	var countErr error
	if query == "" {
		countErr = s.DB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM (SELECT DISTINCT arquivo FROM docs_fts WHERE "+exclusions+" LIMIT ?)",
			noteTypeArgs(nil, noteTypes, ftsCountCap)...,
		).Scan(&total)
	} else {
		countErr = s.DB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM (SELECT DISTINCT arquivo FROM docs_fts WHERE docs_fts MATCH ? AND "+exclusions+" LIMIT ?)",
			noteTypeArgs([]interface{}{query}, noteTypes, ftsCountCap)...,
		).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, fmt.Errorf("fts count: %w", countErr)
	}

	var rows *sql.Rows
	var err error
	if query == "" {
		rows, err = s.DB.QueryContext(ctx, `
			SELECT doc_id, tipo, arquivo, secao, texto, tags, 0.0 as rank, '' as snippet_text
			FROM docs_fts
			WHERE `+exclusions+`
			ORDER BY rowid DESC
			LIMIT ? OFFSET ?`, noteTypeArgs(nil, noteTypes, size, from)...)
	} else {
		rows, err = s.DB.QueryContext(ctx, `
			SELECT doc_id, tipo, arquivo, secao, texto, tags, rank, snippet(docs_fts, 4, '__HL_START__', '__HL_END__', '...', 64) as snippet_text
			FROM docs_fts
			WHERE docs_fts MATCH ? AND `+exclusions+`
			ORDER BY rank
			LIMIT ? OFFSET ?`, noteTypeArgs([]interface{}{query}, noteTypes, size, from)...)
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
	return s.SearchFTSLikeWithContext(s.queryCtx(), term, from, size, nil)
}

// SearchFTSLikeWithContext é como SearchFTSLike, mas respeita o contexto.
// noteTypes tem o mesmo papel de SearchFTSWithContext (filtro `tipo:`).
func (s *Store) SearchFTSLikeWithContext(ctx context.Context, term string, from, size int, noteTypes []string) ([]FTSResult, int, error) {
	pattern := "%" + strings.ToLower(term) + "%"
	exclusions := noteTypeExclusions(noteTypes)

	// Mesmo critério do FTS: conta notas (arquivos distintos), não fragmentos.
	var total int
	s.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT DISTINCT arquivo FROM documents
			WHERE (LOWER(texto) LIKE ? OR LOWER(secao) LIKE ? OR LOWER(arquivo) LIKE ?) AND `+exclusions+`
			LIMIT ?
		)`,
		noteTypeArgs([]interface{}{pattern, pattern, pattern}, noteTypes, ftsCountCap)...,
	).Scan(&total)

	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, tipo, arquivo, secao, texto, tags, 0.0 as rank, '' as snippet_text
		FROM documents
		WHERE (LOWER(texto) LIKE ? OR LOWER(secao) LIKE ? OR LOWER(arquivo) LIKE ?) AND `+exclusions+`
		LIMIT ? OFFSET ?`,
		noteTypeArgs([]interface{}{pattern, pattern, pattern}, noteTypes, size, from)...)
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
