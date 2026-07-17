package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	dbgen "ton618/internal/core/db/generated"
	"ton618/internal/processor"

	_ "modernc.org/sqlite"
	_ "modernc.org/sqlite/vec"
)

// defaultQueryTimeout é o timeout padrão para queries SQL individuais.
// Previne que operações no SQLite travaram indefinidamente.
const defaultQueryTimeout = 30 * time.Second

// requestCtxKey is the context key for storing a context in the Store.
type requestCtxKeyType struct{}

// StoreKey is the context key for retrieving the Store from an HTTP request context.
var StoreKey requestCtxKeyType

// Store gerencia a conexão com o banco SQLite e todas as operações.
type Store struct {
	DB      *sql.DB
	Q       *dbgen.Queries
	WriteMu sync.Mutex
}

// WithRequestContext retorna uma cópia superficial do Store que usará o
// contexto fornecido (ex: contexto HTTP da request) em vez de Background().
// Como Store é compartilhado entre requisições, cada middleware cria uma cópia
// com o contexto da request atual. O WriteMu é compartilhado (ponteiro), então
// a serialização de escritas continua funcionando.
func (s *Store) WithRequestContext(ctx context.Context) *Store {
	cp := &Store{DB: s.DB, Q: s.Q, WriteMu: s.WriteMu}
	// Armazena o contexto da request para que queryCtx() o encontre
	_ = cp // Na prática, a Store é passada via context.WithValue no middleware
	return cp
}

// queryCtx retorna um contexto com timeout padrão para queries individuais.
// O cancel é ignorado intencionalmente — o timeout auto-cancela.
func (s *Store) queryCtx() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), defaultQueryTimeout)
	return ctx
}

// QueryWithCtx retorna um contexto com timeout, preferindo o contexto da request
// se fornecido. Use esta função quando um contexto HTTP estiver disponível.
func (s *Store) QueryWithCtx(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return s.queryCtx()
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
	database, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}

	database.SetMaxOpenConns(16) // SQLite concurrent reads with WAL
	database.SetMaxIdleConns(4)

	if err := initSchema(database); err != nil {
		database.Close()
		return nil, fmt.Errorf("db schema: %w", err)
	}

	q := dbgen.New(database)

	return &Store{DB: database, Q: q}, nil
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
		texto_stemmed,
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
		marker     TEXT PRIMARY KEY,
		color      TEXT DEFAULT '#3b82f6',
		active     INTEGER DEFAULT 1,
		sort_order INTEGER DEFAULT 0
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

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS notifications_log (
		id TEXT PRIMARY KEY,
		type TEXT,
		sent_at TEXT
	);

	-- Tabela de chunks de notas para busca semântica
	CREATE TABLE IF NOT EXISTS note_chunks (
		chunk_id    TEXT PRIMARY KEY,
		filename    TEXT NOT NULL,
		chunk_index INTEGER NOT NULL,
		content     TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_note_chunks_filename ON note_chunks(filename);

	-- sqlite-vec: tabela virtual para busca semântica por similaridade de vetores
	-- Cada chunk tem seu próprio embedding, referenciado por chunk_id (ex: "notes/foo.md#0")
	CREATE VIRTUAL TABLE IF NOT EXISTS note_embeddings USING vec0(
		chunk_id TEXT PRIMARY KEY,
		embedding FLOAT[384]
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
// Cada migração tem um número de versão e só é executada se ainda não foi registrada
// na tabela schema_versions. Isso substitui o padrão antigo de "ALTER TABLE + ignorar erro".
func migrate(database *sql.DB) {
	// Cria a tabela de controle de versões se não existir
	database.Exec(`CREATE TABLE IF NOT EXISTS schema_versions (
		version   INTEGER PRIMARY KEY,
		applied_at TEXT DEFAULT (datetime('now'))
	)`)

	// isApplied verifica se uma versão já foi executada
	isApplied := func(v int) bool {
		var count int
		database.QueryRow("SELECT COUNT(*) FROM schema_versions WHERE version = ?", v).Scan(&count)
		return count > 0
	}

	// markApplied registra uma versão como executada
	markApplied := func(v int) {
		database.Exec("INSERT OR IGNORE INTO schema_versions (version) VALUES (?)", v)
	}

	// v1: adiciona coluna keywords à tabela notes
	if !isApplied(1) {
		if _, err := database.Exec("ALTER TABLE notes ADD COLUMN keywords TEXT DEFAULT ''"); err != nil {
			// coluna já existe — ignorado (migração já foi aplicada manualmente)
		}
		markApplied(1)
	}

	// v2: adiciona campos RLHF na tabela popularity
	if !isApplied(2) {
		database.Exec("ALTER TABLE popularity ADD COLUMN weight REAL DEFAULT 1.0")
		database.Exec("ALTER TABLE popularity ADD COLUMN last_interacted_at TEXT DEFAULT ''")
		markApplied(2)
	}

	// v3: cria tabela virtual sqlite-vec para embeddings semânticos
	if !isApplied(3) {
		database.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS note_embeddings USING vec0(
			filename TEXT PRIMARY KEY,
			embedding FLOAT[384]
		)`)
		// Cria tabela note_chunks
		database.Exec(`CREATE TABLE IF NOT EXISTS note_chunks (
			chunk_id    TEXT PRIMARY KEY,
			filename    TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			content     TEXT NOT NULL
		)`)
		database.Exec("CREATE INDEX IF NOT EXISTS idx_note_chunks_filename ON note_chunks(filename)")
		markApplied(3)
	}

	// v4: limpa embeddings legados de notas não-indexáveis
	if !isApplied(4) {
		database.Exec(`
			DELETE FROM note_embeddings
			WHERE filename LIKE '%mapa-%' 
			   OR filename LIKE '%mapa.%' 
			   OR filename LIKE '%.map'
			   OR filename IN (
			       SELECT arquivo FROM tags 
			       WHERE tag IN ('desenho', 'drawing', 'mapa', 'map', 'planilha', 'spreadsheet', 'mermaid')
			   )
		`)
		markApplied(4)
	}

	// v5: remove duplicatas de embeddings
	if !isApplied(5) {
		if rows, err := database.Query("SELECT filename, COUNT(*) as c FROM note_embeddings GROUP BY filename HAVING c > 1"); err == nil {
			var dupFiles []string
			for rows.Next() {
				var filename string
				var count int
				if err := rows.Scan(&filename, &count); err == nil {
					dupFiles = append(dupFiles, filename)
				}
			}
			rows.Close()
			for _, filename := range dupFiles {
				database.Exec("DELETE FROM note_embeddings WHERE filename = ?", filename)
			}
		}
		markApplied(5)
	}

	// v6: migra note_embeddings de filename PK para chunk_id PK
	if !isApplied(6) {
		database.Exec("DROP TABLE IF EXISTS note_embeddings")
		database.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS note_embeddings USING vec0(
			chunk_id TEXT PRIMARY KEY,
			embedding FLOAT[384]
		)`)
		markApplied(6)
	}

	// v7: adiciona coluna indexed_mtime à tabela note_chunks
	if !isApplied(7) {
		database.Exec("ALTER TABLE note_chunks ADD COLUMN indexed_mtime TEXT DEFAULT ''")
		markApplied(7)
	}

	// v8: remove chunks e embeddings orfãos (de notas que foram deletadas)
	if !isApplied(8) {
		database.Exec(`
			DELETE FROM note_chunks
			WHERE filename NOT IN (SELECT filename FROM notes)
		`)
		database.Exec(`
			DELETE FROM note_embeddings
			WHERE chunk_id NOT IN (SELECT chunk_id FROM note_chunks)
		`)
		markApplied(8)
	}

	// v9: recria tabela virtual fts5 para incluir texto_stemmed e limpa file_mods para forçar reindexação completa
	if !isApplied(9) {
		database.Exec("DROP TABLE IF EXISTS docs_fts")
		database.Exec(`
		CREATE VIRTUAL TABLE docs_fts USING fts5(
			doc_id,
			tipo,
			arquivo,
			secao,
			texto,
			tags,
			texto_stemmed,
			tokenize='unicode61'
		)`)
		database.Exec("DELETE FROM file_mods")
		markApplied(9)
	}

	// v10: adiciona coluna sort_order aos marcadores de TODO (0 = sem ordem definida)
	if !isApplied(10) {
		database.Exec("ALTER TABLE todo_markers ADD COLUMN sort_order INTEGER DEFAULT 0")
		markApplied(10)
	}

	// v11: atualiza marcadores padrão para ser apenas TODO, DOING, e DONE, e limpa indexação/todos antigos
	if !isApplied(11) {
		database.Exec("DELETE FROM todo_markers")
		for _, m := range processor.DefaultTodoMarkers {
			active := 0
			if m.Active {
				active = 1
			}
			database.Exec(
				"INSERT OR IGNORE INTO todo_markers (marker, color, active, sort_order) VALUES (?, ?, ?, 0)",
				m.Marker, m.Color, active,
			)
		}
		database.Exec("DELETE FROM todos")
		database.Exec("DELETE FROM file_mods")
		markApplied(11)
	}
}

// seedDefaultMarkers insere marcadores padrão se a tabela estiver vazia.
func seedDefaultMarkers(database *sql.DB) {
	var count int
	database.QueryRow("SELECT COUNT(*) FROM todo_markers").Scan(&count)
	if count > 0 {
		return
	}
	for _, m := range processor.DefaultTodoMarkers {
		active := 0
		if m.Active {
			active = 1
		}
		database.Exec(
			"INSERT OR IGNORE INTO todo_markers (marker, color, active) VALUES (?, ?, ?)",
			m.Marker, m.Color, active,
		)
	}
}

// ── Todo Markers ──

type TodoMarker struct {
	Marker    string `json:"marker"`
	Color     string `json:"color"`
	Active    bool   `json:"active"`
	SortOrder int    `json:"sort_order"` // 0 = sem ordem definida (padrão)
}

// GetTodoMarkers retorna todos os marcadores configurados.
// Ordenação: marcadores com sort_order > 0 primeiro (por sort_order ASC),
// depois os sem ordem definida (sort_order = 0) em ordem alfabética.
func (s *Store) GetTodoMarkers() ([]TodoMarker, error) {
	// Verifica se a coluna sort_order existe (pode não existir antes de reiniciar após migração v10)
	hasSortOrder := s.hasSortOrderColumn()

	var rows *sql.Rows
	var err error
	if hasSortOrder {
		rows, err = s.DB.Query(`
			SELECT marker, color, active, COALESCE(sort_order, 0)
			FROM todo_markers
			ORDER BY
				CASE WHEN COALESCE(sort_order, 0) = 0 THEN 1 ELSE 0 END,
				COALESCE(sort_order, 0) ASC,
				marker ASC
		`)
	} else {
		rows, err = s.DB.Query("SELECT marker, color, active FROM todo_markers ORDER BY marker")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var markers []TodoMarker
	for rows.Next() {
		var m TodoMarker
		var active int
		if hasSortOrder {
			if err := rows.Scan(&m.Marker, &m.Color, &active, &m.SortOrder); err != nil {
				continue
			}
		} else {
			if err := rows.Scan(&m.Marker, &m.Color, &active); err != nil {
				continue
			}
		}
		m.Active = active == 1
		markers = append(markers, m)
	}
	return markers, rows.Err()
}

// GetActiveTodoMarkers retorna apenas os marcadores ativos, respeitando sort_order.
func (s *Store) GetActiveTodoMarkers() ([]TodoMarker, error) {
	hasSortOrder := s.hasSortOrderColumn()

	var rows *sql.Rows
	var err error
	if hasSortOrder {
		rows, err = s.DB.Query(`
			SELECT marker, color, active, COALESCE(sort_order, 0)
			FROM todo_markers
			WHERE active = 1
			ORDER BY
				CASE WHEN COALESCE(sort_order, 0) = 0 THEN 1 ELSE 0 END,
				COALESCE(sort_order, 0) ASC,
				marker ASC
		`)
	} else {
		rows, err = s.DB.Query("SELECT marker, color, active FROM todo_markers WHERE active = 1 ORDER BY marker")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var markers []TodoMarker
	for rows.Next() {
		var m TodoMarker
		var active int
		if hasSortOrder {
			if err := rows.Scan(&m.Marker, &m.Color, &active, &m.SortOrder); err != nil {
				continue
			}
		} else {
			if err := rows.Scan(&m.Marker, &m.Color, &active); err != nil {
				continue
			}
		}
		m.Active = true
		markers = append(markers, m)
	}
	return markers, rows.Err()
}

// hasSortOrderColumn verifica via PRAGMA se a coluna sort_order existe na tabela todo_markers.
// Necessário para retrocompatibilidade com bancos antes da migração v10.
func (s *Store) hasSortOrderColumn() bool {
	rows, err := s.DB.Query("PRAGMA table_info(todo_markers)")
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk) == nil {
			if name == "sort_order" {
				return true
			}
		}
	}
	return false
}

// SaveTodoMarkers substitui todos os marcadores pelos fornecidos.
func (s *Store) SaveTodoMarkers(markers []TodoMarker) error {
	hasSortOrder := s.hasSortOrderColumn()

	return s.RunInTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM todo_markers"); err != nil {
			return err
		}

		for _, m := range markers {
			active := 0
			if m.Active {
				active = 1
			}
			if hasSortOrder {
				if _, err := tx.Exec(
					"INSERT OR REPLACE INTO todo_markers (marker, color, active, sort_order) VALUES (?, ?, ?, ?)",
					m.Marker, m.Color, active, m.SortOrder,
				); err != nil {
					return err
				}
			} else {
				if _, err := tx.Exec(
					"INSERT OR REPLACE INTO todo_markers (marker, color, active) VALUES (?, ?, ?)",
					m.Marker, m.Color, active,
				); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// Close fecha a conexão com o banco.
func (s *Store) Close() error {
	return s.DB.Close()
}
