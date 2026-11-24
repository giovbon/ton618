package db

import (
	"database/sql"

	// ⚠️ Import TRANSITÓRIO (16/10/2026): existe apenas para conseguir DROPAR a
	// tabela virtual legada `note_embeddings` (vec0). Sem o módulo registrado o
	// SQLite recusa o `DROP TABLE` de uma tabela virtual ("no such module: vec0"),
	// e o banco ficaria com a tabela órfã para sempre.
	//
	// Este arquivo inteiro pode ser apagado (junto com o import) quando todos os
	// bancos já tiverem passado pela migração v23 — ver DECISIONS §7.
	_ "modernc.org/sqlite/vec"
)

// dropLegacyEmbeddingTables remove o índice semântico que existia no banco
// (tabela normal `note_chunks` + tabela virtual vec0 `note_embeddings`).
//
// É chamada UMA única vez, pela migração v23 (db.go). O recurso de embeddings
// foi removido do app; isto é limpeza do resíduo em bancos já existentes.
func dropLegacyEmbeddingTables(database *sql.DB) {
	// A tabela virtual vem primeiro: as shadow tables do vec0 são derivadas dela.
	database.Exec("DROP TABLE IF EXISTS note_embeddings")
	database.Exec("DROP TABLE IF EXISTS note_chunks")
	database.Exec("DROP INDEX IF EXISTS idx_note_chunks_filename")
}
