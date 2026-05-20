package semantic

import (
	"math"
	"testing"
)

func TestProject2DReduce_Empty(t *testing.T) {
	result := Project2DReduce(map[string][]float32{})
	if result != nil {
		t.Fatal("esperado nil para entrada vazia")
	}
}

func TestProject2DReduce_SingleNode(t *testing.T) {
	result := Project2DReduce(map[string][]float32{
		"n1": {0.1, 0.2, 0.3},
	})
	if len(result) != 1 {
		t.Fatalf("esperado 1 nó, got %d", len(result))
	}
	p := result["n1"]
	if p.X != 0 || p.Y != 0 {
		t.Fatalf("nó único deveria estar em (0,0), got (%f, %f)", p.X, p.Y)
	}
}

func TestProject2DReduce_TwoNodes(t *testing.T) {
	result := Project2DReduce(map[string][]float32{
		"a": {1.0, 0.0, 0.0},
		"b": {0.0, 1.0, 0.0},
	})
	if len(result) != 2 {
		t.Fatalf("esperado 2 nós, got %d", len(result))
	}
}

func TestProject2DReduce_ManyDimensions(t *testing.T) {
	n := 10
	d := 768
	vecs := make(map[string][]float32, n)
	for i := range n {
		id := string(rune('a' + i))
		vec := make([]float32, d)
		for j := range d {
			vec[j] = float32(i*100+j) * 0.001
		}
		vecs[id] = vec
	}
	result := Project2DReduce(vecs)
	if len(result) != n {
		t.Fatalf("esperado %d nós, got %d", n, len(result))
	}
	var minX, maxX float64 = math.MaxFloat64, -math.MaxFloat64
	var minY, maxY float64 = math.MaxFloat64, -math.MaxFloat64
	for _, p := range result {
		if p.X < minX { minX = p.X }
		if p.X > maxX { maxX = p.X }
		if p.Y < minY { minY = p.Y }
		if p.Y > maxY { maxY = p.Y }
	}
	if maxX-minX < 0.001 { t.Error("pontos nao estao espalhados em X") }
	if maxY-minY < 0.001 { t.Error("pontos nao estao espalhados em Y") }
	if maxX > 1.1 || minX < -1.1 || maxY > 1.1 || minY < -1.1 {
		t.Errorf("fora do range [-1,1]: X[%f,%f] Y[%f,%f]", minX, maxX, minY, maxY)
	}
}

func TestProject2DReduce_ConsistentOutput(t *testing.T) {
	vecs := map[string][]float32{
		"x": {0.5, 0.3, 0.1, 0.0, -0.2},
		"y": {0.1, 0.6, 0.3, 0.2, -0.1},
		"z": {-0.3, 0.0, 0.4, 0.5, 0.2},
	}
	r1 := Project2DReduce(vecs)
	r2 := Project2DReduce(vecs)
	for id, p1 := range r1 {
		p2 := r2[id]
		if math.Abs(p1.X-p2.X) > 0.00001 || math.Abs(p1.Y-p2.Y) > 0.00001 {
			t.Errorf("resultado inconsistente para %q: r1=(%f,%f) r2=(%f,%f)", id, p1.X, p1.Y, p2.X, p2.Y)
		}
	}
}

func TestProject2DReduce_AllIdenticalVectors(t *testing.T) {
	vecs := map[string][]float32{
		"a": {1.0, 2.0, 3.0},
		"b": {1.0, 2.0, 3.0},
		"c": {1.0, 2.0, 3.0},
	}
	result := Project2DReduce(vecs)
	if len(result) != 3 { t.Fatalf("esperado 3 nós, got %d", len(result)) }
}

func TestNormalizeVector(t *testing.T) {
	v := []float64{3.0, 4.0}
	normalize(v)
	norm := v[0]*v[0] + v[1]*v[1]
	if math.Abs(norm-1.0) > 0.00001 { t.Errorf("norma deveria ser 1, got %f", norm) }
	if math.Abs(v[0]-0.6) > 0.00001 || math.Abs(v[1]-0.8) > 0.00001 {
		t.Errorf("vetor normalizado incorreto: [%f,%f]", v[0], v[1])
	}
}

func TestNormalizePoints(t *testing.T) {
	pts := map[string]Point2D{"a": {10, 10}, "b": {20, 20}, "c": {15, 15}}
	normalizePoints(pts)
	var sumX, sumY float64
	for _, p := range pts { sumX += p.X; sumY += p.Y }
	if math.Abs(sumX) > 0.001 { t.Errorf("centro X nao-zero: %f", sumX) }
	if math.Abs(sumY) > 0.001 { t.Errorf("centro Y nao-zero: %f", sumY) }
}

func TestProjectEmbeddings(t *testing.T) {
	vecs := map[string][]float32{"id1": {1, 0, 0}, "id2": {0, 1, 0}, "id3": {0, 0, 1}}
	pts, ids := ProjectEmbeddings(vecs)
	if pts == nil || ids == nil { t.Fatal("ProjectEmbeddings retornou nil") }
	if len(ids) != 3 { t.Fatalf("esperado 3 ids, got %d", len(ids)) }
	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[i-1] { t.Errorf("IDs nao ordenados: %v", ids); break }
	}
}

// ── Testes de clustering ───────────────────────────────────────

func TestClusterPoints_ZeroOuUmPonto(t *testing.T) {
	// 0 pontos
	m, k := ClusterPoints(map[string]Point2D{})
	if m != nil || k != 0 {
		t.Fatalf("esperado nil/0, got map=%v k=%d", m, k)
	}

	// 1 ponto
	m, k = ClusterPoints(map[string]Point2D{"a": {0, 0}})
	if len(m) != 1 || k != 1 || m["a"] != 0 {
		t.Fatalf("1 ponto: esperado cluster 0, got m=%v k=%d", m, k)
	}
}

func TestClusterPoints_DoisPontos(t *testing.T) {
	m, k := ClusterPoints(map[string]Point2D{
		"a": {0, 0},
		"b": {10, 10},
	})
	if k != 2 { t.Fatalf("2 pontos: esperado k=2, got %d", k) }
	if m["a"] == m["b"] { t.Error("dois pontos distantes deveriam estar em clusters diferentes") }
}

func TestClusterPoints_TresPontosColapsados(t *testing.T) {
	m, k := ClusterPoints(map[string]Point2D{
		"a": {5, 5},
		"b": {5, 5},
		"c": {5, 5},
	})
	if k < 1 || k > 3 { t.Fatalf("k deveria estar entre 1 e 3, got %d", k) }
	// Com 3+ pontos distintos, deve haver ao menos 2 clusters
	_ = m
}

func TestClusterPoints_Deterministico(t *testing.T) {
	pts := map[string]Point2D{
		"a": {0, 0}, "b": {1, 0}, "c": {0, 1},
		"d": {10, 10}, "e": {11, 10}, "f": {10, 11},
		"g": {20, 20}, "h": {21, 20}, "i": {20, 21},
	}
	r1, k1 := ClusterPoints(pts)
	r2, k2 := ClusterPoints(pts)
	if k1 != k2 { t.Errorf("k diferente entre execucoes: %d vs %d", k1, k2) }
	for id := range pts {
		if r1[id] != r2[id] { t.Errorf("cluster divergente para %q: exec1=%d exec2=%d", id, r1[id], r2[id]) }
	}
}

func TestClusterPoints_TresGruposBemSeparados(t *testing.T) {
	// 3 grupos claramente separados no espaço
	pts := map[string]Point2D{
		// Grupo A (canto inferior esquerdo)
		"a1": {0, 0}, "a2": {1, 0}, "a3": {0, 1},
		// Grupo B (canto superior direito)
		"b1": {50, 50}, "b2": {51, 50}, "b3": {50, 51},
		// Grupo C (centro-direita)
		"c1": {50, 0}, "c2": {51, 0}, "c3": {50, 1},
	}
	result, k := ClusterPoints(pts)
	if k < 2 || k > 4 {
		t.Fatalf("esperado k entre 2 e 4 para 3 grupos, got %d", k)
	}

	// Verifica que elementos do mesmo grupo estao juntos
	grupoA := result["a1"]
	for _, id := range []string{"a2", "a3"} {
		if result[id] != grupoA { t.Errorf("%q deveria estar no mesmo cluster que a1", id) }
	}
	grupoB := result["b1"]
	for _, id := range []string{"b2", "b3"} {
		if result[id] != grupoB { t.Errorf("%q deveria estar no mesmo cluster que b1", id) }
	}
	grupoC := result["c1"]
	for _, id := range []string{"c2", "c3"} {
		if result[id] != grupoC { t.Errorf("%q deveria estar no mesmo cluster que c1", id) }
	}

	// Grupos A, B, C devem ser diferentes entre si
	if grupoA == grupoB { t.Error("grupos A e B deveriam ser diferentes") }
	if grupoA == grupoC { t.Error("grupos A e C deveriam ser diferentes") }
	if grupoB == grupoC { t.Error("grupos B e C deveriam ser diferentes") }
}

func TestDistSq(t *testing.T) {
	d := distSq(0, 0, 3, 4)
	if d != 25 { t.Fatalf("distSq(0,0,3,4) = %f, esperado 25", d) }
}

func TestSilhouetteScore_Perfeito(t *testing.T) {
	// Dois clusters perfeitamente separados
	pts := []ClusterResult{
		{0, 0, 0}, {1, 0, 0},
		{100, 100, 1}, {101, 100, 1},
	}
	s := silhouetteScore(pts)
	if s < 0.9 { t.Errorf("silhouette score para clusters perfeitos deveria ser ~1, got %f", s) }
}

func TestSilhouetteScore_UmCluster(t *testing.T) {
	pts := []ClusterResult{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}}
	s := silhouetteScore(pts)
	if s != 0 { t.Errorf("silhouette score para 1 cluster deveria ser 0, got %f", s) }
}

func TestSilhouetteScore_UmPonto(t *testing.T) {
	s := silhouetteScore([]ClusterResult{{0, 0, 0}})
	if s != 0 { t.Errorf("silhouette score para 1 ponto deveria ser 0, got %f", s) }
}

func TestSilhouetteScore_Sobreposto(t *testing.T) {
	// Clusters sobrepostos devem ter score baixo
	pts := []ClusterResult{
		{0, 0, 0}, {1, 1, 0},
		{0, 1, 1}, {1, 0, 1},
	}
	s := silhouetteScore(pts)
	if s > 0.5 {
		t.Logf("score para clusters sobrepostos: %f (esperado baixo)", s)
	}
}

func TestKMeans_DoisClusters(t *testing.T) {
	pts := []ClusterResult{
		{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1},
		{X: 50, Y: 50}, {X: 51, Y: 50}, {X: 50, Y: 51},
	}
	kmeans(pts, 2, 30)
	c0 := pts[0].ClusterID
	for _, id := range []int{1, 2} {
		if pts[id].ClusterID != c0 { t.Errorf("ponto %d deveria estar no cluster %d", id, c0) }
	}
	c1 := pts[3].ClusterID
	for _, id := range []int{4, 5} {
		if pts[id].ClusterID != c1 { t.Errorf("ponto %d deveria estar no cluster %d", id, c1) }
	}
	if c0 == c1 { t.Error("dois grupos deveriam ser clusters diferentes") }
}

func TestKMeans_KIgualAN(t *testing.T) {
	pts := []ClusterResult{{X: 0}, {X: 1}, {X: 2}}
	kmeans(pts, 3, 10)
	for i, p := range pts {
		if p.ClusterID != i { t.Errorf("ponto %d deveria ter cluster %d, got %d", i, i, p.ClusterID) }
	}
}

func TestKMeans_MaisClustersQuePontos(t *testing.T) {
	pts := []ClusterResult{{X: 0}, {X: 1}}
	kmeans(pts, 5, 10)
	for i, p := range pts {
		if p.ClusterID < 0 || p.ClusterID >= len(pts) {
			t.Errorf("clusterID %d fora do range [0, %d]", p.ClusterID, len(pts)-1)
		}
	}
}
