package db

import "database/sql"

// ── Metadados externos por arquivo ──────────────────────────────────────
//
// Notas markdown guardam propriedades no próprio frontmatter. ARQUIVOS
// (PDF/EPUB/ZIP/...) são binários e não têm onde guardar nada, então o `pai:`
// deles vive na tabela file_metadata — chaveada pelo caminho do arquivo, o
// mesmo padrão já usado pela tabela `tags` (ver DECISIONS §6.24).
//
// SQL direto (como BatchGetNotesContent em notes.go) em vez de sqlc: mantém a
// feature autocontida e não exige regenerar o pacote dbgen.

// SetFileMetadata grava (upsert) um metadado de arquivo. Valor vazio REMOVE a
// chave — é assim que a UI limpa o `pai` (o arquivo volta para a raiz).
func (s *Store) SetFileMetadata(arquivo, key, value string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	if value == "" {
		_, err := s.DB.ExecContext(s.queryCtx(), `DELETE FROM file_metadata WHERE arquivo = ? AND key = ?`, arquivo, key)
		return err
	}
	_, err := s.DB.ExecContext(s.queryCtx(),
		`INSERT INTO file_metadata (arquivo, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(arquivo, key) DO UPDATE SET value = excluded.value`,
		arquivo, key, value)
	return err
}

// GetFileMetadata devolve todos os metadados de um arquivo (mapa key→value).
// Arquivo sem metadados devolve um mapa vazio (nunca nil).
func (s *Store) GetFileMetadata(arquivo string) (map[string]string, error) {
	rows, err := s.DB.QueryContext(s.queryCtx(), `SELECT key, value FROM file_metadata WHERE arquivo = ?`, arquivo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v sql.NullString
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k.String] = v.String
	}
	return out, rows.Err()
}

// GetAllFileMetadata devolve, por arquivo, todos os metadados. Usado pelo
// Tabulator (injeção nas linhas), pela contagem de filhas da sidebar e pela
// propagação de rename.
func (s *Store) GetAllFileMetadata() (map[string]map[string]string, error) {
	rows, err := s.DB.QueryContext(s.queryCtx(), `SELECT arquivo, key, value FROM file_metadata`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]map[string]string)
	for rows.Next() {
		var arquivo, k, v sql.NullString
		if err := rows.Scan(&arquivo, &k, &v); err != nil {
			return nil, err
		}
		if out[arquivo.String] == nil {
			out[arquivo.String] = make(map[string]string)
		}
		out[arquivo.String][k.String] = v.String
	}
	return out, rows.Err()
}

// DeleteFileMetadata remove todos os metadados de um arquivo.
func (s *Store) DeleteFileMetadata(arquivo string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	_, err := s.DB.ExecContext(s.queryCtx(), `DELETE FROM file_metadata WHERE arquivo = ?`, arquivo)
	return err
}
