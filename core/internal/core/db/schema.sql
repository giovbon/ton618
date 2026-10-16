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
    count   INTEGER DEFAULT 1,
    weight REAL DEFAULT 1.0,
    last_interacted_at TEXT DEFAULT ''
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
    content   TEXT DEFAULT '',
    keywords TEXT DEFAULT ''
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

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS notifications_log (
    id TEXT PRIMARY KEY,
    type TEXT,
    sent_at TEXT
);
