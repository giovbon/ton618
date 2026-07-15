package watcher

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ton618/internal/core/config"
	"ton618/internal/core/db"
	"ton618/internal/core/services"
	"ton618/internal/processor"

	"github.com/fsnotify/fsnotify"
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

// ── Watcher ──

const debounceInterval = 300 * time.Millisecond

type Watcher struct {
	cfg     *config.AppConfig
	store   *db.Store
	watcher *fsnotify.Watcher
	events  chan FileEvent
	wg      sync.WaitGroup

	// Debounce agrupa eventos repetidos do fsnotify para o mesmo arquivo.
	// Quando um evento chega, um timer de debounceInterval é iniciado.
	// Se outro evento para o mesmo arquivo chegar antes do timer disparar,
	// o timer é reiniciado. Só após o período sem novos eventos o evento
	// é enviado ao canal `events` para processamento.
	debounceMu    sync.Mutex
	debounceTimers map[string]*time.Timer

	ntfyService *services.NtfyService
}

// NewWatcher creates a new watcher instance.
func NewWatcher(cfg *config.AppConfig, store *db.Store) *Watcher {
	return &Watcher{
		cfg:            cfg,
		store:          store,
		events:         make(chan FileEvent, 100),
		debounceTimers: make(map[string]*time.Timer),
		ntfyService:    services.NewNtfyService(store),
	}
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

	w.wg.Add(2)
	go w.fsnotifyLoop(ctx)
	go w.pollLoop(ctx)
	slog.Info("Watcher fsnotify iniciado")
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

func (w *Watcher) fsnotifyLoop(ctx context.Context) {
	defer w.wg.Done()

	// Limpa todos os timers de debounce ao finalizar
	defer func() {
		w.debounceMu.Lock()
		for _, t := range w.debounceTimers {
			t.Stop()
		}
		w.debounceTimers = nil
		w.debounceMu.Unlock()
	}()

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

// debounceEvent agrupa eventos repetidos do fsnotify para o mesmo arquivo.
// Cria ou reinicia um timer de debounceInterval. Quando o timer dispara,
// o evento é enviado ao canal `events` para processamento.
func (w *Watcher) debounceEvent(ev FileEvent) {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	// Se já existe um timer para este arquivo, reinicia
	if t, ok := w.debounceTimers[ev.Filename]; ok {
		t.Stop()
		t.Reset(debounceInterval)
		return
	}

	// Cria um novo timer
	w.debounceTimers[ev.Filename] = time.AfterFunc(debounceInterval, func() {
		// Remove o timer do mapa após disparar
		w.debounceMu.Lock()
		delete(w.debounceTimers, ev.Filename)
		w.debounceMu.Unlock()

		// Re-estatísticas do arquivo antes de enviar (mtime pode ter mudado)
		if ev.Type == "modify" {
			info, err := os.Stat(ev.Path)
			if err == nil {
				ev.ModTime = info.ModTime()
			}
		}

		w.events <- ev
	})
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

	w.debounceEvent(FileEvent{
		Path:     absPath,
		Filename: relPath,
		ModTime:  info.ModTime(),
		Type:     "modify",
	})
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

	// Delete events also go through debounce to prevent race
	// with subsequent create events (e.g. editor temp files)
	w.debounceEvent(FileEvent{
		Path:     absPath,
		Filename: relPath,
		Type:     "delete",
	})
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
	// 0. Carrega mods do DB em memória para evitar N+1 queries
	dbFiles, _ := w.store.GetAllFileMods()

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

			// SO processa se o mtime mudou desde a ultima indexacao
			if existingMod, exists := dbFiles[relPath]; exists && existingMod != "" {
				// Parse do mtime do banco e comparação ignorando sub-segundos
				dbTime, err := time.Parse(time.RFC3339, existingMod)
				if err == nil && dbTime.Unix() == info.ModTime().Unix() {
					// Mtime igual (no mesmo segundo) — arquivo nao mudou, pula
					diskFiles[relPath] = true
					return nil
				}
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
			if err := ProcessBatch(w.store, batchEvents); err != nil {
				slog.Error("batch process error", "error", err)
			}
		}
	}

	// 2. Remove do banco arquivos que existem no DB mas não estão no disco
	for filename := range dbFiles {
		if !diskFiles[filename] {
			// Pula arquivos que não têm extensão monitorada pelo watcher
			ext := strings.ToLower(filepath.Ext(filename))
			if _, ok := supportedExts[ext]; !ok {
				continue
			}
			// Pula arquivos processados recentemente pelo HTTP handler
			// (ex: upload de ZIP onde o pollAll escaneou antes do arquivo
			// terminar de ser escrito, mas o registro no DB já foi feito).
			if isRecentlyProcessed(filename) {
				slog.Debug("PollAll: pulando delete de arquivo recentemente processado", "file", filename)
				continue
			}
			fullPath := filepath.Join(w.cfg.DocsDir, filename)
			slog.Info("Arquivo deletado (detectado no poll)", "file", filename)
			w.events <- FileEvent{
				Path:     fullPath,
				Filename: filename,
				Type:     "delete",
			}
		}
	}

	// 3. Verifica se precisa enviar notificações do calendário
	go func() {
		w.ntfyService.CheckAndSendDailyAppointments()
		w.ntfyService.CheckAndSendWeeklySummary()
	}()
}
