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
	"ton618/internal/semantic"

	"github.com/fsnotify/fsnotify"
)

// processMu serializa chamadas ao ProcessFile para evitar condicao de corrida
// entre o processamento direto (HandleFileSave) e o watcher fsnotify.
var processMu sync.Mutex

// MonitoredSubDirs are the subdirectories inside docs/ that the watcher monitors.
var MonitoredSubDirs = []string{"notes", "links", "voice", "pdfs"}

// supportedExts maps file extensions to document types.
var supportedExts = map[string]string{
	".md":   "markdown",
	".pdf":  "pdf",
	".png":  "imagem",
	".jpg":  "imagem",
	".jpeg": "imagem",
	".gif":  "imagem",
	".webp": "imagem",
	".bmp":  "imagem",
	".svg":  "imagem",
}

// ── FileEvent ──

// FileEvent is emitted by the watcher when a file is created, modified, or deleted.
type FileEvent struct {
	Path     string
	Filename string
	ModTime  time.Time
	Type     string // "create", "modify", "delete"
}

// ── Watcher ──

type Watcher struct {
	cfg      *config.AppConfig
	store    *db.Store
	embed    semantic.EmbeddingProvider
	embedAll bool
	watcher  *fsnotify.Watcher
	events   chan FileEvent
	wg       sync.WaitGroup

	// reprojectNeeded é setado quando novas embeddings precisam ser projetadas
	// com t-SNE (o PCA é usado como fallback instantâneo até o t-SNE ficar pronto).
	reprojectMu      sync.Mutex
	reprojectNeeded  bool
	reprojectRunning bool
}

// NewWatcher creates a new watcher instance.
func NewWatcher(cfg *config.AppConfig, store *db.Store) *Watcher {
	return &Watcher{
		cfg:      cfg,
		store:    store,
		embedAll: cfg.EmbeddingAll,
		events:   make(chan FileEvent, 100),
	}
}

// SetEmbedProvider sets the embedding provider for generating vectors.
func (w *Watcher) SetEmbedProvider(embed semantic.EmbeddingProvider) {
	w.embed = embed
}

// Start launches the fsnotify watcher and polling loop.
func (w *Watcher) Start(ctx context.Context) {
	var err error
	w.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		slog.Error("criar watcher", "error", err)
		return
	}

	// Watch monitored subdirectories
	for _, sub := range MonitoredSubDirs {
		dir := filepath.Join(w.cfg.DocsDir, sub)
		os.MkdirAll(dir, 0755)
		w.watcher.Add(dir)
	}

	w.wg.Add(3)
	go w.fsnotifyLoop(ctx)
	go w.pollLoop(ctx)
	go w.reprojectLoop(ctx)
	slog.Info("Watcher fsnotify iniciado")
}

// QueueReproject marca que as embeddings precisam ser reprojetadas com t-SNE.
// O PCA continua sendo usado como fallback instantâneo até o t-SNE ficar pronto.
func (w *Watcher) QueueReproject() {
	w.reprojectMu.Lock()
	w.reprojectNeeded = true
	w.reprojectMu.Unlock()
}

// reprojectLoop roda em background e reprojeta embeddings com t-SNE quando necessário.
func (w *Watcher) reprojectLoop(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.reprojectMu.Lock()
			needed := w.reprojectNeeded && !w.reprojectRunning
			w.reprojectMu.Unlock()
			if needed {
				w.runReprojection()
			}
		}
	}
}

func (w *Watcher) runReprojection() {
	w.reprojectMu.Lock()
	w.reprojectRunning = true
	w.reprojectNeeded = false
	w.reprojectMu.Unlock()

	defer func() {
		w.reprojectMu.Lock()
		w.reprojectRunning = false
		w.reprojectMu.Unlock()
	}()

	embeddings, err := w.store.GetAllEmbeddings()
	if err != nil || len(embeddings) < 2 {
		return
	}

	// Coleta vetores de todas as embeddings
	vecs := make(map[string][]float32)
	for docID, nv := range embeddings {
		if len(nv.Vector) > 0 {
			doc, _ := w.store.GetDocument(docID)
			if doc != nil && doc.Arquivo != "" {
				vecs[doc.Arquivo] = nv.Vector
			}
		}
	}

	if len(vecs) < 2 {
		return
	}

	slog.Info("Reprojetando embeddings com t-SNE", "total", len(vecs))

	// t-SNE (sem limite - roda em background)
	tsne := semantic.DefaultTSNE()
	projected := tsne.Project(vecs)

	// Armazena coordenadas no banco
	for arquivo, pt := range projected {
		docs, _ := w.store.GetDocumentsByFile(arquivo)
		if len(docs) > 0 {
			w.store.SetEmbedding2D(docs[0].ID, pt.X, pt.Y)
		}
	}

	slog.Info("Reprojecao t-SNE concluida", "total", len(vecs))
}

func (w *Watcher) Events() <-chan FileEvent {
	return w.events
}

// ── ProcessFile ──

// ProcessFile processes a single file event: reads, parses, indexes, and optionally embeds the content.
// O mutex processMu serializa chamadas concorrentes para evitar duplicacao
// quando o HandleFileSave e o fsnotify disparam simultaneamente.
func ProcessFile(store *db.Store, ev FileEvent, embed semantic.EmbeddingProvider, embedAll bool) error {
	processMu.Lock()
	defer processMu.Unlock()

	filename := ev.Filename
	ext := strings.ToLower(filepath.Ext(filename))
	tipo, ok := supportedExts[ext]
	if !ok {
		return nil
	}

	if ev.Type == "delete" {
		store.DeleteDocumentsByFile(filename)
		store.DeleteFTSByFile(filename)
		store.DeleteEmbeddingsByFile(filename)
		store.DeleteFileMod(filename)
		store.ResetPopularity(filename)
		store.SetFileTags(filename, nil) // limpa tags
		slog.Info("Arquivo removido do índice", "file", filename)
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

		// Generate embedding if provider is set and note should be embedded
		// PDFs: apenas se o nome do arquivo contiver "embed" (ex: "-embed-", "livrotalembed")
		// Markdown: segue a regra shouldEmbed (tag "embed" no frontmatter ou EMBEDDING_ALL=true)
		if embed != nil && ((doc.Tipo == "pdf" && strings.Contains(strings.ToLower(doc.Arquivo), "embed")) || (doc.Tipo == "markdown" && shouldEmbed(doc.Tags, embedAll))) {
			textToEmbed := doc.Secao + " " + doc.Texto
			textToEmbed = strings.TrimSpace(textToEmbed)
			if textToEmbed != "" && len(textToEmbed) > 10 {
				vec, err := embed.Embed(context.Background(), textToEmbed)
				if err != nil {
					slog.Warn("embedding failed", "id", doc.ID, "arquivo", doc.Arquivo, "error", err)
				} else {
					title := doc.Secao
					if title == "" {
						title = doc.Arquivo
					}
					if err := store.SetEmbedding(doc.ID, vec, title); err != nil {
						slog.Error("store embedding", "id", doc.ID, "error", err)
					} else {
						slog.Info("embedding stored", "id", doc.ID, "arquivo", doc.Arquivo)
					}
				}
			}
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

// shouldEmbed verifies if this document should have an embedding generated.
func shouldEmbed(tags []string, embedAll bool) bool {
	if embedAll {
		return true
	}
	for _, t := range tags {
		if t == "embed" {
			return true
		}
	}
	return false
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
			switch {
			case event.Op&(fsnotify.Create|fsnotify.Write) != 0:
				w.handleCreateOrMod(event.Name)
			case event.Op&(fsnotify.Remove|fsnotify.Rename) != 0:
				w.handleDelete(event.Name)
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("watcher error", "error", err)
		}
	}
}

// relPathFromAbs converte um caminho absoluto para relativo a DocsDir
// e normaliza para usar forward slashes.
func (w *Watcher) relPathFromAbs(absPath string) (string, bool) {
	rel, err := filepath.Rel(w.cfg.DocsDir, absPath)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "" || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return rel, true
}

func (w *Watcher) handleCreateOrMod(absPath string) {
	relPath, ok := w.relPathFromAbs(absPath)
	if !ok {
		return
	}

	ext := strings.ToLower(filepath.Ext(relPath))
	if _, ok := supportedExts[ext]; !ok {
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return
	}

	w.events <- FileEvent{
		Path:     absPath,
		Filename: relPath,
		ModTime:  info.ModTime(),
		Type:     "modify",
	}
}

func (w *Watcher) handleDelete(absPath string) {
	relPath, ok := w.relPathFromAbs(absPath)
	if !ok {
		return
	}

	ext := strings.ToLower(filepath.Ext(relPath))
	if _, ok := supportedExts[ext]; !ok {
		return
	}

	slog.Info("Arquivo deletado", "file", relPath)
	w.events <- FileEvent{
		Path:     absPath,
		Filename: relPath,
		Type:     "delete",
	}
}

func (w *Watcher) pollLoop(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.cfg.PollIntervalSec)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollAll()
		}
	}
}

// PollAll forces an immediate scan of all monitored directories.
func (w *Watcher) PollAll() {
	w.pollAll()
}

// relPathFromWalk normaliza o caminho retornado por filepath.WalkDir
// para relativo a DocsDir com forward slashes.
func (w *Watcher) relPathFromWalk(path string) (string, bool) {
	rel, err := filepath.Rel(w.cfg.DocsDir, path)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "" || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return rel, true
}

func (w *Watcher) pollAll() {
	// 1. Escaneia arquivos no disco
	diskFiles := make(map[string]bool)
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
			relPath, ok := w.relPathFromWalk(path)
			if !ok {
				return nil
			}
			diskFiles[relPath] = true
			w.events <- FileEvent{
				Path:     path,
				Filename: relPath,
				ModTime:  info.ModTime(),
				Type:     "modify",
			}
			return nil
		})
	}

	// 2. Remove do banco arquivos que existem no DB mas não estão no disco
	dbFiles, _ := w.store.GetAllFileMods()
	for filename := range dbFiles {
		if !diskFiles[filename] {
			fullPath := filepath.Join(w.cfg.DocsDir, filename)
			slog.Info("Arquivo deletado (detectado no poll)", "file", filename)
			w.events <- FileEvent{
				Path:     fullPath,
				Filename: filename,
				Type:     "delete",
			}
		}
	}
}
