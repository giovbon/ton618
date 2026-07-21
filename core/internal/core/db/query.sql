-- name: GetNote :one
SELECT content FROM notes WHERE filename = ? LIMIT 1;

-- name: SaveNote :exec
INSERT OR REPLACE INTO notes (filename, content, mtime) VALUES (?, ?, ?);

-- name: DeleteNote :exec
DELETE FROM notes WHERE filename = ?;

-- name: RenameNote :exec
UPDATE notes SET filename = ? WHERE filename = ?;

-- name: GetAllNotes :many
SELECT filename, mtime FROM notes ORDER BY mtime DESC;

-- name: CountNotes :one
SELECT COUNT(*) FROM notes;

-- name: GetAllNotesPaginated :many
SELECT filename, mtime FROM notes ORDER BY mtime DESC LIMIT ? OFFSET ?;

-- name: GetNoteMtime :one
SELECT mtime FROM notes WHERE filename = ? LIMIT 1;

-- name: NoteExists :one
SELECT COUNT(*) FROM notes WHERE filename = ?;

-- name: GetNotesNeedingMarkmapTag :many
SELECT n.filename
FROM notes n
WHERE (
    n.content LIKE '%markmap%' OR n.content LIKE '%mindmap%'
    OR n.content LIKE '%drawing%' OR n.content LIKE '%desenho%'
    OR n.content LIKE '%spreadsheet%' OR n.content LIKE '%planilha%'
    OR n.content LIKE '%typst%'
    OR n.content LIKE '%mermaid%'
    OR n.content LIKE '%mapa%' OR n.content LIKE '%/map%'
    OR n.filename LIKE '%mindmap%' OR n.filename LIKE '%markmap%'
)
  AND n.filename NOT IN (
      SELECT arquivo FROM tags WHERE tag IN ('markmap', 'mindmap', 'drawing', 'spreadsheet', 'typst', 'mermaid', 'map', 'mapa')
  );

-- name: GetAllNotesContent :many
SELECT filename, content FROM notes;

-- name: DeleteFileTags :exec
DELETE FROM tags WHERE arquivo = ?;

-- name: AddTagToFile :exec
INSERT OR IGNORE INTO tags (arquivo, tag) VALUES (?, ?);

-- name: GetFileTags :many
SELECT tag FROM tags WHERE arquivo = ? ORDER BY tag;

-- name: GetAllFileTags :many
SELECT arquivo, tag FROM tags ORDER BY arquivo, tag;

-- name: GetAllTags :many
SELECT DISTINCT tag FROM tags ORDER BY tag;

-- name: GetFilesByTag :many
SELECT arquivo FROM tags WHERE tag = ? ORDER BY arquivo;

-- name: RemoveTagFromFile :exec
DELETE FROM tags WHERE arquivo = ? AND tag = ?;

-- name: AddLink :exec
INSERT OR IGNORE INTO links (from_file, to_file) VALUES (?, ?);

-- name: RemoveLink :exec
DELETE FROM links WHERE from_file = ? AND to_file = ?;

-- name: GetLinks :many
SELECT to_file FROM links WHERE from_file = ? ORDER BY to_file;

-- name: GetLinkCount :one
SELECT COUNT(*) FROM links WHERE from_file = ?;

-- name: GetBacklinks :many
SELECT from_file FROM links WHERE to_file = ? ORDER BY from_file;

-- name: GetBacklinkCount :one
SELECT COUNT(*) FROM links WHERE to_file = ?;

-- name: GetAllLinks :many
SELECT from_file, to_file FROM links ORDER BY from_file, to_file;

-- name: ClearLinks :exec
DELETE FROM links WHERE from_file = ?;

-- name: GetLinksByFiles :many
SELECT DISTINCT to_file FROM links WHERE from_file IN (sqlc.slice('from_files')) ORDER BY to_file;

-- name: CreateAppointment :exec
INSERT INTO appointments (id, description, event_date, year, month, week_number, created_at) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateAppointment :exec
UPDATE appointments SET description = ?, event_date = ?, year = ?, month = ?, week_number = ? WHERE id = ?;

-- name: DeleteAppointment :exec
DELETE FROM appointments WHERE id = ?;

-- name: DeleteOldAppointments :exec
DELETE FROM appointments WHERE event_date < ?;

-- name: GetAppointments :many
SELECT id, description, event_date, year, month, week_number, created_at FROM appointments ORDER BY year ASC, month ASC, week_number ASC, event_date ASC;

-- name: DeleteTodosByFile :exec
DELETE FROM todos WHERE file = ?;

-- name: CreateTodo :exec
INSERT INTO todos (id, file, section, type, status, text, line, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetPopularity :one
SELECT count FROM popularity WHERE arquivo = ?;

-- name: GetAllPopularity :many
SELECT arquivo, count FROM popularity;

-- name: ResetPopularity :exec
DELETE FROM popularity WHERE arquivo = ?;

-- name: ApplyInteractionReward :exec
INSERT INTO popularity (arquivo, count, weight, last_interacted_at) 
VALUES (?, 1, 1.0 + CAST(sqlc.arg(reward) AS REAL), sqlc.arg(last_interacted_at))
ON CONFLICT(arquivo) DO UPDATE SET 
	count = count + 1,
	weight = MAX(0.1, weight + CAST(excluded.weight AS REAL) - 1.0), 
	last_interacted_at = excluded.last_interacted_at;

-- name: GetSynapticWeight :one
SELECT COALESCE(weight, 1.0) AS weight, COALESCE(last_interacted_at, '') AS last_interacted_at FROM popularity WHERE arquivo = ? LIMIT 1;

-- name: InsertDocument :exec
INSERT OR REPLACE INTO documents
(id, tipo, arquivo, secao, texto, tags, pagina, ordem, timestamp, created_at, hash)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteDocument :exec
DELETE FROM documents WHERE id = ?;

-- name: DeleteDocumentsByFile :exec
DELETE FROM documents WHERE arquivo = ?;

-- name: GetDocument :one
SELECT id, tipo, arquivo, secao, texto, tags, pagina, ordem, timestamp, created_at, hash FROM documents WHERE id = ?;

-- name: GetDocumentsByFile :many
SELECT id, tipo, arquivo, secao, texto, tags, pagina, ordem, timestamp, created_at, hash FROM documents WHERE arquivo = ? ORDER BY ordem ASC;

-- name: GetAllDocumentsByFile :many
SELECT id, tipo, arquivo, secao, texto, tags, pagina, ordem, timestamp, created_at, hash FROM documents ORDER BY arquivo, ordem ASC;

-- name: GetAllDocuments :many
SELECT id, tipo, arquivo, secao, texto, tags, pagina, ordem, timestamp, created_at, hash FROM documents ORDER BY arquivo, ordem ASC;

-- name: CountDocumentsWithoutDrawing :one
SELECT COUNT(*) FROM documents WHERE tags NOT LIKE '%drawing%';

-- name: GetDocumentsPaginated :many
SELECT id, tipo, arquivo, secao, texto, tags, pagina, ordem, timestamp, created_at, hash FROM documents WHERE tags NOT LIKE '%drawing%' ORDER BY arquivo, ordem ASC LIMIT ? OFFSET ?;

-- name: GetDocumentCount :one
SELECT COUNT(*) FROM documents;

-- name: GetDistinctFiles :many
SELECT DISTINCT arquivo FROM documents ORDER BY arquivo;

-- name: SearchDocumentText :one
SELECT COUNT(*) FROM documents WHERE texto LIKE ?;

-- name: GetFileMod :one
SELECT mtime FROM file_mods WHERE arquivo = ?;

-- name: SetFileMod :exec
INSERT OR REPLACE INTO file_mods (arquivo, mtime) VALUES (?, ?);

-- name: DeleteFileMod :exec
DELETE FROM file_mods WHERE arquivo = ?;

-- name: GetAllFileMods :many
SELECT arquivo, mtime FROM file_mods;

-- name: GetFilesModsAndTags :many
SELECT f.arquivo, f.mtime, CAST(IFNULL(GROUP_CONCAT(t.tag, ','), '') AS TEXT) as tags
FROM file_mods f
LEFT JOIN tags t ON f.arquivo = t.arquivo
GROUP BY f.arquivo, f.mtime;

-- name: GetSetting :one
SELECT value FROM settings WHERE key = ?;

-- name: SetSetting :exec
INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value;

-- name: HasNotificationBeenSent :one
SELECT COUNT(*) FROM notifications_log WHERE id = ?;

-- name: RecordNotificationSent :exec
INSERT INTO notifications_log (id, type, sent_at) VALUES (?, ?, ?);

-- name: HasNoteEmbedding :one
SELECT COUNT(*) FROM note_chunks WHERE filename = ?;

-- name: GetEmbeddedFiles :many
SELECT DISTINCT filename FROM note_chunks;

-- name: CountEmbeddableNotes :one
SELECT COUNT(*) FROM notes n
WHERE n.content != ''
  AND n.filename NOT LIKE 'pdfs/%'
  AND n.filename NOT LIKE 'attachments/%'
  AND n.filename NOT LIKE 'archives/%'
  AND n.filename NOT LIKE '%mapa-%'
  AND n.filename NOT LIKE '%mapa.%'
  AND n.filename NOT LIKE '%/map'
  AND NOT EXISTS (
    SELECT 1 FROM tags t
    WHERE t.arquivo = n.filename
      AND t.tag IN ('drawing','spreadsheet','mermaid','map','mapa')
  );

-- name: CountIndexedNotes :one
SELECT COUNT(DISTINCT nc.filename)
FROM note_chunks nc
WHERE EXISTS (
  SELECT 1 FROM notes n WHERE n.filename = nc.filename
);

-- name: CountStaleNotes :one
SELECT COUNT(DISTINCT nc.filename)
FROM note_chunks nc
JOIN notes n ON n.filename = nc.filename
WHERE n.mtime != nc.indexed_mtime;

-- name: GetPendingEmbeddingNotes :many
SELECT n.filename, n.content
FROM notes n
WHERE (
  n.filename NOT IN (SELECT DISTINCT filename FROM note_chunks)
  OR EXISTS (
    SELECT 1 FROM note_chunks nc
    WHERE nc.filename = n.filename AND nc.indexed_mtime != n.mtime
  )
)
  AND n.content != ''
  AND n.filename NOT LIKE 'pdfs/%'
  AND n.filename NOT LIKE 'attachments/%'
  AND n.filename NOT LIKE 'archives/%'
  AND n.filename NOT LIKE '%mapa-%'
  AND n.filename NOT LIKE '%mapa.%'
  AND n.filename NOT LIKE '%/map'
  AND NOT EXISTS (
    SELECT 1 FROM tags t
    WHERE t.arquivo = n.filename
      AND t.tag IN ('drawing','spreadsheet','mermaid','map','mapa')
  )
ORDER BY n.mtime DESC
LIMIT ?;

-- name: DeleteNoteChunks :exec
DELETE FROM note_chunks WHERE filename = ?;

-- name: InsertNoteChunk :exec
INSERT INTO note_chunks(chunk_id, filename, chunk_index, content, indexed_mtime) VALUES (?, ?, ?, ?, ?);
