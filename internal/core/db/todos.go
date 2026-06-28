package db

import (
	"database/sql"
	"strings"
	"time"
	"ton618/internal/processor"
)

// DeleteTodosByFile remove todos os TODOs associados a um arquivo específico.
func (s *Store) DeleteTodosByFile(filename string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()
	_, err := s.DB.Exec("DELETE FROM todos WHERE file = ?", filename)
	return err
}

// SaveFileTodos exclui os TODOs antigos de um arquivo e insere os novos em lote.
func (s *Store) SaveFileTodos(filename string, todos []processor.TodoItem) error {
	return s.RunInTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM todos WHERE file = ?", filename); err != nil {
			return err
		}

		if len(todos) == 0 {
			return nil
		}

		stmt, err := tx.Prepare("INSERT INTO todos (id, file, section, type, status, text, line, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, t := range todos {
			createdStr := t.Created.UTC().Format(time.RFC3339)
			if _, err := stmt.Exec(t.ID, t.File, t.Section, t.Type, t.Status, t.Text, t.Line, createdStr); err != nil {
				return err
			}
		}

		return nil
	})
}

// GetTodosFiltered retorna os TODOs baseados nos filtros fornecidos.
func (s *Store) GetTodosFiltered(typeFilter map[string]bool, statusFilter string) ([]processor.TodoItem, error) {
	query := "SELECT id, file, section, type, status, text, line, created_at FROM todos WHERE 1=1"
	var args []interface{}

	if len(typeFilter) > 0 {
		var inClause []string
		for t := range typeFilter {
			inClause = append(inClause, "?")
			args = append(args, t)
		}
		query += " AND type IN (" + strings.Join(inClause, ",") + ")"
	}

	if statusFilter != "" && statusFilter != "all" {
		query += " AND status = ?"
		args = append(args, statusFilter)
	}

	query += " ORDER BY file, line ASC"

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []processor.TodoItem
	for rows.Next() {
		var t processor.TodoItem
		var createdStr string
		if err := rows.Scan(&t.ID, &t.File, &t.Section, &t.Type, &t.Status, &t.Text, &t.Line, &createdStr); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse(time.RFC3339, createdStr); err == nil {
			t.Created = parsed
		}
		todos = append(todos, t)
	}

	return todos, rows.Err()
}
