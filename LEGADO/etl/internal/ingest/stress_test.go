package ingest

import (
	"etl/internal/config"
	"etl/internal/search"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParallelSyncStress(t *testing.T) {
	// 1. Setup ambiente temporário
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "stress.bleve")

	search.CloseIndex()
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	docsDir := filepath.Join(tmpDir, "docs")
	notesDir := filepath.Join(docsDir, "notes")
	os.MkdirAll(notesDir, 0755)

	cfg := &config.AppConfig{
		DocsDir:       docsDir,
		BleveIndexDir: indexDir,
		StateDir:      tmpDir,
		StateFile:     filepath.Join(tmpDir, "state.json"),
	}
	appState := NewAppState(cfg)
	defer appState.Close()

	// 2. Gerar 50 arquivos markdown
	const numFiles = 50
	for i := 0; i < numFiles; i++ {
		filename := fmt.Sprintf("note_%d.md", i)
		path := filepath.Join(notesDir, filename)
		content := fmt.Sprintf("# Nota %d\nConteúdo. #tag%d", i, i%5)
		os.WriteFile(path, []byte(content), 0644)
	}

	// 3. Rodar Sincronização Massiva
	RunSync(cfg, false, "test", appState)

	// 4. Verificações de Integridade
	idx := search.GetIndex()
	count, _ := idx.DocCount()

	if count != uint64(numFiles) {
		t.Errorf("Esperado %d documentos no índice, obteve %d", numFiles, count)
	}

	appState.RebuildKnownTagsCache()
	tagCount := appState.GetKnownTagsCount()
	if tagCount != 5 {
		t.Errorf("Esperado 5 tags únicas no cache, obteve %d", tagCount)
	}
}

func TestTagCacheConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "tags.bleve")

	search.CloseIndex()
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	docsDir := filepath.Join(tmpDir, "docs")
	notesDir := filepath.Join(docsDir, "notes")
	os.MkdirAll(notesDir, 0755)

	cfg := &config.AppConfig{
		DocsDir:       docsDir,
		BleveIndexDir: indexDir,
		StateDir:      tmpDir,
		StateFile:     filepath.Join(tmpDir, "state.json"),
	}
	appState := NewAppState(cfg)
	defer appState.Close()

	// 1. Criar nota com tag
	relPath := "notes/vader.md"
	absPath := filepath.Join(docsDir, relPath)
	os.WriteFile(absPath, []byte("# Darth Vader\n#darkside #force"), 0644)

	// 2. Rodar Sync Completo
	RunSync(cfg, false, "test", appState)
	appState.RebuildKnownTagsCache()

	tags := appState.GetAllKnownTags()
	if !contains(tags, "darkside") {
		t.Errorf("Tag 'darkside' não encontrada no cache. Tags: %v", tags)
	}

	// 3. Deletar a nota e rodar Sync novamente
	os.Remove(absPath)
	RunSync(cfg, false, "test", appState)

	appState.RebuildKnownTagsCache()
	tagsAfter := appState.GetAllKnownTags()
	if contains(tagsAfter, "darkside") {
		t.Errorf("Tag 'darkside' ainda persiste após deleção. Tags: %v", tagsAfter)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
