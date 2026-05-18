package clustering

import (
	"log"
	"math"
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

	rows := len(noteVectors)
	ids := make([]string, 0, rows)
	for id := range noteVectors {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	cols := 0
	for _, id := range ids {
		if len(noteVectors[id]) > cols {
			cols = len(noteVectors[id])
		}
	}
	data := make([]float64, rows*cols)

	for i, id := range ids {
		vec := noteVectors[id]
		if len(vec) != cols {
			log.Printf("[PCA] Aviso: Vetor %s tem dimensao %d (esperado %d).\n", id, len(vec), cols)
		}
		for j := 0; j < cols; j++ {
			if j < len(vec) {
				data[i*cols+j] = float64(vec[j])
			} else {
				data[i*cols+j] = 0
			}
		}
	}

	matrix := mat.NewDense(rows, cols, data)

	var svd mat.SVD
	ok := svd.Factorize(matrix, mat.SVDThin)
	if !ok {
		return make(map[string][2]float64)
	}

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
		result[id] = [2]float64{
			(coords[i][0] - minX) / rangeX * 100,
			(coords[i][1] - minY) / rangeY * 100,
		}
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

	localRand := rand.New(rand.NewSource(42))
	centroids := make([][2]float64, k)

	first := points[localRand.Intn(len(points))]
	centroids[0] = [2]float64{first.X, first.Y}

	for c := 1; c < k; c++ {
		var totalDist float64
		dists := make([]float64, len(points))
		for i, p := range points {
			minDist := -1.0
			for j := 0; j < c; j++ {
				d := distSq(p.X, p.Y, centroids[j][0], centroids[j][1])
				if minDist == -1 || d < minDist {
					minDist = d
				}
			}
			dists[i] = minDist
			totalDist += minDist
		}

		threshold := localRand.Float64() * totalDist
		var cumulative float64
		for i, d := range dists {
			cumulative += d
			if cumulative >= threshold {
				centroids[c] = [2]float64{points[i].X, points[i].Y}
				break
			}
		}
	}

	for iter := 0; iter < iterations; iter++ {
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

// BestK encontra o K otimo usando silhouette score.
// Testa de 2 ate maxK e retorna o K com melhor coesao/separacao.
func BestK(points []Point, maxK int) int {
	n := len(points)
	if n <= 2 {
		return n
	}
	if maxK > n {
		maxK = n
	}
	if maxK < 2 {
		maxK = 2
	}

	bestK := 2
	bestScore := -1.0

	for k := 2; k <= maxK; k++ {
		clone := make([]Point, n)
		copy(clone, points)
		KMeans(clone, k, 15)

		score := silhouetteScore(clone)
		if score > bestScore {
			bestScore = score
			bestK = k
		}
	}

	return bestK
}

func silhouetteScore(points []Point) float64 {
	n := len(points)
	if n <= 1 {
		return 0
	}

	clusters := make(map[int][]int)
	for i, p := range points {
		clusters[p.ClusterID] = append(clusters[p.ClusterID], i)
	}

	if len(clusters) <= 1 {
		return 0
	}

	totalScore := 0.0
	for i, p := range points {
		a := 0.0
		cluster := clusters[p.ClusterID]
		if len(cluster) > 1 {
			for _, j := range cluster {
				if i != j {
					a += math.Sqrt(distSq(p.X, p.Y, points[j].X, points[j].Y))
				}
			}
			a /= float64(len(cluster) - 1)
		}

		b := math.MaxFloat64
		for cid, members := range clusters {
			if cid == p.ClusterID {
				continue
			}
			dist := 0.0
			for _, j := range members {
				dist += math.Sqrt(distSq(p.X, p.Y, points[j].X, points[j].Y))
			}
			dist /= float64(len(members))
			if dist < b {
				b = dist
			}
		}

		if b == math.MaxFloat64 {
			b = 0
		}

		maxAB := a
		if b > maxAB {
			maxAB = b
		}
		if maxAB > 0 {
			totalScore += (b - a) / maxAB
		}
	}

	return totalScore / float64(n)
}

func distSq(x1, y1, x2, y2 float64) float64 {
	return (x1-x2)*(x1-x2) + (y1-y2)*(y1-y2)
}
