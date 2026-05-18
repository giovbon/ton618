package search

import (
	"etl/internal/models"
	"os"
	"math"
	"testing"
)

func TestWeightsPersistenceAndApplication(t *testing.T) {
	// 1. Setup: Criar diretório de estado temporário
	tmpDir, err := os.MkdirTemp("", "seeker-test-*")
	if err != nil {
		t.Fatalf("Erro ao criar tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Inicializar com pesos padrão
	InitializeWeights(tmpDir)

	hit := &models.SearchHit{
		Score: 10.0,
		Source: models.Document{
			Texto: "o rato roeu a roupa do rei",
			Tipo:  "markdown",
		},
	}
	query := []string{"rato", "roeu"}

	// 3. Score com peso original (BoostPhrase default é 1.2 = +120%)
	// Score Base: 10.0
	// Phrase Boost: +12.0 (120% de 10.0)
	// Keyword Bonus: +2.0 (1.0 para 'rato' + 1.0 para 'roeu')
	// Total: 24.0
	scoreBase, _ := ScoreFragment(hit, query, "rato roeu", "rato roeu", 0, 0)
	expectedBase := 24.0
	if math.Abs(scoreBase-expectedBase) > 0.01 {
		t.Errorf("Score base incorreto. Esperado %f, obtido %f", expectedBase, scoreBase)
	}

	// 4. Mudar o peso dinamicamente (Simulando o POST da UI)
	newWeights := GetWeights()
	newWeights.BoostPhrase = 5.0 // +500%
	SaveWeights(newWeights)

	// 5. Verificar se refletiu no cálculo IMEDIATAMENTE (sem restart)
	scoreNovo, details := ScoreFragment(hit, query, "rato roeu", "rato roeu", 0, 0)
	// Base: 10.0
	// Phrase Boost: +50.0 (500% de 10.0)
	// Keyword Bonus: +2.0
	// Total: 62.0
	if math.Abs(scoreNovo-62.0) > 0.01 {
		t.Errorf("O novo peso de frase não foi aplicado corretamente. Score: %f", scoreNovo)
	}
	// Agora os detalhes mostram o bônus real somado (+50.0), não o multiplicador
	if details["proximidade_exata"] != 50.0 {
		t.Errorf("Detalhes do score mostram bônus incorreto: %f", details["proximidade_exata"])
	}

	// 6. Simular RESTART do servidor (Recarregar do disco)
	InitializeWeights(tmpDir)
	weightsNoBoot := GetWeights()
	if weightsNoBoot.BoostPhrase != 5.0 {
		t.Errorf("Persistência falhou. Após recarregar, o peso voltou para o padrão: %f", weightsNoBoot.BoostPhrase)
	}
}
