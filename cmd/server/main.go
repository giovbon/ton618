package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ton618/internal/core/config"
	"ton618/internal/core/db"
	"ton618/internal/features/notes"
	"ton618/internal/features/search"
	"ton618/internal/core/services"
	"ton618/internal/features/system"
	"ton618/internal/features/todos"
	"ton618/internal/features/appointments"
	"ton618/internal/processor"
	"ton618/internal/watcher"
	"ton618/internal/middleware"
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

	// 5. Inicializa os contextos das Features
	backupService := services.NewBackupService(store, store, cfg.DocsDir)
	notesService := notes.NewNoteService(store, store, store, store, store, store, cfg.DocsDir)
	
	sysCtx := system.NewHandlerContext(cfg, store, w, backupService, notesService)
	notesCtx := notes.NewHandlerContext(cfg, store, w, notesService, backupService)
	todosCtx := todos.NewHandlerContext(cfg, store)
	searchCtx := search.NewHandlerContext(cfg, store)
	appointmentsCtx := appointments.NewHandlerContext(cfg, store)

	slog.Info("Sincronizando notas do banco de dados...")
	if err := notesCtx.Notes.SyncDatabase(); err != nil {
		slog.Error("erro ao sincronizar banco de dados", "error", err)
	}

	mux := http.NewServeMux()
	SetupRoutes(mux, sysCtx, notesCtx, todosCtx, searchCtx, appointmentsCtx)

	staticFS := http.FileServer(http.Dir(cfg.WebDir + "/static"))
	staticHandler := http.StripPrefix("/static/", staticFS)

	var handler http.Handler = mux
	handler = middleware.LoggingMiddleware(handler)
	handler = middleware.Recovery(handler)
	handler = middleware.BasicAuthMiddleware(handler, cfg.AuthUser, cfg.AuthPass)

	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) >= 8 && r.URL.Path[:8] == "/static/" {
			if strings.HasSuffix(r.URL.Path, ".js") || strings.HasSuffix(r.URL.Path, ".css") {
				relPath := strings.TrimPrefix(r.URL.Path, "/static/")
				gzPath := filepath.Join(cfg.WebDir, "static", relPath) + ".gz"
				if _, err := os.Stat(gzPath); err == nil {
					w.Header().Set("Content-Encoding", "gzip")
					if strings.HasSuffix(r.URL.Path, ".js") {
						w.Header().Set("Content-Type", "application/javascript")
					} else {
						w.Header().Set("Content-Type", "text/css")
					}
					w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
					http.ServeFile(w, r, gzPath)
					return
				}
			}
			staticHandler.ServeHTTP(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mainHandler,
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
