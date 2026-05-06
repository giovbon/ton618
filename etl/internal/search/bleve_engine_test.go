package search

import (
	"path/filepath"
	"testing"
)

func TestBleveEngine_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	indexBatch := filepath.Join(tmpDir, "lifecycle.bleve")

	// 1. Test InitIndex (Create)
	err := InitIndex(indexBatch)
	if err != nil {
		t.Fatalf("Erro ao inicializar índice: %v", err)
	}
	defer CloseIndex()

	idx := GetIndex()
	if idx == nil {
		t.Fatal("Esperado índice não nulo após InitIndex")
	}

	// 2. Test IndexDocument
	docID := "test-doc"
	docData := map[string]interface{}{
		"texto":   "Conteúdo de teste para o Bleve",
		"arquivo": "test.md",
	}
	err = IndexDocument(docID, docData)
	if err != nil {
		t.Errorf("Erro ao indexar documento: %v", err)
	}

	// 3. Test GetIndex and Search directly
	// (Simples verificação se o índice está operando)
	count, _ := idx.DocCount()
	if count != 1 {
		t.Errorf("Esperado 1 documento no índice, obteve %d", count)
	}

	// 4. Test DeleteDocument
	err = DeleteDocument(docID)
	if err != nil {
		t.Errorf("Erro ao deletar documento: %v", err)
	}

	count, _ = idx.DocCount()
	if count != 0 {
		t.Errorf("Esperado 0 documentos após deleção, obteve %d", count)
	}
}

func TestBleveEngine_Reopen(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "reopen.bleve")

	// Criar inicial
	InitIndex(indexPath)
	IndexDocument("d1", map[string]string{"texto": "t1"})
	CloseIndex()

	// Reabrir
	err := InitIndex(indexPath)
	if err != nil {
		t.Fatalf("Erro ao reabrir índice: %v", err)
	}
	defer CloseIndex()

	idx := GetIndex()
	count, _ := idx.DocCount()
	if count != 1 {
		t.Errorf("Esperado 1 documento após reabertura, obteve %d", count)
	}
}

func TestBleveEngine_Errors(t *testing.T) {
	// 1. IndexDocument sem Init
	CloseIndex()
	err := IndexDocument("any", nil)
	if err == nil {
		t.Error("Esperava erro ao indexar sem inicializar o índice")
	}

	// 2. DeleteDocument sem Init
	err = DeleteDocument("any")
	if err == nil {
		t.Error("Esperava erro ao deletar sem inicializar o índice")
	}

	// 3. InitIndex em caminho inválido (opcional, dependendo do OS)
	// No linux, criar em diretório sem permissão (se test runner não for root)
	err = InitIndex("/root/invalid.bleve")
	if err == nil {
		t.Error("Esperava erro ao criar índice em local proibido")
	}
}
