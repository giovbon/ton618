package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ton618/internal/core/db/generated"
)

// GetNote returns the content of a note by filename.
func (s *Store) GetNote(filename string) (string, error) {
	content, err := s.Q.GetNote(s.queryCtx(), filename)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content.String, err
}

// SaveNote inserts or updates a note's content and modification time.
func (s *Store) SaveNote(filename, content, mtime string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.SaveNote(s.queryCtx(), dbgen.SaveNoteParams{
		Filename: filename,
		Content:  sql.NullString{String: content, Valid: true},
		Mtime:    sql.NullString{String: mtime, Valid: true},
	})
}

// DeleteNote removes a note by filename and cleans up its FTS documents, chunks and embeddings.
func (s *Store) DeleteNote(filename string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	
	if err := s.Q.DeleteNote(s.queryCtx(), filename); err != nil {
		return err
	}
	if err := s.Q.DeleteNoteChunks(s.queryCtx(), filename); err != nil {
		return err
	}
	_, err := s.DB.Exec(`DELETE FROM note_embeddings WHERE chunk_id LIKE ?`, filename+`#%`)
	return err
}

// RenameNote renames a note from old to new filename.
func (s *Store) RenameNote(old, new string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.RenameNote(s.queryCtx(), dbgen.RenameNoteParams{
		Filename:   new,
		Filename_2: old,
	})
}

// GetAllNotes returns all note filenames and their mtimes, ordered by mtime desc.
func (s *Store) GetAllNotes() (map[string]string, error) {
	rows, err := s.Q.GetAllNotes(s.queryCtx())
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, r := range rows {
		result[r.Filename] = r.Mtime.String
	}
	return result, nil
}

// GetAllNotesPaginated returns a paginated list of notes.
func (s *Store) GetAllNotesPaginated(from, size int) (map[string]string, int, error) {
	count, err := s.Q.CountNotes(s.queryCtx())
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.Q.GetAllNotesPaginated(s.queryCtx(), dbgen.GetAllNotesPaginatedParams{
		Limit:  int64(size),
		Offset: int64(from),
	})
	if err != nil {
		return nil, 0, err
	}
	result := make(map[string]string)
	for _, r := range rows {
		result[r.Filename] = r.Mtime.String
	}
	return result, int(count), nil
}

// GetNoteMtime returns just the mtime for a note.
func (s *Store) GetNoteMtime(filename string) (string, error) {
	mtime, err := s.Q.GetNoteMtime(s.queryCtx(), filename)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return mtime.String, err
}

// NoteExists checks if a note exists.
func (s *Store) NoteExists(filename string) bool {
	count, _ := s.Q.NoteExists(s.queryCtx(), filename)
	return count > 0
}



// GetNotesNeedingMarkmapTag retorna filenames de notas cujo conteúdo contém 'type: markmap' ou 'type: mindmap', mas que não possuem as tags correspondentes na tabela tags.
func (s *Store) GetNotesNeedingMarkmapTag() ([]string, error) {
	return s.Q.GetNotesNeedingMarkmapTag(s.queryCtx())
}

// MigrateNotesFromDisk imports all .md files from the docs/notes/ directory into the database.
// It skips files that already exist in the DB (by filename).
// Returns the count of imported notes.
func (s *Store) MigrateNotesFromDisk(docsDir string) (int, error) {
	notesDir := filepath.Join(docsDir, "notes")
	entries, err := os.ReadDir(notesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	imported := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		filename := "notes/" + name
		if s.NoteExists(filename) {
			continue // skip already imported
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fullPath := filepath.Join(notesDir, name)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		mtime := info.ModTime().Format(time.RFC3339)
		if err := s.SaveNote(filename, string(content), mtime); err != nil {
			continue
		}
		imported++
	}
	return imported, nil
}

// GetAllNotesContent returns all note filenames and their content in a single query.
func (s *Store) GetAllNotesContent() (map[string]string, error) {
	rows, err := s.Q.GetAllNotesContent(s.queryCtx())
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, r := range rows {
		result[r.Filename] = r.Content.String
	}
	return result, nil
}
