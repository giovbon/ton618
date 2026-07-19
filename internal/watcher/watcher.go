package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ton618/internal/core/db"
	"ton618/internal/processor"
)

// processMu serializa chamadas ao ProcessFile para evitar condicao de corrida
// entre o processamento direto (HandleFileSave) e o watcher fsnotify.
var processMu sync.Mutex

// recentlyProcessed tracks files that were recently processed by HandleFileSave
// to prevent the watcher from reprocessing them immediately.
var (
	recentlyProcessedMu sync.RWMutex
	recentlyProcessed   = make(map[string]time.Time)
)

// MarkRecentlyProcessed records that a file was just processed by the HTTP handler.
// The watcher will skip this file for the given duration.
func MarkRecentlyProcessed(filename string) {
	recentlyProcessedMu.Lock()
	recentlyProcessed[filename] = time.Now()
	recentlyProcessedMu.Unlock()
}

// isRecentlyProcessed checks if a file was processed recently (within 3 seconds).
// If so, it removes the entry to avoid permanent skipping.
func isRecentlyProcessed(filename string) bool {
	recentlyProcessedMu.RLock()
	t, ok := recentlyProcessed[filename]
	recentlyProcessedMu.RUnlock()
	if !ok {
		return false
	}
	if time.Since(t) < 3*time.Second {
		recentlyProcessedMu.Lock()
		delete(recentlyProcessed, filename)
		recentlyProcessedMu.Unlock()
		return true
	}
	recentlyProcessedMu.Lock()
	delete(recentlyProcessed, filename)
	recentlyProcessedMu.Unlock()
	return false
}

// MonitoredSubDirs are the subdirectories inside docs/ that the watcher monitors.
var MonitoredSubDirs = []string{"pdfs", "attachments", "archives", "epubs"}

// supportedExts maps file extensions to document types.
var supportedExts = map[string]string{
	".pdf":  "pdf",
	".png":  "imagem",
	".jpg":  "imagem",
	".jpeg": "imagem",
	".gif":  "imagem",
	".webp": "imagem",
	".bmp":  "imagem",
	".svg":  "imagem",
	".zip":  "attachment",
	".epub": "epub",
}

// ── FileEvent ──

// FileEvent is emitted by the watcher when a file is created, modified, or deleted.
type FileEvent struct {
	Path     string
	Filename string
	ModTime  time.Time
	Type     string // "create", "modify", "delete"
}

// ── ProcessFile ──

// ProcessBatch processes multiple file events sequentially, holding the
// processMu mutex for the entire batch. This prevents interleaving with
// other goroutines (e.g., HandleFileSave) and allows SQLite to batch
// internal WAL checkpoints, significantly improving throughput during
// bulk operations like initial indexing.
func ProcessBatch(store *db.Store, events []FileEvent) error {
	processMu.Lock()
	defer processMu.Unlock()

	for _, ev := range events {
		if err := processFileLocked(store, ev); err != nil {
			slog.Error("batch process file", "file", ev.Filename, "error", err)
		}
	}
	return nil
}

// ProcessFile processes a single file event: reads, parses, and indexes the content.
func ProcessFile(store *db.Store, ev FileEvent) error {
	processMu.Lock()
	defer processMu.Unlock()
	return processFileLocked(store, ev)
}

// Processa como imagem (cria documento stub, sem FTS)
func processImageFile(filename string, modTime time.Time, creationTime time.Time) []processor.Document {
	return []processor.Document{{
		ID:        processor.HashFunc("img-" + filename),
		Tipo:      "imagem",
		Arquivo:   filename,
		Secao:     "Anexos / Imagens",
		Texto:     "",
		Timestamp: modTime.UTC().Format(time.RFC3339),
		Created:   creationTime.UTC().Format(time.RFC3339),
		Hash:      processor.CalculateHash("img", "", nil),
		Tags:      nil,
	}}
}

// processFileLocked é a implementação compartilhada entre ProcessFile e ProcessBatch.
// REQUER que processMu já esteja lockado pelo caller.
func processFileLocked(store *db.Store, ev FileEvent) error {

	filename := ev.Filename
	ext := strings.ToLower(filepath.Ext(filename))
	tipo, ok := supportedExts[ext]
	if !ok {
		return nil
	}

	if ev.Type == "delete" {
		store.DeleteDocumentsByFile(filename)
		store.DeleteFTSByFile(filename)
		store.DeleteFileMod(filename)
		store.ResetPopularity(filename)
		store.SetFileTags(filename, nil) // limpa tags
		store.ClearLinks(filename)       // limpa links
		slog.Info("Arquivo removido do índice", "file", filename)
		return nil
	}

	// Anexos (ZIPs): nao deleta docs/FTS — foram criados pelo upload handler
	if tipo == "attachment" {
		// Verifica se o arquivo ja estava registrado (existia file_mod).
		// Usamos isso para detectar perda de dados por race condition:
		// se file_mod existe mas docs nao, algo deletou os docs e precisamos
		// recriar o registro básico.
		existingMod, _ := store.GetFileMod(filename)

		store.SetFileMod(filename, ev.ModTime.UTC().Format(time.RFC3339))

		// Recovery: se o arquivo ja estava registrado mas perdeu os documentos
		// (ex: race condition no pollAll que deletou docs mas nao o zip fisico)
		if existingMod != "" {
			existingDocs, _ := store.GetDocumentsByFile(filename)
			if len(existingDocs) == 0 {
				basename := filepath.Base(filename)
				if strings.HasSuffix(strings.ToLower(basename), ".zip") {
					docID := processor.HashFunc("att-" + basename)
					doc := db.Document{
						ID:        docID,
						Tipo:      "attachment",
						Arquivo:   filename,
						Secao:     "\U0001f4e6 " + basename,
						Texto:     "Anexo ZIP: " + basename,
						Timestamp: ev.ModTime.UTC().Format(time.RFC3339),
						CreatedAt: ev.ModTime.UTC().Format(time.RFC3339),
						Hash:      processor.CalculateHash("att", basename, nil),
					}
					store.InsertDocument(doc)
					store.IndexFTS(doc.ID, doc.Tipo, doc.Arquivo, doc.Secao, doc.Texto, "")
					slog.Info("Documento de anexo recriado pelo watcher", "file", filename)
				}
			}
		}
		return nil
	}

	// Remove old docs for this file
	store.DeleteDocumentsByFile(filename)
	store.DeleteFTSByFile(filename)

	var docs []processor.Document
	var links []string
	var fileTags []string

	creationTime := ev.ModTime

	switch tipo {
	case "markdown":
		docs, links, fileTags = processor.ProcessMarkdown(ev.Path, filename, ev.ModTime, creationTime)
	case "pdf":
		docs, links, fileTags = processor.ProcessPDF(ev.Path, filename, ev.ModTime)
	case "imagem":
		docs = processImageFile(filename, ev.ModTime, creationTime)
	case "epub":
		// Não cria documentos (evita indexação de busca e geração de embeddings)
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

	// Store tags: usa as tags extraídas do frontmatter/hashtags do arquivo (apenas para markdown)
	if tipo == "markdown" {
		cleanTags := fileTags
		if len(cleanTags) > 0 {
			store.SetFileTags(filename, cleanTags)
		} else {
			// Se o arquivo não tem tags, limpa as tags existentes
			store.SetFileTags(filename, nil)
		}
	}

	// Track file mod
	store.SetFileMod(filename, ev.ModTime.UTC().Format(time.RFC3339))

	return nil
}

// ── Internal loops ──

// ScanAndIndexAll varre os diretórios monitorados e indexa tudo que encontrar.
// Chamado uma vez na inicialização do servidor.
func ScanAndIndexAll(store *db.Store, docsDir string) {
	dbFiles, _ := store.GetAllFileMods()
	var batchEvents []FileEvent

	for _, sub := range MonitoredSubDirs {
		dir := filepath.Join(docsDir, sub)
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if _, ok := supportedExts[ext]; !ok {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			relPath, err := filepath.Rel(docsDir, path)
			if err != nil {
				return nil
			}
			relPath = filepath.ToSlash(relPath)

			// Só processa se o mtime mudou desde a última indexação
			if existingMod, exists := dbFiles[relPath]; exists && existingMod != "" {
				dbTime, err := time.Parse(time.RFC3339, existingMod)
				if err == nil && dbTime.Unix() == info.ModTime().Unix() {
					return nil
				}
			}

			batchEvents = append(batchEvents, FileEvent{
				Path:     path,
				Filename: relPath,
				ModTime:  info.ModTime(),
				Type:     "modify",
			})
			return nil
		})
	}

	if len(batchEvents) > 0 {
		slog.Info("Indexando arquivos", "count", len(batchEvents))
		ProcessBatch(store, batchEvents)
	}
}
