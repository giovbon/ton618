package notes

import (
	"context"
	"net/url"
	"ton618/internal/core/domain"

	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ton618/internal/core/db"
	"ton618/internal/processor"
	"ton618/internal/repository"
)

// fileOps define as operações de banco de mais alto nível que o NoteService
// precisa do db.Store. Esta interface permite testar o NoteService sem banco real.
type fileOps interface {
	DeleteAllFileRecords(filename string) error
	GetFilesModsAndTags() ([]db.FileModTag, error)
	GetNotesNeedingMarkmapTag() ([]string, error)
	GetActiveTodoMarkers() ([]db.TodoMarker, error)
	ReplaceFileIndexes(ctx context.Context, filename string, docs []processor.Document, links []string, tags []string, todos []processor.TodoItem, modTime time.Time) error
}

// NoteService gerencia o ciclo de vida de notas markdown.
type NoteService struct {
	store   fileOps // operações de banco (desacoplado via interface)
	notes   repository.NoteStore
	tags    repository.TagStore
	links   repository.LinkStore
	pop     repository.PopStore
	fileMod repository.FileModStore
	docsDir string
}

// NewNoteService cria o serviço de notas.
func NewNoteService(
	store fileOps,
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
// Usa context.Background() internamente. Prefira SaveWithContext se tiver
// um contexto HTTP disponível para propagar cancelamento.
func (s *NoteService) Save(filename, content string, rawTags []string) error {
	return s.SaveWithContext(context.Background(), filename, content, rawTags)
}

// SaveWithContext é como Save, mas aceita um contexto para propagar
// cancelamento (ex: cliente desconectou) até a camada de banco.
func (s *NoteService) SaveWithContext(ctx context.Context, filename, content string, rawTags []string) error {
	// Garante o formato do filename
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}
	if !strings.HasPrefix(filename, "notes/") {
		filename = "notes/" + filename
	}

	mtime := time.Now().UTC()
	return s.processAndSave(ctx, filename, content, mtime)
}

// Delete remove uma nota do banco e do disco.
func (s *NoteService) Delete(filename string) error {
	if !strings.HasPrefix(filename, "notes/") {
		filename = "notes/" + filename
	}
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}

	if err := s.store.DeleteAllFileRecords(filename); err != nil {
		slog.Error("delete all file records", "file", filename, "error", err)
	}

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

	// Remove todos os registros antigos do DB numa transação atômica
	if err := s.store.DeleteAllFileRecords(oldName); err != nil {
		slog.Error("delete all file records on rename", "file", oldName, "error", err)
	}

	newPath := filepath.Join(s.docsDir, newName)
	var content string
	hasPhysFile := false

	if data, err := os.ReadFile(newPath); err == nil {
		content = string(data)
		hasPhysFile = true
	} else {
		// Fallback para o conteúdo indexado antigo caso a leitura física falhe
		content, _ = s.notes.GetNote(newName)
	}

	if content != "" {
		parts := strings.Split(newName, "/")
		base := parts[len(parts)-1]
		newTitle := strings.TrimSuffix(base, ".md")

		// Atualiza a propriedade 'title' no frontmatter da nota (tanto físico quanto DB)
		if newContent, err := UpdateFrontmatterProperty(content, "title", newTitle); err == nil {
			content = newContent
			if hasPhysFile {
				if err := os.WriteFile(newPath, []byte(newContent), 0644); err != nil {
					slog.Error("write updated frontmatter after rename", "file", newName, "error", err)
				}
			}
		}

		if err := s.processAndSave(context.Background(), newName, content, time.Now().UTC()); err != nil {
			slog.Error("processAndSave after rename", "file", newName, "error", err)
		}
	}

	// Atualiza os wikilinks e URLs nos arquivos que referenciavam a nota antiga
	if err := s.UpdateBacklinksOnRename(oldName, newName); err != nil {
		slog.Error("update backlinks on rename", "oldName", oldName, "newName", newName, "error", err)
	}

	return nil
}

// UpdateBacklinksOnRename atualiza wikilinks, links markdown e URLs em todas as notas
// quando qualquer arquivo (.md, .zip, .pdf, .epub, etc.) é renomeado.
func (s *NoteService) UpdateBacklinksOnRename(oldName, newName string) error {
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}

	oldBase := filepath.Base(oldName)
	newBase := filepath.Base(newName)

	oldTitle := strings.TrimSuffix(oldBase, filepath.Ext(oldBase))
	newTitle := strings.TrimSuffix(newBase, filepath.Ext(newBase))

	candidateMap := make(map[string]bool)

	// Busca backlinks registrados no banco
	if bl1, err := s.links.GetBacklinks(oldName); err == nil {
		for _, f := range bl1 {
			candidateMap[f] = true
		}
	}
	if bl2, err := s.links.GetBacklinks(oldBase); err == nil {
		for _, f := range bl2 {
			candidateMap[f] = true
		}
	}
	if bl3, err := s.links.GetBacklinks(strings.ToLower(oldName)); err == nil {
		for _, f := range bl3 {
			candidateMap[f] = true
		}
	}
	if bl4, err := s.links.GetBacklinks(strings.ToLower(oldBase)); err == nil {
		for _, f := range bl4 {
			candidateMap[f] = true
		}
	}

	// Garante que todas as notas do sistema sejam testadas caso a indexação estivesse pendente
	if allNotes, err := s.notes.GetAllNotes(); err == nil {
		for f := range allNotes {
			candidateMap[f] = true
		}
	}

	oldEsc := SafeFileQueryEscape(oldName)
	newEsc := SafeFileQueryEscape(newName)
	oldUrlEsc := url.QueryEscape(oldName)
	newUrlEsc := url.QueryEscape(newName)

	oldBaseEsc := SafeFileQueryEscape(oldBase)
	newBaseEsc := SafeFileQueryEscape(newBase)
	oldBaseUrlEsc := url.QueryEscape(oldBase)
	newBaseUrlEsc := url.QueryEscape(newBase)

	for refFile := range candidateMap {
		if refFile == oldName || refFile == newName {
			continue
		}
		refContent, err := s.notes.GetNote(refFile)
		if err != nil || refContent == "" {
			continue
		}

		// 1. Wikilinks [[target]]
		updatedContent := processor.WikilinkRegex.ReplaceAllStringFunc(refContent, func(match string) string {
			submatches := processor.WikilinkRegex.FindStringSubmatch(match)
			if len(submatches) > 1 {
				target := strings.TrimSpace(submatches[1])
				if target == "" {
					return match
				}
				normTarget := strings.ToLower(target)

				// Match com nome completo (ex: attachments/meuarquivo.zip ou notes/nota.md)
				if normTarget == strings.ToLower(oldName) {
					return strings.Replace(match, target, newName, 1)
				}

				// Match com nome base (ex: meuarquivo.zip)
				if normTarget == strings.ToLower(oldBase) {
					return strings.Replace(match, target, newBase, 1)
				}

				// Match com o nome sem extensão (ex: "meudesenho", "manual", "anexo")
				if normTarget == strings.ToLower(oldTitle) {
					return strings.Replace(match, target, newTitle, 1)
				}

				if !strings.Contains(normTarget, ".") {
					oldExt := filepath.Ext(oldBase)
					if oldExt != "" && normTarget+strings.ToLower(oldExt) == strings.ToLower(oldBase) {
						return strings.Replace(match, target, newTitle, 1)
					}
					if normTarget+".md" == strings.ToLower(oldBase) || "notes/"+normTarget+".md" == strings.ToLower(oldName) {
						return strings.Replace(match, target, newTitle, 1)
					}
				}
			}
			return match
		})

		// 2. URLs de download/mídia e caminhos relativos em markdown links
		if strings.Contains(updatedContent, oldName) || strings.Contains(updatedContent, oldEsc) || strings.Contains(updatedContent, oldUrlEsc) ||
			strings.Contains(updatedContent, oldBase) || strings.Contains(updatedContent, oldBaseEsc) || strings.Contains(updatedContent, oldBaseUrlEsc) {

			updatedContent = strings.ReplaceAll(updatedContent, "name="+oldEsc, "name="+newEsc)
			updatedContent = strings.ReplaceAll(updatedContent, "name="+oldUrlEsc, "name="+newUrlEsc)
			updatedContent = strings.ReplaceAll(updatedContent, "name="+oldName, "name="+newName)

			updatedContent = strings.ReplaceAll(updatedContent, "file="+oldEsc, "file="+newEsc)
			updatedContent = strings.ReplaceAll(updatedContent, "file="+oldUrlEsc, "file="+newUrlEsc)
			updatedContent = strings.ReplaceAll(updatedContent, "file="+oldName, "file="+newName)

			for _, prefix := range []string{"attachments/", "archives/", "pdfs/", "epubs/", "notes/"} {
				updatedContent = strings.ReplaceAll(updatedContent, prefix+oldBase, prefix+newBase)
				updatedContent = strings.ReplaceAll(updatedContent, prefix+oldBaseEsc, prefix+newBaseEsc)
				updatedContent = strings.ReplaceAll(updatedContent, prefix+oldBaseUrlEsc, prefix+newBaseUrlEsc)
			}
		}

		if updatedContent != refContent {
			if err := s.Save(refFile, updatedContent, nil); err != nil {
				slog.Error("update referring note during rename", "refFile", refFile, "error", err)
			}
		}
	}

	return nil
}

// GetMany retorna todas as notas para listagem (formato domain.NoteItem).
func (s *NoteService) GetMany() ([]domain.NoteItem, error) {
	dbItems, err := s.store.GetFilesModsAndTags()
	if err != nil {
		return nil, err
	}

	var items []domain.NoteItem
	for _, item := range dbItems {
		var tags []string
		if item.Tags != "" {
			tags = strings.Split(item.Tags, ",")
		}

		noteType := domain.DetectNoteType(tags, "", item.Arquivo)
		userTags := domain.FilterUserTags(tags)

		items = append(items, domain.NoteItem{
			Arquivo: item.Arquivo,
			Tags:    userTags,
			Mtime:   item.Mtime,
			Type:    string(noteType),
		})
	}
	return items, nil
}

// SyncDatabase garante que todas as notas da tabela 'notes' estejam devidamente indexadas no FTS e na tabela de tags.
func (s *NoteService) SyncDatabase() error {
	return s.SyncDatabaseWithContext(context.Background())
}

// SyncDatabaseWithContext é como SyncDatabase, mas aceita contexto para cancelamento.
func (s *NoteService) SyncDatabaseWithContext(ctx context.Context) error {
	allNotes, err := s.notes.GetAllNotes()
	if err != nil {
		return err
	}

	// Identifica markmaps legados através de query otimizada (apenas IDs)
	legacyMarkmaps, _ := s.store.GetNotesNeedingMarkmapTag()
	legacyMap := make(map[string]bool)
	for _, name := range legacyMarkmaps {
		legacyMap[name] = true
	}

	for filename, mtimeStr := range allNotes {
		existingMod, err := s.fileMod.GetFileMod(filename)
		if err != nil {
			continue
		}

		if existingMod == "" || legacyMap[filename] {
			content, getErr := s.notes.GetNote(filename)
			if getErr != nil || content == "" {
				continue
			}
			mtime, err := time.Parse(time.RFC3339, mtimeStr)
			if err != nil {
				mtime = time.Now()
			}
			slog.Info("Auto-reindexando nota no banco", "file", filename)
			if err := s.processAndSave(ctx, filename, content, mtime); err != nil {
				slog.Error("erro ao auto-reindexar nota", "file", filename, "error", err)
			}
		}
	}
	return nil
}

// GetBacklinks retorna os backlinks de 2 níveis para uma nota.
// Nível 1: notas que linkam PARA esta nota.
// Nível 2: notas que as notas de nível 1 linkam (excluindo a nota atual).
func (s *NoteService) GetBacklinks(filename string) (*domain.BacklinksResult, error) {
	// Nível 1: quem linka PARA esta nota
	level1, err := s.links.GetBacklinks(filename)
	if err != nil {
		return nil, fmt.Errorf("get backlinks: %w", err)
	}

	if len(level1) == 0 {
		return &domain.BacklinksResult{}, nil
	}

	// Nível 2: para quem as Level1 linkam (excluindo a nota atual)
	level2, err := s.links.GetLinksByFiles(level1, nil)
	if err != nil {
		return nil, fmt.Errorf("get links by files: %w", err)
	}

	// Filtra a propria nota do nivel 2 (case-insensitive)
	filenameLower := strings.ToLower(filename)
	filtered := make([]string, 0, len(level2))
	for _, l2 := range level2 {
		if strings.ToLower(l2) != filenameLower {
			filtered = append(filtered, l2)
		}
	}

	return &domain.BacklinksResult{
		Level1: level1,
		Level2: filtered,
	}, nil
}

// ── privado ──

// processAndSave centraliza o processamento e indexação do markdown.
// O contexto é propagado para ReplaceFileIndexes, permitindo cancelamento.
func (s *NoteService) processAndSave(ctx context.Context, filename, content string, modTime time.Time) error {
	var docs []processor.Document
	var links []string
	var fileTags []string

	if content != "" {
		docs, links, fileTags = processor.ProcessMarkdownContent(
			[]byte(content), filename, modTime, modTime,
		)
	} else {
		fullPath := filepath.Join(s.docsDir, filename)
		contentBytes, err := os.ReadFile(fullPath)
		if err != nil {
			return err
		}
		content = string(contentBytes)
		docs, links, fileTags = processor.ProcessMarkdownContent(contentBytes, filename, modTime, modTime)
	}

	// 2. Extrai TODOs estruturados
	var todos []processor.TodoItem
	activeMarkers, err := s.store.GetActiveTodoMarkers()
	if err == nil {
		var markers []string
		for _, m := range activeMarkers {
			markers = append(markers, m.Marker)
		}
		todos = processor.ExtractTodos(content, filename, modTime, markers)
	} else {
		slog.Error("get active todo markers", "error", err)
	}

	// 3. Prepara tags limpas
	cleanTags := fileTags

	// 4. Salva o conteúdo da nota no banco
	mtimeStr := modTime.UTC().Format(time.RFC3339)
	if err := s.notes.SaveNote(filename, content, mtimeStr); err != nil {
		return fmt.Errorf("save note: %w", err)
	}

	// 5. Submete documentos, links, tags e todos em transação única
	if err := s.store.ReplaceFileIndexes(ctx, filename, docs, links, cleanTags, todos, modTime); err != nil {
		return fmt.Errorf("replace file indexes: %w", err)
	}

	return nil
}
