package main

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ton618/internal/api"
	"ton618/internal/config"
	"ton618/internal/db"
	"ton618/internal/semantic"
	"ton618/internal/watcher"
)

//go:embed ../../internal/template/*.html
var templatesFS embed.FS

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

	// 3. Embedding provider
	embedProvider := semantic.NewProvider(
		cfg.EmbeddingProvider,
		cfg.EmbeddingAPIKey,
		cfg.EmbeddingModel,
		"", // baseURL (usando defaults por provedor)
		cfg.OllamaHost,
		cfg.OllamaModel,
		cfg.EmbeddingDim,
	)
	slog.Info("Provedor de embedding", "provider", cfg.EmbeddingProvider)

	// 4. Templates
	tpl := template.New("layout.html").Funcs(template.FuncMap{
		"hasPrefix": strings.HasPrefix,
		"lastPath": func(s string) string {
			parts := strings.Split(s, "/")
			if len(parts) > 0 {
				return strings.TrimSuffix(parts[len(parts)-1], ".md")
			}
			return s
		},
	})
	var parseErr error
	tpl, parseErr = tpl.ParseFS(templatesFS, "*.html")
	if parseErr != nil {
		slog.Error("carregar templates", "error", parseErr)
		os.Exit(1)
	}
	slog.Info("Templates carregados")

	// 5. Watcher
	w := watcher.NewWatcher(cfg, store)
	ctxWatcher, cancelWatcher := context.WithCancel(context.Background())
	defer cancelWatcher()
	w.Start(ctxWatcher)

	// Processa eventos do watcher em background
	go func() {
		for ev := range w.Events() {
			slog.Info("Processando", "file", ev.Filename, "type", ev.Type)
			if err := watcher.ProcessFile(store, ev); err != nil {
				slog.Error("processar arquivo", "file", ev.Filename, "error", err)
			}
		}
	}()

	// Indexação inicial de todos os arquivos
	slog.Info("Indexação inicial...")
	w.PollAll()
	slog.Info("Indexação inicial concluída")

	// 6. API context
	apiCtx := api.NewHandlerContext(cfg, store, w, embedProvider)
	apiCtx.SetTemplates(tpl)

	mux := http.NewServeMux()
	apiCtx.SetupRoutes(mux)

	// 7. Server
	var handler http.Handler = mux
	handler = loggingMiddleware(handler)
	handler = api.Recovery(handler)
	handler = api.BasicAuthMiddleware(handler, cfg.AuthUser, cfg.AuthPass)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	// Graceful shutdown
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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
	})
}
