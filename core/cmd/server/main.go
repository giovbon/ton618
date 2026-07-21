package main

import (
	"context"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"ton618/core/internal/core/config"
	"ton618/core/internal/core/db"
	"ton618/core/internal/core/services"
	"ton618/core/internal/core/staticver"
	"ton618/core/internal/features/appointments"
	"ton618/core/internal/features/embeddings"
	"ton618/core/internal/features/notes"
	"ton618/core/internal/features/search"
	"ton618/core/internal/features/system"
	"ton618/core/internal/features/todos"
	"ton618/core/internal/middleware"
	"ton618/core/internal/watcher"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("TON-618 iniciando...")

	// Registra tipos MIME para módulos ES (.mjs) e WebAssembly (.wasm)
	mime.AddExtensionType(".mjs", "text/javascript")
	mime.AddExtensionType(".wasm", "application/wasm")

	// 1. Config
	cfg := config.Load()
	if err := cfg.EnsureDirs(); err != nil {
		slog.Error("criar diretorios", "error", err)
		os.Exit(1)
	}

	// 2. Database
	store, err := db.NewStore(cfg.DBPath)
	if err != nil {
		slog.Error("abrir banco", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	slog.Info("Banco SQLite pronto")

	// 2.5. Migrar notas .md do disco para o banco
	imported, migErr := store.MigrateNotesFromDisk(cfg.DocsDir)
	if migErr != nil {
		slog.Error("migrar notas", "error", migErr)
	}
	if imported > 0 {
		slog.Info("Notas migradas do disco para o SQLite", "count", imported)
	}

	slog.Info("Templates carregados")

	// 4. Indexação inicial
	slog.Info("Indexação inicial...")
	watcher.ScanAndIndexAll(store, cfg.DocsDir)
	slog.Info("Indexação inicial concluída")

	// 5. Agendador de notificações (calendário)
	ntfySvc := services.NewNtfyService(store)
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		// Executa uma vez na inicialização
		ntfySvc.CheckAndSendDailyAppointments()
		ntfySvc.CheckAndSendWeeklySummary()
		for range ticker.C {
			ntfySvc.CheckAndSendDailyAppointments()
			ntfySvc.CheckAndSendWeeklySummary()
		}
	}()

	// 5. Inicializa os repositórios por domínio
	noteRepo := db.NewNoteRepo(store)
	tagRepo := db.NewTagRepo(store)
	linkRepo := db.NewLinkRepo(store)
	popRepo := db.NewPopRepo(store)
	fileModRepo := db.NewFileModRepo(store)

	// 6. Inicializa os contextos das Features
	backupService := services.NewBackupService(noteRepo, fileModRepo, cfg.DocsDir)
	notesService := notes.NewNoteService(store, noteRepo, tagRepo, linkRepo, popRepo, fileModRepo, cfg.DocsDir)

	captureService := notes.NewCaptureService(store)
	typstService := notes.NewTypstService()

	sysCtx := system.NewHandlerContext(cfg, store, backupService, notesService)
	notesCtx := notes.NewHandlerContext(cfg, store, notesService, backupService, captureService, typstService)
	todosCtx := todos.NewHandlerContext(cfg, store)
	searchCtx := search.NewHandlerContext(cfg, store)
	appointmentsCtx := appointments.NewHandlerContext(cfg, store)
	embeddingsCtx := embeddings.NewHandlerContext(cfg, store)

	slog.Info("Sincronizando notas do banco de dados...")
	if err := notesCtx.Notes.SyncDatabase(); err != nil {
		slog.Error("erro ao sincronizar banco de dados", "error", err)
	}

	r := chi.NewRouter()

	// Aplica middlewares globais em todas as requisições
	r.Use(middleware.LoggingMiddleware)
	r.Use(middleware.Recovery)
	r.Use(middleware.SecurityHeadersMiddleware)
	r.Use(middleware.WithRequestContext) // propaga contexto HTTP para cancelamento de queries
	r.Use(chimiddleware.Compress(5, "text/html", "text/css", "application/javascript", "image/svg+xml"))

	// Arquivos estáticos com ETag automático (cache-buster via hash do conteúdo)
	staticDir := filepath.Join(cfg.WebDir, "static")
	staticCache, err := staticver.NewCache(staticDir)
	if err != nil {
		slog.Warn("staticver: erro ao inicializar cache (usando fallback)", "error", err)
	}
	// Monta o handler com ETags automáticos
	r.Handle("/static/*", http.StripPrefix("/static/", staticCache.Handler(staticDir)))

	// Protege as rotas dinâmicas com BasicAuth
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middleware.BasicAuthMiddleware(next, cfg.AuthUser, cfg.AuthPass)
		})
		SetupRoutes(r, sysCtx, notesCtx, todosCtx, searchCtx, appointmentsCtx, embeddingsCtx)
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("Servidor HTTP rodando", "addr", "http://localhost:"+cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("erro servidor", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("Encerrando servidor...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)

	slog.Info("TON-618 encerrado")
}
