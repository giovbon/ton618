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

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS notifications_log (
    id TEXT PRIMARY KEY,
    type TEXT,
    sent_at TEXT
);
