package index

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
	if maxX-minX < 0.001 {
		t.Error("pontos nao estao espalhados em X")
	}
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
	vecs := map[string][]float32{"a": {1.0, 2.0, 3.0}, "b": {1.0, 2.0, 3.0}, "c": {1.0, 2.0, 3.0}}
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
	m, k := ClusterPoints(map[string]Point2D{})
	if m != nil || k != 0 { t.Fatalf("nil/0, got map=%v k=%d", m, k) }
	m, k = ClusterPoints(map[string]Point2D{"a": {0, 0}})
	if len(m) != 1 || k != 1 || m["a"] != 0 { t.Fatalf("1 ponto: cluster 0, got m=%v k=%d", m, k) }
}

func TestClusterPoints_DoisPontos(t *testing.T) {
	m, k := ClusterPoints(map[string]Point2D{"a": {0, 0}, "b": {10, 10}})
	if k != 2 { t.Fatalf("k=2, got %d", k) }
	if m["a"] == m["b"] { t.Error("distantes deveriam estar em clusters diferentes") }
}

func TestClusterPoints_Deterministico(t *testing.T) {
	pts := map[string]Point2D{
		"a": {0, 0}, "b": {1, 0}, "c": {0, 1},
		"d": {10, 10}, "e": {11, 10}, "f": {10, 11},
		"g": {20, 20}, "h": {21, 20}, "i": {20, 21},
	}
	r1, k1 := ClusterPoints(pts)
	r2, k2 := ClusterPoints(pts)
	if k1 != k2 { t.Errorf("k: %d vs %d", k1, k2) }
	for id := range pts { if r1[id] != r2[id] { t.Errorf("%q: %d vs %d", id, r1[id], r2[id]) } }
}

func TestClusterPoints_TresGruposBemSeparados(t *testing.T) {
	pts := map[string]Point2D{
		"a1": {0, 0}, "a2": {1, 0}, "a3": {0, 1},
		"b1": {50, 50}, "b2": {51, 50}, "b3": {50, 51},
		"c1": {50, 0}, "c2": {51, 0}, "c3": {50, 1},
	}
	result, k := ClusterPoints(pts)
	if k < 2 || k > 4 { t.Fatalf("k entre 2 e 4, got %d", k) }
	for _, id := range []string{"a2", "a3"} { if result[id] != result["a1"] { t.Errorf("%q != a1", id) } }
	for _, id := range []string{"b2", "b3"} { if result[id] != result["b1"] { t.Errorf("%q != b1", id) } }
	for _, id := range []string{"c2", "c3"} { if result[id] != result["c1"] { t.Errorf("%q != c1", id) } }
	if result["a1"] == result["b1"] { t.Error("A == B") }
	if result["a1"] == result["c1"] { t.Error("A == C") }
	if result["b1"] == result["c1"] { t.Error("B == C") }
}

func TestDistSq(t *testing.T) {
	d := distSq(0, 0, 3, 4)
	if d != 25 { t.Fatalf("distSq = %f, esperado 25", d) }
}

func TestSilhouetteScore_Perfeito(t *testing.T) {
	pts := []ClusterResult{{0, 0, 0}, {1, 0, 0}, {100, 100, 1}, {101, 100, 1}}
	s := silhouetteScore(pts)
	if s < 0.9 { t.Errorf("esperado ~1, got %f", s) }
}

func TestSilhouetteScore_UmCluster(t *testing.T) {
	if s := silhouetteScore([]ClusterResult{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}}); s != 0 {
		t.Errorf("1 cluster = 0, got %f", s)
	}
}

func TestKMeans_DoisClusters(t *testing.T) {
	pts := []ClusterResult{
		{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1},
		{X: 50, Y: 50}, {X: 51, Y: 50}, {X: 50, Y: 51},
	}
	kmeans(pts, 2, 30)
	c0 := pts[0].ClusterID
	for _, id := range []int{1, 2} { if pts[id].ClusterID != c0 { t.Errorf("%d != %d", id, c0) } }
	c1 := pts[3].ClusterID
	for _, id := range []int{4, 5} { if pts[id].ClusterID != c1 { t.Errorf("%d != %d", id, c1) } }
	if c0 == c1 { t.Error("grupos deveriam ser diferentes") }
}

func TestKMeans_KIgualAN(t *testing.T) {
	pts := []ClusterResult{{X: 0}, {X: 1}, {X: 2}}
	kmeans(pts, 3, 10)
	for i, p := range pts { if p.ClusterID != i { t.Errorf("%d: %d", i, p.ClusterID) } }
}

// ── Testes de Power Iteration ──────────────────────────────────

func TestPowerIteration_2x2Identity(t *testing.T) {
	m := [][]float64{{1, 0}, {0, 1}}
	v := powerIteration(m, 2, 50)
	n := math.Sqrt(v[0]*v[0] + v[1]*v[1])
	if math.Abs(n-1.0) > 0.0001 { t.Errorf("norma = %f, esperado 1", n) }
}

func TestPowerIteration_Diagonal(t *testing.T) {
	m := [][]float64{{2, 0}, {0, 1}}
	v := powerIteration(m, 2, 100)
	if math.Abs(v[0]) < 0.9 { t.Errorf("~[1,0], got [%f,%f]", v[0], v[1]) }
}

func TestPowerIterationDeflated_OrtogonalAoPrimeiro(t *testing.T) {
	m := [][]float64{{3, 0}, {0, 1}}
	e1 := powerIteration(m, 2, 100)
	e2 := powerIterationDeflated(m, 2, e1, 100)
	var dot float64
	for j := 0; j < 2; j++ { dot += e1[j] * e2[j] }
	if math.Abs(dot) > 0.01 { t.Errorf("ortogonais, dot=%f", dot) }
}

func TestProject2DReduce_DimensoesDesiguais(t *testing.T) {
	r := Project2DReduce(map[string][]float32{"a": {0.1, 0.2, 0.3}, "b": {0.1, 0.2}})
	if r != nil { t.Error("dimensoes desiguais deveriam retornar nil") }
}

func TestProject2DReduce_ConsistenciaComDeflacao(t *testing.T) {
	v := map[string][]float32{"a": {1, 0, 0, 0}, "b": {0, 1, 0, 0}, "c": {0, 0, 1, 1}, "d": {0.5, 0.5, 0, 0}}
	r1, r2 := Project2DReduce(v), Project2DReduce(v)
	for id, p1 := range r1 {
		if p2 := r2[id]; math.Abs(p1.X-p2.X) > 0.0001 || math.Abs(p1.Y-p2.Y) > 0.0001 {
			t.Errorf("%q: (%f,%f) vs (%f,%f)", id, p1.X, p1.Y, p2.X, p2.Y)
		}
	}
}

// ── Testes de regressao: determinismo ──────────────────────────

func TestTSNE_Deterministico(t *testing.T) {
	v := map[string][]float32{"a": {0.1, 0.2, 0.3, 0.4}, "b": {0.9, 0.8, 0.7, 0.6}, "c": {0.5, 0.5, 0.5, 0.5}}
	r1, r2 := DefaultTSNE().Project(v), DefaultTSNE().Project(v)
	for id, p1 := range r1 {
		if p2 := r2[id]; math.Abs(p1.X-p2.X) > 0.0001 || math.Abs(p1.Y-p2.Y) > 0.0001 {
			t.Errorf("t-SNE: %q mudou: (%f,%f) vs (%f,%f)", id, p1.X, p1.Y, p2.X, p2.Y)
		}
	}
}

func TestTSNE_OrdemDiferente(t *testing.T) {
	v1 := map[string][]float32{"a": {0.1, 0.2, 0.3, 0.4}, "b": {0.9, 0.8, 0.7, 0.6}, "c": {0.5, 0.5, 0.5, 0.5}}
	v2 := map[string][]float32{"c": {0.5, 0.5, 0.5, 0.5}, "a": {0.1, 0.2, 0.3, 0.4}, "b": {0.9, 0.8, 0.7, 0.6}}
	r1, r2 := DefaultTSNE().Project(v1), DefaultTSNE().Project(v2)
	for id := range r1 {
		p1, p2 := r1[id], r2[id]
		if math.Abs(p1.X-p2.X) > 0.0001 || math.Abs(p1.Y-p2.Y) > 0.0001 {
			t.Errorf("t-SNE ordem: %q: (%f,%f) vs (%f,%f)", id, p1.X, p1.Y, p2.X, p2.Y)
		}
	}
}

func TestTSNE_1Vetor(t *testing.T) {
	r := DefaultTSNE().Project(map[string][]float32{"u": {0.1, 0.2}})
	if r["u"].X != 0 || r["u"].Y != 0 { t.Errorf("(0,0), got (%f,%f)", r["u"].X, r["u"].Y) }
}

func TestTSNE_1536D(t *testing.T) {
	d := 1536
	v := map[string][]float32{"a": make([]float32, d), "b": make([]float32, d)}
	for i := range d { v["a"][i] = float32(i) * 0.001; v["b"][i] = float32(i+100) * 0.001 }
	r := DefaultTSNE().Project(v)
	if len(r) != 2 { t.Fatalf("2, got %d", len(r)) }
}

func TestPCA_Determinismo5Exec(t *testing.T) {
	v := map[string][]float32{"go": {0.9, 0.8, 0, 0}, "rs": {0.8, 0.9, 0, 0}, "po": {0, 0, 0.9, 0.8}}
	var prev map[string]Point2D
	for e := 0; e < 5; e++ {
		c := Project2DReduce(v)
		if prev != nil {
			for id, p := range c {
				if q := prev[id]; math.Abs(p.X-q.X) > 0.0001 || math.Abs(p.Y-q.Y) > 0.0001 {
					t.Errorf("exec %d: %q mudou: (%f,%f) vs (%f,%f)", e, id, p.X, p.Y, q.X, q.Y)
				}
			}
		}
		prev = c
	}
}

func TestClusterHighD_Deterministico(t *testing.T) {
	v := map[string][]float32{"a1": {1, 0, 0}, "a2": {0.9, 0.1, 0}, "b1": {0, 1, 0}, "b2": {0, 0.9, 0.1}, "c1": {0, 0, 1}}
	r1, k1 := ClusterHighD(v)
	r2, k2 := ClusterHighD(v)
	if k1 != k2 { t.Errorf("k: %d vs %d", k1, k2) }
	for id := range v { if r1[id] != r2[id] { t.Errorf("%q: %d vs %d", id, r1[id], r2[id]) } }
}

func TestClusterHighD_OrdemDiferente(t *testing.T) {
	v1 := map[string][]float32{"x": {0.5, 0, 0}, "y": {0, 0.5, 0}, "z": {0, 0, 0.5}}
	v2 := map[string][]float32{"z": {0, 0, 0.5}, "x": {0.5, 0, 0}, "y": {0, 0.5, 0}}
	r1, _ := ClusterHighD(v1)
	r2, _ := ClusterHighD(v2)
	for id := range v1 { if r1[id] != r2[id] { t.Errorf("%q: %d vs %d", id, r1[id], r2[id]) } }
}

func TestClusterHighD_Identicos(t *testing.T) {
	r, _ := ClusterHighD(map[string][]float32{"a": {0.5, 0.5}, "b": {0.5, 0.5}})
	if r["a"] != r["b"] { t.Error("identicos deveriam estar no mesmo cluster") }
}

func TestClusterHighD_Vazio(t *testing.T) {
	r, k := ClusterHighD(map[string][]float32{})
	if r != nil || k != 0 { t.Errorf("nil/0, got map=%v k=%d", r, k) }
}

func TestScalePoints_Determinismo(t *testing.T) {
	pts := map[string]Point2D{"a": {10, 20}, "b": {30, 40}, "c": {50, 60}}
	ScalePoints(pts, 500)
	if math.Abs(pts["a"].X) < 400 { t.Errorf("a longe de 0: (%f,%f)", pts["a"].X, pts["a"].Y) }
	if math.Abs(pts["b"].X) > 0.001 || math.Abs(pts["b"].Y) > 0.001 { t.Errorf("centro (b) != 0: (%f,%f)", pts["b"].X, pts["b"].Y) }
}

func TestScalePoints_Colapsados(t *testing.T) {
	pts := map[string]Point2D{"a": {5, 5}, "b": {5, 5}}
	ScalePoints(pts, 500)
	if pts["a"].X != 0 || pts["a"].Y != 0 { t.Errorf("colapsados deviam ser 0: (%f,%f)", pts["a"].X, pts["a"].Y) }
}
