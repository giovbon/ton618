package db

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"

	"ton618/internal/core/domain"
)

// EmbeddingDim e a dimensao do vetor produzido pelo modelo multilingual-MiniLM-L12-v2.
const EmbeddingDim = 384

// SimilarResult representa um resultado de busca semantica por proximidade vetorial.
type SimilarResult struct {
	Filename string
	Distance float64
}

// EmbeddingStatus contem o status de indexacao semantica.
type EmbeddingStatus struct {
	TotalNotes   int `json:"total_notes"`
	IndexedNotes int `json:"indexed_notes"`
	PendingNotes int `json:"pending_notes"`
	StaleNotes   int `json:"stale_notes"`
}

// serializeEmbedding converte []float32 para []byte no formato little-endian float32,
// que e o formato esperado pelo sqlite-vec para colunas FLOAT[N].
// Retorna erro se o vetor contem valores NaN ou Inf.
func serializeEmbedding(v []float32) ([]byte, error) {
	for i, f := range v {
		if math.IsNaN(float64(f)) {
			return nil, fmt.Errorf("embedding contem NaN na posicao %d", i)
		}
		if math.IsInf(float64(f), 0) {
			return nil, fmt.Errorf("embedding contem Inf na posicao %d", i)
		}
	}
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf, nil
}

// SaveEmbedding persiste o embedding de um chunk individual na tabela note_embeddings.
func (s *Store) SaveEmbedding(chunkID string, embedding []float32) error {
	if len(embedding) != EmbeddingDim {
		return fmt.Errorf("embedding invalido: esperado %d dimensoes, recebido %d", EmbeddingDim, len(embedding))
	}

	blob, err := serializeEmbedding(embedding)
	if err != nil {
		return err
	}

	_, err = s.DB.Exec(
		`INSERT INTO note_embeddings(chunk_id, embedding) VALUES (?, ?)`,
		chunkID, blob,
	)
	return err
}

// ChunkInfo representa um chunk de nota para indexação semântica.
type ChunkInfo struct {
	ChunkID      string    `json:"chunk_id"`
	Filename     string    `json:"filename"`
	ChunkIndex   int       `json:"chunk_index"`
	Content      string    `json:"content"`
	Embedding    []float32 `json:"embedding"`
}

// SaveNoteChunks salva todos os chunks de uma nota em transação atômica.
// Remove chunks antigos do mesmo filename e insere os novos.
// Armazena o mtime da nota para detectar edições futuras.
func (s *Store) SaveNoteChunks(filename string, chunks []ChunkInfo) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	tx, err := s.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// 0. Obtém o mtime atual da nota para detectar alterações futuras
	var indexedMtime string
	if err := tx.QueryRow(`SELECT mtime FROM notes WHERE filename = ?`, filename).Scan(&indexedMtime); err != nil {
		indexedMtime = ""
	}

	// 1. Remove chunks antigos do filename
	if _, err := tx.Exec(`DELETE FROM note_chunks WHERE filename = ?`, filename); err != nil {
		return fmt.Errorf("delete old chunks: %w", err)
	}

	// 2. Remove embeddings antigos (chunk_ids do filename)
	if _, err := tx.Exec(`DELETE FROM note_embeddings WHERE chunk_id LIKE ?`, filename+`#%`); err != nil {
		return fmt.Errorf("delete old embeddings: %w", err)
	}

	// 3. Insere novos chunks e embeddings
	for _, ch := range chunks {
		if _, err := tx.Exec(
			`INSERT INTO note_chunks(chunk_id, filename, chunk_index, content, indexed_mtime) VALUES (?, ?, ?, ?, ?)`,
			ch.ChunkID, ch.Filename, ch.ChunkIndex, ch.Content, indexedMtime,
		); err != nil {
			return fmt.Errorf("insert chunk %s: %w", ch.ChunkID, err)
		}

		blob, err := serializeEmbedding(ch.Embedding)
		if err != nil {
			return fmt.Errorf("serialize chunk %s: %w", ch.ChunkID, err)
		}

		if _, err := tx.Exec(
			`INSERT INTO note_embeddings(chunk_id, embedding) VALUES (?, ?)`,
			ch.ChunkID, blob,
		); err != nil {
			return fmt.Errorf("insert embedding %s: %w", ch.ChunkID, err)
		}
	}

	return tx.Commit()
}

// DeleteEmbedding remove todos os embeddings e chunks de uma nota.
func (s *Store) DeleteEmbedding(filename string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	if _, err := s.DB.Exec(`DELETE FROM note_chunks WHERE filename = ?`, filename); err != nil {
		return err
	}
	_, err := s.DB.Exec(`DELETE FROM note_embeddings WHERE chunk_id LIKE ?`, filename+`#%`)
	return err
}

// HasEmbedding verifica se uma nota ja possui embedding indexado (qualquer chunk).
func (s *Store) HasEmbedding(filename string) bool {
	var count int
	s.DB.QueryRow(`SELECT COUNT(*) FROM note_chunks WHERE filename = ?`, filename).Scan(&count)
	return count > 0
}

// SearchSimilar realiza busca KNN nos chunks via sqlite-vec e agrega por filename.
// Retorna os `limit` documentos mais proximos, deduplicando por filename
// (a menor distância entre chunks de um mesmo filename é a distância da nota).
func (s *Store) SearchSimilar(queryEmbedding []float32, limit int) ([]SimilarResult, error) {
	if len(queryEmbedding) != EmbeddingDim {
		return nil, fmt.Errorf("embedding invalido: esperado %d dimensoes, recebido %d", EmbeddingDim, len(queryEmbedding))
	}
	if limit <= 0 {
		limit = 10
	}

	blob, err := serializeEmbedding(queryEmbedding)
	if err != nil {
		return nil, err
	}

	// sqlite-vec KNN query: busca nos chunks, faz JOIN com note_chunks para obter filename
	rows, err := s.DB.Query(`
		SELECT nc.filename, ne.distance
		FROM note_embeddings ne
		JOIN note_chunks nc ON nc.chunk_id = ne.chunk_id
		WHERE ne.embedding MATCH ?
		  AND ne.k = ?
		ORDER BY ne.distance ASC
	`, blob, limit*5)
	if err != nil {
		return nil, fmt.Errorf("sqlite-vec search: %w", err)
	}
	defer rows.Close()

	// Obtém todas as tags para filtrar por tipo indexável
	allTags, tagsErr := s.GetAllFileTags()
	if tagsErr != nil {
		slog.Warn("SearchSimilar: erro ao obter tags para filtro", "error", tagsErr)
		allTags = make(map[string][]string)
	}

	var results []SimilarResult
	seen := make(map[string]bool) // deduplica por filename
	for rows.Next() {
		var r SimilarResult
		if err := rows.Scan(&r.Filename, &r.Distance); err != nil {
			slog.Warn("SearchSimilar: erro ao fazer scan de resultado", "error", err)
			continue
		}
		// Deduplica: a primeira ocorrência de cada filename tem a menor distância
		if seen[r.Filename] {
			continue
		}
		seen[r.Filename] = true

		// Filtra por tipo embeddable (pode ter mudado desde a indexação)
		if !s.isNoteEmbeddable(r.Filename, allTags[r.Filename]) {
			continue
		}
		results = append(results, r)
		if len(results) >= limit {
			break
		}
	}
	return results, rows.Err()
}

// isNoteEmbeddable determina se uma nota deve ser indexada para busca semântica com base no seu tipo.
func (s *Store) isNoteEmbeddable(filename string, tags []string) bool {
	noteType := domain.DetectNoteType(tags, "", filename)
	return noteType == domain.NoteTypeMarkdown ||
		noteType == domain.NoteTypeTypst ||
		noteType == domain.NoteTypeMindmap ||
		noteType == domain.NoteTypeYoutube ||
		noteType == domain.NoteTypeArticle ||
		noteType == domain.NoteTypeCapture
}

// IsNoteEmbeddable é a versão pública de isNoteEmbeddable.
func (s *Store) IsNoteEmbeddable(filename string, tags []string) bool {
	return s.isNoteEmbeddable(filename, tags)
}

// GetEmbeddingStatus retorna quantas notas tem embedding vs. total de notas no banco.
func (s *Store) GetEmbeddingStatus() (EmbeddingStatus, error) {
	var status EmbeddingStatus

	// 1. Busca todas as tags do banco para definir tipos
	allTags, err := s.GetAllFileTags()
	if err != nil {
		return status, err
	}

	// 2. Busca todas as notas para contar o total indexável
	allNotes, err := s.GetAllNotes()
	if err != nil {
		return status, err
	}

	totalEmbeddable := 0
	for filename := range allNotes {
		tags := allTags[filename]
		if s.isNoteEmbeddable(filename, tags) {
			totalEmbeddable++
		}
	}
	status.TotalNotes = totalEmbeddable

	// 3. Busca a contagem de notas que possuem pelo menos um chunk indexado
	if err := s.DB.QueryRow(`SELECT COUNT(DISTINCT filename) FROM note_chunks`).Scan(&status.IndexedNotes); err != nil {
		return status, err
	}

	// 4. Calcula pendentes (assegura que não seja negativo)
	status.PendingNotes = status.TotalNotes - status.IndexedNotes
	if status.PendingNotes < 0 {
		status.PendingNotes = 0
	}

	// 5. Conta notas com chunks desatualizados (mtime mudou desde a última indexação)
	if err := s.DB.QueryRow(`
		SELECT COUNT(DISTINCT nc.filename)
		FROM note_chunks nc
		JOIN notes n ON n.filename = nc.filename
		WHERE n.mtime != nc.indexed_mtime
	`).Scan(&status.StaleNotes); err != nil {
		status.StaleNotes = 0
	}

	// 6. Adiciona notas desatualizadas (stale) aos pendentes, pois elas precisam ser reindexadas
	status.PendingNotes += status.StaleNotes

	return status, nil
}

// PendingNote representa uma nota que ainda nao possui embedding indexado.
type PendingNote struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// GetPendingEmbeddingNotes retorna notas sem chunks indexados, em batches de `limit`.
func (s *Store) GetPendingEmbeddingNotes(limit int) ([]PendingNote, error) {
	if limit <= 0 {
		limit = 20
	}

	// 1. Busca todas as tags
	allTags, err := s.GetAllFileTags()
	if err != nil {
		return nil, err
	}

	// 2. Busca candidatos pendentes (notas que não têm chunks ou que têm chunks desatualizados)
	rows, err := s.DB.Query(`
		SELECT filename, content
		FROM notes n
		WHERE (
			filename NOT IN (SELECT DISTINCT filename FROM note_chunks)
			OR EXISTS (
				SELECT 1 FROM note_chunks nc
				WHERE nc.filename = n.filename AND nc.indexed_mtime != n.mtime
			)
		)
		  AND content != ''
		ORDER BY mtime DESC
		LIMIT ?
	`, limit*10)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PendingNote
	for rows.Next() {
		var p PendingNote
		if err := rows.Scan(&p.Filename, &p.Content); err != nil {
			continue
		}

		tags := allTags[p.Filename]
		if s.isNoteEmbeddable(p.Filename, tags) {
			result = append(result, p)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, rows.Err()
}

// GetEmbeddedFiles retorna o conjunto de nomes de arquivo que possuem chunks indexados.
func (s *Store) GetEmbeddedFiles() (map[string]bool, error) {
	rows, err := s.DB.Query("SELECT DISTINCT filename FROM note_chunks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var fname string
		if err := rows.Scan(&fname); err != nil {
			continue
		}
		result[fname] = true
	}
	return result, rows.Err()
}

// GetNoteEmbeddings recupera e deserializa todos os vetores de embedding (float32) de uma nota.
func (s *Store) GetNoteEmbeddings(filename string) ([][]float32, error) {
	rows, err := s.DB.Query(`
		SELECT embedding
		FROM note_embeddings
		WHERE chunk_id LIKE ?
		ORDER BY chunk_id ASC
	`, filename+"#%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var embeddings [][]float32
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			return nil, err
		}
		if len(blob)%4 != 0 {
			continue
		}
		emb := make([]float32, len(blob)/4)
		for i := 0; i < len(emb); i++ {
			bits := binary.LittleEndian.Uint32(blob[i*4 : (i+1)*4])
			emb[i] = math.Float32frombits(bits)
		}
		embeddings = append(embeddings, emb)
	}
	return embeddings, rows.Err()
}
