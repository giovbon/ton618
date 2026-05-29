package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Store gerencia a conexão com o banco SQLite e todas as operações.
type Store struct {
	DB *sql.DB
}

// NewStore abre (ou cria) o banco e inicializa o schema.
func NewStore(path string) (*Store, error) {
	database, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
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
	// NOTE: FTS5 table is created below with IF NOT EXISTS.
	// The old contentless FTS5 schema was already migrated in a previous version.
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
			hash        TEXT DEFAULT ''
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

	CREATE INDEX IF NOT EXISTS idx_documents_arquivo ON documents(arquivo);
	CREATE INDEX IF NOT EXISTS idx_documents_secao ON documents(secao);
	CREATE INDEX IF NOT EXISTS idx_documents_timestamp ON documents(timestamp);

	CREATE TABLE IF NOT EXISTS tasks (
		id            TEXT PRIMARY KEY,
		title         TEXT NOT NULL,
		description   TEXT DEFAULT '',
		status        TEXT DEFAULT 'pending',
		priority      TEXT DEFAULT 'normal',
		category      TEXT DEFAULT '',
		start_time    TEXT NOT NULL,
		end_time      TEXT NOT NULL,
		all_day       INTEGER DEFAULT 0,
		color         TEXT DEFAULT '',
		recurrence_id TEXT DEFAULT '',
		note_link     TEXT DEFAULT '',
		is_exception  INTEGER DEFAULT 0,
		created_at    TEXT DEFAULT '',
		updated_at    TEXT DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS task_recurrence (
		id            TEXT PRIMARY KEY,
		rule          TEXT NOT NULL,
		days_of_week  TEXT DEFAULT '',
		days_of_month TEXT DEFAULT '',
		interval      INTEGER DEFAULT 1,
		end_date      TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_start ON tasks(start_time);
	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_category ON tasks(category);
	CREATE INDEX IF NOT EXISTS idx_tasks_recurrence ON tasks(recurrence_id);
	`

	_, err := database.Exec(schema)
	if err != nil {
		return err
	}

	// Performance PRAGMAs
	database.Exec("PRAGMA synchronous=NORMAL")
	database.Exec("PRAGMA cache_size=-8000")
	database.Exec("PRAGMA temp_store=MEMORY")

	return nil
}

// Close fecha a conexão com o banco.
func (s *Store) Close() error {
	return s.DB.Close()
}
