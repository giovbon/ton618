package service

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ton618/internal/db"
	"ton618/internal/processor"
	"ton618/internal/repository"
)

// NoteService gerencia o ciclo de vida de notas markdown.
type NoteService struct {
	store   *db.Store // concreto: necessário para InsertDocument, IndexFTS, etc.
	notes   repository.NoteStore
	tags    repository.TagStore
	links   repository.LinkStore
	pop     repository.PopStore
	fileMod repository.FileModStore
	docsDir string
}

// NewNoteService cria o serviço de notas.
func NewNoteService(
	store *db.Store,
	notes repository.NoteStore,
	tags repository.TagStore,
	links repository.LinkStore,
	pop repository.PopStore,
	fm repository.FileModStore,
	docsDir string,
) *NoteService {
	return &NoteService{
		store:   store,
		notes:   notes,
		tags:    tags,
		links:   links,
		pop:     pop,
		fileMod: fm,
		docsDir: docsDir,
	}
}

// Save salva conteúdo markdown, reindexa e gerencia tags/links.
func (s *NoteService) Save(filename, content string, rawTags []string) error {
	var cleanTags []string
	for _, t := range rawTags {
		t = strings.TrimSpace(t)
		if t != "" {
			cleanTags = append(cleanTags, t)
		}
	}
	_ = cleanTags // tags são extraídas do conteúdo pelo processor

	// Garante o formato do filename
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}
	if !strings.HasPrefix(filename, "notes/") {
		filename = "notes/" + filename
	}

	mtime := time.Now().UTC().Format(time.RFC3339)
	if err := s.notes.SaveNote(filename, content, mtime); err != nil {
		return fmt.Errorf("save note: %w", err)
	}

	// Indexa (extrai documentos, FTS, links, tags)
	if err := s.reindex(filename, content, time.Now().UTC()); err != nil {
		slog.Error("reindex after save", "file", filename, "error", err)
	}

	return nil
}

// Delete remove uma nota do banco e do disco.
func (s *NoteService) Delete(filename string) error {
	if !strings.HasPrefix(filename, "notes/") {
		filename = "notes/" + filename
	}
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}

	s.notes.DeleteNote(filename)
	s.store.DeleteDocumentsByFile(filename)
	s.store.DeleteFTSByFile(filename)
	s.fileMod.DeleteFileMod(filename)
	s.pop.ResetPopularity(filename)
	s.tags.SetFileTags(filename, nil)

	os.Remove(filepath.Join(s.docsDir, filename))

	return nil
}

// Rename renomeia uma nota e atualiza todos os índices.
func (s *NoteService) Rename(oldName, newName string) error {
	if !strings.HasPrefix(oldName, "notes/") {
		oldName = "notes/" + oldName
	}
	if !strings.HasSuffix(oldName, ".md") {
		oldName += ".md"
	}
	if !strings.HasPrefix(newName, "notes/") {
		newName = "notes/" + newName
	}
	if !strings.HasSuffix(newName, ".md") {
		newName += ".md"
	}

	if oldName == newName {
		return nil
	}

	if err := s.notes.RenameNote(oldName, newName); err != nil {
		return fmt.Errorf("rename note: %w", err)
	}

	os.Rename(filepath.Join(s.docsDir, oldName), filepath.Join(s.docsDir, newName))

	content, err := s.notes.GetNote(newName)
	if err == nil && content != "" {
		s.store.DeleteDocumentsByFile(oldName)
		s.store.DeleteFTSByFile(oldName)
		if err := s.reindex(newName, content, time.Now().UTC()); err != nil {
			slog.Error("reindex after rename", "file", newName, "error", err)
		}
	}

	return nil
}

// GetMany retorna todas as notas para listagem (formato NoteItem).
func (s *NoteService) GetMany() ([]NoteItem, error) {
	mods, err := s.fileMod.GetAllFileMods()
	if err != nil {
		return nil, err
	}
	notesFromDB, _ := s.notes.GetAllNotes()
	for name, mtime := range notesFromDB {
		if _, exists := mods[name]; !exists {
			mods[name] = mtime
		}
	}

	var items []NoteItem
	for arquivo, mtime := range mods {
		tags, _ := s.tags.GetFileTags(arquivo)
		items = append(items, NoteItem{
			Arquivo: arquivo,
			Tags:    tags,
			Mtime:   mtime,
		})
	}
	return items, nil
}

// NoteItem é o formato de listagem compacta.
type NoteItem struct {
	Arquivo string   `json:"arquivo"`
	Tags    []string `json:"tags"`
	Mtime   string   `json:"mtime"`
}

// ── privado ──

func (s *NoteService) reindex(filename, content string, modTime time.Time) error {
	creationTime := modTime

	s.store.DeleteDocumentsByFile(filename)
	s.store.DeleteFTSByFile(filename)

	fullPath := filepath.Join(s.docsDir, filename)
	docs, links, fileTags := processor.ProcessMarkdownContent(
		[]byte(content), filename, modTime, creationTime,
	)
	if len(docs) == 0 {
		docs, links, fileTags = processor.ProcessMarkdown(
			fullPath, filename, modTime, creationTime,
		)
	}

	for _, doc := range docs {
		dbDoc := db.Document{
			ID:        doc.ID,
			Tipo:      doc.Tipo,
			Arquivo:   doc.Arquivo,
			Secao:     doc.Secao,
			Texto:     doc.Texto,
			Tags:      db.SliceToTags(doc.Tags),
			Pagina:    doc.Pagina,
			Ordem:     doc.Ordem,
			Timestamp: doc.Timestamp,
			CreatedAt: doc.Created,
			Hash:      doc.Hash,
		}
		if err := s.store.InsertDocument(dbDoc); err != nil {
			slog.Error("insert doc", "id", doc.ID, "error", err)
			continue
		}
		if err := s.store.IndexFTS(doc.ID, doc.Tipo, doc.Arquivo, doc.Secao, doc.Texto, db.SliceToTags(doc.Tags)); err != nil {
			slog.Error("index fts", "id", doc.ID, "error", err)
		}
	}

	s.links.ClearLinks(filename)
	for _, link := range links {
		s.links.AddLink(filename, link)
	}

	if len(fileTags) > 0 {
		s.tags.SetFileTags(filename, fileTags)
	} else {
		s.tags.SetFileTags(filename, nil)
	}

	s.fileMod.SetFileMod(filename, modTime.Format(time.RFC3339))

	return nil
}
