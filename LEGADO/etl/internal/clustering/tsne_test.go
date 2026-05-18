package clustering

import (
	"math"
	"testing"
)

func TestProjectTSNE_FallbackPCA(t *testing.T) {
	// 1-3 notas devem cair para PCA
	for n := 1; n <= 3; n++ {
		vecs := make(map[string][]float32)
		for i := 0; i < n; i++ {
			vecs[string(rune('a'+i))+".md"] = []float32{float32(i) * 0.1, 0.5, 0.3}
		}
		result := ProjectTSNE(vecs)
		if len(result) != n {
			t.Errorf("n=%d: esperava %d resultados, obteve %d", n, n, len(result))
		}
	}
}

func TestProjectTSNE_Basic(t *testing.T) {
	// 4 notas com vetores distintos - deve usar t-SNE
	vecs := map[string][]float32{
		"a.md": {1.0, 0.0, 0.0},
		"b.md": {0.0, 1.0, 0.0},
		"c.md": {0.0, 0.0, 1.0},
		"d.md": {0.5, 0.5, 0.0},
	}

	result := ProjectTSNE(vecs)
	if len(result) != 4 {
		t.Fatalf("esperava 4 resultados, obteve %d", len(result))
	}

	// Verificar que todos os IDs estao presentes
	for id := range vecs {
		if _, ok := result[id]; !ok {
			t.Errorf("ID %s ausente do resultado", id)
		}
	}

	// Verificar que coordenadas sao finitas e no range 0-100
	for id, coords := range result {
		if math.IsNaN(coords[0]) || math.IsNaN(coords[1]) {
			t.Errorf("%s: coordenada NaN", id)
		}
		if math.IsInf(coords[0], 0) || math.IsInf(coords[1], 0) {
			t.Errorf("%s: coordenada Inf", id)
		}
		if coords[0] < 0 || coords[0] > 100 {
			t.Errorf("%s: X=%f fora do range 0-100", id, coords[0])
		}
		if coords[1] < 0 || coords[1] > 100 {
			t.Errorf("%s: Y=%f fora do range 0-100", id, coords[1])
		}
	}
}

func TestProjectTSNE_Determinism(t *testing.T) {
	// Mesmo input deve produzir mesma saida (deterministico)
	vecs := map[string][]float32{
		"x.md": {0.9, 0.1, 0.0, 0.0},
		"y.md": {0.1, 0.9, 0.0, 0.0},
		"z.md": {0.0, 0.1, 0.9, 0.0},
		"w.md": {0.0, 0.0, 0.1, 0.9},
	}

	r1 := ProjectTSNE(vecs)
	r2 := ProjectTSNE(vecs)

	for id := range vecs {
		if r1[id][0] != r2[id][0] || r1[id][1] != r2[id][1] {
			t.Errorf("%s: resultados diferentes entre chamadas: (%f,%f) vs (%f,%f)",
				id, r1[id][0], r1[id][1], r2[id][0], r2[id][1])
		}
	}
}

func TestBestK(t *testing.T) {
	// 3 clusters bem separados
	points := []Point{
		{ID: "a1", X: 10, Y: 10},
		{ID: "a2", X: 12, Y: 11},
		{ID: "b1", X: 50, Y: 10},
		{ID: "b2", X: 52, Y: 11},
		{ID: "c1", X: 30, Y: 60},
		{ID: "c2", X: 32, Y: 61},
	}

	k := BestK(points, 5)
	if k < 2 || k > 5 {
		t.Errorf("K=%d fora do range esperado [2,5]", k)
	}

	// Para pontos bem separados, K deve ser >= 2
	if k < 2 {
		t.Error("BestK deveria encontrar pelo menos 2 clusters")
	}
}

func TestBestK_SingleCluster(t *testing.T) {
	// Todos os pontos no mesmo lugar
	points := []Point{
		{ID: "a", X: 10, Y: 10},
		{ID: "b", X: 10, Y: 10},
	}
	k := BestK(points, 3)
	if k != 2 {
		t.Errorf("para 2 pontos identicos, esperava 2 clusters, obteve %d", k)
	}
}

func TestSilhouetteScore_Perfect(t *testing.T) {
	// 2 clusters perfeitamente separados
	points := []Point{
		{ID: "a1", X: 0, Y: 0, ClusterID: 0},
		{ID: "a2", X: 1, Y: 0, ClusterID: 0},
		{ID: "b1", X: 100, Y: 0, ClusterID: 1},
		{ID: "b2", X: 101, Y: 0, ClusterID: 1},
	}

	score := silhouetteScore(points)
	if score < 0.9 {
		t.Errorf("silhouette score para clusters perfeitos deveria ser ~1.0, obteve %f", score)
	}
}

func TestSilhouetteScore_Poor(t *testing.T) {
	// Pontos misturados
	points := []Point{
		{ID: "a", X: 5, Y: 5, ClusterID: 0},
		{ID: "b", X: 6, Y: 5, ClusterID: 1},
		{ID: "c", X: 5, Y: 6, ClusterID: 0},
		{ID: "d", X: 6, Y: 6, ClusterID: 1},
	}

	score := silhouetteScore(points)
	if score > 0.3 {
		t.Errorf("silhouette score para clusters misturados deveria ser baixo, obteve %f", score)
	}
}
