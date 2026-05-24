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
	"ton618/internal/index"

	"github.com/fsnotify/fsnotify"
)

// ── Embedding Worker Pool ──
// O embedding (chamada de rede para Gemini/Ollama/OpenAI) é o gargalo.
// Um worker pool executa essas chamadas concorrentemente, enquanto o
// processamento de arquivos (rápido, só DB) continua serializado pelo mutex.

const numEmbedWorkers = 5

// embedJob carrega os dados necessários para gerar um embedding em background.
type embedJob struct {
	store *db.Store
	docID string
	title string
	text  string
	embed index.EmbeddingProvider
}

var (
	embedQueue   chan embedJob
	embedWg      sync.WaitGroup
	embedOnce    sync.Once
	embedStartMu sync.Mutex
)

// reprojectPending é um flag package-level que o worker de embedding seta
// quando um novo embedding é armazenado. O reprojectLoop verifica este flag
// a cada tick e aciona a reprojeção se necessário.
var reprojectPending bool
var reprojectPendingMu sync.Mutex

// startEmbedWorkers inicializa o worker pool (lazy, chamado na primeira vez).
func startEmbedWorkers(ctx context.Context) {
	embedOnce.Do(func() {
		embedQueue = make(chan embedJob, 200)
		for i := range numEmbedWorkers {
			embedWg.Add(1)
			go embedWorker(ctx, i)
		}
		slog.Info("Worker pool de embeddings iniciado", "workers", numEmbedWorkers)
	})
}

// embedWorker processa jobs de embedding concorrentemente.
// Usa select multiplexado para responder tanto a jobs quanto a cancelamento.
func embedWorker(ctx context.Context, id int) {
	defer embedWg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-embedQueue:
			if !ok {
				return // canal fechado
			}
			slog.Debug("embed worker processando", "worker", id, "doc", job.docID, "arquivo", job.title)
			vec, err := job.embed.Embed(ctx, job.text)
			if err != nil {
				slog.Warn("embedding falhou (worker)", "worker", id, "doc", job.docID, "arquivo", job.title, "error", err)
				continue
			}
			if err := job.store.SetEmbedding(job.docID, vec, job.title); err != nil {
				slog.Error("store embedding (worker)", "worker", id, "doc", job.docID, "error", err)
			} else {
				slog.Info("embedding armazenado (worker)", "worker", id, "doc", job.docID, "arquivo", job.title)
				// Sinaliza que novas embeddings precisam de projeção 2D
				reprojectPendingMu.Lock()
				reprojectPending = true
				reprojectPendingMu.Unlock()
			}
		}
	}
}

// stopEmbedWorkers aguarda a fila esvaziar e para os workers.
func stopEmbedWorkers() {
	close(embedQueue)
	embedWg.Wait()
	embedOnce = sync.Once{} // permite reiniciar se necessário
}

// drainEmbedQueue aguarda a fila de embedding esvaziar sem fechar o canal.
// Usado em testes para garantir que embeddings assincronos foram processados.
func drainEmbedQueue() {
	embedWg.Wait()
}

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
var MonitoredSubDirs = []string{"notes", "links", "voice", "pdfs", "attachments"}

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
	".zip":  "attachment",
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
	embed    index.EmbeddingProvider
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
func (w *Watcher) SetEmbedProvider(embed index.EmbeddingProvider) {
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

	// Inicia worker pool de embeddings (chamadas de rede concorrentes)
	startEmbedWorkers(ctx)

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
// Verifica tanto o flag explícito (QueueReproject) quanto o flag automático
// setado pelo worker pool quando um embedding é armazenado.
func (w *Watcher) reprojectLoop(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(5 * time.Second) // mais responsivo
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Verifica flags explícito e automático
			w.reprojectMu.Lock()
			needed := (w.reprojectNeeded || reprojectPending) && !w.reprojectRunning
			if needed {
				w.reprojectNeeded = false
				reprojectPendingMu.Lock()
				reprojectPending = false
				reprojectPendingMu.Unlock()
			}
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

	// Carrega apenas embeddings que AINDA não têm projeção 2D e têm vetor
	// (limitado a 2000 para evitar OOM com muitas notas)
	const maxReproject = 2000
	unprojected, err := w.store.GetEmbeddings2DWithVectors(maxReproject)
	if err != nil || len(unprojected) < 2 {
		return
	}

	// Coleta vetores mapeando por arquivo (um por arquivo)
	vecs := make(map[string][]float32)
	fileSeen := make(map[string]bool)
	for docID, nv := range unprojected {
		if len(nv.Vector) == 0 {
			continue
		}
		doc, _ := w.store.GetDocument(docID)
		if doc == nil || doc.Arquivo == "" || fileSeen[doc.Arquivo] {
			continue
		}
		fileSeen[doc.Arquivo] = true
		vecs[doc.Arquivo] = nv.Vector
	}

	if len(vecs) < 2 {
		return
	}

	slog.Info("Reprojetando embeddings não-projetados com t-SNE", "total", len(vecs), "max", maxReproject)

	// t-SNE (limitado aos não-projetados — rápido e incremental)
	tsne := index.DefaultTSNE()
	projected := tsne.Project(vecs)

	// Armazena coordenadas no banco
	for arquivo, pt := range projected {
		docs, _ := w.store.GetDocumentsByFile(arquivo)
		if len(docs) > 0 {
			w.store.SetEmbedding2D(docs[0].ID, pt.X, pt.Y)
		}
	}

	slog.Info("Reprojecao t-SNE incremental concluida", "projetados", len(vecs))
}

func (w *Watcher) Events() <-chan FileEvent {
	return w.events
}

// ── ProcessFile ──

// ProcessBatch processes multiple file events sequentially, holding the
// processMu mutex for the entire batch. This prevents interleaving with
// other goroutines (e.g., HandleFileSave) and allows SQLite to batch
// internal WAL checkpoints, significantly improving throughput during
// bulk operations like initial indexing.
func ProcessBatch(store *db.Store, events []FileEvent, embed index.EmbeddingProvider, embedAll bool) error {
	processMu.Lock()
	defer processMu.Unlock()

	for _, ev := range events {
		if err := processFileLocked(store, ev, embed, embedAll); err != nil {
			slog.Error("batch process file", "file", ev.Filename, "error", err)
		}
	}
	return nil
}

// ProcessFile processes a single file event: reads, parses, indexes, and optionally embeds the content.
func ProcessFile(store *db.Store, ev FileEvent, embed index.EmbeddingProvider, embedAll bool) error {
	processMu.Lock()
	defer processMu.Unlock()
	return processFileLocked(store, ev, embed, embedAll)
}

// processFileLocked é a implementação compartilhada entre ProcessFile e ProcessBatch.
// REQUER que processMu já esteja lockado pelo caller.
func processFileLocked(store *db.Store, ev FileEvent, embed index.EmbeddingProvider, embedAll bool) error {

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

	// Anexos (ZIPs): nao deleta docs/FTS — foram criados pelo upload handler
	if tipo == "attachment" {
		store.SetFileMod(filename, ev.ModTime.Format(time.RFC3339))
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
	// Busca tags do arquivo no banco (gerenciadas via toggle-embed na UI)
	existingFileTags, _ := store.GetFileTags(filename)

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

		// Generate embedding via worker pool (assíncrono) se o provider estiver configurado.
		// Para TODOS os tipos de documento: verifica se deve gerar embedding.
		// As fontes possiveis da tag "embed" sao:
		// 1. Frontmatter do markdown (doc.Tags)
		// 2. File-level tags no banco (toggle-embed na UI)
		// 3. EMBEDDING_ALL=true
		allTags := append(doc.Tags, existingFileTags...)
		if embed != nil && shouldEmbed(allTags, embedAll) {
			textToEmbed := doc.Secao + " " + doc.Texto
			textToEmbed = strings.TrimSpace(textToEmbed)
			if textToEmbed != "" && len(textToEmbed) > 10 {
				title := doc.Secao
				if title == "" {
					title = doc.Arquivo
				}
				// Enfileira o job para o worker pool — não bloqueia o mutex.
				select {
				case embedQueue <- embedJob{store: store, docID: doc.ID, title: title, text: textToEmbed, embed: embed}:
				default:
					slog.Warn("fila de embedding cheia, pulando", "id", doc.ID, "arquivo", doc.Arquivo)
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

	// Skip if this file was just processed by HandleFileSave
	if isRecentlyProcessed(relPath) {
		slog.Debug("Skipping recently processed file", "file", relPath)
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
	var batchEvents []FileEvent

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
			batchEvents = append(batchEvents, FileEvent{
				Path:     path,
				Filename: relPath,
				ModTime:  info.ModTime(),
				Type:     "modify",
			})
			return nil
		})
	}

	// 2. Processa lotes de eventos em transação única (mais rápido)
	if len(batchEvents) > 0 {
		if len(batchEvents) == 1 {
			// Evento único: ProcessFile já faz lock + transação implícita
			w.events <- batchEvents[0]
		} else {
			// Vários eventos: processa em lote com transação única
			slog.Info("Processando lote de arquivos", "count", len(batchEvents))
			if err := ProcessBatch(w.store, batchEvents, w.embed, w.embedAll); err != nil {
				slog.Error("batch process error", "error", err)
			}
		}
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
