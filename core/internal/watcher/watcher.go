package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
	"ton618/core/internal/processor"
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
// A entrada é sempre removida (dentro ou fora da janela) para não pular o
// arquivo para sempre.
func isRecentlyProcessed(filename string) bool {
	recentlyProcessedMu.Lock()
	t, ok := recentlyProcessed[filename]
	delete(recentlyProcessed, filename)
	recentlyProcessedMu.Unlock()
	return ok && time.Since(t) < 3*time.Second
}

// MonitoredSubDirs are the subdirectories inside docs/ that the watcher monitors.
// "images" guarda as imagens enviadas pelo editor (desde 17/09/2026) — entrar
// aqui garante que elas sejam registradas como documento stub no boot/rescan,
// como já acontece com PDFs e anexos.
//
// Derivado de domain.FileDirs (fonte única dos prefixos/diretórios do docs/ —
// DECISIONS §12): notes/ fica de fora de propósito, pois as notas vivem no
// SQLite e não em disco.
var MonitoredSubDirs = append([]string{}, domain.FileDirs...)

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
	return ProcessBatchParallel(store, events, 1)
}

// ProcessBatchParallel processa os eventos com um pool limitado de workers,
// segurando processMu por todo o lote (mesma garantia de exclusividade do
// ProcessBatch). A fase de parse/extração (CPU-bound: markdown, PDF) roda em
// paralelo; as escritas no SQLite continuam serializadas pelo WriteMu interno
// do Store, então a ordem/atomicidade por arquivo é preservada.
//
// workers <= 0 usa automático (min(GOMAXPROCS, 4)); workers == 1 mantém o
// comportamento sequencial. Nunca excede o número de eventos.
func ProcessBatchParallel(store *db.Store, events []FileEvent, workers int) error {
	if len(events) == 0 {
		return nil
	}
	workers = normalizeWorkers(workers, len(events))

	processMu.Lock()
	defer processMu.Unlock()

	eventsCh := make(chan FileEvent)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ev := range eventsCh {
				if err := processFileLocked(store, ev); err != nil {
					slog.Error("parallel process file", "file", ev.Filename, "error", err)
				}
			}
		}()
	}
	for _, ev := range events {
		eventsCh <- ev
	}
	close(eventsCh)
	wg.Wait()
	return nil
}

// normalizeWorkers mantém o nome local usado pelo watcher; a regra é a mesma
// do backup (domain.NormalizeWorkers).
func normalizeWorkers(workers, n int) int {
	return domain.NormalizeWorkers(workers, n)
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
		Hash:      processor.CalculateHash("img", filename, nil),
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
		removeFileFromIndex(store, filename)
		return nil
	}

	// Anexos (ZIPs): nao deleta docs/FTS — foram criados pelo upload handler
	if tipo == "attachment" {
		handleAttachmentEvent(store, filename, ev)
		return nil
	}

	// Remove old docs for this file
	store.DeleteDocumentsByFile(filename)
	store.DeleteFTSByFile(filename)

	docs, links, fileTags := processFileByType(ev, tipo, filename)
	insertDocuments(store, docs)

	for _, link := range links {
		store.AddLink(filename, link)
	}

	// Store tags: usa as tags extraídas do frontmatter/hashtags do arquivo
	// (apenas para markdown). Sem tags, limpa as existentes.
	if tipo == "markdown" {
		storeFileTags(store, filename, fileTags)
	}

	// Track file mod (apenas para arquivos que não são imagens isoladas)
	if tipo != "imagem" {
		store.SetFileMod(filename, ev.ModTime.UTC().Format(time.RFC3339))
	}

	return nil
}

func removeFileFromIndex(store *db.Store, filename string) {
	store.DeleteDocumentsByFile(filename)
	store.DeleteFTSByFile(filename)
	store.DeleteFileMod(filename)
	store.ResetPopularity(filename)
	store.SetFileTags(filename, nil) // limpa tags
	store.ClearLinks(filename)       // limpa links
	slog.Info("Arquivo removido do índice", "file", filename)
}

// handleAttachmentEvent registra o ZIP e, se o arquivo já era conhecido mas
// perdeu os documentos (race condition no pollAll), recria o stub.
func handleAttachmentEvent(store *db.Store, filename string, ev FileEvent) {
	existingMod, _ := store.GetFileMod(filename)
	store.SetFileMod(filename, ev.ModTime.UTC().Format(time.RFC3339))

	if existingMod == "" {
		return
	}
	existingDocs, _ := store.GetDocumentsByFile(filename)
	if len(existingDocs) != 0 {
		return
	}

	basename := filepath.Base(filename)
	if !strings.HasSuffix(strings.ToLower(basename), ".zip") {
		return
	}
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

// processFileByType delega ao parser do tipo; EPUB não gera documentos (evita
// indexação de busca).
func processFileByType(ev FileEvent, tipo, filename string) (docs []processor.Document, links []string, fileTags []string) {
	creationTime := ev.ModTime
	switch tipo {
	case "markdown":
		return processor.ProcessMarkdown(ev.Path, filename, ev.ModTime, creationTime)
	case "pdf":
		return processor.ProcessPDF(ev.Path, filename, ev.ModTime)
	case "imagem":
		return processImageFile(filename, ev.ModTime, creationTime), nil, nil
	}
	return nil, nil, nil
}

func insertDocuments(store *db.Store, docs []processor.Document) {
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
			NoteType:  doc.NoteType,
		}
		if err := store.InsertDocument(dbDoc); err != nil {
			slog.Error("insert doc", "id", doc.ID, "error", err)
			continue
		}
		if err := store.IndexFTS(doc.ID, doc.Tipo, doc.Arquivo, doc.Secao, doc.Texto, db.SliceToTags(doc.Tags)); err != nil {
			slog.Error("index fts", "id", doc.ID, "error", err)
		}
	}
}

func storeFileTags(store *db.Store, filename string, fileTags []string) {
	if len(fileTags) > 0 {
		store.SetFileTags(filename, fileTags)
		return
	}
	store.SetFileTags(filename, nil)
}

func ScanAndIndexAll(store *db.Store, docsDir string) {
	ScanAndIndexAllParallel(store, docsDir, 0)
}

// ScanAndIndexAllParallel é como ScanAndIndexAll, mas permite configurar o
// número de workers do pool de indexação (workers <= 0 = automático).
func ScanAndIndexAllParallel(store *db.Store, docsDir string, workers int) {
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
		slog.Info("Indexando arquivos", "count", len(batchEvents), "workers", normalizeWorkers(workers, len(batchEvents)))
		ProcessBatchParallel(store, batchEvents, workers)
	}
}
