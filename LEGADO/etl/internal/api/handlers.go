package api

import (
	"encoding/json"
	"net/http"

	appCfg "etl/internal/config"
	"etl/internal/ingest"
	"etl/internal/semantic"
	"log/slog"
	"time"

	"github.com/blevesearch/bleve/v2"
)

const KeyMask = "****MASCARADO****"

// HandlerContext contém a configuração, estado, coordenador e índice necessários para os handlers.
type HandlerContext struct {
	Cfg         *appCfg.AppConfig
	State       *ingest.AppState
	Coordinator *ingest.SyncCoordinator // substitui GlobalCoordinator
	Index       bleve.Index             // substitui search.GetIndex()
}

func (ctx *HandlerContext) HandlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status := map[string]interface{}{
		"status":  "authenticated",
		"version": appCfg.AppVersion,
		"ollama":  semantic.GetMetrics(),
	}

	json.NewEncoder(w).Encode(status)
}

func (ctx *HandlerContext) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	health := map[string]interface{}{
		"status": "up",
		"checks": map[string]string{
			"bbolt":  "up",
			"ollama": "up",
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	// Check BBolt
	if ctx.State == nil || !ctx.State.IsAlive() {
		health["status"] = "down"
		health["checks"].(map[string]string)["bbolt"] = "down"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// Check Ollama
	effectiveHost := ctx.Cfg.OllamaHost
	if err := semantic.Ping(effectiveHost); err != nil {
		slog.Warn("ollama health check failed", "error", err, "host", effectiveHost)
		health["status"] = "degraded"
		health["checks"].(map[string]string)["ollama"] = "down"
		// We don't necessarily return 503 if only Ollama is down,
		// as lexical search still works.
	}

	json.NewEncoder(w).Encode(health)
}
