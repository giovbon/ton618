package ingest

import (
	"context"
	"etl/internal/config"
	"etl/internal/models"
	"etl/internal/search"
	"etl/internal/semantic"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVacuumOrphans(t *testing.T) {
	// 1. Preparar índice Bleve temporário
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "vacuum.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	// 2. Indexar documentos (1 válido e 2 órfãos)
	docs := []models.Document{
		{ID: "valid-1", Arquivo: "test-file.md", Texto: "matchme valid"},
		{ID: "orphan-1", Arquivo: "test-file.md", Texto: "matchme orphan 1"},
		{ID: "orphan-2", Arquivo: "test-file.md", Texto: "matchme orphan 2"},
	}
	for _, doc := range docs {
		search.IndexDocument(doc.ID, doc)
	}

	cfg := &config.AppConfig{}

	// 3. Executar VacuumOrphans indicando que apenas 'valid-1' é esperado
	expectedIds := []string{"valid-1"}
	VacuumOrphans(cfg, "test-file.md", expectedIds)

	// 4. Validar se os órfãos foram deletados
	count, _ := search.GetIndex().DocCount()
	if count != 1 {
		t.Errorf("Esperado 1 documento restante no índice, obteve %d", count)
	}

	// Verificar se o documento que sobrou é o correto
	res, _ := search.ExecuteSearch(context.Background(), "matchme", false, 0, 50)
	if res.Hits.Total.Value == 1 {
		if res.Hits.Hits[0].ID != "valid-1" {
			t.Errorf("Documento errado sobrou: %s", res.Hits.Hits[0].ID)
		}
	}
}

func TestGetLatestWriteTime(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pkm-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Criar subpastas
	os.MkdirAll(filepath.Join(tempDir, "notes"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "links"), 0755)

	// 1. Vazio
	if !getLatestWriteTime(tempDir).IsZero() {
		t.Error("Esperava tempo zero para diretório vazio")
	}

	// 2. Com arquivo em notes
	notePath := filepath.Join(tempDir, "notes", "test.md")
	os.WriteFile(notePath, []byte("test"), 0644)

	now := time.Now()
	os.Chtimes(notePath, now, now)

	latest := getLatestWriteTime(tempDir)
	if latest.Format(time.RFC3339) != now.Format(time.RFC3339) {
		t.Errorf("Tempo incorreto: %v != %v", latest, now)
	}

	// 3. Com arquivo em links mais novo
	linkPath := filepath.Join(tempDir, "links", "test.md")
	future := now.Add(1 * time.Hour)
	os.WriteFile(linkPath, []byte("test"), 0644)
	os.Chtimes(linkPath, future, future)

	latest = getLatestWriteTime(tempDir)
	if latest.Format(time.RFC3339) != future.Format(time.RFC3339) {
		t.Errorf("Tempo futuro não detectado: %v != %v", latest, future)
	}
}

func TestDeleteFileFromBleve_ComplexNames(t *testing.T) {
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "complex.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	filename := "Notes (v1) - Test.pdf"
	docID := "frag-1"
	search.IndexDocument(docID, models.Document{ID: docID, Arquivo: filename, Texto: "teste"})

	cfg := &config.AppConfig{}

	// 2. Executar deleção
	DeleteFileFromBleve(cfg, filename)

	// 3. Verificar
	res, _ := search.ExecuteSearch(context.Background(), "teste", false, 0, 50)
	if res.Hits.Total.Value != 0 {
		t.Error("Arquivo com nome complexo não foi deletado do Bleve")
	}
}

func TestGlobalVacuum_RemovesOrphans(t *testing.T) {
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "global_vacuum.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	os.MkdirAll(filepath.Join(tmpDir, "links"), 0755)

	// "exists-on-disk.md" existe no disco
	existingFilename := "links/exists-on-disk.md"
	existingFile := filepath.Join(tmpDir, existingFilename)
	os.WriteFile(existingFile, []byte("# Nota que existe"), 0644)

	// Indexar 1 existente e 1 fantasma
	search.IndexDocument("doc-real", models.Document{Arquivo: existingFilename, Texto: "borboleta real"})
	search.IndexDocument("doc-ghost", models.Document{Arquivo: "links/ghost.md", Texto: "borboleta fantasma"})

	cfg := &config.AppConfig{DocsDir: tmpDir}

	GlobalVacuum(cfg, nil)

	// Verificar se fantasma sumiu e o real ficou
	resReal, _ := search.ExecuteSearch(context.Background(), "real", false, 0, 50)
	if resReal.Hits.Total.Value == 0 {
		t.Errorf("GlobalVacuum removeu arquivo REAL erroneamente (ou busca falhou). Total: %d", resReal.Hits.Total.Value)
	}

	resGhost, _ := search.ExecuteSearch(context.Background(), "fantasma", false, 0, 50)
	if resGhost.Hits.Total.Value > 0 {
		t.Errorf("GlobalVacuum falhou em remover o arquivo FANTASMA. Total: %d", resGhost.Hits.Total.Value)
	}
}

func TestGlobalVacuum_EmptyIndex(t *testing.T) {
	// Banco vazio não deve causar erro ou panic
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "empty.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	cfg := &config.AppConfig{DocsDir: tmpDir}

	// Não deve entrar em panic ou retornar erro
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("GlobalVacuum com banco vazio causou panic: %v", r)
		}
	}()
	GlobalVacuum(cfg, nil)
}

func TestGlobalVacuum_BleveFailure(t *testing.T) {
	// Se o motor Bleve falhar (ex: index fechado), o GlobalVacuum deve falhar silenciosamente
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "fail.bleve")
	search.InitIndex(indexDir)
	search.CloseIndex() // FECHADO!

	cfg := &config.AppConfig{DocsDir: tmpDir}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("GlobalVacuum com Bleve fechado causou panic: %v", r)
		}
	}()
	GlobalVacuum(cfg, nil) // Deve apenas logar o erro e retornar
}

func TestRebuildKnownTagsCache_Success(t *testing.T) {
	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()

	// 1. Setup: Cache limpo e mapa de arquivos populado
	appState.SetFileTags("nota1.md", []string{"tag1", "tag2"})
	appState.SetFileTags("nota2.md", []string{"tag2", "tag3"})
	appState.SetFileTags("vazio.md", []string{})

	// 2. Executar
	appState.RebuildKnownTagsCache()

	// 3. Validar: O cache deve conter a união única das tags
	if appState.GetKnownTagsCount() != 3 {
		t.Errorf("Esperado 3 tags únicas, obteve %d", appState.GetKnownTagsCount())
	}
	expected := []string{"tag1", "tag2", "tag3"}
	knownTags := appState.GetAllKnownTags()
	knownMap := make(map[string]bool)
	for _, tag := range knownTags {
		knownMap[tag] = true
	}
	for _, k := range expected {
		if !knownMap[k] {
			t.Errorf("Tag esperada %s não encontrada no cache", k)
		}
	}
}

func TestRebuildKnownTagsCache_CleanOnEmpty(t *testing.T) {
	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()
	// 1. Setup: Cache com lixo e mapa vazio
	appState.AddKnownTag("orphaned")

	// 2. Executar
	appState.RebuildKnownTagsCache()

	// 3. Validar: O cache deve estar vazio
	if appState.GetKnownTagsCount() != 0 {
		t.Errorf("Esperado 0 tags, obteve %d", appState.GetKnownTagsCount())
	}
}

func TestUpdateStateAfterOCR_SyncsCaches(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-image-*.png")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	relPath := "attachments/test-image.png"
	docID := semantic.HashFunc("img-" + relPath)
	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()

	docs := []models.Document{
		{
			ID:   docID,
			Hash: "hash-do-ocr-concluido",
			Tags: []string{"screenshot", "notas"},
		},
	}

	UpdateStateAfterOCR(tmpFile.Name(), relPath, docs, appState)

	cachedHash, exists := appState.GetHash(docID)
	if !exists {
		t.Error("HashCache não foi atualizado após UpdateStateAfterOCR")
	}
	if cachedHash != "hash-do-ocr-concluido" {
		t.Errorf("Hash incorreto no cache: %s", cachedHash)
	}

	_, modExists := appState.GetFileMod(tmpFile.Name())
	if !modExists {
		t.Error("FileModCache não foi atualizado")
	}

	appState.RebuildKnownTagsCache()
	if appState.GetKnownTagsCount() != 2 {
		t.Errorf("Tags não foram atualizadas: %d tags", appState.GetKnownTagsCount())
	}
}

func TestUpdateStateAfterOCR_EmptyDocsDoesNothing(t *testing.T) {
	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()
	appState.hashCacheMu.RLock()
	before := len(appState.hashCache)
	appState.hashCacheMu.RUnlock()
	UpdateStateAfterOCR("/tmp/blank.png", "attachments/blank-image.png", nil, appState)
	appState.hashCacheMu.RLock()
	after := len(appState.hashCache)
	appState.hashCacheMu.RUnlock()

	if after != before {
		t.Errorf("HashCache foi modificado: antes=%d depois=%d", before, after)
	}
}

// --- Novos testes de Modularidade, no-embed e Ciclo de Vida ---

func TestHasNoEmbedTag(t *testing.T) {
	tests := []struct {
		tags     []string
		expected bool
	}{
		{[]string{"no-embed"}, true},
		{[]string{"NO-EMBED"}, true},
		{[]string{"No-Embed"}, true},
		{[]string{"tag1", "no-embed", "tag2"}, true},
		{[]string{"important"}, false},
		{[]string{}, false},
		{nil, false},
	}

	for _, tt := range tests {
		result := HasNoEmbedTag(tt.tags)
		if result != tt.expected {
			t.Errorf("Para tags %v, esperado %v, obteve %v", tt.tags, tt.expected, result)
		}
	}
}

func TestHasEmbedTag(t *testing.T) {
	tests := []struct {
		tags     []string
		expected bool
	}{
		{[]string{"embed"}, true},
		{[]string{"EMBED"}, true},
		{[]string{"Embed"}, true},
		{[]string{"tag1", "embed", "tag2"}, true},
		{[]string{"important"}, false},
		{[]string{}, false},
		{nil, false},
	}

	for _, tt := range tests {
		result := HasEmbedTag(tt.tags)
		if result != tt.expected {
			t.Errorf("Para tags %v, esperado %v, obteve %v", tt.tags, tt.expected, result)
		}
	}
}

func TestSendToEngines_RespectsFlags(t *testing.T) {
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "engines.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()
	cfg := &config.AppConfig{SemanticEnable: true, DocsDir: tmpDir}

	// Criar arquivos físicos para passar na verificação de os.Stat
	os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("teste"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "no-embed.md"), []byte("não me indexe"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "embed.md"), []byte("me indexe"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "imagem.png"), []byte("imagem"), 0644)

	// Cenário A: Global ON, mas Usuário OFF -> Deve PULAR semântica
	appState.settings.SemanticEnable = false
	doc := models.Document{ID: "test-1", Arquivo: "test.md", Texto: "teste", Tipo: "markdown"}

	SendToEngines(cfg, []models.Document{doc}, []models.Document{doc}, appState)

	count, _ := search.GetIndex().DocCount()
	if count != 1 {
		t.Errorf("Deveria ter indexado no Bleve mesmo com semântica OFF")
	}

	// Cenário B: Global ON, Usuário ON, nota SEM tag 'embed' -> Deve PULAR semântica
	appState.settings.SemanticEnable = true
	docNoEmbed := models.Document{
		ID:      "test-no-embed",
		Arquivo: "no-embed.md",
		Texto:   "não me indexe",
		Tipo:    "markdown",
		Tags:    []string{"outra-tag"},
	}

	SendToEngines(cfg, []models.Document{docNoEmbed}, []models.Document{docNoEmbed}, appState)

	// Nota C: Global ON, Usuário ON, nota COM tag 'embed' -> Deve PROCESSAR semântica
	docEmbed := models.Document{
		ID:      "test-embed",
		Arquivo: "embed.md",
		Texto:   "me indexe",
		Tipo:    "markdown",
		Tags:    []string{"embed"},
	}
	SendToEngines(cfg, []models.Document{docEmbed}, []models.Document{docEmbed}, appState)

	// Nota D: Imagem SEM tag 'embed' -> Deve PROCESSAR semântica por padrão
	docImg := models.Document{
		ID:      "test-img",
		Arquivo: "imagem.png",
		Texto:   "texto extraído",
		Tipo:    "image",
	}
	SendToEngines(cfg, []models.Document{docImg}, []models.Document{docImg}, appState)

	count, _ = search.GetIndex().DocCount()
	if count != 4 {
		t.Errorf("Deveria ter 4 documentos no Bleve, obteve %d", count)
	}
}

func TestChunkLifecycle(t *testing.T) {
	// 1. Setup
	tmpDir := t.TempDir()
	indexDir := filepath.Join(tmpDir, "lifecycle.bleve")
	search.InitIndex(indexDir)
	defer search.CloseIndex()

	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(filepath.Join(docsDir, "notes"), 0755)

	cfg := &config.AppConfig{DocsDir: docsDir, BleveIndexDir: indexDir}
	appState := NewAppState(&config.AppConfig{StateDir: t.TempDir()})
	defer appState.Close()
	filename := "notes/lifecycle.md"
	absPath := filepath.Join(docsDir, filename)

	// --- FASE 1: CRIAÇÃO ---
	content1 := "# Seção 1\nConteúdo 1\n\n# Seção 2\nConteúdo 2"
	os.WriteFile(absPath, []byte(content1), 0644)

	RunSync(cfg, false, "test", appState)

	count, _ := search.GetIndex().DocCount()
	if count != 2 {
		t.Errorf("FASE 1: Esperava 2 chunks, obteve %d", count)
	}

	// --- FASE 2: ATUALIZAÇÃO (Adição) ---
	content2 := "# Seção 1\nConteúdo 1\n\n# Seção 2\nConteúdo 2\n\n# Seção 3\nConteúdo 3"
	os.WriteFile(absPath, []byte(content2), 0644)
	os.Chtimes(absPath, time.Now().Add(1*time.Second), time.Now().Add(1*time.Second))

	RunSync(cfg, false, "test", appState)

	count, _ = search.GetIndex().DocCount()
	if count != 3 {
		t.Errorf("FASE 2: Esperava 3 chunks após adição, obteve %d", count)
	}

	// --- FASE 3: ATUALIZAÇÃO (Remoção / Orfanagem) ---
	content3 := "# Seção 1\nConteúdo 1 mudado"
	os.WriteFile(absPath, []byte(content3), 0644)
	os.Chtimes(absPath, time.Now().Add(2*time.Second), time.Now().Add(2*time.Second))

	RunSync(cfg, false, "test", appState)

	count, _ = search.GetIndex().DocCount()
	if count != 1 {
		t.Errorf("FASE 3: Esperava 1 chunk após remoção (Vacuum falhou?), obteve %d", count)
	}

	// --- FASE 4: EXCLUSÃO ---
	os.Remove(absPath)
	RunSync(cfg, false, "test", appState)

	count, _ = search.GetIndex().DocCount()
	if count != 0 {
		t.Errorf("FASE 4: Esperava 0 chunks após deletar arquivo, obteve %d", count)
	}
}
