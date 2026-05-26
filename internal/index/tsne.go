package index

import (
	"math"
	"math/rand"
	"sort"
)

// TSNE implements t-SNE dimensionality reduction with adaptive perplexity.
// Handles 2-500 points efficiently in pure Go.
type TSNE struct {
	Perplexity float64 // 5-50, default 30
	MaxIter    int     // default 750
	Eta        float64 // learning rate, default 200
	Exaggerate int     // early exaggeration epochs, default 100
	Seed       int64   // random seed, default 42
}

// DefaultTSNE returns sensible defaults for typical note collections.
func DefaultTSNE() TSNE {
	return TSNE{
		Perplexity: 30,
		MaxIter:    750,
		Eta:        200,
		Exaggerate: 100,
		Seed:       42,
	}
}

// Project runs t-SNE on high-dimensional vectors, returning 2D coordinates.
func (tsne TSNE) Project(vectors map[string][]float32) map[string]Point2D {
	n := len(vectors)
	if n <= 1 {
		r := make(map[string]Point2D)
		for k := range vectors {
			r[k] = Point2D{X: 0, Y: 0}
		}
		return r
	}

	keys := make([]string, 0, n)
	for k := range vectors {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	data := make([][]float64, n)
	for i, k := range keys {
		v := vectors[k]
		row := make([]float64, len(v))
		for j, x := range v {
			row[j] = float64(x)
		}
		data[i] = row
		i++
	}

	rng := rand.New(rand.NewSource(tsne.Seed))

	// 1. High-dimensional affinities P
	P := computeAffinities(data, tsne.Perplexity)

	// 2. Initialize Y randomly near origin
	Y := make([]float64, 2*n)
	for i := 0; i < 2*n; i++ {
		Y[i] = rng.Float64()*1e-4 - 5e-5
	}

	// 3. Gradient descent
	dY := make([]float64, 2*n)
	gains := make([]float64, 2*n)
	sign := make([]bool, 2*n)
	for i := range gains {
		gains[i] = 1.0
	}

	for iter := 0; iter < tsne.MaxIter; iter++ {
		// Early exaggeration
		alpha := 4.0
		if iter >= tsne.Exaggerate {
			alpha = 1.0
		}

		// Compute low-dimensional Q
		var Z float64
		Q := make([]float64, n*n)
		for a := 0; a < n; a++ {
			for b := a + 1; b < n; b++ {
				dy := Y[2*a] - Y[2*b]
				dx := Y[2*a+1] - Y[2*b+1]
				q := 1.0 / (1.0 + dy*dy + dx*dx)
				Q[a*n+b] = q
				Q[b*n+a] = q
				Z += 2 * q
			}
		}

		// Gradient
		grad := make([]float64, 2*n)
		for a := 0; a < n; a++ {
			for b := 0; b < n; b++ {
				if a == b {
					continue
				}
				dy := Y[2*a] - Y[2*b]
				dx := Y[2*a+1] - Y[2*b+1]
				distQ := 1.0 + dy*dy + dx*dx

				// attractive: +4 * alpha * P * (yi - yj) / distQ
				// repulsive: -4 * (Q/Z) * (yi - yj) / distQ
				att := alpha * P[a*n+b]
				rep := Q[a*n+b] / Z

				force := 4.0 * (att - rep) / distQ
				grad[2*a] += force * dy
				grad[2*a+1] += force * dx
			}
		}

		// Momentum schedule
		mom := 0.5
		if iter >= 250 {
			mom = 0.8
		}

		// Update
		for i := 0; i < 2*n; i++ {
			curSign := grad[i] >= 0
			if iter > 0 && sign[i] == curSign {
				gains[i] *= 0.8
				if gains[i] < 0.01 {
					gains[i] = 0.01
				}
			} else {
				gains[i] += 0.2
			}
			sign[i] = curSign

			dY[i] = mom*dY[i] - tsne.Eta*gains[i]*grad[i]
			Y[i] += dY[i]
		}

		// Zero-center
		var mx, my float64
		for i := 0; i < n; i++ {
			mx += Y[2*i]
			my += Y[2*i+1]
		}
		mx /= float64(n)
		my /= float64(n)
		for i := 0; i < n; i++ {
			Y[2*i] -= mx
			Y[2*i+1] -= my
		}
	}

	result := make(map[string]Point2D, n)
	for i := 0; i < n; i++ {
		result[keys[i]] = Point2D{X: Y[2*i], Y: Y[2*i+1]}
	}
	scalePoints2D(result, 500)
	return result
}

// ── Affinities ──

func computeAffinities(data [][]float64, perp float64) []float64 {
	n := len(data)

	if perp > float64(n-1)/3.0 {
		perp = float64(n-1) / 3.0
	}
	if perp < 1 {
		perp = 1
	}

	// Pairwise squared distances
	distSq := make([]float64, n*n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			d := euclideanDistSq(data[i], data[j])
			distSq[i*n+j] = d
			distSq[j*n+i] = d
		}
	}

	P := make([]float64, n*n)
	targetH := math.Log(perp)

	for i := 0; i < n; i++ {
		sigma := 1.0
		lo, hi := 1e-20, 1e20
		for k := 0; k < 60; k++ {
			sigma = (lo + hi) / 2

			var sum float64
			for j := 0; j < n; j++ {
				if j == i {
					continue
				}
				p := math.Exp(-distSq[i*n+j] / (2 * sigma * sigma))
				sum += p
			}
			if sum < 1e-300 {
				lo = sigma
				continue
			}

			entropy := 0.0
			for j := 0; j < n; j++ {
				if j == i {
					continue
				}
				p := math.Exp(-distSq[i*n+j]/(2*sigma*sigma)) / sum
				if p > 1e-300 {
					entropy -= p * math.Log(p)
				}
			}

			if math.Abs(entropy-targetH) < 1e-5 {
				break
			}
			if entropy > targetH {
				hi = sigma
			} else {
				lo = sigma
			}
		}

		// Store P(j|i)
		var sum float64
		for j := 0; j < n; j++ {
			if j == i {
				continue
			}
			P[i*n+j] = math.Exp(-distSq[i*n+j] / (2 * sigma * sigma))
			sum += P[i*n+j]
		}
		for j := 0; j < n; j++ {
			if j != i {
				P[i*n+j] /= sum
			}
		}
	}

	// Symmetrize
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			sym := (P[i*n+j] + P[j*n+i]) / (2 * float64(n))
			P[i*n+j] = sym
			P[j*n+i] = sym
		}
	}

	return P
}

// ── Helpers ──

func euclideanDistSq(a, b []float64) float64 {
	var sum float64
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return sum
}

func scalePoints2D(pts map[string]Point2D, target float64) {
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
	for k, p := range pts {
		pts[k] = Point2D{
			X: (p.X - midX) / (rangeX / (2 * target)),
			Y: (p.Y - midY) / (rangeY / (2 * target)),
		}
	}
}
