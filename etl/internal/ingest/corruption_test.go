package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"etl/internal/config"
)

// TestCorruptedBBoltFile verifica que NewAppState se recupera quando o
// arquivo state.db contém dados aleatórios (simula corrupção total).
func TestCorruptedBBoltFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	// Escrever lixo aleatório no arquivo (simula corrupção total)
	garbage := make([]byte, 65536)
	for i := range garbage {
		garbage[i] = byte(i % 256)
	}
	if err := os.WriteFile(dbPath, garbage, 0644); err != nil {
		t.Fatalf("Erro ao escrever arquivo corrompido: %v", err)
	}

	cfg := &config.AppConfig{StateDir: tmpDir}

	// Não deve panic — deve recriar o banco do zero
	appState := NewAppState(cfg)
	defer appState.Close()

	// Deve estar vivo após recuperação
	if !appState.IsAlive() {
		t.Fatal("AppState não está vivo após se recuperar de DB corrompido")
	}

	// Deve conseguir operar normalmente
	appState.SetHash("test", "hash123")
	hash, ok := appState.GetHash("test")
	if !ok || hash != "hash123" {
		t.Errorf("Operação básica falhou após recuperação de corrupção: esperado 'hash123', obteve '%s' (ok=%v)", hash, ok)
	}
}

// TestZeroByteBBoltFile verifica que NewAppState lida corretamente com
// um arquivo state.db vazio (0 bytes).
func TestZeroByteBBoltFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	// Criar arquivo vazio
	if err := os.WriteFile(dbPath, []byte{}, 0644); err != nil {
		t.Fatalf("Erro ao criar arquivo vazio: %v", err)
	}

	cfg := &config.AppConfig{StateDir: tmpDir}

	// Não deve panic — bbolt deve inicializar um banco novo
	appState := NewAppState(cfg)
	defer appState.Close()

	if !appState.IsAlive() {
		t.Fatal("AppState não está vivo após abrir DB vazio")
	}

	// Deve conseguir operar normalmente
	appState.SetHash("file1", "abc123")
	hash, ok := appState.GetHash("file1")
	if !ok || hash != "abc123" {
		t.Errorf("Operação básica falhou após abrir DB vazio: esperado 'abc123', obteve '%s'", hash)
	}

	// Verificar persistência fechando e reabrindo
	appState.Close()
	appState2 := NewAppState(cfg)
	defer appState2.Close()
	appState2.Load(cfg)

	hash2, ok2 := appState2.GetHash("file1")
	if !ok2 || hash2 != "abc123" {
		t.Errorf("Dados não persistiram após reabrir DB vazio: esperado 'abc123', obteve '%s' (ok=%v)", hash2, ok2)
	}
}

// TestPartialWriteCorruption simula uma falha de escrita parcial:
// cria um estado válido, corrompe alguns bytes no meio do arquivo,
// e verifica que NewAppState lida com isso sem panic.
func TestPartialWriteCorruption(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{StateDir: tmpDir}

	// 1. Criar estado válido com dados
	appState := NewAppState(cfg)
	appState.SetHash("doc1", "hash1")
	appState.SetHash("doc2", "hash2")
	appState.SetFileTags("doc1", []string{"tag-a", "tag-b"})
	appState.SetFileMetadata("doc1", map[string]interface{}{
		"status": "processed",
	})
	appState.Close()

	// 2. Corromper APENAS alguns bytes no meio do arquivo
	dbPath := filepath.Join(tmpDir, "state.db")
	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("Erro ao ler arquivo do banco: %v", err)
	}

	// Só corrompe se o arquivo for grande o suficiente
	if len(data) > 1024 {
		// Corromper 32 bytes no meio do arquivo
		mid := len(data) / 2
		for i := 0; i < 32 && mid+i < len(data); i++ {
			data[mid+i] = 0xFF
		}
		if err := os.WriteFile(dbPath, data, 0644); err != nil {
			t.Fatalf("Erro ao escrever arquivo corrompido: %v", err)
		}
	}

	// 3. Abrir novamente — não deve panic
	appState2 := NewAppState(cfg)
	defer appState2.Close()

	// Pode estar vivo (se a corrupção não afetou os meta-páginas)
	// ou ter sido recriado. De qualquer forma, não deve crashar.
	if !appState2.IsAlive() {
		t.Fatal("AppState não está vivo após abrir DB com corrupção parcial")
	}

	// 4. Tentar operações não deve causar panic
	appState2.SetHash("new-doc", "new-hash")
	appState2.GetHash("doc1")
	appState2.GetFileTags("doc1")
	appState2.GetFileMetadata("doc1")
}

// TestConcurrentCorruptionRecovery verifica que após uma corrupção
// e recuperação, o sistema permanece estável e não panica ao realizar
// operações normais.
func TestConcurrentCorruptionRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{StateDir: tmpDir}

	// 1. Criar estado válido com dados
	appState := NewAppState(cfg)
	appState.SetHash("existing", "existing-hash")
	appState.SetFileTags("existing", []string{"existing-tag"})
	appState.SetFileMetadata("existing", map[string]interface{}{
		"priority": 5.0,
	})
	appState.Close()

	// 2. Corromper o arquivo completamente
	dbPath := filepath.Join(tmpDir, "state.db")
	garbage := make([]byte, 65536)
	for i := range garbage {
		garbage[i] = byte((i*7 + 13) % 256)
	}
	if err := os.WriteFile(dbPath, garbage, 0644); err != nil {
		t.Fatalf("Erro ao corromper arquivo: %v", err)
	}

	// 3. Abrir novamente — deve recriar o banco
	appState2 := NewAppState(cfg)
	defer appState2.Close()

	if !appState2.IsAlive() {
		t.Fatal("AppState não está vivo após recuperação de corrupção total")
	}

	// 4. Realizar operações normais — nenhuma deve causar panic
	// Operações de escrita
	appState2.SetHash("recovered-1", "hash-recovered-1")
	appState2.SetHash("recovered-2", "hash-recovered-2")
	appState2.SetFileTags("recovered-1", []string{"tag1", "tag2"})
	appState2.SetFileMetadata("recovered-1", map[string]interface{}{
		"corrupted": false,
		"recovered": true,
	})

	// Operações de leitura
	if hash, ok := appState2.GetHash("recovered-1"); !ok || hash != "hash-recovered-1" {
		t.Errorf("GetHash falhou após recuperação: esperado 'hash-recovered-1', obteve '%s' (ok=%v)", hash, ok)
	}

	tags := appState2.GetFileTags("recovered-1")
	if len(tags) != 2 || tags[0] != "tag1" || tags[1] != "tag2" {
		t.Errorf("GetFileTags falhou após recuperação: esperado [tag1 tag2], obteve %v", tags)
	}

	meta := appState2.GetFileMetadata("recovered-1")
	if meta == nil {
		t.Fatal("GetFileMetadata falhou após recuperação: retornou nil")
	}
	recovered, ok := meta["recovered"].(bool)
	if !ok || !recovered {
		t.Errorf("GetFileMetadata falhou após recuperação: esperado recovered=true")
	}

	// Operações com tags
	appState2.AddKnownTag("recovered-tag")
	knownTags := appState2.GetAllKnownTags()
	found := false
	for _, tag := range knownTags {
		if tag == "recovered-tag" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("AddKnownTag/GetAllKnownTags falhou após recuperação: tag 'recovered-tag' não encontrada em %v", knownTags)
	}

	// Salvar (não deve panic)
	appState2.Save(cfg)

	// Fechar e reabrir para garantir persistência
	appState2.Close()
	appState3 := NewAppState(cfg)
	defer appState3.Close()
	appState3.Load(cfg)

	if hash, ok := appState3.GetHash("recovered-1"); !ok || hash != "hash-recovered-1" {
		t.Errorf("Dados não persistiram após reabrir: esperado 'hash-recovered-1', obteve '%s' (ok=%v)", hash, ok)
	}
}
