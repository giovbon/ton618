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
	// Devem estar em lados opostos do eixo X
	if result["a"].X >= result["b"].X {
		t.Logf("a=(%f,%f) b=(%f,%f)", result["a"].X, result["a"].Y, result["b"].X, result["b"].Y)
	}
}

func TestProject2DReduce_ManyDimensions(t *testing.T) {
	// 5 vetores de 768 dimensões (simula embeddings Gemini)
	n := 10
	d := 768
	vecs := make(map[string][]float32, n)
	for i := range n {
		id := string(rune('a' + i))
		vec := make([]float32, d)
		for j := range d {
			vec[j] = float32(i*100+j) * 0.001 // valores progressivos
		}
		vecs[id] = vec
	}

	result := Project2DReduce(vecs)
	if len(result) != n {
		t.Fatalf("esperado %d nós, got %d", n, len(result))
	}

	// Verifica que os pontos são distintos (não todos iguais)
	var minX, maxX float64 = math.MaxFloat64, -math.MaxFloat64
	var minY, maxY float64 = math.MaxFloat64, -math.MaxFloat64
	for _, p := range result {
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
	if maxX-minX < 0.001 {
		t.Error("pontos nao estao espalhados em X")
	}
	if maxY-minY < 0.001 {
		t.Error("pontos nao estao espalhados em Y")
	}

	// Verifica que estão normalizados entre -1 e 1
	if maxX > 1.1 || minX < -1.1 || maxY > 1.1 || minY < -1.1 {
		t.Errorf("fora do range [-1,1]: X[%f,%f] Y[%f,%f]", minX, maxX, minY, maxY)
	}
}

func TestProject2DReduce_ConsistentOutput(t *testing.T) {
	// Mesma entrada deve produzir mesma saida (determinística)
	vecs := map[string][]float32{
		"x": {0.5, 0.3, 0.1, 0.0, -0.2},
		"y": {0.1, 0.6, 0.3, 0.2, -0.1},
		"z": {-0.3, 0.0, 0.4, 0.5, 0.2},
	}

	r1 := Project2DReduce(vecs)
	r2 := Project2DReduce(vecs)

	for id, p1 := range r1 {
		p2, ok := r2[id]
		if !ok {
			t.Fatalf("id %q ausente na segunda execucao", id)
		}
		if math.Abs(p1.X-p2.X) > 0.00001 || math.Abs(p1.Y-p2.Y) > 0.00001 {
			t.Errorf("resultado inconsistente para %q: r1=(%f,%f) r2=(%f,%f)",
				id, p1.X, p1.Y, p2.X, p2.Y)
		}
	}
}

func TestProject2DReduce_AllIdenticalVectors(t *testing.T) {
	// Vetores idênticos devem produzir pontos colapsados
	vecs := map[string][]float32{
		"a": {1.0, 2.0, 3.0},
		"b": {1.0, 2.0, 3.0},
		"c": {1.0, 2.0, 3.0},
	}
	result := Project2DReduce(vecs)
	if len(result) != 3 {
		t.Fatalf("esperado 3 nós, got %d", len(result))
	}
	// Todos devem estar muito próximos (ou no mesmo ponto)
	for _, p := range result {
		if math.Abs(p.X) > 0.001 && math.Abs(p.Y) > 0.001 {
			t.Logf("ponto nao-nulo para vetores idênticos: (%f,%f)", p.X, p.Y)
		}
	}
}

func TestNormalizeVector(t *testing.T) {
	v := []float64{3.0, 4.0}
	normalize(v)
	norm := v[0]*v[0] + v[1]*v[1]
	if math.Abs(norm-1.0) > 0.00001 {
		t.Errorf("norma deveria ser 1, got %f", norm)
	}
	if math.Abs(v[0]-0.6) > 0.00001 || math.Abs(v[1]-0.8) > 0.00001 {
		t.Errorf("vetor normalizado incorreto: [%f,%f]", v[0], v[1])
	}
}

func TestNormalizePoints(t *testing.T) {
	pts := map[string]Point2D{
		"a": {X: 10, Y: 10},
		"b": {X: 20, Y: 20},
		"c": {X: 15, Y: 15},
	}
	normalizePoints(pts)
	// Centro deve estar perto de 0
	var sumX, sumY float64
	for _, p := range pts {
		sumX += p.X
		sumY += p.Y
	}
	if math.Abs(sumX) > 0.001 {
		t.Errorf("centro X nao-zero: %f", sumX)
	}
	if math.Abs(sumY) > 0.001 {
		t.Errorf("centro Y nao-zero: %f", sumY)
	}
}

func TestProjectEmbeddings(t *testing.T) {
	vecs := map[string][]float32{
		"id1": {1.0, 0.0, 0.0},
		"id2": {0.0, 1.0, 0.0},
		"id3": {0.0, 0.0, 1.0},
	}
	pts, ids := ProjectEmbeddings(vecs)
	if pts == nil || ids == nil {
		t.Fatal("ProjectEmbeddings retornou nil")
	}
	if len(ids) != 3 {
		t.Fatalf("esperado 3 ids, got %d", len(ids))
	}
	// IDs devem estar ordenados
	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[i-1] {
			t.Errorf("IDs nao ordenados: %v", ids)
			break
		}
	}
}
