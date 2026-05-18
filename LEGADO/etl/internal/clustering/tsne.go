package clustering

import (
	"log"
	"math"
	"sort"
)

// ProjectTSNE projeta vetores de alta dimensao para 2D usando t-SNE.
// Oferece separacao visual muito superior ao PCA, preservando estrutura local.
func ProjectTSNE(noteVectors map[string][]float32) map[string][2]float64 {
	n := len(noteVectors)
	if n < 4 {
		return ProjectPCA(noteVectors)
	}

	ids := make([]string, 0, n)
	for id := range noteVectors {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	dim := 0
	for _, id := range ids {
		if len(noteVectors[id]) > dim {
			dim = len(noteVectors[id])
		}
	}

	X := make([][]float64, n)
	for i, id := range ids {
		vec := noteVectors[id]
		X[i] = make([]float64, dim)
		for j := 0; j < dim; j++ {
			if j < len(vec) {
				X[i][j] = float64(vec[j])
			}
		}
	}

	D := make([][]float64, n)
	for i := 0; i < n; i++ {
		D[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			dot := 0.0
			for k := 0; k < dim; k++ {
				dot += X[i][k] * X[j][k]
			}
			D[i][j] = math.Max(0, 1.0-dot)
		}
	}

	perplexity := math.Min(30, float64(n-1)/3.0)
	if perplexity < 5 {
		perplexity = 5
	}
	P := computeTSNEAffinities(D, perplexity)

	Y := initTSNE(X, n, dim)

	eta := 500.0
	momentum := 0.8
	iterations := 500
	if n > 100 {
		iterations = 300
	}

	dY := make([][]float64, n)
	iY := make([][]float64, n)
	for i := 0; i < n; i++ {
		dY[i] = make([]float64, 2)
		iY[i] = make([]float64, 2)
	}

	for iter := 0; iter < iterations; iter++ {
		Q := computeLowDimsAffinities(Y)

		for i := 0; i < n; i++ {
			dY[i][0] = 0
			dY[i][1] = 0
		}

		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if i == j {
					continue
				}
				if len(P[i]) <= j || len(Q[i]) <= j {
					continue
				}
				pij := P[i][j]
				qij := Q[i][j]
				if math.IsNaN(pij) || math.IsNaN(qij) || math.IsInf(pij, 0) || math.IsInf(qij, 0) {
					continue
				}
				diff := (pij - qij) * qij * float64(n)
				dY[i][0] += diff * (Y[i][0] - Y[j][0])
				dY[i][1] += diff * (Y[i][1] - Y[j][1])
			}
		}

		for i := 0; i < n; i++ {
			iY[i][0] = momentum*iY[i][0] - eta*dY[i][0]
			iY[i][1] = momentum*iY[i][1] - eta*dY[i][1]
			Y[i][0] += iY[i][0]
			Y[i][1] += iY[i][1]
		}

		cx, cy := 0.0, 0.0
		for i := 0; i < n; i++ {
			cx += Y[i][0]
			cy += Y[i][1]
		}
		cx /= float64(n)
		cy /= float64(n)
		for i := 0; i < n; i++ {
			Y[i][0] -= cx
			Y[i][1] -= cy
		}

		if iter == 250 {
			eta *= 0.5
		}
		if iter == 400 {
			momentum = 0.5
		}
	}

	minX, maxX := Y[0][0], Y[0][0]
	minY, maxY := Y[0][1], Y[0][1]
	for i := 0; i < n; i++ {
		if Y[i][0] < minX {
			minX = Y[i][0]
		}
		if Y[i][0] > maxX {
			maxX = Y[i][0]
		}
		if Y[i][1] < minY {
			minY = Y[i][1]
		}
		if Y[i][1] > maxY {
			maxY = Y[i][1]
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

	result := make(map[string][2]float64)
	for i, id := range ids {
		result[id] = [2]float64{
			(Y[i][0] - minX) / rangeX * 100,
			(Y[i][1] - minY) / rangeY * 100,
		}
	}

	log.Printf("[t-SNE] Projecao concluida: %d notas em 2D\n", n)
	return result
}

func computeTSNEAffinities(D [][]float64, perplexity float64) [][]float64 {
	n := len(D)
	P := make([][]float64, n)
	targetEntropy := math.Log(perplexity)

	for i := 0; i < n; i++ {
		P[i] = make([]float64, n)
		sigmaMin, sigmaMax := 0.001, 1000.0

		for iter := 0; iter < 50; iter++ {
			sigma := (sigmaMin + sigmaMax) / 2.0
			sum := 0.0
			entropy := 0.0

			for j := 0; j < n; j++ {
				if i == j {
					continue
				}
				P[i][j] = math.Exp(-D[i][j] / sigma)
				sum += P[i][j]
			}

			if sum > 0 {
				for j := 0; j < n; j++ {
					if i == j {
						continue
					}
					P[i][j] /= sum
					if P[i][j] > 1e-10 {
						entropy -= P[i][j] * math.Log(P[i][j])
					}
				}
			}

			if math.Abs(entropy-targetEntropy) < 0.01 {
				break
			}
			if entropy > targetEntropy {
				sigmaMax = sigma
			} else {
				sigmaMin = sigma
			}
		}
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			p := (P[i][j] + P[j][i]) / (2.0 * float64(n))
			P[i][j] = p
			P[j][i] = p
		}
	}

	return P
}

func computeLowDimsAffinities(Y [][]float64) [][]float64 {
	n := len(Y)
	if n < 2 {
		return [][]float64{{1.0}}
	}

	// Pre-allocate Q as dense n×n matrix of zeros
	Q := make([][]float64, n)
	for i := 0; i < n; i++ {
		Q[i] = make([]float64, n)
	}

	var sum float64
	for i := 0; i < n; i++ {
		if len(Y[i]) < 2 {
			continue
		}
		for j := i + 1; j < n; j++ {
			if len(Y[j]) < 2 {
				continue
			}
			dx := Y[i][0] - Y[j][0]
			dy := Y[i][1] - Y[j][1]
			q := 1.0 / (1.0 + dx*dx + dy*dy)
			if i < n && j < n {
				Q[i][j] = q
				Q[j][i] = q
			}
			sum += 2 * q
		}
	}

	if sum > 0 {
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if i != j {
					Q[i][j] /= sum
				}
			}
		}
	}

	return Q
}

func initTSNE(X [][]float64, n, dim int) [][]float64 {
	mean := make([]float64, dim)
	for i := 0; i < n; i++ {
		for j := 0; j < dim; j++ {
			mean[j] += X[i][j]
		}
	}
	for j := 0; j < dim; j++ {
		mean[j] /= float64(n)
	}

	Y := make([][]float64, n)
	for i := 0; i < n; i++ {
		Y[i] = make([]float64, 2)
		dot1, dot2 := 0.0, 0.0
		for j := 0; j < dim; j++ {
			x := X[i][j] - mean[j]
			dot1 += x * math.Cos(float64(j)*0.7)
			dot2 += x * math.Sin(float64(j)*0.7)
		}
		Y[i][0] = dot1 * 0.01
		Y[i][1] = dot2 * 0.01
	}

	return Y
}
