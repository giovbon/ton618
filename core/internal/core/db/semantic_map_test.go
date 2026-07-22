package db

import (
	"fmt"
	"math"
	"sync"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════
// TESTES DA PCA (384D → 2D)
// ═══════════════════════════════════════════════════════════════════

func TestComputePCA2D_menosDe2Pontos(t *testing.T) {
	t.Parallel()
	// ⚠️ Guard-clause: N < 2 deve retornar pontos em (0,0) sem erro
	embeddings := map[string][]float32{
		"notas/unicanota.md": makeEmbedding(0.5),
	}

	points, err := computePCA2D(embeddings)
	if err != nil {
		t.Fatalf("computePCA2D com 1 ponto não deveria errar: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("esperado 1 ponto, got %d", len(points))
	}
	if points[0].X != 0 || points[0].Y != 0 {
		t.Errorf("ponto único deveria ser (0,0), got (%.4f, %.4f)", points[0].X, points[0].Y)
	}
	if points[0].Filename != "notas/unicanota.md" {
		t.Errorf("filename errado: %s", points[0].Filename)
	}
}

func TestComputePCA2D_zeroEmbeddings(t *testing.T) {
	t.Parallel()
	// Nenhum embedding → mapa vazio
	points, err := computePCA2D(map[string][]float32{})
	if err != nil {
		t.Fatalf("computePCA2D vazio não deveria errar: %v", err)
	}
	if len(points) != 0 {
		t.Errorf("esperado 0 pontos, got %d", len(points))
	}
}

func TestComputePCA2D_embeddingsIdenticos(t *testing.T) {
	t.Parallel()
	// Todos os embeddings com o mesmo valor → matriz de covariância zero
	embeddings := map[string][]float32{
		"notas/a.md": makeEmbedding(0.5),
		"notas/b.md": makeEmbedding(0.5),
		"notas/c.md": makeEmbedding(0.5),
	}

	points, err := computePCA2D(embeddings)
	if err != nil {
		t.Fatalf("computePCA2D com embeddings idênticos falhou: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("esperado 3 pontos, got %d", len(points))
	}
	for _, p := range points {
		if p.X != 0 || p.Y != 0 {
			t.Errorf("embedding idêntico deveria dar (0,0), got (%.6f, %.6f) para %s", p.X, p.Y, p.Filename)
		}
	}
}

func TestComputePCA2D_embeddingValido(t *testing.T) {
	t.Parallel()
	embeddings := map[string][]float32{
		"notas/grupoA_1.md": rampEmbedding(1.0, 0.0),
		"notas/grupoA_2.md": rampEmbedding(0.9, 0.1),
		"notas/grupoB_1.md": rampEmbedding(-1.0, 0.0),
	}

	points, err := computePCA2D(embeddings)
	if err != nil {
		t.Fatalf("computePCA2D falhou: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("esperado 3 pontos, got %d", len(points))
	}
	for _, p := range points {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) {
			t.Errorf("ponto %s tem NaN: (%.4f, %.4f)", p.Filename, p.X, p.Y)
		}
		if math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
			t.Errorf("ponto %s tem Inf: (%.4f, %.4f)", p.Filename, p.X, p.Y)
		}
	}
	// Verifica que filenames foram preservados
	files := make(map[string]bool)
	for _, p := range points {
		files[p.Filename] = true
	}
	for f := range embeddings {
		if !files[f] {
			t.Errorf("filename %s não está nos resultados", f)
		}
	}
}

func TestComputePCA2D_muitosEmbeddings(t *testing.T) {
	t.Parallel()
	n := 100
	embeddings := make(map[string][]float32, n)
	for i := 0; i < n; i++ {
		scale := float32(i%3 - 1)
		embeddings[sprintf("notas/nota_%d.md", i)] = rampEmbedding(scale, float32(i)*0.01)
	}

	points, err := computePCA2D(embeddings)
	if err != nil {
		t.Fatalf("computePCA2D com %d embeddings falhou: %v", n, err)
	}
	if len(points) != n {
		t.Fatalf("esperado %d pontos, got %d", n, len(points))
	}
	for _, p := range points {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) {
			t.Errorf("ponto %s tem NaN", p.Filename)
		}
		if p.Cluster < 0 || p.Cluster >= maxClusters {
			t.Errorf("cluster %d fora do range [0,%d)", p.Cluster, maxClusters)
		}
	}
}

func TestComputePCA2D_determinismo(t *testing.T) {
	t.Parallel()
	embeddings := map[string][]float32{
		"notas/a.md": rampEmbedding(1.0, 0.0),
		"notas/b.md": rampEmbedding(-1.0, 0.0),
		"notas/c.md": rampEmbedding(0.5, 0.2),
	}

	points1, _ := computePCA2D(embeddings)
	points2, _ := computePCA2D(embeddings)

	if len(points1) != len(points2) {
		t.Fatalf("número de pontos diferente entre execuções")
	}
	for i := range points1 {
		if points1[i].Filename != points2[i].Filename {
			t.Errorf("filename diferente na posição %d", i)
		}
		if math.Abs(points1[i].X-points2[i].X) > 1e-10 {
			t.Errorf("X diferente para %s: %.10f vs %.10f", points1[i].Filename, points1[i].X, points2[i].X)
		}
		if math.Abs(points1[i].Y-points2[i].Y) > 1e-10 {
			t.Errorf("Y diferente para %s: %.10f vs %.10f", points1[i].Filename, points1[i].Y, points2[i].Y)
		}
		if points1[i].Cluster != points2[i].Cluster {
			t.Errorf("cluster diferente para %s: %d vs %d", points1[i].Filename, points1[i].Cluster, points2[i].Cluster)
		}
	}
}

func TestComputePCA2D_notasSimilaresAgrupadas(t *testing.T) {
	t.Parallel()
	embeddings := map[string][]float32{
		"grupoA_1.md": rampEmbedding(1.0, 0.0),
		"grupoA_2.md": rampEmbedding(0.95, 0.05),
		"grupoB_1.md": rampEmbedding(-1.0, 0.0),
		"grupoB_2.md": rampEmbedding(-0.95, -0.05),
		"grupoC_1.md": rampEmbedding(0.1, 0.0),
		"grupoC_2.md": rampEmbedding(0.15, -0.02),
	}

	points, err := computePCA2D(embeddings)
	if err != nil {
		t.Fatalf("computePCA2D falhou: %v", err)
	}

	// Agrupa por prefixo (grupoA_, grupoB_, grupoC_)
	groups := make(map[string][]SemanticMapPoint)
	for _, p := range points {
		prefix := p.Filename[:7]
		groups[prefix] = append(groups[prefix], p)
	}

	for gName, gPoints := range groups {
		if len(gPoints) < 2 {
			continue
		}
		dx := gPoints[0].X - gPoints[1].X
		dy := gPoints[0].Y - gPoints[1].Y
		intraDist := dx*dx + dy*dy

		for otherName, otherPoints := range groups {
			if otherName == gName {
				continue
			}
			for _, op := range otherPoints {
				dx = gPoints[0].X - op.X
				dy = gPoints[0].Y - op.Y
				interDist := dx*dx + dy*dy
				if interDist < intraDist*0.9 {
					t.Errorf(
						"%s: dist intra=%.4f < dist inter=%.4f com %s (esperado intra menor)",
						gName, intraDist, interDist, otherName,
					)
				}
			}
		}
	}
}

// ═══════════════════════════════════════════════════════════════════
// TESTES DO K-MEANS
// ═══════════════════════════════════════════════════════════════════

func TestKMeans2D_doisClusters(t *testing.T) {
	t.Parallel()
	points := []SemanticMapPoint{
		{Filename: "a.md", X: 1, Y: 1},
		{Filename: "b.md", X: 1.1, Y: 0.9},
		{Filename: "c.md", X: 10, Y: 10},
		{Filename: "d.md", X: 10.1, Y: 9.9},
	}
	labels := kMeans2D(points, 2, 20)
	if len(labels) != 4 {
		t.Fatalf("esperado 4 labels, got %d", len(labels))
	}
	if labels[0] != labels[1] {
		t.Errorf("a e b deveriam estar no mesmo cluster: %d vs %d", labels[0], labels[1])
	}
	if labels[2] != labels[3] {
		t.Errorf("c e d deveriam estar no mesmo cluster: %d vs %d", labels[2], labels[3])
	}
	if labels[0] == labels[2] {
		t.Errorf("clusters deveriam ser diferentes: %d", labels[0])
	}
}

func TestKMeans2D_umCluster(t *testing.T) {
	t.Parallel()
	labels := kMeans2D([]SemanticMapPoint{
		{Filename: "a.md", X: 1, Y: 1},
		{Filename: "b.md", X: 2, Y: 2},
	}, 1, 10)
	if len(labels) != 2 {
		t.Fatalf("esperado 2 labels, got %d", len(labels))
	}
	if labels[0] != 0 || labels[1] != 0 {
		t.Errorf("ambos deveriam ser cluster 0: %d, %d", labels[0], labels[1])
	}
}

func TestKMeans2D_kMaiorQueN(t *testing.T) {
	t.Parallel()
	labels := kMeans2D([]SemanticMapPoint{
		{Filename: "a.md", X: 0, Y: 0},
	}, 5, 10)
	if len(labels) != 1 {
		t.Fatalf("esperado 1 label, got %d", len(labels))
	}
}

func TestKMeans2D_listaVazia(t *testing.T) {
	t.Parallel()
	labels := kMeans2D(nil, 3, 10)
	if labels != nil {
		t.Errorf("lista vazia deveria retornar nil, got %v", labels)
	}
}

func TestKMeans2D_tresClustersBemSeparados(t *testing.T) {
	t.Parallel()
	points := []SemanticMapPoint{
		{Filename: "a1", X: 0, Y: 0},
		{Filename: "a2", X: 0.1, Y: 0.1},
		{Filename: "b1", X: 10, Y: 10},
		{Filename: "b2", X: 10.1, Y: 9.9},
		{Filename: "c1", X: 20, Y: 20},
		{Filename: "c2", X: 20.1, Y: 20.1},
	}
	labels := kMeans2D(points, 3, 20)

	for _, pair := range [][2]int{{0, 1}, {2, 3}, {4, 5}} {
		if labels[pair[0]] != labels[pair[1]] {
			t.Errorf("pontos %d e %d deveriam estar no mesmo cluster", pair[0], pair[1])
		}
	}
	if labels[0] == labels[2] || labels[0] == labels[4] || labels[2] == labels[4] {
		t.Errorf("clusters deveriam ser distintos: %d, %d, %d", labels[0], labels[2], labels[4])
	}
}

// ═══════════════════════════════════════════════════════════════════
// TESTES DO CACHE
// ═══════════════════════════════════════════════════════════════════

func TestComputeChecksum_consistente(t *testing.T) {
	t.Parallel()
	m1 := map[string][]float32{"a.md": {}, "b.md": {}}
	m2 := map[string][]float32{"b.md": {}, "a.md": {}}
	if computeChecksum(m1) != computeChecksum(m2) {
		t.Errorf("checksums deveriam ser iguais independente da ordem")
	}
}

func TestComputeChecksum_diferente(t *testing.T) {
	t.Parallel()
	if computeChecksum(map[string][]float32{"a.md": {}}) == computeChecksum(map[string][]float32{"b.md": {}}) {
		t.Error("checksums diferentes não deveriam colidir")
	}
}

func TestComputeChecksum_vazio(t *testing.T) {
	t.Parallel()
	_ = computeChecksum(map[string][]float32{}) // não deve panicar
}

func TestComputeChecksum_muitosElementos(t *testing.T) {
	t.Parallel()
	m := make(map[string][]float32, 1000)
	for i := 0; i < 1000; i++ {
		m[sprintf("notas/nota_%d.md", i)] = nil
	}
	if computeChecksum(m) == 0 {
		t.Error("checksum não deveria ser 0 para 1000 elementos")
	}
}

func TestCacheThreadSafety(t *testing.T) {
	// Cria cache local para não interferir com outros testes
	localCache := &semanticMapCache{}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			localCache.mu.RLock()
			_ = localCache.checksum
			_ = localCache.points
			localCache.mu.RUnlock()
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			localCache.mu.Lock()
			localCache.checksum = 42
			localCache.points = []SemanticMapPoint{{Filename: "test.md"}}
			localCache.mu.Unlock()
		}()
	}
	wg.Wait()
}

// ═══════════════════════════════════════════════════════════════════
// TESTES DE DISPLAY NAME
// ═══════════════════════════════════════════════════════════════════

func TestDisplayName(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		input, expected string
	}{
		{"notes/foo.md", "foo"},
		{"notes/bar/baz.md", "baz"},
		{"drawings/meu-desenho.excalidraw", "meu-desenho"},
		{"notas/sem_extensao", "sem_extensao"},
		{"apenasnome", "apenasnome"},
		{"pasta/subpasta/arquivo.composto.txt", "arquivo.composto"},
		{"a/b/c/d/e/f/g.md", "g"},
		{"", ""},
		{"sem_ponto", "sem_ponto"},
	} {
		got := displayName(tt.input)
		if got != tt.expected {
			t.Errorf("displayName(%q) = %q, esperado %q", tt.input, got, tt.expected)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════
// TESTES DE ÁLGEBRA LINEAR
// ═══════════════════════════════════════════════════════════════════

func TestNormalize(t *testing.T) {
	t.Parallel()
	v := normalize([]float64{3, 4})
	if math.Abs(v[0]-0.6) > 1e-10 || math.Abs(v[1]-0.8) > 1e-10 {
		t.Errorf("normalize([3,4]) = [%.10f, %.10f]", v[0], v[1])
	}
	// Vetor nulo não produz NaN
	n0 := normalize([]float64{0, 0, 0})
	for i, val := range n0 {
		if math.IsNaN(val) {
			t.Errorf("vetor nulo produziu NaN na posição %d", i)
		}
	}
	// Vetor já unitário
	nu := normalize([]float64{1, 0, 0})
	if math.Abs(nu[0]-1) > 1e-15 {
		t.Errorf("normalize([1,0,0]) = [%.10f]", nu[0])
	}
}

func TestNormalize_mantemDirecao(t *testing.T) {
	t.Parallel()
	n := normalize([]float64{100, 200, 300})
	normE := math.Sqrt(1*1 + 2*2 + 3*3)
	expected := []float64{1 / normE, 2 / normE, 3 / normE}
	for i := range n {
		if math.Abs(n[i]-expected[i]) > 1e-10 {
			t.Errorf("direção incorreta na posição %d: %.10f vs %.10f", i, n[i], expected[i])
		}
	}
}

func TestPowerIteration(t *testing.T) {
	t.Parallel()
	dim := 20
	cov := make([][]float64, dim)
	for i := 0; i < dim; i++ {
		cov[i] = make([]float64, dim)
		if i == 0 {
			cov[i][i] = 100.0
		} else {
			cov[i][i] = 1.0
		}
	}
	v := powerIteration(cov, dim, 30)
	if math.Abs(math.Abs(v[0])-1) > 0.1 {
		t.Errorf("primeiro componente deveria ser ~±1, got %.6f", v[0])
	}
}

func TestPowerIterationDeflated(t *testing.T) {
	t.Parallel()
	dim := 10
	cov := make([][]float64, dim)
	for i := 0; i < dim; i++ {
		cov[i] = make([]float64, dim)
		cov[i][i] = float64(dim - i)
	}
	comp0 := powerIteration(cov, dim, 50)
	comp1 := powerIterationDeflated(cov, dim, comp0, 50)

	var dot float64
	for i := 0; i < dim; i++ {
		dot += comp0[i] * comp1[i]
	}
	if math.Abs(dot) > 0.1 {
		t.Errorf("componentes deveriam ser ortogonais, dot=%.6f", dot)
	}
	var n0, n1 float64
	for i := 0; i < dim; i++ {
		n0 += comp0[i] * comp0[i]
		n1 += comp1[i] * comp1[i]
	}
	if math.Abs(n0-1) > 0.1 {
		t.Errorf("comp0 não é unitário: |v|=%.6f", n0)
	}
	if math.Abs(n1-1) > 0.1 {
		t.Errorf("comp1 não é unitário: |v|=%.6f", n1)
	}
}

// ═══════════════════════════════════════════════════════════════════
// TESTES DE INTEGRAÇÃO COM BANCO REAL
// ═══════════════════════════════════════════════════════════════════

func TestGetSemanticMapPoints_bancoVazio(t *testing.T) {
	store := newTestStore(t)
	points, err := store.GetSemanticMapPoints()
	if err != nil {
		t.Fatalf("banco vazio falhou: %v", err)
	}
	if len(points) != 0 {
		t.Errorf("banco vazio deveria ter 0 pontos, got %d", len(points))
	}
}

func TestGetSemanticMapPoints_comEmbeddings(t *testing.T) {
	store := newTestStore(t)
	blob1, _ := serializeEmbedding(rampEmbedding(1.0, 0.0))
	blob2, _ := serializeEmbedding(rampEmbedding(-1.0, 0.0))
	blob3, _ := serializeEmbedding(rampEmbedding(0.5, 0.2))

	for _, q := range [][2]interface{}{
		{"notes/teste1.md#0", blob1},
		{"notes/teste2.md#0", blob2},
		{"notes/teste3.md#0", blob3},
	} {
		_, err := store.DB.Exec(`INSERT INTO note_embeddings(chunk_id, embedding) VALUES (?, ?)`, q[0], q[1])
		if err != nil {
			t.Fatalf("inserir embedding: %v", err)
		}
	}

	points, err := store.GetSemanticMapPoints()
	if err != nil {
		t.Fatalf("GetSemanticMapPoints falhou: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("esperado 3 pontos, got %d", len(points))
	}

	files := make(map[string]bool)
	for _, p := range points {
		files[p.Filename] = true
	}
	for _, expected := range []string{"notes/teste1.md", "notes/teste2.md", "notes/teste3.md"} {
		if !files[expected] {
			t.Errorf("filename %s não encontrado", expected)
		}
	}
	for _, p := range points {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) {
			t.Errorf("ponto %s tem NaN", p.Filename)
		}
	}
}

func TestGetSemanticMapPoints_cacheFunciona(t *testing.T) {
	store := newTestStore(t)
	blob, _ := serializeEmbedding(rampEmbedding(1.0, 0.0))
	store.DB.Exec(`INSERT INTO note_embeddings(chunk_id, embedding) VALUES (?, ?)`,
		"notes/teste.md#0", blob)

	points1, _ := store.GetSemanticMapPoints()
	points2, _ := store.GetSemanticMapPoints()

	if len(points1) != len(points2) {
		t.Fatalf("cache: tamanhos diferentes")
	}
	for i := range points1 {
		if points1[i].X != points2[i].X || points1[i].Y != points2[i].Y {
			t.Errorf("cache retornou valores diferentes")
		}
	}
}

func TestGetSemanticMapPoints_cacheInvalida(t *testing.T) {
	store := newTestStore(t)
	blob1, _ := serializeEmbedding(rampEmbedding(1.0, 0.0))
	store.DB.Exec(`INSERT INTO note_embeddings(chunk_id, embedding) VALUES (?, ?)`,
		"notes/teste1.md#0", blob1)
	points1, _ := store.GetSemanticMapPoints()

	blob2, _ := serializeEmbedding(rampEmbedding(-1.0, 0.0))
	store.DB.Exec(`INSERT INTO note_embeddings(chunk_id, embedding) VALUES (?, ?)`,
		"notes/teste2.md#0", blob2)
	points2, _ := store.GetSemanticMapPoints()

	if len(points2) <= len(points1) {
		t.Errorf("cache não invalidou: antes %d, depois %d", len(points1), len(points2))
	}
}

func TestGetSemanticMapPoints_apenasChunkZero(t *testing.T) {
	store := newTestStore(t)
	blob, _ := serializeEmbedding(rampEmbedding(1.0, 0.0))
	store.DB.Exec(`INSERT INTO note_embeddings(chunk_id, embedding) VALUES (?, ?)`, "notes/teste.md#0", blob)
	store.DB.Exec(`INSERT INTO note_embeddings(chunk_id, embedding) VALUES (?, ?)`, "notes/teste.md#1", blob)
	store.DB.Exec(`INSERT INTO note_embeddings(chunk_id, embedding) VALUES (?, ?)`, "notes/teste.md#2", blob)

	points, _ := store.GetSemanticMapPoints()
	if len(points) != 1 {
		t.Errorf("esperado 1 ponto (1 nota, 3 chunks), got %d", len(points))
	}
}

// ═══════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════

func makeEmbedding(val float32) []float32 {
	v := make([]float32, EmbeddingDim)
	for i := range v {
		v[i] = val
	}
	return v
}

func rampEmbedding(scale, offset float32) []float32 {
	v := make([]float32, EmbeddingDim)
	for i := range v {
		v[i] = scale*float32(i)/float32(EmbeddingDim) + offset
	}
	return v
}

func sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
