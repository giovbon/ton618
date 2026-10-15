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

	// 1.1. Modelo de embeddings local — baixado do HuggingFace no 1º boot para o
	// volume persistente (STATE_DIR/models) e servido em /models/*. Enquanto o
	// download não termina, o browser usa o CDN como fallback (allowRemoteModels).
	//
	// Com MODEL_DOWNLOAD=false o download local é desligado: são ~120MB que não vão
	// para o disco e o browser busca o modelo do CDN — exatamente o mesmo caminho
	// que o desktop usa enquanto o download não termina.
	var modelStore *embeddings.LocalModel
	if cfg.ModelDownload {
		localModel := embeddings.NewLocalModel(cfg.ModelDir)
		modelStore = localModel
		go func() {
			if err := localModel.Ensure(context.Background()); err != nil {
				slog.Warn("modelo de embeddings: download incompleto — browser usará o CDN como fallback", "error", err)
				return
			}
			slog.Info("Modelo de embeddings disponível localmente", "dir", cfg.ModelDir)
		}()
	} else {
		slog.Info("Download local do modelo de embeddings desativado (MODEL_DOWNLOAD=false) — o browser usará o CDN")
	}

	// 2. Database
	store, err := db.NewStore(cfg.DBPath)
	if err != nil {
		slog.Error("abrir banco", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	slog.Info("Banco SQLite pronto")

	// Se o modelo de embeddings mudou, descarta os embeddings antigos para
	// que o browser re-indexe tudo com o modelo novo (ex: MiniLM-L12 → e5-small).
	if reset, verr := store.EnsureEmbeddingModelVersion(db.EmbeddingModelVersion); verr != nil {
		slog.Warn("verificar versao do modelo de embeddings", "error", verr)
	} else if reset {
		slog.Info("Modelo de embeddings mudou — embeddings antigos removidos; aguardando re-indexação no browser")
	}

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
	watcher.ScanAndIndexAllParallel(store, cfg.DocsDir, cfg.ScanWorkers)
	slog.Info("Indexação inicial concluída")

	// 5. Agendador de notificações (calendário)
	// O intervalo (NTFY_POLL_INTERVAL_SEC, padrão 60s) define a precisão dos
	// lembretes: um evento é notificado no primeiro poll após (evento − lead).
	ntfySvc := services.NewNtfyService(store)
	go func() {
		ticker := time.NewTicker(cfg.NtfyPollInterval)
		defer ticker.Stop()
		// Executa uma vez na inicialização
		ntfySvc.CheckAndSendEventReminders()
		ntfySvc.CheckAndSendWeeklySummary()
		for range ticker.C {
			ntfySvc.CheckAndSendEventReminders()
			ntfySvc.CheckAndSendWeeklySummary()
		}
	}()

	// 6. Inicializa os contextos das Features
	backupService := services.NewBackupService(store, store, cfg.DocsDir)
	notesService := notes.NewNoteService(store, store, store, store, store, store, cfg.DocsDir)

	captureService := notes.NewCaptureService(store)

	// 7. Auto-Tag (Inatividade): não roda mais em background —
	// o usuário dispara a aplicação manualmente via POST /api/settings/auto-tag/apply.

	sysCtx := system.NewHandlerContext(cfg, store, backupService, notesService)
	notesCtx := notes.NewHandlerContext(cfg, store, notesService, backupService, captureService)
	todosCtx := todos.NewHandlerContext(cfg, store)
	searchCtx := search.NewHandlerContext(cfg, store)
	appointmentsCtx := appointments.NewHandlerContext(cfg, store)
	embeddingsCtx := embeddings.NewHandlerContext(cfg, store)
	if modelStore != nil {
		embeddingsCtx = embeddingsCtx.WithModel(modelStore)
	}

	go func() {
		slog.Info("Sincronizando notas do banco de dados em segundo plano...")
		if err := notesCtx.Notes.SyncDatabase(); err != nil {
			slog.Error("erro ao sincronizar banco de dados", "error", err)
		}
	}()

	r := chi.NewRouter()

	// Aplica middlewares globais em todas as requisições
	r.Use(middleware.LoggingMiddleware)
	r.Use(middleware.Recovery)
	r.Use(middleware.SecurityHeadersMiddleware)
	r.Use(middleware.WithRequestContext) // propaga contexto HTTP para cancelamento de queries
	r.Use(chimiddleware.Compress(5, "text/html", "text/css", "application/javascript", "image/svg+xml"))

	// Rotas públicas (estáticos + modelo de embeddings) e, em seguida, as rotas
	// dinâmicas protegidas por BasicAuth (com pprof em /debug/pprof).
	registerPublicRoutes(r, cfg, modelStore)

	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middleware.BasicAuthMiddleware(next, cfg.AuthUser, cfg.AuthPass)
		})
		r.Mount("/debug", chimiddleware.Profiler())
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

	// Encerra no Ctrl+C/SIGTERM ou quando uma operação pede reinício (restauração
	// de backup troca o arquivo do banco: é preciso reabrir com o novo arquivo).
	select {
	case <-ctx.Done():
	case <-services.RestartRequested:
		slog.Info("Reinício solicitado (restauração de backup) — encerrando para reabrir o banco")
	}
	slog.Info("Encerrando servidor...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)

	slog.Info("TON-618 encerrado")
}

// registerPublicRoutes monta as rotas servidas SEM autenticação:
//
//   - /static/* — bundle web (CSS/JS/fontes);
//   - /models/* — arquivos do modelo de embeddings, buscados pelo Web Worker do
//     Transformers.js antes de cair no CDN do HuggingFace.
//
// ⚠️ A rota do modelo precisa ficar FORA do grupo com BasicAuth: se ela cair no
// grupo autenticado o worker recebe 401/302, cai no CDN e a cópia local (~120MB
// baixados no boot) nunca é usada.
//
// Só é montada quando o download local está ligado (ver MODEL_DOWNLOAD); com
// model == nil não existe arquivo para servir — e registrar um handler nil
// daria panic a cada request.
func registerPublicRoutes(r chi.Router, cfg *config.AppConfig, model *embeddings.LocalModel) {
	// Arquivos estáticos com ETag automático (cache-buster via hash do conteúdo)
	staticDir := filepath.Join(cfg.WebDir, "static")
	staticCache, err := staticver.NewCache(staticDir)
	if err != nil {
		slog.Warn("staticver: erro ao inicializar cache (usando fallback)", "error", err)
	}
	// Monta o handler com ETags automáticos
	r.Handle("/static/*", http.StripPrefix("/static/", staticCache.Handler(staticDir)))
	staticver.SetDefault(staticCache) // disponibiliza URL() para os templates

	if model != nil {
		// O handler espera chegar sem o prefixo (/models/<repo>/<arquivo>).
		r.Handle("/models/*", http.StripPrefix("/models/", model))
	}
}
