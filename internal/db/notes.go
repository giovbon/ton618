package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GetNote returns the content of a note by filename.
func (s *Store) GetNote(filename string) (string, error) {
	var content string
	err := s.DB.QueryRow("SELECT content FROM notes WHERE filename = ?", filename).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return content, err
}

// SaveNote inserts or updates a note's content and modification time.
func (s *Store) SaveNote(filename, content, mtime string) error {
	_, err := s.DB.Exec(
		"INSERT OR REPLACE INTO notes (filename, content, mtime) VALUES (?, ?, ?)",
		filename, content, mtime,
	)
	return err
}

// DeleteNote removes a note by filename.
func (s *Store) DeleteNote(filename string) error {
	_, err := s.DB.Exec("DELETE FROM notes WHERE filename = ?", filename)
	return err
}

// RenameNote renames a note from old to new filename.
func (s *Store) RenameNote(old, new string) error {
	_, err := s.DB.Exec("UPDATE notes SET filename = ? WHERE filename = ?", new, old)
	return err
}

// GetAllNotes returns all note filenames and their mtimes, ordered by mtime desc.
func (s *Store) GetAllNotes() (map[string]string, error) {
	rows, err := s.DB.Query("SELECT filename, mtime FROM notes ORDER BY mtime DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var filename, mtime string
		if err := rows.Scan(&filename, &mtime); err != nil {
			continue
		}
		result[filename] = mtime
	}
	return result, rows.Err()
}

// GetAllNotesPaginated returns a paginated list of notes.
func (s *Store) GetAllNotesPaginated(from, size int) (map[string]string, int, error) {
	var total int
	s.DB.QueryRow("SELECT COUNT(*) FROM notes").Scan(&total)

	rows, err := s.DB.Query(
		"SELECT filename, mtime FROM notes ORDER BY mtime DESC LIMIT ? OFFSET ?",
		size, from,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var filename, mtime string
		if err := rows.Scan(&filename, &mtime); err != nil {
			continue
		}
		result[filename] = mtime
	}
	return result, total, rows.Err()
}

// GetNoteMtime returns just the mtime for a note.
func (s *Store) GetNoteMtime(filename string) (string, error) {
	var mtime string
	err := s.DB.QueryRow("SELECT mtime FROM notes WHERE filename = ?", filename).Scan(&mtime)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return mtime, err
}

// NoteExists checks if a note exists.
func (s *Store) NoteExists(filename string) bool {
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM notes WHERE filename = ?", filename).Scan(&count)
	return count > 0
}

// SetNoteKeywords atualiza as keywords extraídas de uma nota.
func (s *Store) SetNoteKeywords(filename string, keywords []string) error {
	kw := strings.Join(keywords, ",")
	_, err := s.DB.Exec("UPDATE notes SET keywords = ? WHERE filename = ?", kw, filename)
	return err
}

// GetNoteKeywords retorna as keywords extraídas de uma nota.
// Retorna slice vazio se não houver keywords ou a nota não existir.
func (s *Store) GetNoteKeywords(filename string) ([]string, error) {
	var kw string
	err := s.DB.QueryRow("SELECT keywords FROM notes WHERE filename = ?", filename).Scan(&kw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if kw == "" {
		return nil, nil
	}
	return strings.Split(kw, ","), nil
}

// GetAllNotesKeywords returns a map of all note filenames to their list of keywords.
func (s *Store) GetAllNotesKeywords() (map[string][]string, error) {
	rows, err := s.DB.Query("SELECT filename, keywords FROM notes WHERE keywords IS NOT NULL AND keywords != ''")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var filename, keywords string
		if err := rows.Scan(&filename, &keywords); err != nil {
			return nil, err
		}
		if keywords != "" {
			result[filename] = strings.Split(keywords, ",")
		}
	}
	return result, rows.Err()
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
