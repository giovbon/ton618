package semantic

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
// vectors is a map from identifier to float32 vector.
// Returns a map from identifier to 2D point.
//
// Algorithm:
//  1. Center the data (subtract mean)
//  2. Compute covariance matrix (d × d)
//  3. Power iteration to find top 2 eigenvectors
//  4. Project centered data onto eigenvectors
func Project2DReduce(vectors map[string][]float32) map[string]Point2D {
	n := len(vectors)
	if n == 0 {
		return nil
	}

	// Extract IDs and build matrix
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
			return nil // inconsistent dimensions
		}
	}

	// Edge cases: 1 or 2 vectors
	if n == 1 {
		return map[string]Point2D{ids[0]: {X: 0, Y: 0}}
	}
	if n == 2 {
		// Use distance along the vector connecting the two points
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

	// 1. Compute mean
	mean := make([]float64, d)
	for _, row := range matrix {
		for j, val := range row {
			mean[j] += val
		}
	}
	for j := range mean {
		mean[j] /= float64(n)
	}

	// 2. Center the data
	centered := make([][]float64, n)
	for i, row := range matrix {
		centered[i] = make([]float64, d)
		for j, val := range row {
			centered[i][j] = val - mean[j]
		}
	}

	// 3. Compute covariance matrix: C = centered^T × centered / (n-1)
	// C is d × d. We compute only the upper/lower triangle as needed.
	// But for simplicity and correctness, compute the full matrix.
	cov := make([][]float64, d)
	for j := range d {
		cov[j] = make([]float64, d)
	}

	// C[j][k] = sum_i centered[i][j] * centered[i][k] / (n-1)
	factor := 1.0 / float64(n-1)
	for j := range d {
		for k := j; k < d; k++ {
			var sum float64
			for i := range n {
				sum += centered[i][j] * centered[i][k]
			}
			cov[j][k] = sum * factor
			cov[k][j] = cov[j][k]
		}
	}

	// 4. Power iteration for top 2 eigenvectors
	eig1 := powerIteration(cov, d, 100)
	eig2 := powerIterationDeflated(cov, d, eig1, 100)

	// 5. Project centered data onto eigenvectors
	result := make(map[string]Point2D, n)
	for i, row := range centered {
		var x, y float64
		for j, val := range row {
			x += val * eig1[j]
			y += val * eig2[j]
		}
		result[ids[i]] = Point2D{X: x, Y: y}
	}

	// 6. Normalize positions to a reasonable range (-1 to 1 roughly)
	normalizePoints(result)

	return result
}

// powerIteration finds the dominant eigenvector of a square matrix.
func powerIteration(matrix [][]float64, d int, maxIter int) []float64 {
	rng := rand.New(rand.NewSource(42))
	v := make([]float64, d)
	for i := range v {
		v[i] = rng.Float64()*2 - 1
	}
	normalize(v)

	for iter := range maxIter {
		// v_new = matrix × v
		vNew := make([]float64, d)
		for j := range d {
			var sum float64
			for k := range d {
				sum += matrix[j][k] * v[k]
			}
			vNew[j] = sum
		}
		normalize(vNew)

		// Check convergence (cosine similarity)
		var dot float64
		for j := range d {
			dot += v[j] * vNew[j]
		}
		if dot < 0 {
			// Flip sign for consistency
			for j := range d {
				vNew[j] = -vNew[j]
			}
			dot = -dot
		}
		// If cosine > 0.99999, converged
		if dot > 0.99999 && iter > 5 {
			break
		}
		v = vNew
	}
	return v
}

// powerIterationDeflated finds the second eigenvector by deflating the first.
func powerIterationDeflated(matrix [][]float64, d int, eig1 []float64, maxIter int) []float64 {
	// Compute eigenvalue λ = eig1^T × matrix × eig1
	aux := make([]float64, d)
	for j := range d {
		var sum float64
		for k := range d {
			sum += matrix[j][k] * eig1[k]
		}
		aux[j] = sum
	}
	var lambda float64
	for j := range d {
		lambda += eig1[j] * aux[j]
	}

	// Deflated matrix: matrix' = matrix - λ * eig1 × eig1^T
	// We don't explicitly construct this; we compute matrix × v - λ * eig1 * (eig1^T × v)

	rng := rand.New(rand.NewSource(123))
	v := make([]float64, d)
	for i := range v {
		v[i] = rng.Float64()*2 - 1
	}
	// Remove projection onto eig1
	var dot float64
	for j := range d {
		dot += eig1[j] * v[j]
	}
	for j := range d {
		v[j] -= dot * eig1[j]
	}
	normalize(v)

	for iter := range maxIter {
		// v_new = matrix × v
		vNew := make([]float64, d)
		for j := range d {
			var sum float64
			for k := range d {
				sum += matrix[j][k] * v[k]
			}
			vNew[j] = sum
		}

		// Deflate: vNew -= λ * eig1 * (eig1^T × v)
		var proj float64
		for j := range d {
			proj += eig1[j] * v[j]
		}
		proj *= lambda
		for j := range d {
			vNew[j] -= proj * eig1[j]
		}

		// Re-orthogonalize against eig1 (numerical stability)
		dot = 0
		for j := range d {
			dot += eig1[j] * vNew[j]
		}
		for j := range d {
			vNew[j] -= dot * eig1[j]
		}

		normalize(vNew)

		// Check convergence
		dot = 0
		for j := range d {
			dot += v[j] * vNew[j]
		}
		if dot < 0 {
			for j := range d {
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

// normalize normalizes a vector to unit L2 norm.
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

// normalizePoints scales and centers 2D points to fit in [-1, 1] range.
func normalizePoints(pts map[string]Point2D) {
	if len(pts) == 0 {
		return
	}

	// Find min and max for each axis
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

// ── Helper for Map[K]V → values only ──

// Project2D takes all embeddings as map[docID]NoteVector and returns 2D projections.
// This is a convenience wrapper around Project2DReduce.
func Project2D(embeddings map[string][]float32) map[string]Point2D {
	return Project2DReduce(embeddings)
}

// ── Project for map[string]NoteVector ──

// NoteVector is defined in db package, but we import our own minimal copy here
// to avoid circular imports.

// ProjectEmbeddings takes a map of docID → []float32 and returns 2D coordinates.
func ProjectEmbeddings(vecs map[string][]float32) (map[string]Point2D, []string) {
	result := Project2DReduce(vecs)
	if result == nil {
		return nil, nil
	}

	// Sort IDs for deterministic ordering
	ids := make([]string, 0, len(result))
	for id := range result {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return result, ids
}
