package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// Store gerencia a conexão com o banco SQLite e todas as operações.
type Store struct {
	DB *sql.DB
}

// NewStore abre (ou cria) o banco e inicializa o schema.
func NewStore(path string) (*Store, error) {
	database, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}

	database.SetMaxOpenConns(1) // SQLite serialized

	if err := initSchema(database); err != nil {
		database.Close()
		return nil, fmt.Errorf("db schema: %w", err)
	}

	return &Store{DB: database}, nil
}

// initSchema cria as tabelas necessárias se ainda não existirem.
func initSchema(database *sql.DB) error {
	// Drop old contentless FTS5 to recreate with column-qualified query support
	database.Exec("DROP TABLE IF EXISTS docs_fts")

	schema := `
	CREATE TABLE IF NOT EXISTS documents (
		id          TEXT PRIMARY KEY,
		tipo        TEXT DEFAULT '',
		arquivo     TEXT DEFAULT '',
		secao       TEXT DEFAULT '',
		texto       TEXT DEFAULT '',
		tags        TEXT DEFAULT '',
		pagina      INTEGER DEFAULT 0,
		ordem       INTEGER DEFAULT 0,
		timestamp   TEXT DEFAULT '',
		created_at  TEXT DEFAULT '',
		hash        TEXT DEFAULT '',
		vector_hash TEXT DEFAULT ''
	);
	`

	schema += `
	CREATE VIRTUAL TABLE IF NOT EXISTS docs_fts USING fts5(
		doc_id,
		tipo,
		arquivo,
		secao,
		texto,
		tags,
		tokenize='unicode61'
	);

	CREATE TABLE IF NOT EXISTS popularity (
		arquivo TEXT PRIMARY KEY,
		count   INTEGER DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS tags (
		arquivo TEXT NOT NULL,
		tag     TEXT NOT NULL,
		PRIMARY KEY (arquivo, tag)
	);

	CREATE TABLE IF NOT EXISTS links (
		from_file TEXT NOT NULL,
		to_file   TEXT NOT NULL,
		PRIMARY KEY (from_file, to_file)
	);

	CREATE TABLE IF NOT EXISTS file_mods (
		arquivo TEXT PRIMARY KEY,
		mtime   TEXT DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS embeddings (
		doc_id     TEXT PRIMARY KEY,
		vector     BLOB,
		title      TEXT DEFAULT '',
		x          REAL DEFAULT 0,
		y          REAL DEFAULT 0,
		created_at TEXT DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS semantic_links (
		from_file TEXT NOT NULL,
		to_file   TEXT NOT NULL,
		PRIMARY KEY (from_file, to_file)
	);

	CREATE TABLE IF NOT EXISTS semantic_topics (
		topic TEXT PRIMARY KEY
	);

	CREATE INDEX IF NOT EXISTS idx_documents_arquivo ON documents(arquivo);
	CREATE INDEX IF NOT EXISTS idx_documents_secao ON documents(secao);
	CREATE INDEX IF NOT EXISTS idx_documents_timestamp ON documents(timestamp);
	`

	_, err := database.Exec(schema)
	return err
}

// Close fecha a conexão com o banco.
func (s *Store) Close() error {
	return s.DB.Close()
}
