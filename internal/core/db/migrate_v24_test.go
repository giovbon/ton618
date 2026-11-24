package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigrateV24_UpgradeDeBancoAntigo simula um banco criado ANTES da migração
// v24 (documents sem note_type, docs_fts sem a coluna, dados e file_mods
// presentes) e verifica o upgrade: coluna adicionada, FTS recriado com a
// coluna, file_mods limpo (força reindex completa) e versão registrada.
func TestMigrateV24_UpgradeDeBancoAntigo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "antigo.db")

	// 1. Monta o schema ANTIGO à mão (pré-v24) com uma nota e um file_mod.
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	stmts := []string{
		`CREATE TABLE documents (
			id TEXT PRIMARY KEY, tipo TEXT DEFAULT '', arquivo TEXT DEFAULT '',
			secao TEXT DEFAULT '', texto TEXT DEFAULT '', tags TEXT DEFAULT '',
			pagina INTEGER DEFAULT 0, ordem INTEGER DEFAULT 0,
			timestamp TEXT DEFAULT '', created_at TEXT DEFAULT '', hash TEXT DEFAULT ''
		)`,
		`CREATE VIRTUAL TABLE docs_fts USING fts5(
			doc_id, tipo, arquivo, secao, texto, tags, tokenize='unicode61'
		)`,
		`CREATE TABLE file_mods (arquivo TEXT PRIMARY KEY, mtime TEXT DEFAULT '')`,
		`INSERT INTO documents (id, tipo, arquivo, texto) VALUES ('doc-1', 'markdown', 'notes/velha.md', 'conteudo antigo')`,
		`INSERT INTO docs_fts (doc_id, tipo, arquivo, secao, texto, tags) VALUES ('doc-1', 'markdown', 'notes/velha.md', 'Geral', 'conteudo antigo', '')`,
		`INSERT INTO file_mods (arquivo, mtime) VALUES ('notes/velha.md', '2024-01-01T00:00:00Z')`,
	}
	for _, stmt := range stmts {
		if _, err := old.Exec(stmt); err != nil {
			t.Fatalf("setup schema antigo (%s): %v", stmt, err)
		}
	}
	old.Close()

	// 2. Abre com NewStore — roda initSchema (no-op) + migrate (v24).
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore (upgrade): %v", err)
	}
	defer s.Close()

	// 3. documents ganhou a coluna e preservou os dados (note_type vazio,
	// aguardando a reindexação/backfill).
	var noteType string
	if err := s.DB.QueryRow(`SELECT note_type FROM documents WHERE id = 'doc-1'`).Scan(&noteType); err != nil {
		t.Fatalf("documents.note_type após migração: %v", err)
	}
	if noteType != "" {
		t.Errorf("note_type deveria começar vazio, got %q", noteType)
	}

	// 4. docs_fts foi recriada COM a coluna (consulta parseável; linhas antigas
	// descartadas — o índice é reconstruído pela reindexação forçada).
	var total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM docs_fts WHERE note_type = ''`).Scan(&total); err != nil {
		t.Fatalf("docs_fts.note_type após migração: %v", err)
	}
	if total != 0 {
		t.Errorf("docs_fts deveria estar vazia após recriação, got %d linhas", total)
	}

	// 5. file_mods limpo ⇒ SyncDatabase/ScanAndIndexAll reindexam tudo no boot.
	var mods int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM file_mods`).Scan(&mods); err != nil {
		t.Fatalf("file_mods: %v", err)
	}
	if mods != 0 {
		t.Errorf("file_mods deveria estar vazio pós-migração, got %d", mods)
	}

	// 6. Versão registrada (idempotência: segunda abertura não re-executa).
	var applied int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM schema_versions WHERE version = 24`).Scan(&applied); err != nil {
		t.Fatalf("schema_versions: %v", err)
	}
	if applied != 1 {
		t.Errorf("migração v24 deveria estar registrada uma vez, got %d", applied)
	}

	// 7. Derivação de note_type em InsertDocument (caminho → vocabulário PT).
	if err := s.InsertDocument(Document{ID: "doc-pdf", Tipo: "pdf", Arquivo: "pdfs/manual.pdf", Texto: "x"}); err != nil {
		t.Fatalf("InsertDocument: %v", err)
	}
	var pdfType string
	if err := s.DB.QueryRow(`SELECT note_type FROM documents WHERE id = 'doc-pdf'`).Scan(&pdfType); err != nil {
		t.Fatalf("select note_type doc-pdf: %v", err)
	}
	if pdfType != "pdf" {
		t.Errorf("note_type de pdfs/manual.pdf = %q, want \"pdf\"", pdfType)
	}
}
