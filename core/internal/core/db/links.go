package db

import (
	"strings"
	"ton618/core/internal/core/db/generated"
)

// AddLink creates a directed link from one file to another.
func (s *Store) AddLink(fromFile, toFile string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.AddLink(s.queryCtx(), dbgen.AddLinkParams{
		FromFile: fromFile,
		ToFile:   toFile,
	})
}

// RemoveLink deletes a directed link between two files.
func (s *Store) RemoveLink(fromFile, toFile string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.RemoveLink(s.queryCtx(), dbgen.RemoveLinkParams{
		FromFile: fromFile,
		ToFile:   toFile,
	})
}

// GetLinks returns all outbound links from a file.
func (s *Store) GetLinks(fromFile string) ([]string, error) {
	return s.Q.GetLinks(s.queryCtx(), fromFile)
}

// GetLinkCount returns the number of outbound links from a file.
func (s *Store) GetLinkCount(fromFile string) int {
	count, _ := s.Q.GetLinkCount(s.queryCtx(), fromFile)
	return int(count)
}

// GetBacklinks returns all files that link to the given file.
func (s *Store) GetBacklinks(toFile string) ([]string, error) {
	return s.Q.GetBacklinks(s.queryCtx(), strings.ToLower(toFile))
}

// GetBacklinkCount returns the number of files that link to the given file.
func (s *Store) GetBacklinkCount(toFile string) int {
	count, _ := s.Q.GetBacklinkCount(s.queryCtx(), strings.ToLower(toFile))
	return int(count)
}

// GetAllLinks returns all links as a map of from_file -> []to_file.
func (s *Store) GetAllLinks() (map[string][]string, error) {
	rows, err := s.Q.GetAllLinks(s.queryCtx())
	if err != nil {
		return nil, err
	}

	result := make(map[string][]string)
	for _, r := range rows {
		result[r.FromFile] = append(result[r.FromFile], r.ToFile)
	}
	return result, nil
}

// ClearLinks removes all links originating from a file.
func (s *Store) ClearLinks(fromFile string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	return s.Q.ClearLinks(s.queryCtx(), fromFile)
}

// GetLinksByFiles returns all unique files that the given files link TO.
// It excludes any files listed in the exclude set (used to avoid self-references).
func (s *Store) GetLinksByFiles(fromFiles []string, exclude map[string]bool) ([]string, error) {
	if len(fromFiles) == 0 {
		return nil, nil
	}
	
	rows, err := s.Q.GetLinksByFiles(s.queryCtx(), fromFiles)
	if err != nil {
		return nil, err
	}
	
	normExclude := make(map[string]bool, len(exclude))
	for k, v := range exclude {
		normExclude[strings.ToLower(k)] = v
	}

	var links []string
	for _, to := range rows {
		if exclude != nil && normExclude[to] {
			continue
		}
		links = append(links, to)
	}
	return links, nil
}
