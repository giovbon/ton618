package db

import (
	"context"
	"database/sql"
	"ton618/internal/core/db/generated"
)

// SetFileTags replaces the entire set of tags for a file atomically.
func (s *Store) SetFileTags(arquivo string, tags []string) error {
	return s.RunInTx(func(tx *sql.Tx) error {
		qtx := s.Q.WithTx(tx)
		if err := qtx.DeleteFileTags(context.Background(), arquivo); err != nil {
			return err
		}
		for _, tag := range tags {
			if err := qtx.AddTagToFile(context.Background(), dbgen.AddTagToFileParams{
				Arquivo: arquivo,
				Tag:     tag,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetFileTags returns all tags associated with a file.
func (s *Store) GetFileTags(arquivo string) ([]string, error) {
	return s.Q.GetFileTags(context.Background(), arquivo)
}

// GetAllFileTags returns a map of all files to their list of tags.
func (s *Store) GetAllFileTags() (map[string][]string, error) {
	rows, err := s.Q.GetAllFileTags(context.Background())
	if err != nil {
		return nil, err
	}
	result := make(map[string][]string)
	for _, r := range rows {
		result[r.Arquivo] = append(result[r.Arquivo], r.Tag)
	}
	return result, nil
}

// GetAllTags returns every distinct tag present in the database.
func (s *Store) GetAllTags() ([]string, error) {
	return s.Q.GetAllTags(context.Background())
}

// GetFilesByTag returns all file paths that have a specific tag.
func (s *Store) GetFilesByTag(tag string) ([]string, error) {
	return s.Q.GetFilesByTag(context.Background(), tag)
}

// AddTagToFile adds a single tag to a file (no-op if already present).
func (s *Store) AddTagToFile(arquivo, tag string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.AddTagToFile(context.Background(), dbgen.AddTagToFileParams{
		Arquivo: arquivo,
		Tag:     tag,
	})
}

// RemoveTagFromFile removes a single tag from a file.
func (s *Store) RemoveTagFromFile(arquivo, tag string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.RemoveTagFromFile(context.Background(), dbgen.RemoveTagFromFileParams{
		Arquivo: arquivo,
		Tag:     tag,
	})
}
