package clustering

import (
	"testing"
)

func TestProjectPCA_DimensionMismatch(t *testing.T) {
	// Vetores com tamanhos diferentes (simulando troca de modelo)
	noteVectors := map[string][]float32{
		"note1.md": {1.0, 0.0, 0.5, 0.2}, // Dim 4
		"note2.md": {0.5, 0.8},           // Dim 2 (será preenchido com zeros)
		"note3.md": {0.0, 0.1, 0.9, 1.0, 0.5}, // Dim 5 (será truncado para o tamanho da primeira nota em ordem alfabética?)
	}

	// O código atual define 'cols' com base no primeiro ID em ordem alfabética.
	// note1.md é o primeiro. Então cols = 4.

	projections := ProjectPCA(noteVectors)

	if len(projections) != 3 {
		t.Errorf("Esperava 3 projeções, obteve %d", len(projections))
	}

	for id, coords := range projections {
		if id == "" {
			t.Error("ID vazio na projeção")
		}
		// Verificar se não são NaN
		if coords[0] != coords[0] || coords[1] != coords[1] {
			t.Errorf("Projeção para %s contém NaN: %v", id, coords)
		}
	}
}

func TestProjectPCA_SingleNote(t *testing.T) {
	noteVectors := map[string][]float32{
		"note1.md": {1.0, 0.5, 0.2},
	}

	projections := ProjectPCA(noteVectors)

	if len(projections) != 1 {
		t.Errorf("Esperava 1 projeção, obteve %d", len(projections))
	}

	if projections["note1.md"] != [2]float64{0, 0} {
		t.Errorf("Esperava [0,0] para nota única, obteve %v", projections["note1.md"])
	}
}
