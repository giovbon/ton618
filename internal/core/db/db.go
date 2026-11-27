package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	dbgen "ton618/core/internal/core/db/generated"

	_ "modernc.org/sqlite"
)

// defaultQueryTimeout é o timeout padrão para queries SQL individuais.
// Previne que operações no SQLite travaram indefinidamente.
const defaultQueryTimeout = 30 * time.Second

// Store gerencia a conexão com o banco SQLite e todas as operações.
type Store struct {
	DB      *sql.DB
	Q       *dbgen.Queries
	WriteMu sync.Mutex

	// Granularidade de locks (#3): WriteMu serializa as escritas do "núcleo"
	// (notes, documents/FTS, links/tags, file_metadata, popularity e as
	// transações cross-domain ReplaceFileIndexes/DeleteAllFileRecords). O
	// domínio de settings/notificações nunca aparece nessas transações, então
	// tem lock próprio e NÃO bloqueia nem é bloqueado pelo restante. Isso
	// permite, ex: salvar uma nota (WriteMu) em paralelo com um save de
	// configuração (settingsMu).
	settingsMu sync.Mutex

	// cancelMu + queryPending registram os cancel() dos contextos criados por
	// queryCtx(). O go vet (lostcancel) exige que o cancel não seja descartado;
	// o registro permite cancelar tudo no Close sem alterar os ~80 pontos de
	// chamada que usam s.queryCtx() inline.
	cancelMu     sync.Mutex
	queryPending []pendingCancel
}

// pendingCancel guarda o par ctx/cancel de um timeout de query individual.
type pendingCancel struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// queryCtx retorna um contexto com timeout padrão para queries individuais.
// O timeout de 30s auto-cancela o contexto; o cancel() fica registrado apenas
// para liberação antecipada de timer no Close e para satisfazer o go vet.
func (s *Store) queryCtx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), defaultQueryTimeout)

	s.cancelMu.Lock()
	// Poda o registro para acompanhar a concorrência real: remove entradas cujo
	// contexto já finalizou (timeout disparado ou cancelado).
	if len(s.queryPending) >= 256 {
		kept := s.queryPending[:0]
		for _, p := range s.queryPending {
			if p.ctx.Err() == nil {
				kept = append(kept, p)
			}
		}
		s.queryPending = kept
	}
	s.queryPending = append(s.queryPending, pendingCancel{ctx: ctx, cancel: cancel})
	s.cancelMu.Unlock()

	return ctx
}

// cancelPendingQueries invoca todos os cancels registrados por queryCtx.
// Chamado no Close para liberar timers ainda pendentes.
func (s *Store) cancelPendingQueries() {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	for _, p := range s.queryPending {
		p.cancel()
	}
	s.queryPending = s.queryPending[:0]
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
	// #6: snapshot do último estado bom ANTES de aplicar migrations (se uma
	// migration falhar, o backup em <dir>/backups/pre-migration.db permite
	// restaurar). Tolerante a falha — é só uma rede de segurança.
	backupBeforeMigration(path)

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
			hash        TEXT DEFAULT '',
			note_type   TEXT DEFAULT ''
		);
	`

	schema += `
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

	-- Metadados externos por arquivo (ex: pai de PDF/EPUB/ZIP). Arquivos
	-- binarios nao tem frontmatter, entao o vinculo de hierarquia vive aqui.
	-- Chave/valor genericos para reusar em futuros metadados (ver DECISIONS 6.24).
	CREATE TABLE IF NOT EXISTS file_metadata (
		arquivo TEXT NOT NULL,
		key     TEXT NOT NULL,
		value   TEXT DEFAULT '',
		PRIMARY KEY (arquivo, key)
	);
	CREATE INDEX IF NOT EXISTS idx_file_metadata_key ON file_metadata(key);

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

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS notifications_log (
		id TEXT PRIMARY KEY,
		type TEXT,
		sent_at TEXT
	);
	`

	_, err := database.Exec(schema)
	if err != nil {
		return err
	}

	// Migrations evolutivas
	migrate(database)

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

	// v3–v8 (REMOVIDAS em 16/10/2026): criavam e migravam `note_embeddings`
	// (vec0) e `note_chunks`. O recurso de embeddings foi removido do app — ver
	// migração v23 (limpeza) e DECISIONS §7.

	// v9 e v22 (REMOVIDAS em 25/11/2026): recriavam o índice FTS5 (docs_fts).
	// A busca por conteúdo foi removida do app — a busca agora é só por título
	// (arquivo), servida pela lista de notas da sidebar. A migração v25 derruba
	// o índice legado em bancos antigos.

	// v23: remove o índice semântico (note_chunks + vec0 note_embeddings) e as
	// configurações órfãs do recurso de IA — o "modo Perguntar", a síntese via
	// Ollama e os embeddings foram removidos do app em 16/10/2026 (DECISIONS §7).
	if !isApplied(23) {
		dropLegacyEmbeddingTables(database)
		database.Exec(`DELETE FROM settings WHERE key IN (
			'embedding_model_version',
			'semantic_search_threshold',
			'hybrid_semantic_threshold',
			'rrf_k',
			'semantic_device',
			'ollama_url',
			'ollama_model'
		)`)
		markApplied(23)
	}

	// v24: campo `tipo:` — coluna note_type em documents, usada pelo filtro
	// `tipo:` do Acervo (vocabulário PT snake_case: anexo, nota, ...).
	// O DELETE FROM file_mods força a reindexação completa de arquivos
	// (notas também são reindexadas: SyncDatabaseWithContext reprocessa as que
	// estão sem file_mods) — é o que preenche note_type em todas as linhas;
	// junto, EnsureNoteTypeField faz o backfill do frontmatter das notas.
	if !isApplied(24) {
		database.Exec("ALTER TABLE documents ADD COLUMN note_type TEXT DEFAULT ''")
		database.Exec("DELETE FROM file_mods")
		markApplied(24)
	}

	// v25: remove a busca por conteúdo (índice FTS5 docs_fts) e a Rotina
	// (day_activities + day_activity_seeds) — ambas as funcionalidades foram
	// removidas do app em 25/11/2026. As configurações órfãs saem junto.
	if !isApplied(25) {
		database.Exec("DROP TABLE IF EXISTS docs_fts")
		database.Exec("DROP TABLE IF EXISTS day_activities")
		database.Exec("DROP TABLE IF EXISTS day_activity_seeds")
		database.Exec("DELETE FROM settings WHERE key IN ('rotina_enabled', 'rotina_timeline_native')")
		markApplied(25)
	}

	// v26: remove o tipo de nota Markmap (mapa mental) — o recurso foi removido
	// em 27/11/2026. As tags internas do tipo deixam de existir e o note_type
	// legado passa a "nota" (as notas continuam abrindo no editor markdown).
	if !isApplied(26) {
		database.Exec("DELETE FROM tags WHERE tag IN ('markmap', 'mindmap')")
		database.Exec("UPDATE documents SET note_type = 'nota' WHERE note_type = 'mapa_mental'")
		markApplied(26)
	}
}

// Close fecha a conexão com o banco.
func (s *Store) Close() error {
	// Libera timers de timeout ainda pendentes antes de fechar o pool.
	s.cancelPendingQueries()
	return s.DB.Close()
}
