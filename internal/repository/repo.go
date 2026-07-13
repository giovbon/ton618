// Package repository define as interfaces de acesso a dados.
// db.Store satisfaz todas implicitamente (duck typing do Go).
package repository

import "time"

// ── Notas ──

// NoteStore define operações na tabela notes.
type NoteStore interface {
	GetNote(filename string) (string, error)
	SaveNote(filename, content, mtime string) error
	DeleteNote(filename string) error
	RenameNote(old, new string) error
	GetAllNotes() (map[string]string, error)
	GetNoteMtime(filename string) (string, error)
	NoteExists(filename string) bool
}

// ── File mods ──

// FileModStore define operações na tabela file_mods.
type FileModStore interface {
	GetFileMod(arquivo string) (string, error)
	SetFileMod(arquivo, mtime string) error
	DeleteFileMod(arquivo string) error
	GetAllFileMods() (map[string]string, error)
}

// ── Tags ──

// TagStore define operações na tabela tags.
type TagStore interface {
	SetFileTags(arquivo string, tags []string) error
	GetFileTags(arquivo string) ([]string, error)
	GetAllTags() ([]string, error)
}

// ── Links ──

// LinkStore define operações na tabela links.
type LinkStore interface {
	AddLink(fromFile, toFile string) error
	ClearLinks(fromFile string) error
	GetBacklinks(toFile string) ([]string, error)
	GetLinks(fromFile string) ([]string, error)
	GetLinksByFiles(fromFiles []string, exclude map[string]bool) ([]string, error)
}

// ── Keywords ──

// KeywordStore define operações na coluna keywords da tabela notes.
type KeywordStore interface {
	SetNoteKeywords(filename string, keywords []string) error
	GetNoteKeywords(filename string) ([]string, error)
}

// ── Popularidade ──

// PopStore define operações na tabela popularity.
type PopStore interface {
	GetPopularity(arquivo string) int
	IncrementPopularity(arquivo string) error
	ResetPopularity(arquivo string) error
}

// ── Helpers ──

// ParseMtime converte string RFC3339 para time.Time.
func ParseMtime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
