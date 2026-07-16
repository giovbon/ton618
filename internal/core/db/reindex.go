package db

import (
	"context"
	"time"

	"ton618/internal/processor"
)

// ReplaceFileIndexes replaces all indexing data for a file in a single transaction.
// O parâmetro ctx permite cancelamento da operação (ex: cliente desconectou).
// Se ctx for nil, usa o contexto padrão com timeout.
func (s *Store) ReplaceFileIndexes(
	ctx context.Context,
	filename string,
	docs []processor.Document,
	links []string,
	tags []string,
	todos []processor.TodoItem,
	modTime time.Time,
) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	if ctx == nil {
		ctx = s.queryCtx()
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Verifica cancelamento antes de cada etapa crítica
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 1. Documents & FTS
	if _, err := tx.Exec("DELETE FROM documents WHERE arquivo = ?", filename); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM docs_fts WHERE arquivo = ?", filename); err != nil {
		return err
	}

	docStmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO documents 
		(id, tipo, arquivo, secao, texto, tags, pagina, ordem, timestamp, created_at, hash) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer docStmt.Close()

	ftsStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO docs_fts (doc_id, tipo, arquivo, secao, texto, tags, texto_stemmed) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer ftsStmt.Close()

	for _, doc := range docs {
		tagsStr := SliceToTags(doc.Tags)
		if _, err := docStmt.Exec(
			doc.ID, doc.Tipo, doc.Arquivo, doc.Secao, doc.Texto, tagsStr,
			doc.Pagina, doc.Ordem, doc.Timestamp, doc.Created, doc.Hash,
		); err != nil {
			return err
		}
		stemmedText := processor.StemText(doc.Texto)
		if _, err := ftsStmt.Exec(
			doc.ID, doc.Tipo, doc.Arquivo, doc.Secao, doc.Texto, tagsStr, stemmedText,
		); err != nil {
			return err
		}
	}

	// 2. Links
	if _, err := tx.Exec("DELETE FROM links WHERE from_file = ?", filename); err != nil {
		return err
	}
	if len(links) > 0 {
		linkStmt, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO links (from_file, to_file) VALUES (?, ?)")
		if err != nil {
			return err
		}
		defer linkStmt.Close()
		for _, link := range links {
			if _, err := linkStmt.Exec(filename, link); err != nil {
				return err
			}
		}
	}

	// 3. Tags
	if _, err := tx.Exec("DELETE FROM tags WHERE arquivo = ?", filename); err != nil {
		return err
	}
	if len(tags) > 0 {
		tagStmt, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO tags (arquivo, tag) VALUES (?, ?)")
		if err != nil {
			return err
		}
		defer tagStmt.Close()
		for _, tag := range tags {
			if _, err := tagStmt.Exec(filename, tag); err != nil {
				return err
			}
		}
	}

	// 3.5. Clean up chunks and embeddings if it became non-embeddable
	if !s.isNoteEmbeddable(filename, tags) {
		if _, err := tx.Exec("DELETE FROM note_chunks WHERE filename = ?", filename); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM note_embeddings WHERE chunk_id LIKE ?", filename+`#%`); err != nil {
			return err
		}
	}

	// 4. Todos
	if _, err := tx.Exec("DELETE FROM todos WHERE file = ?", filename); err != nil {
		return err
	}
	if len(todos) > 0 {
		todoStmt, err := tx.PrepareContext(ctx, "INSERT INTO todos (id, file, section, type, status, text, line, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			return err
		}
		defer todoStmt.Close()
		for _, t := range todos {
			createdStr := t.Created.UTC().Format(time.RFC3339)
			if _, err := todoStmt.Exec(t.ID, t.File, t.Section, t.Type, t.Status, t.Text, t.Line, createdStr); err != nil {
				return err
			}
		}
	}

	// 5. File Mod
	modTimeStr := modTime.UTC().Format(time.RFC3339)
	if _, err := tx.Exec("INSERT OR REPLACE INTO file_mods (arquivo, mtime) VALUES (?, ?)", filename, modTimeStr); err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteAllFileRecords atomically deletes all DB records related to a specific file.
func (s *Store) DeleteAllFileRecords(filename string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM documents WHERE arquivo = ?", filename); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM docs_fts WHERE arquivo = ?", filename); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM links WHERE from_file = ?", filename); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM tags WHERE arquivo = ?", filename); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM todos WHERE file = ?", filename); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM file_mods WHERE arquivo = ?", filename); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM popularity WHERE arquivo = ?", filename); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM notes WHERE filename = ?", filename); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM note_chunks WHERE filename = ?", filename); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM note_embeddings WHERE chunk_id LIKE ?", filename+`#%`); err != nil {
		return err
	}

	return tx.Commit()
}
