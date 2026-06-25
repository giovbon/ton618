package service

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
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
	// Garante o formato do filename
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}
	if !strings.HasPrefix(filename, "notes/") {
		filename = "notes/" + filename
	}

	mtime := time.Now().UTC()
	return s.processAndSave(filename, content, mtime)
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

	// Obtém os backlinks antes de qualquer alteração no banco
	backlinks, err := s.links.GetBacklinks(oldName)
	if err != nil {
		slog.Error("get backlinks for rename", "oldName", oldName, "error", err)
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

		if err := s.processAndSave(newName, content, time.Now().UTC()); err != nil {
			slog.Error("processAndSave after rename", "file", newName, "error", err)
		}
	}

	// Atualiza os wikilinks nos arquivos que referenciavam a nota antiga
	if len(backlinks) > 0 {
		newTitle := strings.TrimSuffix(filepath.Base(newName), ".md")
		wikilinkRegex := regexp.MustCompile(`\[\[([^\]|#]+)(?:[|#][^\]]*)?\]\]`)

		for _, refFile := range backlinks {
			if refFile == oldName || refFile == newName {
				continue
			}
			refContent, err := s.notes.GetNote(refFile)
			if err != nil || refContent == "" {
				continue
			}

			updatedContent := wikilinkRegex.ReplaceAllStringFunc(refContent, func(match string) string {
				submatches := wikilinkRegex.FindStringSubmatch(match)
				if len(submatches) > 1 {
					target := strings.TrimSpace(submatches[1])
					if target == "" {
						return match
					}
					normTarget := strings.ToLower(target)
					if !strings.Contains(normTarget, ".") {
						normTarget += ".md"
					}
					if strings.HasSuffix(normTarget, ".md") && !strings.Contains(normTarget, "/") {
						normTarget = "notes/" + normTarget
					}

					if normTarget == strings.ToLower(oldName) {
						return strings.Replace(match, target, newTitle, 1)
					}
				}
				return match
			})

			if updatedContent != refContent {
				// s.Save processa tags, links e salva tanto no DB quanto no disco
				if err := s.Save(refFile, updatedContent, nil); err != nil {
					slog.Error("update referring note during rename", "refFile", refFile, "error", err)
				}
			}
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

	allTags, err := s.store.GetAllFileTags()
	if err != nil {
		allTags = make(map[string][]string)
	}

	var items []NoteItem
	for arquivo, mtime := range mods {
		tags := allTags[arquivo]
		if tags == nil {
			tags = []string{}
		}

		noteType := "nota"
		isDrawing := false
		isSpreadsheet := false
		isTypst := false
		isMermaid := false
		isYoutube := false
		isArticle := false
		isCapture := false
		for _, t := range tags {
			lowerT := strings.ToLower(t)
			switch lowerT {
			case "drawing":
				isDrawing = true
			case "spreadsheet":
				isSpreadsheet = true
			case "typst":
				isTypst = true
			case "mermaid":
				isMermaid = true
			case "youtube":
				isYoutube = true
			case "artigo", "article":
				isArticle = true
			case "captura", "capture":
				isCapture = true
			}
		}
		if isDrawing {
			noteType = "desenho"
		} else if isSpreadsheet {
			noteType = "planilha"
		} else if isTypst {
			noteType = "typst"
		} else if isMermaid {
			noteType = "mermaid"
		} else if isYoutube {
			noteType = "youtube"
		} else if isArticle {
			noteType = "artigo"
		} else if isCapture {
			noteType = "captura"
		} else if strings.HasPrefix(arquivo, "pdfs/") {
			noteType = "pdf"
		} else if strings.HasPrefix(arquivo, "attachments/") {
			noteType = "anexo"
		} else if strings.HasPrefix(arquivo, "archives/") {
			noteType = "arquivo"
		}

		// Filtra tags de tipo de nota para ocultá-las da interface do usuário
		var userTags []string
		for _, t := range tags {
			lowerT := strings.ToLower(t)
			if lowerT != "typst" && lowerT != "drawing" && lowerT != "spreadsheet" && lowerT != "mermaid" {
				userTags = append(userTags, t)
			}
		}

		items = append(items, NoteItem{
			Arquivo:  arquivo,
			Tags:     userTags,
			Mtime:    mtime,
			Type:     noteType,
		})
	}
	return items, nil
}

// SyncDatabase garante que todas as notas da tabela 'notes' estejam devidamente indexadas no FTS e na tabela de tags.
func (s *NoteService) SyncDatabase() error {
	allNotes, err := s.notes.GetAllNotes()
	if err != nil {
		return err
	}

	for filename, mtimeStr := range allNotes {
		existingMod, err := s.fileMod.GetFileMod(filename)
		if err != nil {
			continue
		}

		// Se a nota não estiver indexada na tabela file_mods, fazemos o reindex
		if existingMod == "" {
			content, err := s.notes.GetNote(filename)
			if err != nil || content == "" {
				continue
			}
			mtime, err := time.Parse(time.RFC3339, mtimeStr)
			if err != nil {
				mtime = time.Now()
			}
			slog.Info("Auto-reindexando nota no banco", "file", filename)
			if err := s.processAndSave(filename, content, mtime); err != nil {
				slog.Error("erro ao auto-reindexar nota", "file", filename, "error", err)
			}
		}
	}
	return nil
}

// NoteItem é o formato de listagem compacta.
type NoteItem struct {
	Arquivo  string   `json:"arquivo"`
	Tags     []string `json:"tags"`
	Mtime    string   `json:"mtime"`
	Type     string   `json:"type"`
}

// BacklinksResult contém os dois níveis de backlinks para uma nota.
type BacklinksResult struct {
	// Level1 são notas que linkam PARA esta nota.
	Level1 []string `json:"level1"`
	// Level2 são notas para as quais as Level1 também linkam (excluindo a própria nota).
	Level2 []string `json:"level2"`
}

// GetBacklinks retorna os backlinks de 2 níveis para uma nota.
// Nível 1: notas que linkam PARA esta nota.
// Nível 2: notas que as notas de nível 1 linkam (excluindo a nota atual).
func (s *NoteService) GetBacklinks(filename string) (*BacklinksResult, error) {
	// Nível 1: quem linka PARA esta nota
	level1, err := s.links.GetBacklinks(filename)
	if err != nil {
		return nil, fmt.Errorf("get backlinks: %w", err)
	}

	if len(level1) == 0 {
		return &BacklinksResult{}, nil
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

	return &BacklinksResult{
		Level1: level1,
		Level2: filtered,
	}, nil
}

// ── privado ──

// processAndSave centraliza o processamento e indexação do markdown.
func (s *NoteService) processAndSave(filename, content string, modTime time.Time) error {
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
	// 1. Extrai keywords via RAKE (apenas se keywords: true ou #keywords)
	var newContent = content
	var extractedKeywords []string
	var hasKeywordsTag = processor.HasKeywords(fileTags)
	if hasKeywordsTag {
		extractedKeywords = processor.ExtractKeywords(content, processor.KeywordsCount(content))
		if len(extractedKeywords) > 0 {
			if updated, err := UpdateFrontmatterProperty(newContent, "keywords", strings.Join(extractedKeywords, ", ")); err == nil {
				newContent = updated
			}
		} else {
			if updated, err := UpdateFrontmatterProperty(newContent, "keywords", nil); err == nil {
				newContent = updated
			}
		}
	} else {
		if updated, err := UpdateFrontmatterProperty(newContent, "keywords", nil); err == nil {
			newContent = updated
		}
	}

	// Se o conteúdo mudou (injetado keyword), reprocessa as tags e docs para garantir consistência
	if newContent != content {
		content = newContent
		docs, links, fileTags = processor.ProcessMarkdownContent(
			[]byte(content), filename, modTime, modTime,
		)
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

	// 3. Prepara Tags limpas
	cleanTags := processor.FilterKeywords(fileTags)

	// 4. Salva a nota no banco 
	mtimeStr := modTime.UTC().Format(time.RFC3339)
	if err := s.notes.SaveNote(filename, content, mtimeStr); err != nil {
		return fmt.Errorf("save note: %w", err)
	}

	// 5. Salva as keywords na coluna do banco (precisa ser depois de SaveNote por causa do INSERT OR REPLACE)
	if hasKeywordsTag {
		if err := s.store.SetNoteKeywords(filename, extractedKeywords); err != nil {
			slog.Error("set keywords", "file", filename, "error", err)
		}
	} else {
		if err := s.store.SetNoteKeywords(filename, nil); err != nil {
			slog.Error("clear keywords", "file", filename, "error", err)
		}
	}

	// 6. Submete tudo para a transação única de FTS e índices
	if err := s.store.ReplaceFileIndexes(filename, docs, links, cleanTags, todos, modTime); err != nil {
		return fmt.Errorf("replace file indexes: %w", err)
	}

	return nil
}
