package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

func (ctx *HandlerContext) HandleKnowledgeMap(w http.ResponseWriter, r *http.Request) {
	// 1. Recuperar todos os vetores de notas do BBolt
	allVectors := ctx.State.GetAllNoteVectors()

	// Filtrar apenas notas que possuem a tag #embed
	noteVectors := make(map[string][]float32)
	index := ctx.Index

	for id, vec := range allVectors {
		// 1. Verificar tags (rápido - cache em RAM)
		tags := ctx.State.GetFileTags(id)
		hasEmbed := false
		for _, t := range tags {
			if strings.ToLower(t) == "embed" {
				hasEmbed = true
				break
			}
		}

		if hasEmbed {
			noteVectors[id] = vec
			continue
		}

		// 2. Fallback: Verificar se o texto bruto em algum fragmento contém #embed
		q := bleve.NewTermQuery(id)
		q.SetField("arquivo")
		searchReq := bleve.NewSearchRequest(q)
		searchReq.Fields = []string{"texto"}
		searchRes, err := index.Search(searchReq)

		if err == nil && searchRes.Total > 0 {
			for _, hit := range searchRes.Hits {
				if txt, ok := hit.Fields["texto"].(string); ok {
					if strings.Contains(txt, "#embed") {
						noteVectors[id] = vec
						break
					}
				}
			}
		}
	}

	log.Printf("[KnowledgeMap] Vetores com #embed encontrados: %d\n", len(noteVectors))
	if len(noteVectors) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(KnowledgeMapResponse{Notes: []clustering.Point{}, Clusters: []clustering.Cluster{}})
		return
	}

	// 2. Tentar carregar projeções do cache
	projections := ctx.State.GetAllNoteProjections()
	useCache := len(projections) == len(noteVectors) && len(noteVectors) > 0

	// Validar se os IDs no cache batem com os IDs atuais
	if useCache {
		for id := range noteVectors {
			if _, ok := projections[id]; !ok {
				useCache = false
				break
			}
		}
	}

	if useCache {
		log.Printf("[KnowledgeMap] Usando %d projeções do cache persistente\n", len(projections))
	} else {
		// 3. Projetar para 2D usando PCA (Cálculo pesado)
		log.Printf("[KnowledgeMap] Calculando novas projeções PCA para %d notas...\n", len(noteVectors))
		rawProjections := clustering.ProjectPCA(noteVectors)

		// Converter map[string][2]float64 para map[string][]float64 para o cache
		projections = make(map[string][]float64)
		for id, coords := range rawProjections {
			projections[id] = []float64{coords[0], coords[1]}
		}
		ctx.State.SetNoteProjections(projections)
	}

	// 4. Preparar pontos para Clustering (Ordenação determinística é vital aqui!)
	var points []clustering.Point
	for id, coords := range projections {
		points = append(points, clustering.Point{
			ID: id,
			X:  coords[0],
			Y:  coords[1],
		})
	}
	// IMPORTANTE: Ordenar por ID garante que o K-Means receba os mesmos dados na mesma ordem
	sort.Slice(points, func(i, j int) bool { return points[i].ID < points[j].ID })

	// 4. Executar K-Means
	k := int(3.0 + 0.5*float64(len(points))/5.0)
	if k > 10 {
		k = 10
	}
	if k > len(points) {
		k = len(points)
	}

	points = clustering.KMeans(points, k, 20)
	log.Printf("[KnowledgeMap] Pontos agrupados em %d clusters\n", k)

	// 5. Gerar Labels para os Clusters via TF-IDF
	clusterMap := make(map[int]*clustering.Cluster)
	clusterTexts := make(map[int][]string)

	// Usar o índice já obtido
	for i, p := range points {
		// Buscar o conteúdo real fazendo uma query pelo nome do arquivo
		docContent := p.ID
		q := bleve.NewTermQuery(p.ID)
		q.SetField("arquivo")
		searchReq := bleve.NewSearchRequest(q)
		searchReq.Fields = []string{"texto"}
		searchRes, err := index.Search(searchReq)

		if err == nil && searchRes.Total > 0 {
			if txt, ok := searchRes.Hits[0].Fields["texto"].(string); ok {
				docContent = txt
				// Extrair a primeira linha para ser o título do ponto
				lines := strings.Split(txt, "\n")
				for _, line := range lines {
					clean := strings.TrimSpace(strings.TrimLeft(line, "# "))
					if clean != "" {
						points[i].Title = clean
						break
					}
				}
			}
		}

		if points[i].Title == "" {
			parts := strings.Split(p.ID, "/")
			points[i].Title = parts[len(parts)-1]
		}

		preview := docContent
		if len(preview) > 50 {
			preview = preview[:50]
		}
		log.Printf("[KnowledgeMap] Batizando com conteúdo de %s: %s...\n", p.ID, preview)
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
		log.Printf("[KnowledgeMap] ILHA %d BATIZADA COMO: %s (Size: %d, Keywords: %v)\n", id, cluster.Label, cluster.Size, cluster.Keywords)
	}

	var clusters []clustering.Cluster
	for _, c := range clusterMap {
		clusters = append(clusters, *c)
	}
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].ID < clusters[j].ID
	})

	// 6. Responder
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(KnowledgeMapResponse{
		Notes:    points,
		Clusters: clusters,
	})
}

// reindexRunning impede execuções paralelas do reindex
var reindexRunning int32

// HandleReindexVectors popula o bucket note_vectors para todas as notas já no Bleve.
func (ctx *HandlerContext) HandleReindexVectors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não suportado", http.StatusMethodNotAllowed)
		return
	}

	if !atomic.CompareAndSwapInt32(&reindexRunning, 0, 1) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"status": "já em execução"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "iniciado"})

	go func() {
		defer atomic.StoreInt32(&reindexRunning, 0)

		atomic.StoreInt32(&reindexRunning, 1)
		atomic.StoreInt32(&reindexProcessed, 0)
		atomic.StoreInt32(&reindexTotal, 0)

		cfg := ctx.Cfg
		if cfg == nil {
			log.Println("[Reindex] Motor vetorial não configurado, abortando.")
			events.GetHub().Broadcast("sync:error", map[string]string{"message": "Motor vetorial desativado"})
			return
		}

		events.GetHub().Broadcast("sync:started", map[string]string{"mode": "reindex"})
		log.Println("[Reindex] Iniciando varredura profunda de arquivos...")

		idx := ctx.Index
		req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
		req.Size = 50000
		req.Fields = []string{"arquivo"}
		searchRes, err := idx.Search(req)
		if err != nil {
			log.Printf("[Reindex] Erro crítico ao buscar documentos no Bleve: %v\n", err)
			events.GetHub().Broadcast("sync:error", map[string]string{"message": "Erro ao acessar índice"})
			return
		}

		filesToProcess := make(map[string]string)
		log.Printf("[Reindex] Varrendo %d fragmentos do Bleve...\n", searchRes.Total)

		for _, hit := range searchRes.Hits {
			var arquivo string
			if val, ok := hit.Fields["arquivo"].(string); ok {
				arquivo = val
			}

			if arquivo != "" {
				if _, exists := filesToProcess[arquivo]; exists {
					continue
				}

				// Whitelist obrigatório: Apenas notas com #embed são reindexadas
				tags := ctx.State.GetFileTags(arquivo)
				if ingest.HasEmbedTag(tags) {
					absPath := filepath.Join(cfg.DocsDir, arquivo)
					content, err := os.ReadFile(absPath)
					if err == nil {
						filesToProcess[arquivo] = string(content)
					} else {
						log.Printf("[Reindex] Aviso: Não foi possível ler arquivo %s: %v\n", arquivo, err)
					}
				} else {
					// Log opcional para depuração (pode ser ruidoso)
					// log.Printf("[Reindex] Ignorando %s (Tags: %v, Estratégia: %s)\n", arquivo, tags, strategy)
				}
			}
		}

		total := len(filesToProcess)
		atomic.StoreInt32(&reindexTotal, int32(total))
		log.Printf("[Reindex] Total de notas elegíveis para o mapa: %d\n", total)

		if total == 0 {
			log.Println("[Reindex] Nenhuma nota encontrada com tag de mapa. Verifique se usou #embed.")
			events.GetHub().Broadcast("sync:finished", map[string]interface{}{"new_docs": 0, "mode": "reindex"})
			return
		}

		newVectors := make(map[string][]float32)
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 2)

		for filename, content := range filesToProcess {
			wg.Add(1)
			go func(fname, txt string) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-time.After(1 * time.Minute): // Timeout para pegar slot
					log.Printf("[Reindex] Timeout ao aguardar slot para %s\n", fname)
					return
				}

				log.Printf("[Reindex] Processando embedding para: %s (%d chars)...\n", fname, len(txt))

				ctxEmbed, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer cancel()

				embFunc := semantic.NewOllamaEmbedding(cfg.OllamaModel, cfg.OllamaHost)
				vec, err := embFunc(ctxEmbed, txt)
				if err != nil {
					log.Printf("[Reindex] ERRO em %s: %v\n", fname, err)
				} else {
					mu.Lock()
					newVectors[fname] = vec
					mu.Unlock()
					log.Printf("[Reindex] SUCESSO: %s vetorizada.\n", fname)
				}
				atomic.AddInt32(&reindexProcessed, 1)
				events.GetHub().Broadcast("sync:progress", map[string]interface{}{
					"filename": fname,
					"current":  atomic.LoadInt32(&reindexProcessed),
					"total":    total,
				})
			}(filename, content)
		}

		wg.Wait()

		log.Printf("[Reindex] Processamento concluído. Sucesso em %d de %d notas.\n", len(newVectors), total)

		if len(newVectors) > 0 {
			ctx.State.ClearNoteVectors()
			ctx.State.ClearNoteProjections()
			for fname, vec := range newVectors {
				ctx.State.SetNoteVector(fname, vec)
			}
			log.Println("[Reindex] Mapa atualizado com novos vetores.")
		} else {
			log.Println("[Reindex] NENHUM vetor foi gerado com sucesso. Mantendo estado anterior.")
		}

		events.GetHub().Broadcast("sync:finished", map[string]interface{}{"new_docs": len(newVectors), "mode": "reindex"})
		log.Println("[Reindex] Procedimento finalizado.")
	}()
}
