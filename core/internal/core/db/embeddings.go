package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"

	dbgen "ton618/core/internal/core/db/generated"
	"ton618/core/internal/core/domain"
)

// EmbeddingDim e a dimensao do vetor produzido pelo modelo e5-small (384 dims).
const EmbeddingDim = 384

// EmbeddingModelName é o modelo de embeddings usado pelo frontend. Deve permanecer
// em sincronia com o MODEL_NAME do semantic-worker.js (fonte única no backend).
// ATENÇÃO: o e5-small foi testado e REVERTIDO em 09/08/2026 — produz embeddings
// colapsados (cosine ~0.85 para textos não relacionados) no Transformers.js v4 + q8.
const EmbeddingModelName = "Xenova/paraphrase-multilingual-MiniLM-L12-v2"

// Parâmetros de chunking (devem refletir web/src/semantic.js).
// 700/100 desde 10/08/2026: chunks menores melhoram a precisão da busca
// semântica (notas longas não viram um vetor médio ruidoso).
const chunkMaxChars = 700
const chunkOverlapChars = 100

// EmbeddingModelVersion é um fingerprint determinístico do pipeline de embeddings
// (modelo + chunking + prefixos). Quando qualquer parâmetro muda, o fingerprint
// muda automaticamente e os embeddings antigos são invalidados no próximo boot —
// sem bump manual.
var EmbeddingModelVersion = computeEmbeddingVersion()

func computeEmbeddingVersion() string {
	payload := EmbeddingModelName +
		"|chunk:" + strconv.Itoa(chunkMaxChars) + "/" + strconv.Itoa(chunkOverlapChars)
	sum := sha256.Sum256([]byte(payload))
	return "minilm-" + hex.EncodeToString(sum[:6])
}

// EmbeddingModelVersionKey é a chave em settings que guarda a versão corrente do modelo.
const EmbeddingModelVersionKey = "embedding_model_version"

// SimilarResult representa um resultado de busca semantica por proximidade vetorial.
type SimilarResult struct {
	Filename     string
	Distance     float64
	NoteType     domain.NoteType
	ChunkMatches int // chunks dentro do corte (voto majoritário)
	TotalChunks  int // total de chunks indexados da nota
}

// EmbeddingStatus contem o status de indexacao semantica.
type EmbeddingStatus struct {
	TotalNotes   int `json:"total_notes"`
	IndexedNotes int `json:"indexed_notes"`
	PendingNotes int `json:"pending_notes"`
	StaleNotes   int `json:"stale_notes"`
	EmbeddingDim int `json:"embedding_dim"`
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
	ChunkID    string    `json:"chunk_id"`
	Filename   string    `json:"filename"`
	ChunkIndex int       `json:"chunk_index"`
	Content    string    `json:"content"`
	Embedding  []float32 `json:"embedding"`
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

	qtx := s.Q.WithTx(tx)

	// 0. Verifica se a nota ainda existe no banco (pode ter sido deletada
	//    enquanto o browser gerava os embeddings — race condition).
	//    Se não existir, aborta silenciosamente para evitar chunks órfãos.
	var indexedMtime string
	if err := tx.QueryRow(`SELECT mtime FROM notes WHERE filename = ?`, filename).Scan(&indexedMtime); err != nil {
		// Nota não encontrada: aborta. O DeleteAllFileRecords/DeleteNote
		// já limpou os chunks/embeddings, e o Rollback desfaz qualquer
		// writtenção parcial.
		return nil
	}

	// 1. Remove chunks antigos do filename
	if err := qtx.DeleteNoteChunks(s.queryCtx(), filename); err != nil {
		return fmt.Errorf("delete old chunks: %w", err)
	}

	// 2. Remove embeddings antigos (chunk_ids do filename)
	if _, err := tx.Exec(`DELETE FROM note_embeddings WHERE chunk_id LIKE ?`, filename+`#%`); err != nil {
		return fmt.Errorf("delete old embeddings: %w", err)
	}

	// 3. Insere novos chunks e embeddings
	for _, ch := range chunks {
		if err := qtx.InsertNoteChunk(s.queryCtx(), dbgen.InsertNoteChunkParams{
			ChunkID:      ch.ChunkID,
			Filename:     ch.Filename,
			ChunkIndex:   int64(ch.ChunkIndex),
			Content:      ch.Content,
			IndexedMtime: sql.NullString{String: indexedMtime, Valid: true},
		}); err != nil {
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

// EnsureEmbeddingModelVersion verifica se a versão do modelo persistida em settings
// corresponde à esperada. Se não corresponder, limpa todos os embeddings/chunks
// (forçando re-indexação completa pelo browser) e grava a nova versão.
// Retorna true se houve reset. Usado para invalidar embeddings ao trocar de modelo.
func (s *Store) EnsureEmbeddingModelVersion(version string) (bool, error) {
	current, err := s.GetSetting(EmbeddingModelVersionKey)
	if err != nil {
		return false, err
	}
	if current == version {
		return false, nil
	}
	if err := s.ResetAllEmbeddings(); err != nil {
		return false, err
	}
	if err := s.SetSetting(EmbeddingModelVersionKey, version); err != nil {
		return false, err
	}
	return true, nil
}

// ResetAllEmbeddings apaga todos os registros de chunks e embeddings
// das tabelas note_chunks e note_embeddings. Usado para reset completo
// do índice semântico (ex: pela aba Semântica das Configurações).
func (s *Store) ResetAllEmbeddings() error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	if _, err := s.DB.Exec("DELETE FROM note_chunks"); err != nil {
		return fmt.Errorf("delete note_chunks: %w", err)
	}
	if _, err := s.DB.Exec("DELETE FROM note_embeddings"); err != nil {
		return fmt.Errorf("delete note_embeddings: %w", err)
	}
	return nil
}

// DeleteEmbedding remove todos os embeddings e chunks de uma nota.
func (s *Store) DeleteEmbedding(filename string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	if err := s.Q.DeleteNoteChunks(s.queryCtx(), filename); err != nil {
		return err
	}
	_, err := s.DB.Exec(`DELETE FROM note_embeddings WHERE chunk_id LIKE ?`, filename+`#%`)
	return err
}

// HasEmbedding verifica se uma nota ja possui embedding indexado (qualquer chunk).
func (s *Store) HasEmbedding(filename string) bool {
	count, err := s.Q.HasNoteEmbedding(s.queryCtx(), filename)
	if err != nil {
		return false
	}
	return count > 0
}

// knnCandidateMultiplier amplia os candidatos do KNN. Antes, uma única nota com
// mais chunks que `limit*5` ocupava todos os candidatos e "engolia" o corpus
// (SearchSimilar retornava 1 resultado). Com o multiplicador maior + agregação
// em Go varrendo TODOS os candidatos, cada nota contribui com 1 resultado.
const knnCandidateMultiplier = 50

// SearchSimilar realiza busca KNN nos chunks via sqlite-vec e agrega por filename.
// Retorna os `limit` documentos mais proximos, deduplicando por filename
// (a menor distância entre chunks de um mesmo filename é a distância da nota).
func (s *Store) SearchSimilar(queryEmbedding []float32, limit int) ([]SimilarResult, error) {
	return s.SearchSimilarCtx(s.queryCtx(), queryEmbedding, limit)
}

// SearchSimilarCtx é como SearchSimilar, mas respeita o contexto fornecido
// (cancelamento/timeout do request — ex: busca híbrida).
func (s *Store) SearchSimilarCtx(ctx context.Context, queryEmbedding []float32, limit int) ([]SimilarResult, error) {
	// Sem corte de distância: todos os chunks do janela contam como match.
	return s.SearchSimilarWithConsensus(ctx, queryEmbedding, limit, math.MaxFloat64)
}

// SearchSimilarWithConsensus é como SearchSimilarCtx, mas retorna, por filename,
// quantos chunks estão dentro do corte (maxDist) e o total de chunks indexados —
// os dados do voto majoritário usado pela busca híbrida para candidatos
// só-semânticos (nota longa com 1 match acidental é rejeitada no handler).
func (s *Store) SearchSimilarWithConsensus(ctx context.Context, queryEmbedding []float32, limit int, maxDist float64) ([]SimilarResult, error) {
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

	// sqlite-vec KNN query: busca os `k` chunks mais próximos (k ampliado) e faz
	// JOIN com note_chunks para obter filename. A deduplicação por filename
	// acontece em Go (primeira ocorrência = menor distância, já que a lista vem
	// ordenada por distância) — varrendo todos os candidatos, uma nota gigante
	// não monopoliza o resultado.
	k := limit * knnCandidateMultiplier
	rows, err := s.DB.QueryContext(ctx, `
		SELECT nc.filename, ne.distance
		FROM note_embeddings ne
		JOIN note_chunks nc ON nc.chunk_id = ne.chunk_id
		WHERE ne.embedding MATCH ?
		  AND ne.k = ?
		ORDER BY ne.distance ASC
	`, blob, k)
	if err != nil {
		return nil, fmt.Errorf("sqlite-vec search: %w", err)
	}
	defer rows.Close()

	// Obtém todas as tags para filtrar por tipo indexável
	allTags, tagsErr := s.GetAllFileTags()
	if tagsErr != nil {
		slog.Warn("SearchSimilarWithConsensus: erro ao obter tags para filtro", "error", tagsErr)
		allTags = make(map[string][]string)
	}

	// Primeira passada: menor distância + contagem de chunks dentro do corte,
	// por filename (a lista vem ordenada por distância).
	type acc struct {
		res     SimilarResult
		matches int
	}
	first := make(map[string]*acc)
	for rows.Next() {
		var filename string
		var distance float64
		if err := rows.Scan(&filename, &distance); err != nil {
			slog.Warn("SearchSimilarWithConsensus: erro ao fazer scan de resultado", "error", err)
			continue
		}
		a, ok := first[filename]
		if !ok {
			// Filtra por tipo embeddable (pode ter mudado desde a indexação)
			fileTags := allTags[filename]
			if !s.isNoteEmbeddable(filename, fileTags) {
				continue
			}
			a = &acc{res: SimilarResult{
				Filename: filename,
				Distance: distance,
				NoteType: domain.DetectNoteType(fileTags, filename),
			}}
			first[filename] = a
		}
		if distance <= maxDist {
			a.matches++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Ordena por distância e trunca em `limit`.
	sorted := make([]SimilarResult, 0, len(first))
	for _, a := range first {
		sorted = append(sorted, a.res)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Distance < sorted[j].Distance })
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	// Total de chunks por candidato (voto majoritário distingue nota curta de
	// nota longa com 1 match acidental).
	names := make([]string, len(sorted))
	for i, r := range sorted {
		names[i] = r.Filename
	}
	totals, err := s.countChunksByFile(ctx, names)
	if err != nil {
		slog.Warn("SearchSimilarWithConsensus: erro ao contar chunks", "error", err)
		totals = make(map[string]int)
	}

	for i := range sorted {
		a := first[sorted[i].Filename]
		sorted[i].ChunkMatches = a.matches
		sorted[i].TotalChunks = totals[sorted[i].Filename]
	}
	return sorted, nil
}

// countChunksByFile retorna o total de chunks indexados por filename (uma query).
func (s *Store) countChunksByFile(ctx context.Context, filenames []string) (map[string]int, error) {
	result := make(map[string]int, len(filenames))
	if len(filenames) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(filenames))
	args := make([]any, len(filenames))
	for i, name := range filenames {
		placeholders[i] = "?"
		args[i] = name
	}

	query := "SELECT filename, COUNT(*) FROM note_chunks WHERE filename IN (" + strings.Join(placeholders, ",") + ") GROUP BY filename"
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return result, err
		}
		result[name] = count
	}
	return result, rows.Err()
}

// isNoteEmbeddable determina se uma nota deve ser indexada para busca semântica com base no seu tipo.
// Deve permanecer em paridade com as queries SQL (CountEmbeddableNotes / GetPendingEmbeddingNotes).
func (s *Store) isNoteEmbeddable(filename string, tags []string) bool {
	// Notas marcadas para exclusão (tag "deletar") não são indexadas — paridade com o SQL.
	for _, t := range tags {
		if strings.ToLower(strings.TrimSpace(t)) == "deletar" {
			return false
		}
	}

	noteType := domain.DetectNoteType(tags, filename)
	return noteType == domain.NoteTypeMarkdown ||
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
// Usa SQL para contagem eficiente em vez de carregar todas as notas em memória.
func (s *Store) GetEmbeddingStatus() (EmbeddingStatus, error) {
	var status EmbeddingStatus
	status.EmbeddingDim = EmbeddingDim

	// 1. Conta total de notas indexáveis
	total, err := s.Q.CountEmbeddableNotes(s.queryCtx())
	if err != nil {
		return status, err
	}
	status.TotalNotes = int(total)

	// 2. Conta notas que possuem pelo menos um chunk indexado
	indexed, err := s.Q.CountIndexedNotes(s.queryCtx())
	if err != nil {
		return status, err
	}
	status.IndexedNotes = int(indexed)

	// 3. Calcula pendentes
	status.PendingNotes = status.TotalNotes - status.IndexedNotes
	if status.PendingNotes < 0 {
		status.PendingNotes = 0
	}

	// 4. Conta notas com chunks desatualizados
	stale, err := s.Q.CountStaleNotes(s.queryCtx())
	if err != nil {
		status.StaleNotes = 0
	} else {
		status.StaleNotes = int(stale)
	}

	// 5. Adiciona stale notes aos pendentes
	status.PendingNotes += status.StaleNotes

	return status, nil
}

// PendingNote representa uma nota que ainda nao possui embedding indexado.
type PendingNote struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// GetPendingEmbeddingNotes retorna notas sem chunks indexados, em batches de `limit`.
// Usa SQL para filtrar notas não-indexáveis e aplica isNoteEmbeddable para garantia total.
func (s *Store) GetPendingEmbeddingNotes(limit int) ([]PendingNote, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.Q.GetPendingEmbeddingNotes(s.queryCtx(), int64(limit))
	if err != nil {
		return nil, err
	}

	allTags, _ := s.GetAllFileTags()

	result := make([]PendingNote, 0, len(rows))
	for _, r := range rows {
		fileTags := allTags[r.Filename]
		if !s.isNoteEmbeddable(r.Filename, fileTags) {
			continue
		}

		content := ""
		if r.Content.Valid {
			content = r.Content.String
		}
		result = append(result, PendingNote{
			Filename: r.Filename,
			Content:  content,
		})
	}
	return result, nil
}

// GetEmbeddedFiles retorna o conjunto de nomes de arquivo que possuem chunks indexados.
func (s *Store) GetEmbeddedFiles() (map[string]bool, error) {
	filenames, err := s.Q.GetEmbeddedFiles(s.queryCtx())
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool, len(filenames))
	for _, fname := range filenames {
		result[fname] = true
	}
	return result, nil
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
