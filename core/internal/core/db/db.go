package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	dbgen "ton618/core/internal/core/db/generated"
	"ton618/core/internal/processor"

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
	// (notes, documents/FTS, links/tags, todos, file_metadata, popularity e as
	// transações cross-domain ReplaceFileIndexes/DeleteAllFileRecords). Dois
	// domínios totalmente independentes — settings/notificações e appointmenos —
	// nunca aparecem nessas transações, então têm locks próprios e NÃO bloqueiam
	// nem são bloqueados pelo restante. Isso permite, ex: salvar uma nota
	// (WriteMu) em paralelo com um save de configuração (settingsMu) ou um
	// update de agenda (appointmentsMu).
	settingsMu     sync.Mutex
	appointmentsMu sync.Mutex
	// dayPlanMu protege a tabela day_activities (Rotina), outro domínio
	// independente do núcleo: criar/editar/excluir uma atividade nunca participa
	// das transações de notas/FTS, então não bloqueia nem é bloqueado pelo
	// WriteMu.
	dayPlanMu sync.Mutex

	// cancelMu + queryPending registram os cancel() dos contextos criados por
	// queryCtx(). O go vet (lostcancel) exige que o cancel não seja descartado;
	// o registro permite cancelar tudo no Close sem alterar os ~80 pontos de
	// chamada que usam s.queryCtx() inline.
	cancelMu     sync.Mutex
	queryPending []pendingCancel

	// markerColsOnce/markerCols memoizam o PRAGMA table_info(todo_markers).
	// As colunas opcionais (sort_order, count_in_badge) eram consultadas por
	// PRAGMA em CADA chamada de GetTodoMarkers/GetActiveTodoMarkers/
	// SaveTodoMarkers — e o badge do cabeçalho chama GetActiveTodoMarkers a cada
	// carga de página. As migrações rodam em NewStore, então o resultado não
	// muda depois do boot.
	markerColsOnce sync.Once
	markerCols     map[string]bool
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
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

// NewReaderStore abre um banco EXISTENTE em modo somente leitura, sem backup
// pré-migração e sem aplicar migrations. É o construtor para consumidores que
// não podem alterar o acervo (ex.: o servidor MCP) e ainda assim precisam das
// mesmas queries do Store.
//
// Diferenças em relação a NewStore:
//   - falha cedo com mensagem clara quando o arquivo não existe (NewStore cria);
//   - usa mode=ro, então qualquer tentativa de escrita é rejeitada pelo SQLite;
//   - não roda initSchema nem backupBeforeMigration.
func NewReaderStore(path string) (*Store, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("db inexistente %q: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("db %q é um diretório, não um arquivo SQLite", path)
	}

	dsn := "file:" + path + "?mode=ro&_busy_timeout=5000"
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db open (read-only): %w", err)
	}
	database.SetMaxOpenConns(8)
	database.SetMaxIdleConns(4)

	// sql.Open é lazy: sem Ping o erro de arquivo/DSN só apareceria na primeira
	// query, já dentro de uma tool MCP. Falhar no boot é mais fácil de diagnosticar.
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("db open (read-only) %q: %w", path, err)
	}

	return &Store{DB: database, Q: dbgen.New(database)}, nil
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
	CREATE VIRTUAL TABLE IF NOT EXISTS docs_fts USING fts5(
		doc_id,
		tipo,
		arquivo,
		secao,
		texto,
		tags,
		note_type,
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

	CREATE TABLE IF NOT EXISTS todo_markers (
		marker     TEXT PRIMARY KEY,
		color      TEXT DEFAULT '#3b82f6',
		active     INTEGER DEFAULT 1,
		sort_order INTEGER DEFAULT 0,
		count_in_badge INTEGER DEFAULT 1
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

	CREATE TABLE IF NOT EXISTS day_activities (
		id TEXT PRIMARY KEY,
		day TEXT DEFAULT '',
		start_time TEXT DEFAULT '',
		end_time TEXT DEFAULT '',
		description TEXT DEFAULT '',
		created_at TEXT DEFAULT '',
		updated_at TEXT DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_day_activities_day ON day_activities(day);

	CREATE TABLE IF NOT EXISTS day_activity_seeds (
		day TEXT PRIMARY KEY,
		source_day TEXT DEFAULT '',
		copied INTEGER DEFAULT 0,
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

	// v3–v8 (REMOVIDAS em 16/10/2026): criavam e migravam `note_embeddings`
	// (vec0) e `note_chunks`. O recurso de embeddings foi removido do app — ver
	// migração v23 (limpeza) e DECISIONS §7.

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

	// v9b: recria tabela fts5 sem texto_stemmed (lematização removida)
	if !isApplied(22) {
		database.Exec("DROP TABLE IF EXISTS docs_fts")
		database.Exec(`
		CREATE VIRTUAL TABLE docs_fts USING fts5(
			doc_id,
			tipo,
			arquivo,
			secao,
			texto,
			tags,
			tokenize='unicode61'
		)`)
		database.Exec("DELETE FROM file_mods")
		markApplied(22)
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

	// v12: adiciona coluna count_in_badge aos marcadores de TODO.
	// Define se as tarefas DESTE marcador entram na contagem exibida no ícone
	// Task do cabeçalho (ex: DONE normalmente fica fora da contagem).
	// Default 1 = mantém o comportamento anterior (tudo conta).
	if !isApplied(12) {
		database.Exec("ALTER TABLE todo_markers ADD COLUMN count_in_badge INTEGER DEFAULT 1")
		markApplied(12)
	}

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

	// v24: campo `tipo:` — coluna note_type em documents e docs_fts, usada pelo
	// filtro `tipo:` da busca global e do Acervo (vocabulário PT snake_case:
	// mapa_mental, anexo, nota, ...). A tabela virtual FTS5 não aceita ALTER
	// TABLE ADD COLUMN: docs_fts é dropada e recriada com a coluna nova. O
	// DELETE FROM file_mods força a reindexação completa de arquivos (notas
	// também são reindexadas: SyncDatabaseWithContext reprocessa as que estão
	// sem file_mods) — é o que preenche note_type em todas as linhas; junto,
	// EnsureNoteTypeField faz o backfill do frontmatter das notas.
	if !isApplied(24) {
		database.Exec("ALTER TABLE documents ADD COLUMN note_type TEXT DEFAULT ''")
		database.Exec("DROP TABLE IF EXISTS docs_fts")
		database.Exec(`
		CREATE VIRTUAL TABLE docs_fts USING fts5(
			doc_id,
			tipo,
			arquivo,
			secao,
			texto,
			tags,
			note_type,
			tokenize='unicode61'
		)`)
		database.Exec("DELETE FROM file_mods")
		markApplied(24)
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
	// CountInBadge define se as tarefas deste marcador entram na contagem do
	// ícone Task no cabeçalho. Default true (marcadores antigos contam).
	CountInBadge bool `json:"count_in_badge"`
}

// GetTodoMarkers retorna todos os marcadores configurados.
// Ordenação: marcadores com sort_order > 0 primeiro (por sort_order ASC),
// depois os sem ordem definida (sort_order = 0) em ordem alfabética.
func (s *Store) GetTodoMarkers() ([]TodoMarker, error) {
	// Colunas opcionais podem não existir antes de reiniciar após a migração
	// (v10 = sort_order, v12 = count_in_badge).
	hasSortOrder := s.hasMarkerColumn("sort_order")
	hasCountColumn := s.hasMarkerColumn("count_in_badge")

	query := "SELECT marker, color, active"
	if hasSortOrder {
		query += ", COALESCE(sort_order, 0)"
	}
	if hasCountColumn {
		query += ", COALESCE(count_in_badge, 1)"
	}
	query += " FROM todo_markers"
	if hasSortOrder {
		query += `
			ORDER BY
				CASE WHEN COALESCE(sort_order, 0) = 0 THEN 1 ELSE 0 END,
				COALESCE(sort_order, 0) ASC,
				marker ASC`
	} else {
		query += " ORDER BY marker"
	}

	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var markers []TodoMarker
	for rows.Next() {
		m, err := scanTodoMarker(rows, hasSortOrder, hasCountColumn)
		if err != nil {
			continue
		}
		markers = append(markers, m)
	}
	return markers, rows.Err()
}

// GetActiveTodoMarkers retorna apenas os marcadores ativos, respeitando sort_order.
func (s *Store) GetActiveTodoMarkers() ([]TodoMarker, error) {
	hasSortOrder := s.hasMarkerColumn("sort_order")
	hasCountColumn := s.hasMarkerColumn("count_in_badge")

	query := "SELECT marker, color, active"
	if hasSortOrder {
		query += ", COALESCE(sort_order, 0)"
	}
	if hasCountColumn {
		query += ", COALESCE(count_in_badge, 1)"
	}
	query += " FROM todo_markers WHERE active = 1"
	if hasSortOrder {
		query += `
			ORDER BY
				CASE WHEN COALESCE(sort_order, 0) = 0 THEN 1 ELSE 0 END,
				COALESCE(sort_order, 0) ASC,
				marker ASC`
	} else {
		query += " ORDER BY marker"
	}

	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var markers []TodoMarker
	for rows.Next() {
		m, err := scanTodoMarker(rows, hasSortOrder, hasCountColumn)
		if err != nil {
			continue
		}
		m.Active = true
		markers = append(markers, m)
	}
	return markers, rows.Err()
}

// scanTodoMarker lê uma linha de todo_markers respeitando as colunas opcionais.
func scanTodoMarker(rows *sql.Rows, hasSortOrder, hasCountColumn bool) (TodoMarker, error) {
	var m TodoMarker
	var active int
	dest := []any{&m.Marker, &m.Color, &active}
	if hasSortOrder {
		dest = append(dest, &m.SortOrder)
	}
	if hasCountColumn {
		dest = append(dest, &m.CountInBadge)
	}
	if err := rows.Scan(dest...); err != nil {
		return m, err
	}
	m.Active = active == 1
	return m, nil
}

// hasMarkerColumn verifica via PRAGMA se uma coluna existe na tabela todo_markers.
// Necessário para retrocompatibilidade com bancos criados antes das migrações
// que adicionaram colunas (v10 = sort_order, v12 = count_in_badge).
//
// O resultado é memoizado por Store: as migrações rodam em NewStore, antes de
// qualquer query, então a lista de colunas não muda durante a vida do processo.
func (s *Store) hasMarkerColumn(name string) bool {
	s.markerColsOnce.Do(func() {
		s.markerCols = map[string]bool{}
		rows, err := s.DB.Query("PRAGMA table_info(todo_markers)")
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var cid int
			var colName, colType string
			var notNull int
			var dfltValue sql.NullString
			var pk int
			if rows.Scan(&cid, &colName, &colType, &notNull, &dfltValue, &pk) == nil {
				s.markerCols[colName] = true
			}
		}
	})
	return s.markerCols[name]
}

// SaveTodoMarkers substitui todos os marcadores pelos fornecidos.
func (s *Store) SaveTodoMarkers(markers []TodoMarker) error {
	hasSortOrder := s.hasMarkerColumn("sort_order")
	hasCountColumn := s.hasMarkerColumn("count_in_badge")

	cols := "marker, color, active"
	placeholders := "?, ?, ?"
	if hasSortOrder {
		cols += ", sort_order"
		placeholders += ", ?"
	}
	if hasCountColumn {
		cols += ", count_in_badge"
		placeholders += ", ?"
	}

	return s.RunInTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM todo_markers"); err != nil {
			return err
		}

		for _, m := range markers {
			active := 0
			if m.Active {
				active = 1
			}
			countInBadge := 0
			if m.CountInBadge {
				countInBadge = 1
			}

			args := []any{m.Marker, m.Color, active}
			if hasSortOrder {
				args = append(args, m.SortOrder)
			}
			if hasCountColumn {
				args = append(args, countInBadge)
			}

			if _, err := tx.Exec(
				"INSERT OR REPLACE INTO todo_markers ("+cols+") VALUES ("+placeholders+")",
				args...,
			); err != nil {
				return err
			}
		}

		return nil
	})
}

// TodoMarkerUpdate descreve uma alteração pontual de um marcador.
// Campo nil = "não mexe neste campo".
type TodoMarkerUpdate struct {
	Active       *bool
	CountInBadge *bool
	Color        *string
	SortOrder    *int
}

// UpdateTodoMarker atualiza APENAS os campos informados, em um único UPDATE.
// Retorna false quando o marcador não existe (o handler responde 404 em vez de
// fingir que atualizou).
//
// Por que não usar SaveTodoMarkers: o fluxo antigo dos handlers era
// ler a tabela inteira, alterar um item em memória e regravar tudo (DELETE +
// INSERT). Isso tinha dois defeitos reais: (1) duas requisições concorrentes
// (dois cliques rápidos no painel de marcadores) podiam perder uma das
// alterações — read-modify-write fora do WriteMu; (2) qualquer erro de leitura
// que passasse batido fazia o save gravar uma lista incompleta, apagando a
// configuração de marcadores do usuário.
func (s *Store) UpdateTodoMarker(marker string, u TodoMarkerUpdate) (bool, error) {
	marker = strings.TrimSpace(marker)
	if marker == "" {
		return false, fmt.Errorf("update todo marker: marcador vazio")
	}

	var set []string
	var args []any

	if u.Active != nil {
		set = append(set, "active = ?")
		args = append(args, boolToInt(*u.Active))
	}
	if u.Color != nil {
		set = append(set, "color = ?")
		args = append(args, *u.Color)
	}
	if u.SortOrder != nil {
		if !s.hasMarkerColumn("sort_order") {
			return false, fmt.Errorf("update todo marker: coluna sort_order ausente")
		}
		set = append(set, "sort_order = ?")
		args = append(args, *u.SortOrder)
	}
	if u.CountInBadge != nil {
		if !s.hasMarkerColumn("count_in_badge") {
			return false, fmt.Errorf("update todo marker: coluna count_in_badge ausente")
		}
		set = append(set, "count_in_badge = ?")
		args = append(args, boolToInt(*u.CountInBadge))
	}

	if len(set) == 0 {
		return false, fmt.Errorf("update todo marker: nenhum campo informado")
	}

	args = append(args, marker)

	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	res, err := s.DB.Exec("UPDATE todo_markers SET "+strings.Join(set, ", ")+" WHERE marker = ?", args...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// AddTodoMarker cria o marcador se ele ainda não existir.
// Retorna false quando já existia (idempotente — a UI pode reenviar).
func (s *Store) AddTodoMarker(m TodoMarker) (bool, error) {
	m.Marker = strings.TrimSpace(m.Marker)
	if m.Marker == "" {
		return false, fmt.Errorf("add todo marker: marcador vazio")
	}

	cols := "marker, color, active"
	placeholders := "?, ?, ?"
	args := []any{m.Marker, m.Color, boolToInt(m.Active)}

	if s.hasMarkerColumn("sort_order") {
		cols += ", sort_order"
		placeholders += ", ?"
		args = append(args, m.SortOrder)
	}
	if s.hasMarkerColumn("count_in_badge") {
		cols += ", count_in_badge"
		placeholders += ", ?"
		args = append(args, boolToInt(m.CountInBadge))
	}

	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	res, err := s.DB.Exec("INSERT OR IGNORE INTO todo_markers ("+cols+") VALUES ("+placeholders+")", args...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// RemoveTodoMarker apaga um marcador. Retorna false quando ele não existia.
// A configuração dos demais marcadores não é tocada.
func (s *Store) RemoveTodoMarker(marker string) (bool, error) {
	marker = strings.TrimSpace(marker)
	if marker == "" {
		return false, fmt.Errorf("remove todo marker: marcador vazio")
	}

	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	res, err := s.DB.Exec("DELETE FROM todo_markers WHERE marker = ?", marker)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Close fecha a conexão com o banco.
func (s *Store) Close() error {
	// Libera timers de timeout ainda pendentes antes de fechar o pool.
	s.cancelPendingQueries()
	return s.DB.Close()
}
