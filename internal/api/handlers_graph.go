package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"ton618/internal/index"
)

type node struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	X          float64  `json:"x"`
	Y          float64  `json:"y"`
	ClusterID  int      `json:"cluster_id"`
	NoteType   string   `json:"note_type"`
	Tags       []string `json:"tags"`
	Popularity int      `json:"popularity"`
	Radius     float64  `json:"radius"`
	Color      string   `json:"color"`
}

type link struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// tagColor gera uma cor HSL deterministica a partir de uma string de tag.
// Mesma tag sempre gera a mesma cor.
func tagColor(tag string) string {
	h := 0
	for _, c := range tag {
		h = (h*31 + int(c)) % 360
	}
	// HSL: hue variavel, saturacao 60%, lightness 55%
	return fmt.Sprintf("hsl(%d, 60%%, 55%%)", h)
}

func (ctx *HandlerContext) HandleGraphData(w http.ResponseWriter, r *http.Request) {
	limit := 500
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 2000 {
			limit = v
		}
	}

	// 1. Carrega embeddings ja projetadas (2D) — rápido, sem BLOBs
	emb2D, err := ctx.Store.GetEmbeddings2DForGraph(limit)
	if err != nil {
		slog.Error("graph 2d query", "error", err)
	}

	links, _ := ctx.Store.GetAllLinks()

	fileNodes := make(map[string]node)
	fileSeen := make(map[string]bool)

	// 2. Processa embeddings 2D existentes
	for _, e := range emb2D {
		if e.Arquivo == "" || fileSeen[e.Arquivo] {
			continue
		}
		fileSeen[e.Arquivo] = true

		parts := strings.Split(e.Arquivo, "/")
		baseName := parts[len(parts)-1]
		baseName = strings.TrimSuffix(baseName, ".md")
		baseName = strings.TrimSuffix(baseName, ".pdf")

		noteType := "note"
		if strings.HasPrefix(e.Arquivo, "pdfs/") || strings.HasSuffix(strings.ToLower(e.Arquivo), ".pdf") {
			noteType = "pdf"
		}

		fileTags := []string{}
		if t, err := ctx.Store.GetFileTags(e.Arquivo); err == nil {
			fileTags = t
			for _, tag := range fileTags {
				switch strings.ToLower(strings.TrimSpace(tag)) {
				case "youtube", "video":
					noteType = "video"
				case "artigo", "article", "captura":
					if noteType != "video" {
						noteType = "article"
					}
				}
			}
		}

		pop := ctx.Store.GetPopularity(e.Arquivo)
		radius := 6.0 + math.Log2(float64(pop)+1)*2.0
		if radius > 20 {
			radius = 20
		}

		color := "#38bdf8"
		if len(fileTags) > 0 {
			color = tagColor(fileTags[0])
		} else if noteType == "pdf" {
			color = "#f59e0b"
		}

		fileNodes[e.Arquivo] = node{
			ID:         e.Arquivo,
			Title:      baseName,
			X:          e.X,
			Y:          e.Y,
			NoteType:   noteType,
			Tags:       fileTags,
			Popularity: pop,
			Radius:     radius,
			Color:      color,
		}
	}

	// 3. Se ha poucos nos, projeta embeddings sem 2D via PCA e agenda t-SNE
	if len(fileNodes) < limit/2 {
		vecsForProjection, err := ctx.Store.GetEmbeddings2DWithVectors(limit)
		if err == nil && len(vecsForProjection) > 0 {
			vecs := make(map[string][]float32)
			fileToDocID := make(map[string]string) // arquivo -> docID
			for docID, nv := range vecsForProjection {
				doc, _ := ctx.Store.GetDocument(docID)
				if doc == nil || doc.Arquivo == "" || fileSeen[doc.Arquivo] || len(nv.Vector) == 0 {
					continue
				}
				if _, ok := fileToDocID[doc.Arquivo]; ok {
					continue
				}
				vecs[doc.Arquivo] = nv.Vector
				fileToDocID[doc.Arquivo] = docID
			}

			if len(vecs) > 1 {
				projected := index.Project2DReduce(vecs)
				index.ScalePoints(projected, 500)
				for arquivo, pt := range projected {
					if docID, ok := fileToDocID[arquivo]; ok {
						ctx.Store.SetEmbedding2D(docID, pt.X, pt.Y)
					}
					if !fileSeen[arquivo] {
						fileSeen[arquivo] = true
						parts := strings.Split(arquivo, "/")
						baseName := parts[len(parts)-1]
						baseName = strings.TrimSuffix(baseName, ".md")
						baseName = strings.TrimSuffix(baseName, ".pdf")
						fileNodes[arquivo] = node{
							ID:       arquivo,
							Title:    baseName,
							X:        pt.X,
							Y:        pt.Y,
							NoteType: "note",
							Radius:   6,
							Color:    "#38bdf8",
						}
					}
				}
				ctx.Watcher.QueueReproject()
			}
		}
	}

	// 4. Clustering no espaço original (high-D) em vez de 2D
	//    Carrega vetores originais para agrupar por similaridade semântica real.
	//    Fallback para clustering 2D se vetores não estiverem disponíveis.
	var clusterMap map[string]int
	var clusterCount int
	{
		highDVecs := make(map[string][]float32)
		for arquivo := range fileNodes {
			docs, err := ctx.Store.GetDocumentsByFile(arquivo)
			if err != nil || len(docs) == 0 {
				continue
			}
			nv, err := ctx.Store.GetEmbedding(docs[0].ID)
			if err == nil && nv != nil && len(nv.Vector) > 0 {
				highDVecs[arquivo] = nv.Vector
			}
		}

		if len(highDVecs) >= 3 {
			// Cluster no espaço high-D (768D) — agrupa por assunto real
			clusterMap, clusterCount = index.ClusterHighD(highDVecs)
			slog.Debug("grafo: cluster high-D", "notas", len(highDVecs), "clusters", clusterCount)
		}

		// Fallback: se high-D falhou ou deu poucos pontos, usa 2D
		if clusterCount < 2 {
			pts := make(map[string]index.Point2D)
			for arquivo, n := range fileNodes {
				pts[arquivo] = index.Point2D{X: n.X, Y: n.Y}
			}
			clusterMap, clusterCount = index.ClusterPoints(pts)
			slog.Debug("grafo: fallback cluster 2D", "notas", len(pts), "clusters", clusterCount)
		}
	}

	// Itera em ordem deterministica (sorted keys) para que o grid fallback
	// produza as mesmas posicoes independente da ordem aleatoria do map.
	// Go maps tem iteracao randomizada — sem sort, a posicao das notas
	// sem embedding (X=0,Y=0) muda a cada carregamento.
	fileKeys := make([]string, 0, len(fileNodes))
	for k := range fileNodes {
		fileKeys = append(fileKeys, k)
	}
	sort.Strings(fileKeys)

	var nodes []node
	for _, key := range fileKeys {
		n := fileNodes[key]
		clusterID := 0
		if c, ok := clusterMap[n.ID]; ok {
			clusterID = c
		}

		if n.X == 0 && n.Y == 0 {
			idx := len(nodes)
			cols := math.Ceil(math.Sqrt(float64(len(fileNodes))))
			if cols < 3 {
				cols = 3
			}
			n.X = float64(int(idx)%int(cols))*120 + 60
			n.Y = float64(int(idx)/int(cols))*120 + 60
		}

		nodes = append(nodes, node{
			ID:         n.ID,
			Title:      n.Title,
			X:          n.X,
			Y:          n.Y,
			ClusterID:  clusterID,
			NoteType:   n.NoteType,
			Tags:       n.Tags,
			Popularity: n.Popularity,
			Radius:     n.Radius,
			Color:      n.Color,
		})
	}

	var edgeList []link
	for fromFile, toFiles := range links {
		if !fileSeen[fromFile] {
			continue
		}
		for _, toFile := range toFiles {
			if fileSeen[toFile] {
				edgeList = append(edgeList, link{Source: fromFile, Target: toFile})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	result := map[string]interface{}{
		"nodes":    nodes,
		"links":    edgeList,
		"clusters": clusterCount,
	}
	json.NewEncoder(w).Encode(result)
}

func (ctx *HandlerContext) HandleGraphProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	allFileEmbs, err := ctx.Store.GetAllFileEmbeddings()
	if err != nil {
		http.Error(w, "erro ao carregar embeddings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	vecs := make(map[string][]float32)
	fileToDoc := make(map[string]string)
	for _, fe := range allFileEmbs {
		if len(fe.Vector) == 0 {
			continue
		}
		fileToDoc[fe.Arquivo] = fe.DocID
		vecs[fe.Arquivo] = fe.Vector
	}
	if len(vecs) < 2 {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"nodes":%d}`, len(vecs))
		return
	}

	// Usa t-SNE em vez de PCA — preserva vizinhanças semânticas muito melhor
	// e agrupa notas similares naturalmente no espaço 2D.
	slog.Info("Reprojetando mapa semântico com t-SNE", "notas", len(vecs))
	tsne := index.DefaultTSNE()
	projected := tsne.Project(vecs)

	count := 0
	for arquivo, pt := range projected {
		if docID, ok := fileToDoc[arquivo]; ok {
			if err := ctx.Store.SetEmbedding2D(docID, pt.X, pt.Y); err == nil {
				count++
			}
		}
	}
	slog.Info("Projeção t-SNE concluída", "projetadas", count)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":true,"nodes":%d,"projected":%d,"method":"tsne"}`, len(vecs), count)
}

func (ctx *HandlerContext) HandleGraphQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set request timeout
	rCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var body struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Query == "" {
		http.Error(w, "query required", http.StatusBadRequest)
		return
	}
	if ctx.Embed == nil {
		http.Error(w, "embedding not configured", http.StatusServiceUnavailable)
		return
	}
	queryVec, err := ctx.Embed.Embed(rCtx, body.Query)
	if err != nil {
		http.Error(w, "embedding failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type nearest struct {
		Arquivo    string  `json:"arquivo"`
		Title      string  `json:"title"`
		Similarity float64 `json:"similarity"`
		X          float64 `json:"x"`
		Y          float64 `json:"y"`
	}
	var results []nearest

	// 1. Carrega pool de candidatos com coordenadas 2D (leve, sem BLOBs).
	//    Sem limite fixo — coordenadas 2D são apenas 2 float64 por embedding.
	//    O SQLite lida bem com 20K+ registros só com x,y.
	candidates, err := ctx.Store.GetEmbeddings2DForGraph(10000)
	if err != nil {
		slog.Error("graph query: candidates", "error", err)
	}

	// 2. Filtragem geométrica em 2D: ordena candidatos por distância Euclidiana
	//    aproximada a partir da origem (centro do gráfico). Isso evita carregar
	//    BLOBs de vetores para notas que estão longe no espaço semântico.
	//    Usamos apenas os topN candidatos mais próximos do centro para a busca
	//    exata por similaridade de cosseno.
	const topN = 500
	if len(candidates) > topN {
		// Ordena por distância ao centro (0,0) — notas perto do centro
		// tendem a ser semanticamente médias/relevantes. Notas nos extremos
		// são especializadas e menos propensas a matches genéricos.
		sort.Slice(candidates, func(i, j int) bool {
			di := candidates[i].X*candidates[i].X + candidates[i].Y*candidates[i].Y
			dj := candidates[j].X*candidates[j].X + candidates[j].Y*candidates[j].Y
			return di < dj
		})
		candidates = candidates[:topN]
	}

	// 3. Carrega vetores em lote (1 query, não N queries individuais)
	docIDs := make([]string, len(candidates))
	for i, e := range candidates {
		docIDs[i] = e.DocID
	}
	vecMap, err := ctx.Store.GetEmbeddingsByDocIDs(docIDs)
	if err != nil {
		slog.Error("graph query: batch load", "error", err)
	}

	// 4. Calcula similaridade de cosseno exata
	for _, e := range candidates {
		nv, ok := vecMap[e.DocID]
		if !ok || len(nv.Vector) == 0 {
			continue
		}
		sim := index.CosineSimilarity(queryVec, nv.Vector)
		if sim < 0.7 {
			continue
		}
		title := e.Title
		if title == "" {
			parts := strings.Split(e.Arquivo, "/")
			title = parts[len(parts)-1]
			title = strings.TrimSuffix(title, ".md")
			title = strings.TrimSuffix(title, ".pdf")
		}
		results = append(results, nearest{
			Arquivo:    e.Arquivo,
			Title:      title,
			Similarity: sim,
			X:          e.X,
			Y:          e.Y,
		})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Similarity > results[j].Similarity })

	if len(results) > 20 {
		results = results[:20]
	}
	var qx, qy, totalWeight float64
	n := 5
	if len(results) < n {
		n = len(results)
	}
	for i := 0; i < n; i++ {
		weight := results[i].Similarity
		qx += results[i].X * weight
		qy += results[i].Y * weight
		totalWeight += weight
	}
	if totalWeight > 0 {
		qx /= totalWeight
		qy /= totalWeight
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query_x": qx, "query_y": qy, "query_text": body.Query, "nearest": results,
	})
}
