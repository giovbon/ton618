package index

import (
	"math"
	"math/rand"
	"sort"
)

// Point2D represents a 2D coordinate in the projection.
type Point2D struct {
	X, Y float64
}

// Project2DReduce performs PCA dimensionality reduction from d dimensions to 2D.
func Project2DReduce(vectors map[string][]float32) map[string]Point2D {
	n := len(vectors)
	if n == 0 {
		return nil
	}

	ids := make([]string, 0, n)
	matrix := make([][]float64, n)
	i := 0
	for id, vec := range vectors {
		ids = append(ids, id)
		row := make([]float64, len(vec))
		for j, v := range vec {
			row[j] = float64(v)
		}
		matrix[i] = row
		i++
	}

	d := len(matrix[0])
	for _, row := range matrix {
		if len(row) != d {
			return nil
		}
	}

	if n == 1 {
		return map[string]Point2D{ids[0]: {X: 0, Y: 0}}
	}
	if n == 2 {
		dx := matrix[0][0] - matrix[1][0]
		dy := matrix[0][1] - matrix[1][1]
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist == 0 {
			dist = 1
		}
		return map[string]Point2D{
			ids[0]: {X: -dist / 2, Y: 0},
			ids[1]: {X: dist / 2, Y: 0},
		}
	}

	mean := make([]float64, d)
	for _, row := range matrix {
		for j, val := range row {
			mean[j] += val
		}
	}
	for j := range mean {
		mean[j] /= float64(n)
	}

	centered := make([][]float64, n)
	for i, row := range matrix {
		centered[i] = make([]float64, d)
		for j, val := range row {
			centered[i][j] = val - mean[j]
		}
	}

	cov := make([][]float64, d)
	for j := 0; j < d; j++ {
		cov[j] = make([]float64, d)
	}
	factor := 1.0 / float64(n-1)
	for j := 0; j < d; j++ {
		for k := j; k < d; k++ {
			var sum float64
			for i := 0; i < n; i++ {
				sum += centered[i][j] * centered[i][k]
			}
			cov[j][k] = sum * factor
			cov[k][j] = cov[j][k]
		}
	}

	eig1 := powerIteration(cov, d, 100)
	eig2 := powerIterationDeflated(cov, d, eig1, 100)

	result := make(map[string]Point2D, n)
	for i, row := range centered {
		var x, y float64
		for j, val := range row {
			x += val * eig1[j]
			y += val * eig2[j]
		}
		result[ids[i]] = Point2D{X: x, Y: y}
	}

	normalizePoints(result)

	return result
}

func powerIteration(matrix [][]float64, d int, maxIter int) []float64 {
	rng := rand.New(rand.NewSource(42))
	v := make([]float64, d)
	for i := range v {
		v[i] = rng.Float64()*2 - 1
	}
	normalize(v)
	for iter := 0; iter < maxIter; iter++ {
		vNew := make([]float64, d)
		for j := 0; j < d; j++ {
			var sum float64
			for k := 0; k < d; k++ {
				sum += matrix[j][k] * v[k]
			}
			vNew[j] = sum
		}
		normalize(vNew)
		var dot float64
		for j := 0; j < d; j++ {
			dot += v[j] * vNew[j]
		}
		if dot < 0 {
			for j := 0; j < d; j++ {
				vNew[j] = -vNew[j]
			}
			dot = -dot
		}
		if dot > 0.99999 && iter > 5 {
			break
		}
		v = vNew
	}
	return v
}

func powerIterationDeflated(matrix [][]float64, d int, eig1 []float64, maxIter int) []float64 {
	aux := make([]float64, d)
	for j := 0; j < d; j++ {
		var sum float64
		for k := 0; k < d; k++ {
			sum += matrix[j][k] * eig1[k]
		}
		aux[j] = sum
	}
	var lambda float64
	for j := 0; j < d; j++ {
		lambda += eig1[j] * aux[j]
	}
	rng := rand.New(rand.NewSource(123))
	v := make([]float64, d)
	for i := range v {
		v[i] = rng.Float64()*2 - 1
	}
	var dot float64
	for j := 0; j < d; j++ {
		dot += eig1[j] * v[j]
	}
	for j := 0; j < d; j++ {
		v[j] -= dot * eig1[j]
	}
	normalize(v)
	for iter := 0; iter < maxIter; iter++ {
		vNew := make([]float64, d)
		for j := 0; j < d; j++ {
			var sum float64
			for k := 0; k < d; k++ {
				sum += matrix[j][k] * v[k]
			}
			vNew[j] = sum
		}
		var proj float64
		for j := 0; j < d; j++ {
			proj += eig1[j] * v[j]
		}
		proj *= lambda
		for j := 0; j < d; j++ {
			vNew[j] -= proj * eig1[j]
		}
		dot = 0
		for j := 0; j < d; j++ {
			dot += eig1[j] * vNew[j]
		}
		for j := 0; j < d; j++ {
			vNew[j] -= dot * eig1[j]
		}
		normalize(vNew)
		dot = 0
		for j := 0; j < d; j++ {
			dot += v[j] * vNew[j]
		}
		if dot < 0 {
			for j := 0; j < d; j++ {
			}
			dot = -dot
		}
		if dot > 0.99999 && iter > 5 {
			break
		}
		v = vNew
	}
	return v
}

func normalize(v []float64) {
	var norm float64
	for _, x := range v {
		norm += x * x
	}
	norm = math.Sqrt(norm)
	if norm > 1e-15 {
		for i := range v {
			v[i] /= norm
		}
	}
}

func normalizePoints(pts map[string]Point2D) {
	if len(pts) == 0 {
		return
	}
	minX, maxX := math.MaxFloat64, -math.MaxFloat64
	minY, maxY := math.MaxFloat64, -math.MaxFloat64
	for _, p := range pts {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	rangeX := maxX - minX
	rangeY := maxY - minY
	if rangeX < 1e-10 {
		rangeX = 1
	}
	if rangeY < 1e-10 {
		rangeY = 1
	}
	midX := (minX + maxX) / 2
	midY := (minY + maxY) / 2
	for id, p := range pts {
		pts[id] = Point2D{
			X: (p.X - midX) / (rangeX / 2),
			Y: (p.Y - midY) / (rangeY / 2),
		}
	}
}

// ── K-Means clustering com silhouette score ──

// ClusterResult holds the clustering output for a single point.
type ClusterResult struct {
	X         float64
	Y         float64
	ClusterID int
}

// ClusterPoints performs k-means clustering on 2D points and finds the optimal k
// via silhouette score. Returns the cluster assignment for each input point
// (in the same order as input ids).
func ClusterPoints(pts map[string]Point2D) (map[string]int, int) {
	n := len(pts)
	if n == 0 {
		return nil, 0
	}
	if n <= 2 {
		result := make(map[string]int)
		i := 0
		for id := range pts {
			result[id] = i
			i++
		}
		return result, n
	}

	// Extract ordered points with deterministic key sorting.
	ids := make([]string, 0, n)
	points := make([]ClusterResult, 0, n)
	for id := range pts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := pts[id]
		points = append(points, ClusterResult{X: p.X, Y: p.Y})
	}

	// Determine maxK
	maxK := int(3.0 + 0.5*float64(n)/5.0)
	if maxK > 10 {
		maxK = 10
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
		clone := make([]ClusterResult, n)
		copy(clone, points)
		kmeans(clone, k, 20)
		score := silhouetteScore(clone)
		if score > bestScore {
			bestScore = score
			bestK = k
		}
	}

	// Run final clustering with bestK
	kmeans(points, bestK, 30)

	result := make(map[string]int, n)
	for i, id := range ids {
		result[id] = points[i].ClusterID
	}
	return result, bestK
}

func distSq(x1, y1, x2, y2 float64) float64 {
	return (x1-x2)*(x1-x2) + (y1-y2)*(y1-y2)
}

func kmeans(points []ClusterResult, k int, iterations int) {
	n := len(points)
	if n <= k {
		for i := range points {
			points[i].ClusterID = i
		}
		return
	}

	rng := rand.New(rand.NewSource(42))
	centroids := make([][2]float64, k)

	// k-means++ initialization
	centroids[0] = [2]float64{points[rng.Intn(n)].X, points[rng.Intn(n)].Y}
	for c := 1; c < k; c++ {
		dists := make([]float64, n)
		var totalDist float64
		for i, p := range points {
			minD := math.MaxFloat64
			for j := 0; j < c; j++ {
				d := distSq(p.X, p.Y, centroids[j][0], centroids[j][1])
				if d < minD {
					minD = d
				}
			}
			dists[i] = minD
			totalDist += minD
		}
		threshold := rng.Float64() * totalDist
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
			minD := math.MaxFloat64
			best := 0
			for c := 0; c < k; c++ {
				d := distSq(p.X, p.Y, centroids[c][0], centroids[c][1])
				if d < minD {
					minD = d
					best = c
				}
			}
			if points[i].ClusterID != best {
				points[i].ClusterID = best
				changed = true
			}
		}
		if !changed {
			break
		}
		sums := make([][2]float64, k)
		counts := make([]int, k)
		for _, p := range points {
			sums[p.ClusterID][0] += p.X
			sums[p.ClusterID][1] += p.Y
			counts[p.ClusterID]++
		}
		for c := 0; c < k; c++ {
			if counts[c] > 0 {
				centroids[c][0] = sums[c][0] / float64(counts[c])
				centroids[c][1] = sums[c][1] / float64(counts[c])
			}
		}
	}
}

func silhouetteScore(points []ClusterResult) float64 {
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
		members := clusters[p.ClusterID]
		if len(members) > 1 {
			for _, j := range members {
				if i != j {
					a += math.Sqrt(distSq(p.X, p.Y, points[j].X, points[j].Y))
				}
			}
			a /= float64(len(members) - 1)
		}

		b := math.MaxFloat64
		for cid, members := range clusters {
			if cid == p.ClusterID {
				continue
			}
			var dist float64
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

// ── Wrappers ──

func Project2D(embeddings map[string][]float32) map[string]Point2D {
	return Project2DReduce(embeddings)
}

func ProjectEmbeddings(vecs map[string][]float32) (map[string]Point2D, []string) {
	result := Project2DReduce(vecs)
	if result == nil {
		return nil, nil
	}
	ids := make([]string, 0, len(result))
	for id := range result {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return result, ids
}
