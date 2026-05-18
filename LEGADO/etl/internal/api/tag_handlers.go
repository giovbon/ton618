package api

import (
	"encoding/json"
	"net/http"
	"sort"
)

// HandleGetTags retorna a lista de todas as tags conhecidas obtidas no cache local
func (ctx *HandlerContext) HandleGetTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodGet {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	tagList := ctx.State.GetAllKnownTags()

	sort.Strings(tagList)

	response := map[string]interface{}{
		"tags":  tagList,
		"total": len(tagList),
	}

	json.NewEncoder(w).Encode(response)
}
