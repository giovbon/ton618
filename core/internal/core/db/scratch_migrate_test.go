package db

import (
	"fmt"
	"testing"
)

// TestScratch_MigrateSafety verifica se há arquivos no disco mais novos que no banco
func TestScratch_MigrateSafety(t *testing.T) {
	dbPath := "/home/giobon/Área de trabalho/ton618/core/data/ton618.db"
	docsDir := "/home/giobon/Área de trabalho/ton618/core/docs"

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	count, err := store.MigrateNotesFromDisk(docsDir)
	if err != nil {
		t.Fatalf("MigrateNotesFromDisk: %v", err)
	}
	t.Logf("Notas importadas do disco (não estavam no banco): %d", count)

	if count > 0 {
		t.Errorf("⚠️  ATENÇÃO: %d nota(s) existiam no disco mas não no banco — foram importadas. Verifique antes de deletar a pasta.", count)
	} else {
		t.Logf("✅ Banco está completo — nenhuma nota nova encontrada no disco. Seguro deletar docs/notes/")
	}
	fmt.Println("done")
}
