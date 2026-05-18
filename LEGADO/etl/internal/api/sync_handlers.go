package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"etl/internal/ingest"
)

func (ctx *HandlerContext) HandleManualSync(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force") == "true"
	slog.Info("Solicitação de sincronização manual", "tag", "API", "force", force)

	if ctx.Coordinator != nil {
		ctx.Coordinator.Push("global", ingest.JobFullSync, force)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "success", "message": "background sync started", "force": %v}`, force)
}
