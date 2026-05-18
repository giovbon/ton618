package api

import (
	"encoding/json"
	"etl/internal/query"
	"net/http"
)

func (ctx *HandlerContext) HandleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Query string `json:"query"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	if payload.Query == "" {
		http.Error(w, "Query não pode estar vazia", http.StatusBadRequest)
		return
	}

	result, err := query.Execute(payload.Query, ctx.State, ctx.Cfg)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
