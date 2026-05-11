package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"etl/internal/api"
	"etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/middleware"
	"etl/internal/search"
	"etl/internal/semantic"
	"log/slog"
)

func main() {
	// 0. Configurar Structured Logging (JSON)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("Iniciando Servidor TON-618 All-in-One...")

	// 0.1 Contexto Global para Graceful Shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Carregar configurações e estado
	cfg := config.LoadConfig()
	appState := ingest.NewAppState(cfg)
	defer appState.Close()
	appState.Load(cfg)

	// Limpeza inicial de registros órfãos (arquivos deletados com o servidor desligado)
	slog.Info("Realizando limpeza inicial de registros órfãos...")
	ingest.GlobalVacuum(cfg, appState)

	// 1.1 Inicializa o Coordenador de Sincronização (Fila de Trabalho com Debounce)
	coordinator := ingest.NewSyncCoordinator(cfg, appState)
	coordinator.Start()
	defer coordinator.Stop()

	slog.Debug("Estado carregado. Inicializando pesos...")
	search.InitializeWeights(cfg.StateDir)
	slog.Debug("Pesos inicializados. Abrindo índice Bleve...")

	// Inicializa o Motor de Busca Interno (Bleve)
	if err := search.InitIndex(cfg.BleveIndexDir); err != nil {
		slog.Error("erro crítico ao inicializar motor de busca", "error", err)
		os.Exit(1)
	}
	slog.Info("Motor de busca pronto. Inicializando motor semântico...")
	defer search.CloseIndex()

	// Inicializa o cache Semântico (Vector) persistente
	semantic.InitCache(filepath.Join(cfg.StateDir, "embeddings_cache.json"))

	// 2. Inicia Goroutine de ingestão em plano de fundo (passando o contexto)
	go ingest.StartWatcher(ctx, cfg, appState, coordinator)
	slog.Info("Watcher iniciado. Configurando rotas da API...")

	// 3. Inicializa Contexto da API e Rotas
	apiCtx := &api.HandlerContext{
		Cfg:         cfg,
		State:       appState,
		Coordinator: coordinator,
		Index:       search.GetIndex(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/search", apiCtx.HandleSearch)
	mux.HandleFunc("/api/sync", apiCtx.HandleManualSync)
	mux.HandleFunc("/api/file", apiCtx.HandleFile)
	mux.HandleFunc("/api/track", apiCtx.HandleTrack)
	mux.HandleFunc("/api/upload", apiCtx.HandleUpload)
	mux.HandleFunc("/api/weights", apiCtx.HandleWeights)
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			apiCtx.HandleGetSettings(w, r)
		} else if r.Method == http.MethodPost {
			apiCtx.HandleSaveSettings(w, r)
		} else {
			http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/status", apiCtx.HandlePing)
	mux.HandleFunc("/api/health", apiCtx.HandleHealth)
	mux.HandleFunc("/api/events", apiCtx.HandleEvents)
	mux.HandleFunc("/api/link", apiCtx.HandleLink)
	mux.HandleFunc("/api/rename", apiCtx.HandleRename)
	mux.HandleFunc("/api/tags", apiCtx.HandleGetTags)
	mux.HandleFunc("/api/notes", apiCtx.HandleGetNotes)
	mux.HandleFunc("/api/query", apiCtx.HandleQuery)
	mux.HandleFunc("/api/maintenance/stale", apiCtx.HandleGetStaleFiles)
	mux.HandleFunc("/api/maintenance/cleanup", apiCtx.HandleCleanupFiles)
	mux.HandleFunc("/api/graph/map", apiCtx.HandleKnowledgeMap)
	mux.HandleFunc("/api/graph/reindex", apiCtx.HandleReindexVectors)
	mux.HandleFunc("/api/graph/status", apiCtx.HandleKnowledgeMapStatus)
	mux.HandleFunc("/api/graph/query-point", apiCtx.HandleGraphQueryPoint)
	mux.HandleFunc("/api/graph/semantic-topics", apiCtx.HandleSemanticTopics)
	mux.HandleFunc("/api/graph/manual-map", apiCtx.HandleManualSemanticMap)
	mux.HandleFunc("/api/bundle", apiCtx.HandleBundleUpload)
	mux.HandleFunc("/api/backup/download", apiCtx.HandleDownloadBackup)
	mux.HandleFunc("/api/backup/size", apiCtx.HandleGetBackupSize)

	mux.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.Dir(cfg.DocsDir))))

	// SPA fallback: serve index.html para qualquer rota que não seja arquivo existente
	fs := http.FileServer(http.Dir(cfg.WebDir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Se for uma requisição de arquivo (tem extensão), tenta servir o arquivo
		path := r.URL.Path
		if path != "/" && filepath.Ext(path) != "" {
			fs.ServeHTTP(w, r)
			return
		}
		// Caso contrário, tenta o arquivo real; se não existir, serve index.html (SPA fallback)
		if path == "/" {
			fs.ServeHTTP(w, r)
			return
		}
		// Verifica se o arquivo existe antes de servir
		fullPath := filepath.Join(cfg.WebDir, path)
		if _, err := os.Stat(fullPath); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		// Fallback: serve index.html para o router do frontend
		http.ServeFile(w, r, filepath.Join(cfg.WebDir, "index.html"))
	})

	rateLimitedHandler := middleware.RateLimitMiddleware(30, 10)(mux)
	protectedHandler := api.BasicAuthMiddleware(rateLimitedHandler, cfg.AuthUser, cfg.AuthPass)

	// 4. Configuração do Servidor com Suporte a Shutdown
	noCacheHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Desativar cache para arquivos estáticos sensíveis
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		protectedHandler.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: middleware.Recovery(middleware.Logger(noCacheHandler)),
	}

	// Inicia o servidor em uma goroutine
	go func() {
		slog.Info("Frontend Web rodando", "url", "http://localhost:"+cfg.Port)
		slog.Info("Servindo arquivos estáticos", "dir", cfg.WebDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("erro no servidor HTTP", "error", err)
			os.Exit(1)
		}
	}()

	// 5. Aguarda sinal de encerramento
	<-ctx.Done()
	slog.Info("Sinal de encerramento recebido. Iniciando encerramento suave...")

	// Contexto com timeout para o shutdown (máximo 10 segundos)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("erro durante o shutdown do servidor", "error", err)
	}

	// O fechamento do índice Bleve é garantido pelo defer search.CloseIndex() no topo

	slog.Info("Servidor TON-618 encerrado com sucesso.")
}
