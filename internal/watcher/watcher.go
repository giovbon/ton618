package watcher

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ton618/internal/config"
	"ton618/internal/db"
	"ton618/internal/processor"

	"github.com/fsnotify/fsnotify"
)

// MonitoredSubDirs are the subdirectories inside docs/ that the watcher monitors.
var MonitoredSubDirs = []string{"notes", "links", "voice"}

// supportedExts maps file extensions to their internal type identifier.
var supportedExts = map[string]string{
	".md":   "markdown",
	".pdf":  "pdf",
	".png":  "imagem",
	".jpg":  "imagem",
	".jpeg": "imagem",
}

// FileEvent representa um arquivo que precisa ser processado.
type FileEvent struct {
	Path     string
	Filename string
	ModTime  time.Time
	Type     string // "create", "modify", "delete"
}

// Watcher é o monitor de sistema de arquivos.
type Watcher struct {
	cfg     *config.AppConfig
	store   *db.Store
	events  chan FileEvent
	watcher *fsnotify.Watcher
	wg      sync.WaitGroup
}

// NewWatcher cria um novo monitor.
func NewWatcher(cfg *config.AppConfig, store *db.Store) *Watcher {
	return &Watcher{
		cfg:    cfg,
		store:  store,
		events: make(chan FileEvent, 100),
	}
}

// Start inicia o watcher (fsnotify + polling).
func (w *Watcher) Start(ctx context.Context) {
	// Try fsnotify
	fsWatcher, err := fsnotify.NewWatcher()
	if err == nil {
		w.watcher = fsWatcher

		// Add directories to watch
		for _, sub := range MonitoredSubDirs {
			dir := filepath.Join(w.cfg.DocsDir, sub)
			os.MkdirAll(dir, 0755)
			w.watcher.Add(dir)
		}

		w.wg.Add(1)
		go w.fsnotifyLoop(ctx)

		slog.Info("Watcher fsnotify iniciado")
	} else {
		slog.Warn("fsnotify indisponível, usando apenas polling", "error", err)
	}

	// Polling fallback
	w.wg.Add(1)
	go w.pollingLoop(ctx)
}

// Stop encerra o watcher.
func (w *Watcher) Stop() {
	if w.watcher != nil {
		w.watcher.Close()
	}
	close(w.events)
	w.wg.Wait()
}

// Events retorna o canal de eventos.
func (w *Watcher) Events() <-chan FileEvent {
	return w.events
}

// ProcessFile processa um evento de arquivo (indexa no banco).
func ProcessFile(store *db.Store, ev FileEvent) error {
	filename := ev.Filename
	ext := strings.ToLower(filepath.Ext(filename))
	tipo, ok := supportedExts[ext]
	if !ok {
		return nil
	}

	if ev.Type == "delete" {
		store.DeleteDocumentsByFile(filename)
		store.DeleteFTSByFile(filename)
		// NOTE: DeleteEmbedding takes a doc_id, not a filename. Orphaned embeddings
		// for this file will remain in the table until a future cleanup pass.
		store.DeleteEmbedding(filename)
		return nil
	}

	content, err := os.ReadFile(ev.Path)
	if err != nil {
		return err
	}

	// Remove old docs for this file
	store.DeleteDocumentsByFile(filename)
	store.DeleteFTSByFile(filename)

	var docs []processor.Document
	var links []string
	var fileTags []string

	creationTime := ev.ModTime // Simplified: use modtime as creation time

	switch tipo {
	case "markdown":
		docs, links, fileTags = processor.ProcessMarkdown(ev.Path, filename, ev.ModTime, creationTime)
	case "imagem":
		// Images are indexed with minimal text (OCR not in this version)
		docs = []processor.Document{{
			ID:         processor.HashFunc("img-" + filename),
			Tipo:       "imagem",
			Arquivo:    filename,
			Secao:      "Anexos / Imagens",
			Texto:      "",
			Timestamp:  ev.ModTime.UTC().Format(time.RFC3339),
			Created:    creationTime.UTC().Format(time.RFC3339),
			Hash:       processor.CalculateHash("img", "", nil),
			VectorHash: processor.CalculateVectorHash("img", ""),
			Tags:       nil,
		}}
	}

	// Insert documents
	for _, doc := range docs {
		dbDoc := db.Document{
			ID:         doc.ID,
			Tipo:       doc.Tipo,
			Arquivo:    doc.Arquivo,
			Secao:      doc.Secao,
			Texto:      doc.Texto,
			Tags:       db.SliceToTags(doc.Tags),
			Pagina:     doc.Pagina,
			Ordem:      doc.Ordem,
			Timestamp:  doc.Timestamp,
			CreatedAt:  doc.Created,
			Hash:       doc.Hash,
			VectorHash: doc.VectorHash,
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

	// Store tags
	if len(fileTags) > 0 {
		store.SetFileTags(filename, fileTags)
	}

	// Track file mod
	store.SetFileMod(filename, ev.ModTime.Format(time.RFC3339))

	return nil
}

// ── Internal loops ──

func (w *Watcher) fsnotifyLoop(ctx context.Context) {
	defer w.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
				w.handleEvent(event.Name)
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("fsnotify error", "error", err)
		}
	}
}

func (w *Watcher) pollingLoop(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollIntervalSec)
	defer ticker.Stop()
	defer w.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollAll()
		}
	}
}

func (w *Watcher) handleEvent(absPath string) {
	rel, err := filepath.Rel(w.cfg.DocsDir, absPath)
	if err != nil {
		return
	}
	// Check if it's in a monitored subdir
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 2 {
		return
	}
	isMonitored := false
	for _, m := range MonitoredSubDirs {
		if parts[0] == m {
			isMonitored = true
			break
		}
	}
	if !isMonitored {
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return
	}
	if info.IsDir() {
		return
	}

	select {
	case w.events <- FileEvent{
		Path:     absPath,
		Filename: rel,
		ModTime:  info.ModTime(),
		Type:     "modify",
	}:
	default:
		slog.Warn("event channel full, dropping", "file", rel)
	}
}

// PollAll forces an immediate scan of all monitored directories.
func (w *Watcher) PollAll() {
	w.pollAll()
}

func (w *Watcher) pollAll() {
	for _, sub := range MonitoredSubDirs {
		dir := filepath.Join(w.cfg.DocsDir, sub)
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

			rel, _ := filepath.Rel(w.cfg.DocsDir, path)
			cachedMod, err := w.store.GetFileMod(rel)
			if err != nil {
				cachedMod = ""
			}
			currentMod := info.ModTime().Format(time.RFC3339)

			if cachedMod != currentMod {
				select {
				case w.events <- FileEvent{
					Path:     path,
					Filename: rel,
					ModTime:  info.ModTime(),
					Type:     "modify",
				}:
				default:
				}
			}
			return nil
		})
	}
}
