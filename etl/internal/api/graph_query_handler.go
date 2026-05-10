package api

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"etl/internal/semantic"
)

type QueryPointRequest struct {
	Query string `json:"query"`
}

type NearestNote struct {
	ID   string  `json:"id"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Dist float64 `json:"dist"`
}

type QueryPointResponse struct {
	X            float64       `json:"x"`
	Y            float64       `json:"y"`
	NearestNotes []NearestNote `json:"nearest_notes"`
}

// cosineSimilarity returns cosine similarity in [−1, 1].
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// HandleGraphQueryPoint embeds a question, computes similarity against all note
// vectors, and returns an interpolated 2-D position + the 3 nearest notes.
func (ctx *HandlerContext) HandleGraphQueryPoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	var req QueryPointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Query) == "" {
		http.Error(w, "Payload inválido ou query vazia", http.StatusBadRequest)
		return
	}

	cfg := ctx.Cfg
	if cfg == nil {
		http.Error(w, "Motor semântico não configurado", http.StatusServiceUnavailable)
		return
	}

	// 1. Generate embedding for the question
	embedCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	effectiveHost := ctx.State.GetEffectiveOllamaHost(cfg)
	embFunc := semantic.NewOllamaEmbedding(cfg.OllamaModel, effectiveHost)
	queryVec, err := embFunc(embedCtx, req.Query)
	if err != nil {
		log.Printf("[QueryPoint] Erro ao gerar embedding: %v\n", err)
		http.Error(w, "Erro ao gerar embedding da pergunta", http.StatusInternalServerError)
		return
	}

	// 2. Load stored vectors and projections
	allVectors := ctx.State.GetAllNoteVectors()
	projections := ctx.State.GetAllNoteProjections()

	if len(allVectors) == 0 || len(projections) == 0 {
		http.Error(w, "Mapa semântico vazio — execute o reindex primeiro", http.StatusConflict)
		return
	}

	// 3. Compute cosine similarity to every note that has a 2-D projection
	type scored struct {
		id   string
		sim  float64
		x, y float64
	}
	var candidates []scored

	for id, vec := range allVectors {
		coords, ok := projections[id]
		if !ok || len(coords) < 2 {
			continue
		}
		sim := cosineSimilarity(queryVec, vec)
		candidates = append(candidates, scored{id: id, sim: sim, x: coords[0], y: coords[1]})
	}

	if len(candidates) == 0 {
		http.Error(w, "Nenhuma nota com projeção disponível", http.StatusConflict)
		return
	}

	// 4. Sort by similarity (descending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].sim > candidates[j].sim
	})

	// 5. Use top-3 by similarity for both position AND nearest-notes.
	// Using fewer, higher-quality candidates keeps the dot close to the
	// actual relevant notes instead of drifting to a cluster boundary.
	topK := 3
	if len(candidates) < topK {
		topK = len(candidates)
	}

	var sumX, sumY, totalWeight float64
	for i := 0; i < topK; i++ {
		// Similarity is already in [0,1] range for well-matched notes;
		// square it to bias strongly toward the closest match.
		w := math.Max(0, candidates[i].sim)
		w = w * w
		sumX += candidates[i].x * w
		sumY += candidates[i].y * w
		totalWeight += w
	}

	var qx, qy float64
	if totalWeight > 0 {
		qx = sumX / totalWeight
		qy = sumY / totalWeight
	} else {
		// fallback: plain average of top-K
		for i := 0; i < topK; i++ {
			qx += candidates[i].x
			qy += candidates[i].y
		}
		qx /= float64(topK)
		qy /= float64(topK)
	}

	// 6. Return the same top-3 (by similarity) as nearest_notes.
	nearestNotes := make([]NearestNote, topK)
	for i := 0; i < topK; i++ {
		nearestNotes[i] = NearestNote{
			ID:   candidates[i].id,
			X:    candidates[i].x,
			Y:    candidates[i].y,
			Dist: math.Hypot(candidates[i].x-qx, candidates[i].y-qy),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(QueryPointResponse{
		X:            qx,
		Y:            qy,
		NearestNotes: nearestNotes,
	})
}
