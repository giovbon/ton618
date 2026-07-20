package embeddings

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"ton618/core/internal/core/db"
	"ton618/core/internal/httputil"
)

// ── Request/Response types ──

// saveChunkRequest representa um chunk individual na requisição de save.
type saveChunkRequest struct {
	ChunkID   string    `json:"chunk_id"`
	Filename  string    `json:"filename"`
	Index     int       `json:"index"`
	Content   string    `json:"content"`
	Embedding []float32 `json:"embedding"`
}

type saveEmbeddingRequest struct {
	Filename string             `json:"filename"`
	Chunks   []saveChunkRequest `json:"chunks"`
}

type searchEmbeddingRequest struct {
	Embedding []float32 `json:"embedding"`
	Limit     int       `json:"limit"`
}

type semanticSearchResult struct {
	Filename string  `json:"filename"`
	Distance float64 `json:"distance"`
}

type searchEmbeddingResponse struct {
	Results []semanticSearchResult `json:"results"`
}

// HandleEmbeddingSave recebe chunks com embeddings gerados no browser e os persiste.
// POST /api/embeddings/save
func (ctx *HandlerContext) HandleEmbeddingSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req saveEmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invalido: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Filename == "" {
		http.Error(w, "filename obrigatorio", http.StatusBadRequest)
		return
	}

	if len(req.Chunks) == 0 {
		http.Error(w, "ao menos 1 chunk obrigatorio", http.StatusBadRequest)
		return
	}

	// Verifica se a nota é do tipo indexável (evita indexar drawings, mapas, etc.)
	tags, _ := ctx.Store.GetFileTags(req.Filename)
	if !ctx.Store.IsNoteEmbeddable(req.Filename, tags) {
		http.Error(w, "tipo de nota não suporta indexação semântica", http.StatusBadRequest)
		return
	}

	// Valida cada chunk
	chunks := make([]db.ChunkInfo, 0, len(req.Chunks))
	for _, c := range req.Chunks {
		if len(c.Embedding) != db.EmbeddingDim {
			http.Error(w, "embedding do chunk deve ter 384 dimensoes", http.StatusBadRequest)
			return
		}
		chunks = append(chunks, db.ChunkInfo{
			ChunkID:    c.ChunkID,
			Filename:   c.Filename,
			ChunkIndex: c.Index,
			Content:    c.Content,
			Embedding:  c.Embedding,
		})
	}

	if err := ctx.Store.SaveNoteChunks(req.Filename, chunks); err != nil {
		slog.Error("SaveNoteChunks", "filename", req.Filename, "chunks", len(chunks), "error", err)
		http.Error(w, "erro ao salvar chunks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// HandleEmbeddingSearch recebe o embedding de uma query e retorna documentos similares via KNN.
// POST /api/embeddings/search
func (ctx *HandlerContext) HandleEmbeddingSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req searchEmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invalido: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Embedding) != db.EmbeddingDim {
		http.Error(w, "embedding deve ter 384 dimensoes", http.StatusBadRequest)
		return
	}

	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
	}

	hits, err := ctx.Store.SearchSimilar(req.Embedding, req.Limit)
	if err != nil {
		slog.Error("SearchSimilar", "error", err)
		http.Error(w, "erro na busca semantica: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Carregar o limite configurado (padrão: 20% para busca semântica em MiniLM-L12-v2)
	searchThresholdPct := 20
	if val, err := ctx.Store.GetSetting("semantic_search_threshold"); err == nil && val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			if v == 50 {
				searchThresholdPct = 20
			} else {
				searchThresholdPct = v
			}
		}
	}
	
	// Converter % para distância L2
	// cosSim = 1.0 - (dist^2 / 2) => dist = sqrt(2 * (1 - cosSim))
	cosSimLimit := float64(searchThresholdPct) / 100.0
	maxDist := math.Sqrt(2.0 * (1.0 - cosSimLimit))

	results := make([]semanticSearchResult, 0, len(hits))
	for _, h := range hits {
		if h.Distance > maxDist {
			continue // filtra itens com similaridade menor que o exigido
		}
		results = append(results, semanticSearchResult{
			Filename: h.Filename,
			Distance: h.Distance,
		})
	}

	httputil.WriteJSON(w, searchEmbeddingResponse{Results: results})
}

// HandleEmbeddingStatus retorna status de indexacao semantica.
// GET /api/embeddings/status
func (ctx *HandlerContext) HandleEmbeddingStatus(w http.ResponseWriter, r *http.Request) {
	status, err := ctx.Store.GetEmbeddingStatus()
	if err != nil {
		slog.Error("GetEmbeddingStatus", "error", err)
		http.Error(w, "erro ao obter status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-cache, max-age=10")
	httputil.WriteJSON(w, status)
}

// HandleEmbeddingPending retorna notas que ainda nao possuem embedding indexado.
// Retorna em batches (default 20 por vez) para uso no auto-indexador do browser.
// Apenas notas .md com conteudo nao vazio sao incluidas.
//
// GET /api/embeddings/pending?limit=N
func (ctx *HandlerContext) HandleEmbeddingPending(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	pending, err := ctx.Store.GetPendingEmbeddingNotes(limit)
	if err != nil {
		slog.Error("GetPendingEmbeddingNotes", "error", err)
		http.Error(w, "erro ao buscar pendentes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Retorna conteudo para chunking no frontend (truncado a 10000 chars para performance)
	type pendingItem struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	}
	items := make([]pendingItem, 0, len(pending))
	for _, p := range pending {
		c := p.Content
		if len(c) > 10000 {
			slog.Debug("conteudo truncado para chunking semantic", "file", p.Filename, "original_len", len(c), "truncated_to", 10000)
			c = c[:10000]
		}
		items = append(items, pendingItem{Filename: p.Filename, Content: c})
	}

	httputil.WriteJSON(w, items)
}

// HandleEmbeddingReset apaga todos os chunks e embeddings do banco.
// Usado pela aba "Semântica" nas Configurações para resetar o índice.
// POST /api/embeddings/reset
func (ctx *HandlerContext) HandleEmbeddingReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := ctx.Store.ResetAllEmbeddings(); err != nil {
		slog.Error("ResetAllEmbeddings", "error", err)
		http.Error(w, "erro ao resetar índice semântico: "+err.Error(), http.StatusInternalServerError)
		return
	}

	httputil.WriteJSON(w, map[string]string{"status": "ok"})
}
