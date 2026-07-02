package db

import (
	"database/sql"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

// Store gerencia a conexão com o banco SQLite e todas as operações.
type Store struct {
	DB      *sql.DB
	WriteMu sync.Mutex
}

// RunInTx executes the given function within a database transaction.
// It acquires the WriteMu lock to ensure exclusive write access.
func (s *Store) RunInTx(fn func(tx *sql.Tx) error) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// NewStore abre (ou cria) o banco e inicializa o schema.
func NewStore(path string) (*Store, error) {
	database, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}

	database.SetMaxOpenConns(16) // SQLite concurrent reads with WAL
	database.SetMaxIdleConns(4)

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

	CREATE TABLE IF NOT EXISTS notes (
		filename  TEXT PRIMARY KEY,
		mtime     TEXT DEFAULT '',
		content   TEXT DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_documents_arquivo ON documents(arquivo);
	CREATE INDEX IF NOT EXISTS idx_documents_secao ON documents(secao);
	CREATE INDEX IF NOT EXISTS idx_documents_timestamp ON documents(timestamp);
	CREATE INDEX IF NOT EXISTS idx_links_to_file ON links(to_file);
	CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);

	CREATE TABLE IF NOT EXISTS todo_markers (
		marker TEXT PRIMARY KEY,
		color  TEXT DEFAULT '#3b82f6',
		active INTEGER DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS todos (
		id TEXT PRIMARY KEY,
		file TEXT NOT NULL,
		section TEXT DEFAULT '',
		type TEXT DEFAULT '',
		status TEXT DEFAULT '',
		text TEXT DEFAULT '',
		line INTEGER DEFAULT 0,
		created_at TEXT DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_todos_file ON todos(file);

	CREATE TABLE IF NOT EXISTS appointments (
		id TEXT PRIMARY KEY,
		description TEXT DEFAULT '',
		event_date TEXT DEFAULT '',
		year INTEGER DEFAULT 0,
		month INTEGER DEFAULT 0,
		week_number INTEGER DEFAULT 0,
		created_at TEXT DEFAULT ''
	);
	`

	_, err := database.Exec(schema)
	if err != nil {
		return err
	}

	// Migrations evolutivas
	migrate(database)

	// Seed default markers if empty
	seedDefaultMarkers(database)

	// Performance PRAGMAs
	database.Exec("PRAGMA journal_mode=WAL")
	database.Exec("PRAGMA busy_timeout=5000")
	database.Exec("PRAGMA synchronous=NORMAL")
	database.Exec("PRAGMA cache_size=-8000")
	database.Exec("PRAGMA temp_store=MEMORY")

	// Força checkpoint do WAL ao iniciar para garantir que dados pendentes
	// de sessões anteriores sejam incorporados ao arquivo principal.
	database.Exec("PRAGMA wal_checkpoint(FULL)")

	return nil
}

// migrate aplica migrações evolutivas do schema (colunas novas, etc).
// Cada migração é idempotente — usa ALTER TABLE e ignora erro se já existir.
func migrate(database *sql.DB) {
	// v1: adiciona coluna keywords à tabela notes
	if _, err := database.Exec("ALTER TABLE notes ADD COLUMN keywords TEXT DEFAULT ''"); err != nil {
		// coluna já existe — ignorado
	}

	// v2: adiciona campos RLHF na tabela popularity
	if _, err := database.Exec("ALTER TABLE popularity ADD COLUMN weight REAL DEFAULT 1.0"); err != nil {
		// coluna já existe — ignorado
	}
	if _, err := database.Exec("ALTER TABLE popularity ADD COLUMN last_interacted_at TEXT DEFAULT ''"); err != nil {
		// coluna já existe — ignorado
	}
}

// seedDefaultMarkers insere marcadores padrão se a tabela estiver vazia.
func seedDefaultMarkers(database *sql.DB) {
	var count int
	database.QueryRow("SELECT COUNT(*) FROM todo_markers").Scan(&count)
	if count > 0 {
		return
	}
	defaults := []struct {
		marker string
		color  string
		active bool
	}{
		{"TODO", "#3b82f6", true},
		{"FIXME", "#f59e0b", true},
		{"BUG", "#ef4444", true},
		{"HACK", "#8b5cf6", false},
		{"NOTE", "#06b6d4", false},
		{"OPTIMIZE", "#10b981", false},
		{"REVIEW", "#f97316", false},
	}
	for _, m := range defaults {
		active := 0
		if m.active {
			active = 1
		}
		database.Exec(
			"INSERT OR IGNORE INTO todo_markers (marker, color, active) VALUES (?, ?, ?)",
			m.marker, m.color, active,
		)
	}
}

// ── Todo Markers ──

type TodoMarker struct {
	Marker string `json:"marker"`
	Color  string `json:"color"`
	Active bool   `json:"active"`
}

// GetTodoMarkers retorna todos os marcadores configurados.
func (s *Store) GetTodoMarkers() ([]TodoMarker, error) {
	rows, err := s.DB.Query("SELECT marker, color, active FROM todo_markers ORDER BY marker")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var markers []TodoMarker
	for rows.Next() {
		var m TodoMarker
		var active int
		if err := rows.Scan(&m.Marker, &m.Color, &active); err != nil {
			continue
		}
		m.Active = active == 1
		markers = append(markers, m)
	}
	return markers, rows.Err()
}

// GetActiveTodoMarkers retorna apenas os marcadores ativos.
func (s *Store) GetActiveTodoMarkers() ([]TodoMarker, error) {
	rows, err := s.DB.Query("SELECT marker, color, active FROM todo_markers WHERE active = 1 ORDER BY marker")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var markers []TodoMarker
	for rows.Next() {
		var m TodoMarker
		var active int
		if err := rows.Scan(&m.Marker, &m.Color, &active); err != nil {
			continue
		}
		m.Active = true
		markers = append(markers, m)
	}
	return markers, rows.Err()
}

// SaveTodoMarkers substitui todos os marcadores pelos fornecidos.
func (s *Store) SaveTodoMarkers(markers []TodoMarker) error {
	return s.RunInTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM todo_markers"); err != nil {
			return err
		}

		stmt, err := tx.Prepare("INSERT INTO todo_markers (marker, color, active) VALUES (?, ?, ?)")
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, m := range markers {
			active := 0
			if m.Active {
				active = 1
			}
			if _, err := stmt.Exec(m.Marker, m.Color, active); err != nil {
				return err
			}
		}

		return nil
	})
}

// Close fecha a conexão com o banco.
func (s *Store) Close() error {
	return s.DB.Close()
}
