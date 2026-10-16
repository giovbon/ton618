package db

import (
	"path/filepath"
	"testing"
)

// TestMigrationV23_DropaLimpaIndiceSemanticoLegado cobre o caminho de UPGRADE de
// um banco que ainda tem o índice semântico da versão anterior do app:
// `note_chunks` (tabela normal) + `note_embeddings` (tabela virtual vec0), criadas
// pelas migrações v3–v8 — que foram REMOVIDAS do código em 16/10/2026.
//
// A migração v23 é a única coisa que impede esse banco de ficar com uma tabela
// virtual órfã (que o SQLite não consegue dropar sem o módulo `vec` registrado).
func TestMigrationV23_DropaLimpaIndiceSemanticoLegado(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")

	// 1. Banco "novo" (schema completo, todas as migrações aplicadas).
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// 2. Simula o resíduo da versão anterior: as tabelas do índice semântico,
	//    uma linha de cada, as settings de IA e a migração v23 NÃO aplicada.
	if _, err := store.DB.Exec(`CREATE TABLE note_chunks (
		chunk_id    TEXT PRIMARY KEY,
		filename    TEXT NOT NULL,
		chunk_index INTEGER NOT NULL,
		content     TEXT NOT NULL,
		indexed_mtime TEXT DEFAULT ''
	)`); err != nil {
		t.Fatalf("criar note_chunks: %v", err)
	}
	if _, err := store.DB.Exec(`CREATE VIRTUAL TABLE note_embeddings USING vec0(
		chunk_id TEXT PRIMARY KEY,
		embedding FLOAT[384]
	)`); err != nil {
		t.Fatalf("criar note_embeddings (vec0): %v", err)
	}
	if _, err := store.DB.Exec(
		`INSERT INTO note_chunks (chunk_id, filename, chunk_index, content) VALUES ('notes/a.md#0','notes/a.md',0,'conteudo')`,
	); err != nil {
		t.Fatalf("insert note_chunks: %v", err)
	}
	blob := make([]byte, 384*4)
	if _, err := store.DB.Exec(
		`INSERT INTO note_embeddings (chunk_id, embedding) VALUES ('notes/a.md#0', ?)`, blob,
	); err != nil {
		t.Fatalf("insert note_embeddings: %v", err)
	}
	for _, kv := range [][2]string{
		{"ollama_url", "http://127.0.0.1:11434"},
		{"ollama_model", "llama3.2"},
		{"semantic_search_threshold", "35"},
		{"hybrid_semantic_threshold", "55"},
		{"rrf_k", "60"},
		{"semantic_device", "wasm"},
		{"embedding_model_version", "minilm-abc123"},
	} {
		if err := store.SetSetting(kv[0], kv[1]); err != nil {
			t.Fatalf("SetSetting(%s): %v", kv[0], err)
		}
	}
	if _, err := store.DB.Exec("DELETE FROM schema_versions WHERE version = 23"); err != nil {
		t.Fatalf("desmarcar v23: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 3. Reabre: NewStore roda as migrações e a v23 deve limpar tudo.
	reopened, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore (reabertura): %v", err)
	}
	defer reopened.Close()

	for _, tabela := range []string{"note_chunks", "note_embeddings", "idx_note_chunks_filename"} {
		var n int
		if err := reopened.DB.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE name = ?`, tabela,
		).Scan(&n); err != nil {
			t.Fatalf("consultar sqlite_master (%s): %v", tabela, err)
		}
		if n != 0 {
			t.Errorf("%s deveria ter sido removida pela migração v23 (encontrada %d vez(es))", tabela, n)
		}
	}

	for _, chave := range []string{
		"ollama_url", "ollama_model", "semantic_search_threshold",
		"hybrid_semantic_threshold", "rrf_k", "semantic_device", "embedding_model_version",
	} {
		v, err := reopened.GetSetting(chave)
		if err != nil {
			t.Fatalf("GetSetting(%s): %v", chave, err)
		}
		if v != "" {
			t.Errorf("setting órfã %q deveria ter sido apagada, got %q", chave, v)
		}
	}

	// A migração é idempotente: reabrir de novo não pode falhar nem recriar nada.
	reopened.Close()
	again, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore (3ª abertura): %v", err)
	}
	defer again.Close()

	// E o banco continua funcional para o que importa.
	if err := again.SaveNote("notes/nova.md", "# Nova\nconteudo", "2026-10-16T00:00:00Z"); err != nil {
		t.Fatalf("SaveNote após a migração: %v", err)
	}
}
