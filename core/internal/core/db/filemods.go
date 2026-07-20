package db

import (
	"database/sql"
	"ton618/core/internal/core/db/generated"
)

// ---------------------------------------------------------------------------
// file_mods
// ---------------------------------------------------------------------------

// GetFileMod returns the stored modification time for a file, or empty string if not found.
func (s *Store) GetFileMod(arquivo string) (string, error) {
	mtime, err := s.Q.GetFileMod(s.queryCtx(), arquivo)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return mtime.String, nil
}

// SetFileMod inserts or updates the modification time for a file.
func (s *Store) SetFileMod(arquivo, mtime string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.SetFileMod(s.queryCtx(), dbgen.SetFileModParams{
		Arquivo: arquivo,
		Mtime:   sql.NullString{String: mtime, Valid: true},
	})
}

// DeleteFileMod removes the modification-time record for a file.
func (s *Store) DeleteFileMod(arquivo string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.DeleteFileMod(s.queryCtx(), arquivo)
}

// GetAllFileMods returns all stored file modification times as a map.
func (s *Store) GetAllFileMods() (map[string]string, error) {
	rows, err := s.Q.GetAllFileMods(s.queryCtx())
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, r := range rows {
		result[r.Arquivo] = r.Mtime.String
	}
	return result, nil
}

// FileModTag represents a file's modification time and its associated tags (comma separated).
type FileModTag struct {
	Arquivo string
	Mtime   string
	Tags    string
}

// GetFilesModsAndTags returns all files with their mtime and concatenated tags in a single query.
func (s *Store) GetFilesModsAndTags() ([]FileModTag, error) {
	rows, err := s.Q.GetFilesModsAndTags(s.queryCtx())
	if err != nil {
		return nil, err
	}

	var result []FileModTag
	for _, r := range rows {
		result = append(result, FileModTag{
			Arquivo: r.Arquivo,
			Mtime:   r.Mtime.String,
			Tags:    r.Tags,
		})
	}
	return result, nil
}
