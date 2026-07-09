package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"ton618/internal/core/config"
	"ton618/internal/core/db"
	"ton618/internal/core/services"
	"ton618/internal/features/appointments"
	"ton618/internal/features/notes"
	"ton618/internal/features/search"
	"ton618/internal/features/system"
	"ton618/internal/features/todos"
	"ton618/internal/middleware"
	"ton618/internal/processor"
	"ton618/internal/watcher"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	slog.Info("TON-618 iniciando...")

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

	// 3.5. Carrega stopwords personalizadas do usuário
	if err := processor.LoadCustomStopwords(cfg.DocsDir); err != nil {
		slog.Error("carregar stopwords custom", "error", err)
	} else {
		customWords := processor.GetCustomStopwords()
		if len(customWords) > 0 {
			slog.Info("Stopwords custom carregadas", "count", len(customWords))
		}
	}

	// 4. Watcher
	w := watcher.NewWatcher(cfg, store)
	ctxWatcher, cancelWatcher := context.WithCancel(context.Background())
	defer cancelWatcher()
	w.Start(ctxWatcher)

	go func() {
		for ev := range w.Events() {
			slog.Info("Processando", "file", ev.Filename, "type", ev.Type)
			if err := watcher.ProcessFile(store, ev); err != nil {
				slog.Error("processar arquivo", "file", ev.Filename, "error", err)
			}
		}
	}()

	slog.Info("Indexação inicial...")
	w.PollAll()
	slog.Info("Indexação inicial concluída")

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

	sysCtx := system.NewHandlerContext(cfg, store, w, backupService, notesService)
	notesCtx := notes.NewHandlerContext(cfg, store, w, notesService, backupService, captureService, typstService)
	todosCtx := todos.NewHandlerContext(cfg, store)
	searchCtx := search.NewHandlerContext(cfg, store)
	appointmentsCtx := appointments.NewHandlerContext(cfg, store)

	slog.Info("Sincronizando notas do banco de dados...")
	if err := notesCtx.Notes.SyncDatabase(); err != nil {
		slog.Error("erro ao sincronizar banco de dados", "error", err)
	}

	r := chi.NewRouter()

	// Aplica middlewares globais em todas as requisições
	r.Use(middleware.LoggingMiddleware)
	r.Use(middleware.Recovery)
	r.Use(middleware.SecurityHeadersMiddleware)
	r.Use(chimiddleware.Compress(5, "text/html", "text/css", "application/javascript", "image/svg+xml"))

	// Arquivos estáticos
	staticFS := http.FileServer(http.Dir(filepath.Join(cfg.WebDir, "static")))
	r.Handle("/static/*", http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("v") != "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		}
		staticFS.ServeHTTP(w, req)
	})))

	// Protege as rotas dinâmicas com BasicAuth
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middleware.BasicAuthMiddleware(next, cfg.AuthUser, cfg.AuthPass)
		})
		SetupRoutes(r, sysCtx, notesCtx, todosCtx, searchCtx, appointmentsCtx)
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
