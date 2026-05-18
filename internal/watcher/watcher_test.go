package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ton618/internal/config"
	"ton618/internal/db"
)

// ── helpers ─────────────────────────────────────────────────────

func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := db.NewStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func newTestCfgAndWatcher(t *testing.T, store *db.Store) (*config.AppConfig, *Watcher) {
	t.Helper()
	docsDir := t.TempDir()
	for _, sub := range MonitoredSubDirs {
		os.MkdirAll(filepath.Join(docsDir, sub), 0755)
	}

	cfg := &config.AppConfig{
		DocsDir: docsDir,
	}

	w := NewWatcher(cfg, store)
	return cfg, w
}

// seedDB insere dados em todas as tabelas para um arquivo, simulando
// um arquivo que já foi indexado.
func seedDB(t *testing.T, store *db.Store, filename string) {
	t.Helper()

	// documents
	doc := db.Document{
		ID:        "test-id-1",
		Tipo:      "markdown",
		Arquivo:   filename,
		Secao:     "Teste",
		Texto:     "conteudo de teste para validar limpeza",
		Tags:      "teste,limpeza",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if err := store.InsertDocument(doc); err != nil {
		t.Fatalf("InsertDocument: %v", err)
	}

	// FTS
	if err := store.IndexFTS(doc.ID, doc.Tipo, doc.Arquivo, doc.Secao, doc.Texto, doc.Tags); err != nil {
		t.Fatalf("IndexFTS: %v", err)
	}

	// file_mods
	if err := store.SetFileMod(filename, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("SetFileMod: %v", err)
	}

	// popularity
	if err := store.IncrementPopularity(filename); err != nil {
		t.Fatalf("IncrementPopularity: %v", err)
	}

	// tags
	if err := store.SetFileTags(filename, []string{"teste", "limpeza"}); err != nil {
		t.Fatalf("SetFileTags: %v", err)
	}
}

// assertNotInDB verifica que todas as tabelas estão limpas para o arquivo.
func assertNotInDB(t *testing.T, store *db.Store, filename string) {
	t.Helper()

	// documents
	count := store.GetDocumentCount()
	if count > 0 {
		t.Errorf("documents ainda tem %d registros, esperado 0", count)
	}

	// file_mods
	mods, err := store.GetAllFileMods()
	if err != nil {
		t.Fatalf("GetAllFileMods: %v", err)
	}
	if _, exists := mods[filename]; exists {
		t.Error("file_mods ainda contem o arquivo")
	}

	// tags
	tags, err := store.GetFileTags(filename)
	if err != nil {
		t.Fatalf("GetFileTags: %v", err)
	}
	if len(tags) > 0 {
		t.Errorf("tags ainda tem %d tags para o arquivo", len(tags))
	}

	// popularity
	pop := store.GetPopularity(filename)
	if pop > 0 {
		t.Errorf("popularity ainda tem contagem %d para o arquivo", pop)
	}

	// embeddings (se existiam)
	allEmb := store.GetEmbeddingCount()
	if allEmb > 0 {
		t.Errorf("embeddings ainda tem %d registros", allEmb)
	}
}

// ── ProcessFile — delete ────────────────────────────────────────

func TestProcessFile_Delete_LimpaTodasTabelas(t *testing.T) {
	store := newTestStore(t)
	_, w := newTestCfgAndWatcher(t, store)

	filename := "notes/teste-delete.md"
	seedDB(t, store, filename)

	// Verificar que os dados estao no banco antes
	mods, _ := store.GetAllFileMods()
	if _, ok := mods[filename]; !ok {
		t.Fatal("seed falhou: file_mods devia conter o arquivo")
	}

	// Processar delecao
	ev := FileEvent{
		Path:     filepath.Join(w.cfg.DocsDir, filename),
		Filename: filename,
		Type:     "delete",
	}
	if err := ProcessFile(store, ev, nil, false); err != nil {
		t.Fatalf("ProcessFile(delete): %v", err)
	}

	assertNotInDB(t, store, filename)
}

func TestProcessFile_Delete_ArquivoInexistente(t *testing.T) {
	store := newTestStore(t)
	_, w := newTestCfgAndWatcher(t, store)

	// Deletar arquivo que nunca existiu — não deve crashar
	ev := FileEvent{
		Path:     filepath.Join(w.cfg.DocsDir, "notes/fantasma.md"),
		Filename: "notes/fantasma.md",
		Type:     "delete",
	}
	if err := ProcessFile(store, ev, nil, false); err != nil {
		t.Fatalf("ProcessFile(delete) para arquivo inexistente: %v", err)
	}
}

func TestProcessFile_Delete_ExtensaoNaoSuportada(t *testing.T) {
	store := newTestStore(t)

	// Arquivo .txt nao esta em supportedExts — deve ser ignorado
	ev := FileEvent{
		Filename: "notes/notas.txt",
		Type:     "delete",
	}
	if err := ProcessFile(store, ev, nil, false); err != nil {
		t.Fatalf("ProcessFile com extensao nao suportada deveria retornar nil, got %v", err)
	}
}

func TestProcessFile_Delete_MultiplosDocumentos(t *testing.T) {
	store := newTestStore(t)
	_, w := newTestCfgAndWatcher(t, store)

	filename := "notes/multiplos-docs.md"

	// Inserir varios documentos para o mesmo arquivo
	for i := 0; i < 3; i++ {
		doc := db.Document{
			ID:        "test-id-multi",
			Tipo:      "markdown",
			Arquivo:   filename,
			Secao:     "Teste",
			Texto:     "multi",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		if err := store.InsertDocument(doc); err != nil {
			t.Fatalf("InsertDocument #%d: %v", i, err)
		}
	}

	// file_mods
	store.SetFileMod(filename, time.Now().UTC().Format(time.RFC3339))

	// Deletar
	ev := FileEvent{
		Path:     filepath.Join(w.cfg.DocsDir, filename),
		Filename: filename,
		Type:     "delete",
	}
	if err := ProcessFile(store, ev, nil, false); err != nil {
		t.Fatalf("ProcessFile(delete): %v", err)
	}

	assertNotInDB(t, store, filename)
}

// ── ProcessFile — modify (reindex) ──────────────────────────────

func TestProcessFile_Modify_RemoveAntesDeInserir(t *testing.T) {
	store := newTestStore(t)
	cfg, w := newTestCfgAndWatcher(t, store)

	filename := "notes/reindex.md"
	fullPath := filepath.Join(cfg.DocsDir, filename)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("# Reindex\n\nconteudo novo"), 0644)

	// Seed com dados antigos
	seedDB(t, store, filename)

	// Reindexar (modify)
	ev := FileEvent{
		Path:     fullPath,
		Filename: filename,
		ModTime:  time.Now(),
		Type:     "modify",
	}
	if err := ProcessFile(store, ev, nil, false); err != nil {
		t.Fatalf("ProcessFile(modify): %v", err)
	}

	// Deve ter o novo documento
	count, err := store.GetDocumentCount()
	if err != nil {
		t.Fatalf("GetDocumentCount: %v", err)
	}
	if count == 0 {
		t.Error("modify deveria ter inserido novo documento")
	}

	// file_mods deve existir
	mods, _ := store.GetAllFileMods()
	if _, ok := mods[filename]; !ok {
		t.Error("file_mods deveria conter o arquivo apos reindex")
	}
}

// ── PollAll — limpeza de orfaos ─────────────────────────────────

func TestPollAll_RemoveOrfaos(t *testing.T) {
	store := newTestStore(t)
	_, w := newTestCfgAndWatcher(t, store)

	// Seed com dados de um arquivo que NAO existe no disco
	orphanFile := "notes/orfao.md"
	seedDB(t, store, orphanFile)

	// pollAll deve detectar que orphanFile esta no DB mas nao no disco
	w.PollAll()

	// Consumir eventos do canal (ate timeout)
	timeout := time.After(2 * time.Second)
	foundDelete := false
	for {
		select {
		case ev := <-w.Events():
			if ev.Type == "delete" && ev.Filename == orphanFile {
				foundDelete = true
				// Processar o delete
				ProcessFile(store, ev, nil, false)
			}
		case <-timeout:
			goto done
		}
	}
done:

	if !foundDelete {
		t.Fatal("pollAll nao emitiu evento delete para arquivo orfao")
	}

	assertNotInDB(t, store, orphanFile)
}

func TestPollAll_ArquivoNoDisco_Preserva(t *testing.T) {
	store := newTestStore(t)
	cfg, w := newTestCfgAndWatcher(t, store)

	// Criar arquivo real no disco
	filename := "notes/preservar.md"
	fullPath := filepath.Join(cfg.DocsDir, filename)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	os.WriteFile(fullPath, []byte("# Preservar"), 0644)

	// Indexar via ProcessFile
	ev := FileEvent{
		Path:     fullPath,
		Filename: filename,
		ModTime:  time.Now(),
		Type:     "modify",
	}
	ProcessFile(store, ev, nil, false)

	// pollAll nao deve emitir delete para este arquivo
	w.PollAll()

	timeout := time.After(1 * time.Second)
	deleteEmitted := false
	for {
		select {
		case ev := <-w.Events():
			if ev.Type == "delete" && ev.Filename == filename {
				deleteEmitted = true
			}
		case <-timeout:
			goto done2
		}
	}
done2:

	if deleteEmitted {
		t.Error("pollAll emitiu delete para arquivo que existe no disco")
	}

	// Verificar que o arquivo ainda esta no DB
	mods, _ := store.GetAllFileMods()
	if _, ok := mods[filename]; !ok {
		t.Error("arquivo que existe no disco deveria permanecer no DB")
	}
}

func TestPollAll_SemOrfaos_NaoEmiteDelete(t *testing.T) {
	store := newTestStore(t)
	_, w := newTestCfgAndWatcher(t, store)

	// Nenhum dado no banco, nenhum arquivo no disco
	w.PollAll()

	timeout := time.After(1 * time.Second)
	deleteCount := 0
	for {
		select {
		case ev := <-w.Events():
			if ev.Type == "delete" {
				deleteCount++
			}
		case <-timeout:
			goto done3
		}
	}
done3:

	if deleteCount > 0 {
		t.Errorf("pollAll emitiu %d eventos delete sem orfaos", deleteCount)
	}
}
