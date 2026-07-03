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

	mux := http.NewServeMux()
	SetupRoutes(mux, sysCtx, notesCtx, todosCtx, searchCtx, appointmentsCtx)

	staticFS := http.FileServer(http.Dir(cfg.WebDir + "/static"))
	staticHandler := http.StripPrefix("/static/", staticFS)

	// Protege as rotas dinâmicas do mux com BasicAuth
	var appHandler http.Handler = mux
	appHandler = middleware.BasicAuthMiddleware(appHandler, cfg.AuthUser, cfg.AuthPass)

	// Define o roteador principal (arquivos estáticos vs rotas dinâmicas)
	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) >= 8 && r.URL.Path[:8] == "/static/" {
			if strings.HasSuffix(r.URL.Path, ".js") || strings.HasSuffix(r.URL.Path, ".css") {
				relPath := strings.TrimPrefix(r.URL.Path, "/static/")
				basePath := filepath.Join(cfg.WebDir, "static", relPath)

				// 1. Define cabeçalhos de content-type
				if strings.HasSuffix(r.URL.Path, ".js") {
					w.Header().Set("Content-Type", "application/javascript")
				} else {
					w.Header().Set("Content-Type", "text/css")
				}

				// 2. Cache longo se o parâmetro de versão 'v' estiver presente
				if r.URL.Query().Get("v") != "" {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "no-cache, must-revalidate")
				}

				// 3. Tenta servir Brotli (.br) se suportado pelo cliente
				if strings.Contains(r.Header.Get("Accept-Encoding"), "br") {
					brPath := basePath + ".br"
					if _, err := os.Stat(brPath); err == nil {
						w.Header().Set("Content-Encoding", "br")
						http.ServeFile(w, r, brPath)
						return
					}
				}

				// 4. Fallback para Gzip (.gz) se suportado pelo cliente
				if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
					gzPath := basePath + ".gz"
					if _, err := os.Stat(gzPath); err == nil {
						w.Header().Set("Content-Encoding", "gzip")
						http.ServeFile(w, r, gzPath)
						return
					}
				}
			}
			staticHandler.ServeHTTP(w, r)
			return
		}
		appHandler.ServeHTTP(w, r)
	})

	// Aplica middlewares globais em todas as requisições (incluindo estáticos)
	var globalHandler http.Handler = mainHandler
	globalHandler = middleware.LoggingMiddleware(globalHandler)
	globalHandler = middleware.Recovery(globalHandler)
	globalHandler = middleware.SecurityHeadersMiddleware(globalHandler)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: globalHandler,
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
