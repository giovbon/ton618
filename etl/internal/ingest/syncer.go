package ingest

import (
	"context"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"etl/internal/config"
	"etl/internal/events"
	"etl/internal/models"
	"etl/internal/search"
	"etl/internal/semantic"
)

// syncMu é usado internamente pelo RunSync e SyncCoordinator para evitar conflitos
// entre operações de sync e operações de rename/delete da API.
var syncMu sync.Mutex
var (
	indexingFiles = make(map[string]bool)
	indexingMu    sync.RWMutex
)

// SetFileIndexing marca um arquivo como em processamento (ou concluído) e avisa o frontend.
func SetFileIndexing(filename string, isIndexing bool) {
	indexingMu.Lock()
	defer indexingMu.Unlock()
	if isIndexing {
		indexingFiles[filename] = true
		events.GetHub().Broadcast("file:vectorizing", map[string]string{"filename": filename})
	} else {
		delete(indexingFiles, filename)
		events.GetHub().Broadcast("file:ready", map[string]string{"filename": filename})
		search.InvalidateFile(filename)
	}
}

// IsFileIndexing retorna se um arquivo está sendo vetorizado no momento.
func IsFileIndexing(filename string) bool {
	indexingMu.RLock()
	defer indexingMu.RUnlock()
	return indexingFiles[filename]
}

// HasNoEmbedTag verifica se a tag 'no-embed' está presente na lista.
func HasNoEmbedTag(tags []string) bool {
	for _, t := range tags {
		if strings.ToLower(t) == "no-embed" {
			return true
		}
	}
	return false
}

// HasEmbedTag verifica se a tag 'embed' está presente na lista (Modo Whitelist).
func HasEmbedTag(tags []string) bool {
	for _, t := range tags {
		if strings.ToLower(t) == "embed" {
			return true
		}
	}
	return false
}

// StartWatcher monitora DOCS_DIR por mudanças usando fsnotify (preferencial)
// ou polling como fallback em plataformas sem suporte a inotify/kqueue.
func StartWatcher(ctx context.Context, cfg *config.AppConfig, appState *AppState, coordinator *SyncCoordinator) {
	// Tentar fsnotify primeiro
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("fsnotify nao disponivel, usando polling fallback", "error", err)
		startPollingWatcher(ctx, cfg, appState, coordinator)
		return
	}
	defer watcher.Close()

	// Adicionar DOCS_DIR e subdiretórios recursivamente
	if err := addDirsRecursively(watcher, cfg.DocsDir); err != nil {
		slog.Error("Erro ao adicionar diretorios ao fsnotify, usando polling fallback", "error", err)
		startPollingWatcher(ctx, cfg, appState, coordinator)
		return
	}

	slog.Info("fsnotify watcher iniciado", "dir", cfg.DocsDir)

	// Sincronização de boot
	RunSync(cfg, false, "auto", appState)

	// Loop principal de eventos
	for {
		select {
		case <-ctx.Done():
			slog.Info("Watcher encerrando por sinal do sistema")
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Se um novo diretório for criado, adicioná-lo ao watcher
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					watcher.Add(event.Name)
				}
			}
			// Qualquer evento de arquivo dispara sync (Create, Write, Remove, Rename)
			if coordinator != nil {
				coordinator.Push("global", JobFullSync, false)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			slog.Error("Erro no fsnotify watcher", "error", err)
		}
	}
}

// addDirsRecursively adiciona todos os subdiretórios de rootDir ao watcher.
func addDirsRecursively(watcher *fsnotify.Watcher, rootDir string) error {
	return filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
}

// startPollingWatcher é o comportamento antigo baseado em polling, mantido como fallback.
func startPollingWatcher(ctx context.Context, cfg *config.AppConfig, appState *AppState, coordinator *SyncCoordinator) {
	checkInterval := cfg.PollIntervalSec
	if checkInterval == 0 {
		checkInterval = 5 * time.Second
	}

	slog.Info("Watcher: Modo Polling ativo", "interval", checkInterval)

	// Sincronização de boot
	RunSync(cfg, false, "auto", appState)

	lastSyncTime := getLatestWriteTime(cfg.DocsDir)
	if lastSyncTime.IsZero() {
		lastSyncTime = time.Now()
	}
	var lastDetectedChange time.Time

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Polling watcher encerrando por sinal do sistema")
			return
		case <-ticker.C:
			currentLatest := getLatestWriteTime(cfg.DocsDir)
			if currentLatest.After(lastSyncTime) {
				if currentLatest.After(lastDetectedChange) {
					lastDetectedChange = currentLatest
				}
				if coordinator != nil {
					coordinator.Push("global", JobFullSync, false)
				}
				lastSyncTime = currentLatest
			}
		}
	}
}

func getLatestWriteTime(baseDir string) time.Time {
	var latest time.Time
	filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest
}

func RunSync(cfg *config.AppConfig, forceScanIndex bool, mode string, appState *AppState) {
	syncMu.Lock()
	// Liberamos o lock logo após a varredura para permitir que a API de exclusão/renomeação
	// funcione enquanto o motor semântico (lento) está rodando.

	// Realiza limpeza de registros órfãos (arquivos deletados manualmente)
	GlobalVacuum(cfg, appState)
	// Limpeza de pacotes agora é manual via exclusão da nota (cascata)
	events.GetHub().Broadcast("sync:started", map[string]string{"mode": mode})

	docs, deletedFiles, validIdsMap := ProcessDocs(cfg, forceScanIndex, appState)
	syncMu.Unlock() // LIBERADO: Agora a API pode excluir arquivos enquanto o RunSync termina.

	if len(deletedFiles) > 0 {
		for _, file := range deletedFiles {
			log.Printf("[Sync] Detectada Deleção: %s\n", file)
			relPath, _ := filepath.Rel(cfg.DocsDir, file)

			deletedIDs := CollectBleveIDsForFile(cfg, relPath)
			DeleteFileFromBleve(cfg, relPath)

			appState.DeleteFileTags(relPath)
			appState.DeleteFileLinks(relPath)
			appState.DeleteFileMetadata(relPath)
			appState.DeleteHashesByIDs(deletedIDs)
		}
		appState.Save(cfg)
		semantic.SaveCache() // Invalidação por deleção
	}

	// Reconstruir o mapa global de autoridade por links (Backlinks)
	allLinks := appState.GetAllFileLinks()
	newCounts := make(map[string]int)
	for _, links := range allLinks {
		for _, target := range links {
			newCounts[target]++
		}
	}
	appState.UpdateLinkCounts(newCounts)

	if len(docs) > 0 {
		var bleveDocs []models.Document
		var vectorDocs []models.Document

		for _, doc := range docs {
			// 1. Verificar se precisa indexar no Bleve (léxico)
			oldHash, exists := appState.GetHash(doc.ID)
			hasChanged := forceScanIndex || !exists || oldHash != doc.Hash

			if hasChanged {
				bleveDocs = append(bleveDocs, doc)
				appState.SetHash(doc.ID, doc.Hash)
			}

			// 2. Verificar se precisa vetorizar (semântico)
			if appState.GetSettings().SemanticEnable {
				oldVecHash, vecExists := appState.GetVectorHash(doc.ID)
				if !vecExists || oldVecHash != doc.VectorHash {
					vectorDocs = append(vectorDocs, doc)
				}
			}
		}

		if len(bleveDocs) > 0 {
			log.Printf("Indexando %d fragmentos (Bleve: %d, Vetores: %d)...\n", len(docs), len(bleveDocs), len(vectorDocs))

			filesToVectorize := make(map[string]bool)
			for _, doc := range vectorDocs {
				filesToVectorize[doc.Arquivo] = true
			}
			for f := range filesToVectorize {
				SetFileIndexing(f, true)
			}

			SendToEngines(cfg, bleveDocs, vectorDocs, appState)

			semantic.SaveCache()
		} else {
			log.Println("[Sync] Nenhum conteúdo de fragmento alterado. Pulando indexação.")
		}
		for filename, expectedIds := range validIdsMap {
			VacuumOrphans(cfg, filename, expectedIds)
		}

		processedInBatch := make(map[string]bool)
		for _, doc := range docs {
			if !processedInBatch[doc.Arquivo] {
				appState.SetFileTags(doc.Arquivo, doc.Tags)
				processedInBatch[doc.Arquivo] = true
			}
		}
	}

	appState.Save(cfg)
	appState.RebuildKnownTagsCache()
	tagCount := appState.GetKnownTagsCount()

	search.ClearCache()

	events.GetHub().Broadcast("sync:finished", map[string]interface{}{
		"new_docs": len(docs),
		"tags":     tagCount,
		"mode":     mode,
	})
}

func UpdateStateAfterOCR(absPath, relPath string, docs []models.Document, appState *AppState) {
	if len(docs) == 0 {
		return
	}

	for _, doc := range docs {
		appState.SetHash(doc.ID, doc.Hash)
	}

	if info, err := os.Stat(absPath); err == nil {
		appState.SetFileMod(absPath, info.ModTime())
	}

	appState.SetFileTags(relPath, docs[0].Tags)
}

func ProcessDocs(cfg *config.AppConfig, force bool, appState *AppState) ([]models.Document, []string, map[string][]string) {
	var deltaDocs []models.Document
	var mu sync.Mutex
	seenFiles := make(map[string]bool)
	var seenFilesMu sync.Mutex
	validIdsMap := make(map[string][]string)
	var validIdsMu sync.Mutex

	if _, err := os.Stat(cfg.DocsDir); os.IsNotExist(err) {
		os.MkdirAll(cfg.DocsDir, 0777)
	}

	for _, sub := range models.MonitoredSubDirs {
		os.MkdirAll(filepath.Join(cfg.DocsDir, sub), 0777)
	}

	var wg sync.WaitGroup
	numWorkers := runtime.NumCPU()
	if numWorkers < 2 {
		numWorkers = 2
	}

	sem := make(chan struct{}, numWorkers)

	log.Printf("[Sync] Sincronização paralela iniciada com %d workers.\n", numWorkers)

	filepath.WalkDir(cfg.DocsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		// Filtrar para indexar apenas arquivos dentro de MonitoredSubDirs
		rel, _ := filepath.Rel(cfg.DocsDir, path)
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 2 {
			return nil
		}
		isMonitored := false
		for _, m := range models.MonitoredSubDirs {
			if parts[0] == m {
				isMonitored = true
				break
			}
		}
		if !isMonitored {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		isMD := false
		for _, e := range models.ExtMarkdown {
			if e == ext {
				isMD = true
				break
			}
		}
		isPDF := false
		for _, e := range models.ExtPDF {
			if e == ext {
				isPDF = true
				break
			}
		}
		isImage := false
		for _, e := range models.ExtImage {
			if e == ext {
				isImage = true
				break
			}
		}

		if !isMD && !isPDF && !isImage {
			return nil
		}

		seenFilesMu.Lock()
		seenFiles[path] = true
		seenFilesMu.Unlock()

		modTime := info.ModTime()
		lastMod, exists := appState.GetFileMod(path)
		relPath, _ := filepath.Rel(cfg.DocsDir, path)

		if isImage && exists && !modTime.After(lastMod) && appState.HasTags(relPath) {
			id := semantic.HashFunc("img-" + relPath)
			if _, hashExists := appState.GetHash(id); hashExists {
				validIdsMu.Lock()
				validIdsMap[relPath] = append(validIdsMap[relPath], id)
				validIdsMu.Unlock()
				return nil
			}
		}

		if force || !exists || modTime.After(lastMod) || !appState.HasTags(relPath) || appState.GetFileMetadata(relPath) == nil {
			wg.Add(1)
			go func(p, f string, mt time.Time) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[ERRO CRÍTICO] Panic detectado ao processar %s: %v\n", f, r)
					}
					<-sem
				}()

				var docs []models.Document
				var links []string
				var metadata map[string]interface{}
				var tags []string
				if isMD {
					docs, links, metadata, tags = ProcessMarkdown(p, f, mt, appState)
				} else if isPDF {
					docs = ProcessPDF(p, f, mt, appState)
					tags = appState.GetFileTags(f)
				} else if isImage {
					docs = ProcessImage(p, f, mt, appState)
					tags = appState.GetFileTags(f)
				}

				if tags == nil {
					tags = []string{}
				}
				appState.SetFileTags(f, tags)

				if len(links) > 0 {
					appState.SetFileLinks(f, links)
				} else if isMD {
					appState.DeleteFileLinks(f)
				}

				if metadata == nil {
					metadata = make(map[string]interface{})
				}
				appState.SetFileMetadata(f, metadata)

				if len(docs) > 0 {
					mu.Lock()
					deltaDocs = append(deltaDocs, docs...)
					mu.Unlock()

					validIdsMu.Lock()
					ids := make([]string, 0, len(docs))
					for _, d := range docs {
						ids = append(ids, d.ID)
					}
					validIdsMap[f] = ids
					validIdsMu.Unlock()
				}
				appState.SetFileMod(p, mt)
			}(path, relPath, modTime)
		}
		return nil
	})

	wg.Wait()

	var deletedFiles []string
	allFileMods := appState.GetAllFileMods()
	for path := range allFileMods {
		seenFilesMu.Lock()
		seen := seenFiles[path]
		seenFilesMu.Unlock()
		if !seen {
			deletedFiles = append(deletedFiles, path)
		}
	}

	for _, path := range deletedFiles {
		appState.DeleteFileMod(path)
	}

	return deltaDocs, deletedFiles, validIdsMap
}

func SendToEngines(cfg *config.AppConfig, bleveDocs []models.Document, vectorDocs []models.Document, appState *AppState) {
	semanticEnabled := cfg.SemanticEnable && appState.GetSettings().SemanticEnable

	bleveBatch := make(map[string]interface{})
	for _, doc := range bleveDocs {
		fullPath := filepath.Join(cfg.DocsDir, doc.Arquivo)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			continue
		}
		bleveBatch[doc.ID] = doc
	}

	// 1. Indexar no Bleve (rápido)
	if len(bleveBatch) > 0 {
		if err := search.BatchIndexDocuments(bleveBatch); err != nil {
			log.Printf("[Erro] Falha ao indexar em lote no Bleve: %v\n", err)
		}
	}

	// 2. Determinar quais arquivos precisam de vetorização
	vectorDocsByFile := make(map[string][]models.Document)
	if semanticEnabled {
		for _, doc := range vectorDocs {
			if doc.Tipo == "markdown" || doc.Tipo == "image" || doc.Tipo == "imagem" || doc.Tipo == "link" || doc.Tipo == "youtube" {
				if HasEmbedTag(doc.Tags) {
					vectorDocsByFile[doc.Arquivo] = append(vectorDocsByFile[doc.Arquivo], doc)
				}
			}
		}
	}

	// 3. Liberar imediatamente arquivos que NÃO precisam de vetorização
	filesInBleve := make(map[string]bool)
	for _, doc := range bleveDocs {
		filesInBleve[doc.Arquivo] = true
	}

	for filename := range filesInBleve {
		if _, needsVector := vectorDocsByFile[filename]; !needsVector {
			SetFileIndexing(filename, false)
		}
	}

	// 4. Vetorização em background
	if semanticEnabled && len(vectorDocsByFile) > 0 {
		for filename, docs := range vectorDocsByFile {
			go func(fname string, fragments []models.Document) {
				log.Printf("[Sync] Vetorizando nota em background: %s (%d fragmentos)\n", fname, len(fragments))

				absPath := filepath.Join(cfg.DocsDir, fname)
				content, err := os.ReadFile(absPath)
				if err != nil {
					log.Printf("[Sync] Erro ao ler arquivo para vetorização: %v\n", err)
					SetFileIndexing(fname, false)
					return
				}

				effectiveHost := cfg.OllamaHost
				log.Printf("[Sync] Usando Ollama em %s para vetorizar: %s\n", effectiveHost, fname)
				embFunc := semantic.NewOllamaEmbedding(cfg.OllamaModel, effectiveHost, appState.GetSettings().EmbeddingDimension)
				vec, err := embFunc(context.Background(), string(content))
				if err != nil {
					log.Printf("[Sync] Erro ao gerar embedding para %s: %v\n", fname, err)
				} else {
					// Extrair titulo da primeira linha
					title := extractFirstLineTitle(string(content), fname)
					appState.SetNoteVector(fname, vec, title)
					for _, doc := range fragments {
						appState.SetVectorHash(doc.ID, doc.VectorHash)
					}
					// Invalidar apenas a projecao desta nota (P3.2: granular)
					appState.DeleteNoteProjection(fname)
					log.Printf("[Sync] Vetorizacao concluida para: %s\n", fname)
				}
				SetFileIndexing(fname, false)
			}(filename, docs)
		}
	}

	log.Printf("[Sync] %d fragmentos sincronizados (Vetores processados arquivo por arquivo).\n", len(bleveDocs))
}

// extractFirstLineTitle extrai o titulo da primeira linha nao-vazia do conteudo.
func extractFirstLineTitle(content, filename string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		clean := strings.TrimSpace(strings.TrimLeft(line, "# "))
		if clean != "" {
			return clean
		}
	}
	// Fallback: nome do arquivo
	parts := strings.Split(filename, "/")
	return strings.TrimSuffix(parts[len(parts)-1], ".md")
}
