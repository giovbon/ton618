package api

import (
	"encoding/json"
	"log"
	"net/http"

	"etl/internal/models"
	"etl/internal/search"
)

func (ctx *HandlerContext) HandleWeights(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		weights := search.GetWeights()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(weights)

	case http.MethodPost:
		var weights search.RankingWeights
		if err := json.NewDecoder(r.Body).Decode(&weights); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		if err := search.SaveWeights(weights); err != nil {
			http.Error(w, "Erro ao salvar pesos", http.StatusInternalServerError)
			return
		}
		search.ClearCache() // Invalida cache para aplicar novos pesos
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		defaults := search.GetDefaultWeights()
		search.SaveWeights(defaults)
		search.ClearCache()
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(defaults)

	default:
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
	}
}

func (ctx *HandlerContext) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodGet {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	settings := ctx.State.GetSettings()

	if settings.GoogleVisionKey != "" {
		settings.GoogleVisionKey = KeyMask
	}
	if settings.EmbeddingAPIKey != "" {
		settings.EmbeddingAPIKey = KeyMask
	}

	json.NewEncoder(w).Encode(settings)
}

func (ctx *HandlerContext) HandleSaveSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodPost {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	var newSettings models.AppSettings
	if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
		http.Error(w, "Erro ao decodificar JSON", http.StatusBadRequest)
		return
	}

	oldSettings := ctx.State.GetSettings()
	if newSettings.GoogleVisionKey == KeyMask {
		newSettings.GoogleVisionKey = oldSettings.GoogleVisionKey
	}
	if newSettings.EmbeddingAPIKey == KeyMask {
		newSettings.EmbeddingAPIKey = oldSettings.EmbeddingAPIKey
	}
	ctx.State.SetSettings(newSettings)
	ctx.State.Save(ctx.Cfg)

	log.Printf("[Settings] Configurações salvas.\n")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
