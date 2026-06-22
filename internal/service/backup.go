package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ton618/internal/repository"
)

// BackupService gera backups ZIP de todos os dados (exceto archives/).
type BackupService struct {
	notes   repository.NoteStore
	fileMod repository.FileModStore
	docsDir string
}

// NewBackupService cria o serviço de backup.
func NewBackupService(notes repository.NoteStore, fm repository.FileModStore, docsDir string) *BackupService {
	return &BackupService{notes: notes, fileMod: fm, docsDir: docsDir}
}

// Create gera um ZIP com todas as notas, PDFs e anexos (se full for verdadeiro).
func (s *BackupService) Create(full bool) ([]byte, error) {
	allNotes, _ := s.notes.GetAllNotes()
	allMods, _ := s.fileMod.GetAllFileMods()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	seen := make(map[string]bool)

	// 1. Notas do DB — conteúdo markdown
	for filename, mtimeStr := range allNotes {
		if strings.HasPrefix(filename, "archives/") {
			continue
		}
		content, err := s.notes.GetNote(filename)
		if err != nil || content == "" {
			continue
		}
		if !strings.HasSuffix(filename, ".md") {
			filename += ".md"
		}
		addToZip(zw, filename, []byte(content), repository.ParseMtime(mtimeStr))
		seen[filename] = true
	}

	// 2. Arquivos do disco (PDFs, attachments, notas sem conteúdo no DB)
	if full {
		for filename := range allMods {
			if strings.HasPrefix(filename, "archives/") || seen[filename] {
				continue
			}
			fullPath := filepath.Join(s.docsDir, filename)
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			mtimeStr, _ := s.fileMod.GetFileMod(filename)
			addToZip(zw, filename, data, repository.ParseMtime(mtimeStr))
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("backup: close zip: %w", err)
	}
	return buf.Bytes(), nil
}

func addToZip(zw *zip.Writer, name string, data []byte, modTime time.Time) {
	h := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	if !modTime.IsZero() {
		h.SetModTime(modTime)
	}
	w, err := zw.CreateHeader(h)
	if err != nil {
		return
	}
	w.Write(data)
}
