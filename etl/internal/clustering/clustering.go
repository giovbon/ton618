package clustering

import (
	"math/rand"
	"sort"

	"gonum.org/v1/gonum/mat"
)

// Point representa uma nota no espaço 2D
type Point struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	ClusterID int     `json:"cluster_id"`
}

// Cluster representa uma ilha de conhecimento
type Cluster struct {
	ID       int      `json:"id"`
	Label    string   `json:"label"`
	Keywords []string `json:"keywords"`
	Size     int      `json:"size"`
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
}

// ProjectPCA reduz vetores de alta dimensão para 2D usando PCA.
func ProjectPCA(noteVectors map[string][]float32) map[string][2]float64 {
	if len(noteVectors) < 2 {
		result := make(map[string][2]float64)
		for id := range noteVectors {
			result[id] = [2]float64{0, 0}
		}
		return result
	}

	// 1. Converter para matriz Gonum (ORDENADO para determinismo)
	rows := len(noteVectors)
	ids := make([]string, 0, rows)
	for id := range noteVectors {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	cols := len(noteVectors[ids[0]])
	data := make([]float64, rows*cols)

	for i, id := range ids {
		vec := noteVectors[id]
		for j, v := range vec {
			data[i*cols+j] = float64(v)
		}
	}

	matrix := mat.NewDense(rows, cols, data)

	// 2. Usar SVD para redução de dimensionalidade (Mais robusto que PCA para rows < cols)
	var svd mat.SVD
	ok := svd.Factorize(matrix, mat.SVDThin)
	if !ok {
		return make(map[string][2]float64)
	}

	// Pegar as 2 primeiras componentes (U * Sigma)
	var s mat.Dense
	svd.UTo(&s)

	sigma := svd.Values(nil)
	k := 2
	if len(sigma) < k {
		k = len(sigma)
	}

	result := make(map[string][2]float64)
	minX, maxX, minY, maxY := 0.0, 0.0, 0.0, 0.0

	coords := make([][2]float64, rows)
	for i := 0; i < rows; i++ {
		x := s.At(i, 0) * sigma[0]
		y := 0.0
		if k > 1 {
			y = s.At(i, 1) * sigma[1]
		}
		coords[i] = [2]float64{x, y}

		if i == 0 || x < minX {
			minX = x
		}
		if i == 0 || x > maxX {
			maxX = x
		}
		if i == 0 || y < minY {
			minY = y
		}
		if i == 0 || y > maxY {
			maxY = y
		}
	}

	rangeX := maxX - minX
	rangeY := maxY - minY
	if rangeX == 0 {
		rangeX = 1
	}
	if rangeY == 0 {
		rangeY = 1
	}

	for i, id := range ids {
		x := (coords[i][0] - minX) / rangeX * 100
		y := (coords[i][1] - minY) / rangeY * 100
		result[id] = [2]float64{x, y}
	}

	return result
}

// KMeans agrupa pontos em K clusters.
func KMeans(points []Point, k int, iterations int) []Point {
	if len(points) <= k {
		for i := range points {
			points[i].ClusterID = i
		}
		return points
	}

	// 1. Inicializar centroides aleatórios com semente local fixa
	localRand := rand.New(rand.NewSource(42))
	centroids := make([][2]float64, k)
	for i := 0; i < k; i++ {
		p := points[localRand.Intn(len(points))]
		centroids[i] = [2]float64{p.X, p.Y}
	}

	// 2. Iterar
	for iter := 0; iter < iterations; iter++ {
		// Atribuir cada ponto ao centroide mais próximo
		changed := false
		for i, p := range points {
			minDist := -1.0
			bestCluster := 0
			for c := 0; c < k; c++ {
				d := distSq(p.X, p.Y, centroids[c][0], centroids[c][1])
				if minDist == -1 || d < minDist {
					minDist = d
					bestCluster = c
				}
			}
			if p.ClusterID != bestCluster {
				points[i].ClusterID = bestCluster
				changed = true
			}
		}

		if !changed {
			break
		}

		// Atualizar centroides
		newCentroids := make([][2]float64, k)
		counts := make([]int, k)
		for _, p := range points {
			newCentroids[p.ClusterID][0] += p.X
			newCentroids[p.ClusterID][1] += p.Y
			counts[p.ClusterID]++
		}
		for i := 0; i < k; i++ {
			if counts[i] > 0 {
				centroids[i][0] = newCentroids[i][0] / float64(counts[i])
				centroids[i][1] = newCentroids[i][1] / float64(counts[i])
			}
		}
	}

	return points
}

func distSq(x1, y1, x2, y2 float64) float64 {
	return (x1-x2)*(x1-x2) + (y1-y2)*(y1-y2)
}
