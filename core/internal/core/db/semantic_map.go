package db

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
)


// SemanticMapPoint representa uma nota projetada no plano 2D via PCA.
type SemanticMapPoint struct {
	Filename string  `json:"filename"`
	Title    string  `json:"title"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Cluster  int     `json:"cluster"`
}

// maxClusters é o número máximo de clusters (cores) no mapa.
const maxClusters = 5

// ── Cache thread-safe ──

// semanticMapCache guarda o resultado da PCA com invalidação por checksum.
type semanticMapCache struct {
	mu       sync.RWMutex
	points   []SemanticMapPoint
	checksum uint64 // hash simples dos filenames
}

var mapCache = &semanticMapCache{}

// computeChecksum faz um hash simples dos filenames para detectar mudanças.
func computeChecksum(embeddings map[string][]float32) uint64 {
	var h uint64 = 14695981039346656037 // FNV-1a offset basis (64-bit)
	keys := make([]string, 0, len(embeddings))
	for k := range embeddings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h ^= uint64(k[0])
		h *= 1099511628211 // FNV-1a prime
		for i := 1; i < len(k); i++ {
			h ^= uint64(k[i])
			h *= 1099511628211
		}
	}
	return h
}

// GetSemanticMapPoints retorna os pontos do cache ou computa PCA se desatualizado.
// Thread-safe: leituras concorrentes são livres, escrita exclusiva.
func (s *Store) GetSemanticMapPoints() ([]SemanticMapPoint, error) {
	embeddings, err := s.getAllEmbeddingsGrouped()
	if err != nil {
		return nil, fmt.Errorf("get embeddings: %w", err)
	}

	cs := computeChecksum(embeddings)

	// Fast path: leitura com RLock
	mapCache.mu.RLock()
	if mapCache.checksum == cs && mapCache.points != nil {
		pts := mapCache.points
		mapCache.mu.RUnlock()
		return pts, nil
	}
	mapCache.mu.RUnlock()

	// Cache miss: escrita exclusiva
	mapCache.mu.Lock()
	defer mapCache.mu.Unlock()

	// Double-check after acquiring write lock
	if mapCache.checksum == cs && mapCache.points != nil {
		return mapCache.points, nil
	}

	points, err := computePCA2D(embeddings)
	if err != nil {
		return nil, err
	}

	mapCache.points = points
	mapCache.checksum = cs
	return points, nil
}

// ── PCA 384D → 2D ──

// getAllEmbeddingsGrouped retorna o embedding do primeiro chunk de cada nota.
func (s *Store) getAllEmbeddingsGrouped() (map[string][]float32, error) {
	rows, err := s.DB.Query(`
		SELECT ne.chunk_id, ne.embedding
		FROM note_embeddings ne
		WHERE ne.chunk_id LIKE '%#0'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]float32)
	for rows.Next() {
		var chunkID string
		var blob []byte
		if err := rows.Scan(&chunkID, &blob); err != nil {
			continue
		}
		if len(blob) != EmbeddingDim*4 {
			continue
		}
		// Extrai filename do chunk_id (ex: "notes/foo.md#0" → "notes/foo.md")
		filename := chunkID
		for i := len(chunkID) - 1; i >= 0; i-- {
			if chunkID[i] == '#' {
				filename = chunkID[:i]
				break
			}
		}
		if _, exists := result[filename]; exists {
			continue // já temos, pula
		}

		emb := make([]float32, EmbeddingDim)
		for i := 0; i < EmbeddingDim; i++ {
			bits := binary.LittleEndian.Uint32(blob[i*4 : (i+1)*4])
			emb[i] = math.Float32frombits(bits)
		}
		result[filename] = emb
	}
	return result, rows.Err()
}

// computePCA2D reduz embeddings 384D → 2D via PCA clássica.
// Etapas:
//
//	1. Guard-clause: se N < 2, retorna pontos em (0,0)
//	2. Constrói matriz N×384 a partir do map
//	3. Centraliza (subtrai média de cada dimensão)
//	4. Calcula matriz de covariância 384×384 (amostral: divisão por N-1)
//	5. Power iteration para top-2 autovetores
//	6. Projeta cada ponto nos 2 componentes
//	7. Atribui clusters via K-means (2D)
func computePCA2D(embeddings map[string][]float32) ([]SemanticMapPoint, error) {
	n := len(embeddings)

	// ⚠️ Guard-clause: PCA exige pelo menos 2 pontos
	if n < 2 {
		var pts []SemanticMapPoint
		for filename := range embeddings {
			pts = append(pts, SemanticMapPoint{
				Filename: filename,
				Title:    displayName(filename),
				X:        0,
				Y:        0,
				Cluster:  0,
			})
		}
		return pts, nil
	}

	dim := EmbeddingDim // 384

	// 1. Extrair para slices ordenados (garantir ordem determinística)
	filenames := make([]string, 0, n)
	for f := range embeddings {
		filenames = append(filenames, f)
	}
	sort.Strings(filenames)

	// Matriz N×384 como []float64 para precisão da PCA
	mat := make([][]float64, n)
	for i, fname := range filenames {
		mat[i] = make([]float64, dim)
		emb := embeddings[fname]
		for j := 0; j < dim; j++ {
			mat[i][j] = float64(emb[j])
		}
	}

	// 2. Centralizar: subtrair média de cada dimensão
	for j := 0; j < dim; j++ {
		var mean float64
		for i := 0; i < n; i++ {
			mean += mat[i][j]
		}
		mean /= float64(n)
		for i := 0; i < n; i++ {
			mat[i][j] -= mean
		}
	}

	// 3. Matriz de covariância 384×384
	// Cov(a,b) = (1/(N-1)) * Σ (x_i,a - μ_a)(x_i,b - μ_b)
	// Como já centralizamos: Cov = (1/(N-1)) * X^T * X
	cov := make([][]float64, dim)
	// Primeiro aloca todas as linhas (evita race condition quando i < j)
	for i := 0; i < dim; i++ {
		cov[i] = make([]float64, dim)
	}
	// Depois preenche usando a simetria
	for i := 0; i < dim; i++ {
		for j := i; j < dim; j++ {
			var sum float64
			for k := 0; k < n; k++ {
				sum += mat[k][i] * mat[k][j]
			}
			// ⚠️ Divisão por (N-1) — amostral. Para N=1 o guard-clause já trata.
			cov[i][j] = sum / float64(n-1)
			cov[j][i] = cov[i][j] // simétrica (cov[j] já foi alocado)
		}
	}

	// 4. Power iteration para os 2 maiores autovetores
	// Verifica se a matriz de covariância é não-nula (pode ser zero se
	// todos os embeddings forem idênticos após centralização).
	var covNorm float64
	for i := 0; i < dim && i < 5; i++ {
		for j := 0; j < dim && j < 5; j++ {
			covNorm += cov[i][j] * cov[i][j]
		}
	}
	if covNorm < 1e-15 {
		// Matriz degenerada: todos os pontos no centro
		points := make([]SemanticMapPoint, n)
		for i, fname := range filenames {
			points[i] = SemanticMapPoint{
				Filename: fname,
				Title:    displayName(fname),
				X:        0,
				Y:        0,
				Cluster:  0,
			}
		}
		return points, nil
	}

	comp0 := powerIteration(cov, dim, 50)
	comp1 := powerIterationDeflated(cov, dim, comp0, 50)

	// 5. Projetar cada ponto nos 2 componentes
	points := make([]SemanticMapPoint, n)
	for i, fname := range filenames {
		var x, y float64
		for j := 0; j < dim; j++ {
			x += mat[i][j] * comp0[j]
			y += mat[i][j] * comp1[j]
		}
		points[i] = SemanticMapPoint{
			Filename: fname,
			Title:    displayName(fname),
			X:        x,
			Y:        y,
		}
	}

	// 6. K-means nos pontos 2D para atribuir clusters
	k := maxClusters
	if n < k {
		k = n
	}
	if k < 1 {
		k = 1
	}
	clusters := kMeans2D(points, k, 20)
	for i := range points {
		points[i].Cluster = clusters[i]
	}

	return points, nil
}

// powerIteration encontra o autovetor dominante via power iteration.
func powerIteration(cov [][]float64, dim, iterations int) []float64 {
	// Vetor aleatório inicial (fonte local para thread safety)
	rng := rand.New(rand.NewSource(int64(dim)))
	v := make([]float64, dim)
	for i := 0; i < dim; i++ {
		v[i] = rng.Float64()*2 - 1
	}
	v = normalize(v)

	for iter := 0; iter < iterations; iter++ {
		// v' = cov * v
		vNew := make([]float64, dim)
		for i := 0; i < dim; i++ {
			var sum float64
			for j := 0; j < dim; j++ {
				sum += cov[i][j] * v[j]
			}
			vNew[i] = sum
		}
		v = normalize(vNew)
	}
	return v
}

// powerIterationDeflated encontra o segundo autovetor,
// removendo a contribuição do primeiro (deflação de Hotelling).
func powerIterationDeflated(cov [][]float64, dim int, comp0 []float64, iterations int) []float64 {
	// Deeflação: cov' = cov - λ₁ * v₁ * v₁ᵀ
	// λ₁ ≈ v₁ᵀ * cov * v₁ (Rayleigh quotient)
	var lambda float64
	for i := 0; i < dim; i++ {
		var rowSum float64
		for j := 0; j < dim; j++ {
			rowSum += cov[i][j] * comp0[j]
		}
		lambda += comp0[i] * rowSum
	}

	// Matriz deflacionada (só usamos via multiplicação, não construímos explícito)
	// cov' * v = cov * v - λ₁ * v₁ * (v₁ᵀ * v)
	deflatedMul := func(v []float64) []float64 {
		result := make([]float64, dim)
		// cov * v
		for i := 0; i < dim; i++ {
			var sum float64
			for j := 0; j < dim; j++ {
				sum += cov[i][j] * v[j]
			}
			result[i] = sum
		}
		// - λ₁ * v₁ * (v₁ᵀ * v)
		var dot float64
		for j := 0; j < dim; j++ {
			dot += comp0[j] * v[j]
		}
		for i := 0; i < dim; i++ {
			result[i] -= lambda * comp0[i] * dot
		}
		return result
	}

	// Power iteration na matriz deflacionada
	rng := rand.New(rand.NewSource(int64(dim + 1)))
	v := make([]float64, dim)
	for i := 0; i < dim; i++ {
		v[i] = rng.Float64()*2 - 1
	}
	// Ortogonaliza em relação a comp0
	var dot float64
	for j := 0; j < dim; j++ {
		dot += comp0[j] * v[j]
	}
	for i := 0; i < dim; i++ {
		v[i] -= dot * comp0[i]
	}
	v = normalize(v)

	for iter := 0; iter < iterations; iter++ {
		vNew := deflatedMul(v)
		// Re-ortogonaliza
		var d float64
		for j := 0; j < dim; j++ {
			d += comp0[j] * vNew[j]
		}
		for i := 0; i < dim; i++ {
			vNew[i] -= d * comp0[i]
		}
		v = normalize(vNew)
	}
	return v
}

func normalize(v []float64) []float64 {
	var norm float64
	for _, val := range v {
		norm += val * val
	}
	norm = math.Sqrt(norm)
	if norm < 1e-15 {
		return v
	}
	for i := range v {
		v[i] /= norm
	}
	return v
}

// ── K-Means 2D ──

// kMeans2D atribui cada ponto a um cluster (0..k-1).
// Garantias:
//   - k ≤ len(points)
//   - centróides vazios são recolocados no ponto mais distante do seu centro
func kMeans2D(points []SemanticMapPoint, k, maxIter int) []int {
	n := len(points)
	if n == 0 {
		return nil
	}
	if k <= 1 || n <= 1 {
		labels := make([]int, n)
		return labels
	}
	// ⚠️ Garantia: k não pode ser maior que n
	if k > n {
		k = n
	}

	// Inicialização: k-means++ (escolhe centros distantes)
	centroids := make([][2]float64, k)
	centroids[0] = [2]float64{points[0].X, points[0].Y}
	for c := 1; c < k; c++ {
		// Escolhe ponto proporcional à distância² ao centro mais próximo
		var totalDist float64
		dists := make([]float64, n)
		for i := 0; i < n; i++ {
			minD := math.MaxFloat64
			for j := 0; j < c; j++ {
				dx := points[i].X - centroids[j][0]
				dy := points[i].Y - centroids[j][1]
				d := dx*dx + dy*dy
				if d < minD {
					minD = d
				}
			}
			dists[i] = minD
			totalDist += minD
		}
		// Sorteio ponderado (fonte local para thread safety)
		rng := rand.New(rand.NewSource(int64(c)))
		target := rng.Float64() * totalDist
		var cum float64
		chosen := 0
		for i := 0; i < n; i++ {
			cum += dists[i]
			if cum >= target {
				chosen = i
				break
			}
		}
		centroids[c] = [2]float64{points[chosen].X, points[chosen].Y}
	}

	labels := make([]int, n)
	for iter := 0; iter < maxIter; iter++ {
		// Atribuir cada ponto ao centro mais próximo
		changed := false
		for i := 0; i < n; i++ {
			minD := math.MaxFloat64
			best := 0
			for j := 0; j < k; j++ {
				dx := points[i].X - centroids[j][0]
				dy := points[i].Y - centroids[j][1]
				d := dx*dx + dy*dy
				if d < minD {
					minD = d
					best = j
				}
			}
			if labels[i] != best {
				labels[i] = best
				changed = true
			}
		}
		if !changed {
			break
		}

		// Recalcular centróides
		counts := make([]int, k)
		newCentroids := make([][2]float64, k)
		for i := 0; i < n; i++ {
			c := labels[i]
			counts[c]++
			newCentroids[c][0] += points[i].X
			newCentroids[c][1] += points[i].Y
		}
		for j := 0; j < k; j++ {
			if counts[j] > 0 {
				newCentroids[j][0] /= float64(counts[j])
				newCentroids[j][1] /= float64(counts[j])
			} else {
				// ⚠️ Centróide vazio: recoloca no ponto mais distante do seu centro atual
				var maxD float64
				var farthest int
				for i := 0; i < n; i++ {
					dx := points[i].X - centroids[j][0]
					dy := points[i].Y - centroids[j][1]
					d := dx*dx + dy*dy
					if d > maxD {
						maxD = d
						farthest = i
					}
				}
				newCentroids[j] = [2]float64{points[farthest].X, points[farthest].Y}
				labels[farthest] = j
				counts[j] = 1
			}
		}
		centroids = newCentroids
	}

	return labels
}

// displayName extrai um título amigável do filename.
func displayName(filename string) string {
	// Remove diretório e extensão
	start := 0
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '/' {
			start = i + 1
			break
		}
	}
	end := len(filename)
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			end = i
			break
		}
	}
	if start >= end {
		return filename
	}
	return filename[start:end]
}
