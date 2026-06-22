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
	s.store.DeleteTodosByFile(filename)
	s.fileMod.DeleteFileMod(filename)
	s.pop.ResetPopularity(filename)
	s.tags.SetFileTags(filename, nil)
	s.links.ClearLinks(filename)

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

	// Remove registros antigos do DB (file_mods, popularidade, tags)
	s.fileMod.DeleteFileMod(oldName)
	s.pop.ResetPopularity(oldName)
	s.tags.SetFileTags(oldName, nil)
	s.links.ClearLinks(oldName)

	newPath := filepath.Join(s.docsDir, newName)
	var content string

	if data, err := os.ReadFile(newPath); err == nil {
		content = string(data)
		parts := strings.Split(newName, "/")
		base := parts[len(parts)-1]
		newTitle := strings.TrimSuffix(base, ".md")

		// Atualiza a propriedade 'title' no frontmatter da nota física
		if newContent, err := UpdateFrontmatterProperty(content, "title", newTitle); err == nil {
			content = newContent
			if err := os.WriteFile(newPath, []byte(newContent), 0644); err != nil {
				slog.Error("write updated frontmatter after rename", "file", newName, "error", err)
			}
			// Atualiza também o registro correspondente no banco de dados SQLite
			mtimeStr := time.Now().UTC().Format(time.RFC3339)
			if err := s.notes.SaveNote(newName, newContent, mtimeStr); err != nil {
				slog.Error("save updated note content to sqlite after rename", "file", newName, "error", err)
			}
		}
	} else {
		// Fallback para o conteúdo indexado antigo caso a leitura física falhe
		content, _ = s.notes.GetNote(newName)
	}

	if content != "" {
		s.store.DeleteDocumentsByFile(oldName)
		s.store.DeleteFTSByFile(oldName)
		if err := s.reindex(newName, content, time.Now().UTC()); err != nil {
			slog.Error("reindex after rename", "file", newName, "error", err)
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
			if lowerT != "typst" && lowerT != "drawing" && lowerT != "spreadsheet" {
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
			if err := s.reindex(filename, content, mtime); err != nil {
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

func (s *NoteService) reindex(filename, content string, modTime time.Time) error {
	creationTime := modTime

	s.store.DeleteDocumentsByFile(filename)
	s.store.DeleteFTSByFile(filename)

	var docs []processor.Document
	var links []string
	var fileTags []string

	if content != "" {
		docs, links, fileTags = processor.ProcessMarkdownContent(
			[]byte(content), filename, modTime, creationTime,
		)
	} else {
		fullPath := filepath.Join(s.docsDir, filename)
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

	// Filtra o sentinel __no_keywords__ antes de persistir as tags
	cleanTags := processor.FilterKeywords(fileTags)
	if len(cleanTags) > 0 {
		s.tags.SetFileTags(filename, cleanTags)
	} else {
		s.tags.SetFileTags(filename, nil)
	}

	// Extrai e persiste TODOs estruturados
	activeMarkers, err := s.store.GetActiveTodoMarkers()
	if err == nil {
		var markers []string
		for _, m := range activeMarkers {
			markers = append(markers, m.Marker)
		}
		var todos []processor.TodoItem
		if content != "" {
			todos = processor.ExtractTodos(content, filename, modTime, markers)
		} else {
			if contentBytes, err := os.ReadFile(filepath.Join(s.docsDir, filename)); err == nil {
				todos = processor.ExtractTodos(string(contentBytes), filename, modTime, markers)
			}
		}
		if err := s.store.SaveFileTodos(filename, todos); err != nil {
			slog.Error("save todos", "file", filename, "error", err)
		}
	} else {
		slog.Error("get active todo markers", "error", err)
	}

	s.fileMod.SetFileMod(filename, modTime.UTC().Format(time.RFC3339))

	// Extrai keywords via RAKE (apenas se keywords: true ou #keywords)
	var newContent = content
	if processor.HasKeywords(fileTags) {
		keywords := processor.ExtractKeywords(content, processor.KeywordsCount(content))
		if len(keywords) > 0 {
			if err := s.store.SetNoteKeywords(filename, keywords); err != nil {
				slog.Error("set keywords", "file", filename, "error", err)
			}
			updated, err := UpdateFrontmatterProperty(newContent, "keywords", strings.Join(keywords, ", "))
			if err == nil {
				newContent = updated
			}
		} else {
			s.store.SetNoteKeywords(filename, nil)
			updated, err := UpdateFrontmatterProperty(newContent, "keywords", nil)
			if err == nil {
				newContent = updated
			}
		}
	} else {
		s.store.SetNoteKeywords(filename, nil)
		updated, err := UpdateFrontmatterProperty(newContent, "keywords", nil)
		if err == nil {
			newContent = updated
		}
	}

	if newContent != content && content != "" {
		mtimeStr := modTime.UTC().Format(time.RFC3339)
		s.notes.SaveNote(filename, newContent, mtimeStr)
	}

	return nil
}
