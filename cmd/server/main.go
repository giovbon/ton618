package main

import (
	"context"
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
	"ton618/internal/index"
	internalTpl "ton618/internal/template"
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

	// 3. Embedding provider
	embedProvider := index.NewProvider(
		cfg.EmbeddingProvider,
		cfg.EmbeddingAPIKey,
		cfg.EmbeddingModel,
		"", // baseURL (usando defaults por provedor)
		cfg.OllamaHost,
		cfg.OllamaModel,
		cfg.EmbeddingDim,
	)
	slog.Info("Provedor de embedding", "provider", cfg.EmbeddingProvider)

	// Se nao tem API key configurada, desabilita embedding para evitar erros
	if cfg.EmbeddingAPIKey == "" && cfg.EmbeddingProvider == "gemini" {
		slog.Warn("EMBEDDING_API_KEY nao configurada — embeddings desabilitados")
		cfg.EmbeddingAll = false
		embedProvider = nil
	}

	// 4. Templates
	tpl := template.New("layout.html").Funcs(template.FuncMap{
		"hasPrefix": strings.HasPrefix,
		"baseName": func(s string) string {
			if s == "" {
				return ""
			}
			parts := strings.Split(s, "/")
			return parts[len(parts)-1]
		},
		"noteIcon": func(arquivo string, tags []string) string {
			isPdf := strings.HasPrefix(arquivo, "pdfs/")
			isAttach := strings.HasPrefix(arquivo, "attachments/")
			hasTag := func(tag string) bool {
				for _, t := range tags {
					if t == tag {
						return true
					}
				}
				return false
			}
			if isPdf {
				return "📕"
			} else if hasTag("youtube") {
				return "🎬"
			} else if hasTag("artigo") {
				return "📰"
			} else if hasTag("captura") {
				return "📋"
			} else if isAttach {
				return "📦"
			}
			return "📝"
		},
	})
	var parseErr error
	tpl, parseErr = tpl.ParseFS(internalTpl.TemplatesFS, "*.html")
	if parseErr != nil {
		slog.Error("carregar templates", "error", parseErr)
		os.Exit(1)
	}
	slog.Info("Templates carregados")

	// 5. Watcher
	w := watcher.NewWatcher(cfg, store)
	w.SetEmbedProvider(embedProvider)
	ctxWatcher, cancelWatcher := context.WithCancel(context.Background())
	defer cancelWatcher()
	w.Start(ctxWatcher)

	go func() {
		for ev := range w.Events() {
			slog.Info("Processando", "file", ev.Filename, "type", ev.Type)
			if err := watcher.ProcessFile(store, ev, embedProvider, cfg.EmbeddingAll); err != nil {
				slog.Error("processar arquivo", "file", ev.Filename, "error", err)
			}
		}
	}()

	slog.Info("Indexação inicial...")
	w.PollAll()
	slog.Info("Indexação inicial concluída")

	// 6. API context
	apiCtx := api.NewHandlerContext(cfg, store, w, embedProvider)
	apiCtx.SetTemplates(tpl)

	mux := http.NewServeMux()
	apiCtx.SetupRoutes(mux)

	staticFS := http.FileServer(http.Dir(cfg.WebDir + "/static"))
	staticHandler := http.StripPrefix("/static/", staticFS)

	var handler http.Handler = mux
	handler = loggingMiddleware(handler)
	handler = api.Recovery(handler)
	handler = api.BasicAuthMiddleware(handler, cfg.AuthUser, cfg.AuthPass)

	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) >= 8 && r.URL.Path[:8] == "/static/" {
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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
	})
}
