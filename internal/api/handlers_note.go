package api

import (
	"log/slog"
	"time"

	"ton618/internal/db"
	"ton618/internal/processor"
)

// reindexNote processes a note's content through the markdown processor
// and updates all related indexes (documents, FTS, links, tags, file_mods).
func (ctx *HandlerContext) reindexNote(filename string, content string, modTime time.Time) error {
	store := ctx.Store

	// Remove old docs/FTS for this note
	store.DeleteDocumentsByFile(filename)
	store.DeleteFTSByFile(filename)

	// Process the markdown content directly (no temp file)
	docs, links, fileTags := processor.ProcessMarkdownContent([]byte(content), filename, modTime, modTime)

	// Insert documents and FTS entries
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
		if err := store.InsertDocument(dbDoc); err != nil {
			slog.Error("insert doc", "id", doc.ID, "error", err)
			continue
		}
		if err := store.IndexFTS(doc.ID, doc.Tipo, doc.Arquivo, doc.Secao, doc.Texto, db.SliceToTags(doc.Tags)); err != nil {
			slog.Error("index fts", "id", doc.ID, "error", err)
		}
	}

	// Store links
	for _, link := range links {
		store.AddLink(filename, link)
	}

	// Filtra o sentinel __no_keywords__ antes de persistir as tags
	cleanTags := processor.FilterNoKeywords(fileTags)
	if len(cleanTags) > 0 {
		store.SetFileTags(filename, cleanTags)
	} else {
		store.SetFileTags(filename, nil)
	}

	// Track file mod
	store.SetFileMod(filename, modTime.UTC().Format(time.RFC3339))

	// Extrai keywords via RAKE (quantidade varia conforme o tamanho do texto)
	// Ignorado se a nota tiver no_keywords: true ou tag no-keywords
	if !processor.HasNoKeywords(fileTags) {
		keywords := processor.ExtractKeywords(content, processor.KeywordsCount(content))
		if len(keywords) > 0 {
			if err := store.SetNoteKeywords(filename, keywords); err != nil {
				slog.Error("set keywords", "file", filename, "error", err)
			}
		} else {
			store.SetNoteKeywords(filename, nil)
		}
	} else {
		store.SetNoteKeywords(filename, nil)
	}

	return nil
}
