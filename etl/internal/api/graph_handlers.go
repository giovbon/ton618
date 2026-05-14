package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"etl/internal/clustering"
	"etl/internal/events"
	"etl/internal/ingest"
	"etl/internal/semantic"

	"github.com/blevesearch/bleve/v2"
)

var (
	reindexTotal     int32
	reindexProcessed int32
	pcaCacheMu       sync.Mutex
)

type KnowledgeMapResponse struct {
	Notes    []clustering.Point   `json:"notes"`
	Clusters []clustering.Cluster `json:"clusters"`
}

type KnowledgeMapStatusResponse struct {
	IsReindexing bool `json:"is_reindexing"`
	Total        int  `json:"total"`
	Processed    int  `json:"processed"`
	Percent      int  `json:"percent"`
}

type ManualMapResponse struct {
	Topics []ManualTopic `json:"topics"`
	Links  []ManualLink  `json:"links"`
}

type ManualTopic struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Level   int    `json:"level"`
	Type    string `json:"type"`     // "topic" or "tag"
	HasFile bool   `json:"has_file"` // true se existe notes/Label.md
}

type ManualLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // "hierarchy", "note" or "tag"
}

func (ctx *HandlerContext) HandleKnowledgeMapStatus(w http.ResponseWriter, r *http.Request) {
	total := atomic.LoadInt32(&reindexTotal)
	processed := atomic.LoadInt32(&reindexProcessed)

	percent := 0
	if total > 0 {
		percent = int(float64(processed) / float64(total) * 100)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(KnowledgeMapStatusResponse{
		IsReindexing: atomic.LoadInt32(&reindexRunning) == 1,
		Total:        int(total),
		Processed:    int(processed),
		Percent:      percent,
	})
}

func (ctx *HandlerContext) HandleSemanticTopics(w http.ResponseWriter, r *http.Request) {
	topics := ctx.State.GetAllSemanticTopics()
	sort.Strings(topics)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"topics": topics})
}

func (ctx *HandlerContext) HandleManualSemanticMap(w http.ResponseWriter, r *http.Request) {
	topics := ctx.State.GetAllSemanticTopics()
	allNoteLinks := ctx.State.GetAllFileSemanticLinks()

	log.Printf("[ManualMap] Topics do BBolt: %d | Files com links: %d\n", len(topics), len(allNoteLinks))

	resp := ManualMapResponse{
		Topics: []ManualTopic{},
		Links:  []ManualLink{},
	}

	topicMap := make(map[string]bool)

	for _, topic := range topics {
		parts := strings.Split(topic, "/")
		currentPath := ""
		for i, part := range parts {
			parentPath := currentPath
			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = currentPath + "/" + part
				resp.Links = append(resp.Links, ManualLink{
					Source: parentPath,
					Target: currentPath,
					Type:   "hierarchy",
				})
			}
			if !topicMap[currentPath] {
				// Verifica se existe nota correspondente (pelo label OU pelo id completo)
				hasFile := false
				pathsToCheck := []string{
					filepath.Join(ctx.Cfg.DocsDir, "notes", part+".md"),
					filepath.Join(ctx.Cfg.DocsDir, "notes", currentPath+".md"),
					filepath.Join(ctx.Cfg.DocsDir, part+".md"),
				}
				for _, p := range pathsToCheck {
					if _, err := os.Stat(p); err == nil {
						hasFile = true
						break
					}
				}

				resp.Topics = append(resp.Topics, ManualTopic{
					ID:      currentPath,
					Label:   part,
					Level:   i,
					Type:    "topic",
					HasFile: hasFile,
				})
				topicMap[currentPath] = true
			}
		}
	}

	for note, noteTopics := range allNoteLinks {
		for _, topic := range noteTopics {
			resp.Links = append(resp.Links, ManualLink{
				Source: note,
				Target: topic,
				Type:   "note",
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (ctx *HandlerContext) HandleKnowledgeMap(w http.ResponseWriter, r *http.Request) {
	allVectors := ctx.State.GetAllNoteVectors()

	noteVectors := make(map[string][]float32)
	titles := make(map[string]string)
	coords2D := make(map[string][2]float64)
	all2DCached := true
	index := ctx.Index

	for id, nv := range allVectors {
		if nv.Title != "" {
			titles[id] = nv.Title
		}
		if nv.X != 0 || nv.Y != 0 {
			coords2D[id] = [2]float64{nv.X, nv.Y}
		}

		tags := ctx.State.GetFileTags(id)
		hasEmbed := ingest.HasEmbedTag(tags)

		if hasEmbed {
			noteVectors[id] = nv.Vector
			if nv.X == 0 && nv.Y == 0 {
				all2DCached = false
			}
			continue
		}

		q := bleve.NewTermQuery(id)
		q.SetField("arquivo")
		searchReq := bleve.NewSearchRequest(q)
		searchReq.Fields = []string{"texto"}
		searchRes, err := index.Search(searchReq)

		if err == nil && searchRes.Total > 0 {
			for _, hit := range searchRes.Hits {
				if txt, ok := hit.Fields["texto"].(string); ok {
					if titles[id] == "" {
						lines := strings.Split(txt, "\n")
						for _, line := range lines {
							clean := strings.TrimSpace(strings.TrimLeft(line, "# "))
							if clean != "" {
								titles[id] = clean
								break
							}
						}
					}
					if !hasEmbed && strings.Contains(txt, "#embed") {
						hasEmbed = true
						noteVectors[id] = nv.Vector
						if nv.X == 0 && nv.Y == 0 {
							all2DCached = false
						}
					}
				}
			}
		}
	}

	slog.Info("Map notes filtered", "count", len(noteVectors), "cached2d", all2DCached)
	if len(noteVectors) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(KnowledgeMapResponse{Notes: []clustering.Point{}, Clusters: []clustering.Cluster{}})
		return
	}

	// Projecao 2D: usar cache do NoteVector ou calcular t-SNE
	var projections map[string][]float64
	if all2DCached && len(coords2D) == len(noteVectors) {
		slog.Info("Using NoteVector 2D cache", "count", len(coords2D))
		projections = make(map[string][]float64)
		for id, c := range coords2D {
			projections[id] = []float64{c[0], c[1]}
		}
	} else {
		pcaCacheMu.Lock()
		projections = ctx.State.GetAllNoteProjections()
		useCache := len(projections) == len(noteVectors) && len(noteVectors) > 0
		if useCache {
			for id := range noteVectors {
				if _, ok := projections[id]; !ok {
					useCache = false
					break
				}
			}
		}

		if !useCache {
			slog.Info("Calculating t-SNE projection", "count", len(noteVectors))
			rawProjections := clustering.ProjectTSNE(noteVectors)
			projections = make(map[string][]float64)
			for id, coords := range rawProjections {
				projections[id] = []float64{coords[0], coords[1]}
			}
			go func() {
				ctx.State.SetNoteProjections(projections)
				for id, c := range rawProjections {
					ctx.State.SetNoteVectors2D(id, c[0], c[1])
				}
				slog.Info("t-SNE and 2D coordinates persisted")
			}()
		}
		pcaCacheMu.Unlock()
	}

	// Pontos para clustering
	var points []clustering.Point
	for id, coords := range projections {
		title := titles[id]
		if title == "" {
			parts := strings.Split(id, "/")
			title = strings.TrimSuffix(parts[len(parts)-1], ".md")
		}
		points = append(points, clustering.Point{
			ID:    id,
			Title: title,
			X:     coords[0],
			Y:     coords[1],
		})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].ID < points[j].ID })

	// K adaptativo via silhouette score
	maxK := int(3.0 + 0.5*float64(len(points))/5.0)
	if maxK > 10 {
		maxK = 10
	}
	if maxK > len(points) {
		maxK = len(points)
	}
	k := clustering.BestK(points, maxK)
	points = clustering.KMeans(points, k, 20)
	slog.Info("Adaptive K clustering result", "clusters", k, "maxK", maxK)

	// Cluster labeling (batch DisjunctionQuery)
	clusterMap := make(map[int]*clustering.Cluster)
	clusterTexts := make(map[int][]string)

	disjQuery := bleve.NewDisjunctionQuery()
	for _, p := range points {
		q := bleve.NewTermQuery(p.ID)
		q.SetField("arquivo")
		disjQuery.AddQuery(q)
	}
	batchReq := bleve.NewSearchRequest(disjQuery)
	batchReq.Size = len(points) + 10
	batchReq.Fields = []string{"arquivo", "texto"}
	batchRes, err := index.Search(batchReq)

	textByFile := make(map[string]string)
	if err == nil {
		for _, hit := range batchRes.Hits {
			if arquivo, ok := hit.Fields["arquivo"].(string); ok {
				if txt, ok := hit.Fields["texto"].(string); ok {
					textByFile[arquivo] = txt
				}
			}
		}
	}

	for _, p := range points {
		docContent := p.ID
		if txt, exists := textByFile[p.ID]; exists {
			docContent = txt
		}
		clusterTexts[p.ClusterID] = append(clusterTexts[p.ClusterID], docContent)
		if clusterMap[p.ClusterID] == nil {
			clusterMap[p.ClusterID] = &clustering.Cluster{ID: p.ClusterID}
		}
		clusterMap[p.ClusterID].X += p.X
		clusterMap[p.ClusterID].Y += p.Y
	}

	for id, cluster := range clusterMap {
		count := float64(len(clusterTexts[id]))
		if count > 0 {
			cluster.X /= count
			cluster.Y /= count
		}
		cluster.Size = int(count)
		label, keywords := clustering.GenerateClusterLabel(clusterTexts[id], index)
		cluster.Label = label
		cluster.Keywords = keywords
	}

	var clusters []clustering.Cluster
	for _, c := range clusterMap {
		clusters = append(clusters, *c)
	}
	sort.Slice(clusters, func(i, j int) bool { return clusters[i].ID < clusters[j].ID })

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(KnowledgeMapResponse{
		Notes:    points,
		Clusters: clusters,
	})
}

var reindexRunning int32

func (ctx *HandlerContext) HandleReindexVectors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo nao suportado", http.StatusMethodNotAllowed)
		return
	}

	if !atomic.CompareAndSwapInt32(&reindexRunning, 0, 1) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"status": "ja em execucao"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "iniciado"})

	go func() {
		defer atomic.StoreInt32(&reindexRunning, 0)

		atomic.StoreInt32(&reindexProcessed, 0)
		atomic.StoreInt32(&reindexTotal, 0)

		cfg := ctx.Cfg
		if cfg == nil {
			slog.Warn("Motor vetorial nao configurado, abortando")
			events.GetHub().Broadcast("sync:error", map[string]string{"message": "Motor vetorial desativado"})
			return
		}

		events.GetHub().Broadcast("sync:started", map[string]string{"mode": "reindex"})
		slog.Info("Starting deep file scan")

		idx := ctx.Index
		req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
		req.Size = 50000
		req.Fields = []string{"arquivo"}
		searchRes, err := idx.Search(req)
		if err != nil {
			slog.Error("Erro ao buscar documentos no Bleve", "error", err)
			events.GetHub().Broadcast("sync:error", map[string]string{"message": "Erro ao acessar indice"})
			return
		}

		filesToProcess := make(map[string]string)
		slog.Info("Scanning Bleve fragments", "count", searchRes.Total)

		for _, hit := range searchRes.Hits {
			var arquivo string
			if val, ok := hit.Fields["arquivo"].(string); ok {
				arquivo = val
			}

			if arquivo != "" {
				if _, exists := filesToProcess[arquivo]; exists {
					continue
				}

				tags := ctx.State.GetFileTags(arquivo)
				hasEmbed := ingest.HasEmbedTag(tags)

				absPath := filepath.Join(cfg.DocsDir, arquivo)
				content, err := os.ReadFile(absPath)
				if !hasEmbed && err == nil {
					if strings.Contains(string(content), "#embed") {
						hasEmbed = true
					}
				}

				if hasEmbed {
					if err == nil {
						filesToProcess[arquivo] = string(content)
					} else {
						slog.Warn("Failed to read file", "file", arquivo, "error", err)
					}
				}
			}
		}

		total := len(filesToProcess)
		atomic.StoreInt32(&reindexTotal, int32(total))
		slog.Info("Total eligible notes for map", "total", total)

		if total == 0 {
			slog.Warn("No notes found with map tag. Check if #embed is used.")
			events.GetHub().Broadcast("sync:finished", map[string]interface{}{"new_docs": 0, "mode": "reindex"})
			return
		}

		const batchSize = 10
		var filenames []string
		var contents []string
		for fname, content := range filesToProcess {
			filenames = append(filenames, fname)
			contents = append(contents, content)
		}

		newVectors := make(map[string]ingest.NoteVector)
		var mu sync.Mutex

		for i := 0; i < len(filenames); i += batchSize {
			end := i + batchSize
			if end > len(filenames) {
				end = len(filenames)
			}
			batchFiles := filenames[i:end]
			batchTexts := contents[i:end]
			slog.Info("Processing batch", "batch", i/batchSize+1, "totalBatches", (len(filenames)+batchSize-1)/batchSize, "notes", len(batchFiles))

			ctxEmbed, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			effectiveHost := cfg.OllamaHost
			vecs, err := semantic.EmbedBatch(ctxEmbed, effectiveHost, cfg.OllamaModel, batchTexts, ctx.State.GetSettings().EmbeddingDimension)
			cancel()

			if err != nil {
				slog.Error("Erro no lote, fallback individual", "error", err)
				for j, fname := range batchFiles {
					embFunc := semantic.NewOllamaEmbedding(cfg.OllamaModel, effectiveHost, ctx.State.GetSettings().EmbeddingDimension)
					vec, err := embFunc(context.Background(), batchTexts[j])
					if err != nil {
						slog.Error("ERRO em embedding", "file", fname, "error", err)
					} else {
						title := extractTitle(batchTexts[j], fname)
						mu.Lock()
						newVectors[fname] = ingest.NoteVector{Vector: vec, Title: title}
						mu.Unlock()
					}
					atomic.AddInt32(&reindexProcessed, 1)
					events.GetHub().Broadcast("sync:progress", map[string]interface{}{
						"filename": fname, "current": atomic.LoadInt32(&reindexProcessed), "total": total,
					})
				}
				continue
			}

			for j, fname := range batchFiles {
				vec, ok := vecs[batchTexts[j]]
				if !ok {
					continue
				}
				title := extractTitle(batchTexts[j], fname)
				mu.Lock()
				newVectors[fname] = ingest.NoteVector{Vector: vec, Title: title}
				mu.Unlock()
				atomic.AddInt32(&reindexProcessed, 1)
				events.GetHub().Broadcast("sync:progress", map[string]interface{}{
					"filename": fname, "current": atomic.LoadInt32(&reindexProcessed), "total": total,
				})
			}
		}

		slog.Info("Reindex completed", "success", len(newVectors), "total", total)

		if len(newVectors) > 0 {
			ctx.State.ClearNoteVectors()
			ctx.State.ClearNoteProjections()
			for fname, nv := range newVectors {
				ctx.State.SetNoteVector(fname, nv.Vector, nv.Title)
			}
		}

		events.GetHub().Broadcast("sync:finished", map[string]interface{}{"new_docs": len(newVectors), "mode": "reindex"})
	}()
}

func extractTitle(content, filename string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		clean := strings.TrimSpace(strings.TrimLeft(line, "# "))
		if clean != "" {
			return clean
		}
	}
	parts := strings.Split(filename, "/")
	return strings.TrimSuffix(parts[len(parts)-1], ".md")
}
func (ctx *HandlerContext) HandleRefactorSemanticLinks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		OldTopic string `json:"oldTopic"`
		NewTopic string `json:"newTopic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if req.OldTopic == "" || req.NewTopic == "" {
		http.Error(w, "Tópicos antigo e novo são obrigatórios", http.StatusBadRequest)
		return
	}

	slog.Info("Iniciando refatoração global de links semânticos", "old", req.OldTopic, "new", req.NewTopic)

	count := 0
	err := filepath.Walk(ctx.Cfg.DocsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		modified := false
		newContent := string(content)

		// Regex para encontrar o segmento em qualquer posição:
		// Exemplos: @[Topic], @[Parent/Topic], @[Topic/Child], @[Parent/Topic/Child]
		// O segmento deve estar entre [ ou / e / ou ]
		re := regexp.MustCompile(`(@\[|/)(` + regexp.QuoteMeta(req.OldTopic) + `)(/|\])`)
		
		if re.MatchString(newContent) {
			newContent = re.ReplaceAllStringFunc(newContent, func(match string) string {
				// Precisamos preservar o prefixo e o sufixo
				// match será algo como "@[OldTopic/", "/OldTopic/", "/OldTopic]", etc.
				subMatches := re.FindStringSubmatch(match)
				if len(subMatches) < 4 {
					return match
				}
				prefix := subMatches[1]
				suffix := subMatches[3]
				return prefix + req.NewTopic + suffix
			})
			modified = true
		}

		if modified {
			if err := os.WriteFile(path, []byte(newContent), info.Mode()); err == nil {
				count++
				relPath, _ := filepath.Rel(ctx.Cfg.DocsDir, path)
				if ctx.Coordinator != nil {
					ctx.Coordinator.Push(relPath, ingest.JobFileUpdate, false)
				}
			}
		}
		return nil
	})

	if err != nil {
		http.Error(w, "Erro ao processar arquivos: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "success", "filesUpdated": %d}`, count)
}
