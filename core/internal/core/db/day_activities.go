package db

import (
	"database/sql"
	"errors"
	"time"

	"ton618/core/internal/core/domain"
	"ton618/core/internal/processor"
)

// ── Rotina: atividades do dia (day_activities) ──
//
// SQL cru de propósito: sqlc não está disponível no ambiente de build e a
// tabela é pequena. Todas as escritas usam dayPlanMu, o lock do domínio
// independente (não bloqueia nem é bloqueado pelo WriteMu do núcleo).

const dayActivityColumns = `id, day, start_time, end_time, description, created_at, updated_at`

// GetDayActivities devolve as atividades de um dia (YYYY-MM-DD) ordenadas por
// horário de início. Dia vazio devolve lista vazia.
func (s *Store) GetDayActivities(day string) ([]domain.DayActivity, error) {
	rows, err := s.DB.QueryContext(s.queryCtx(),
		`SELECT `+dayActivityColumns+` FROM day_activities WHERE day = ? ORDER BY start_time ASC, end_time ASC, id ASC`,
		day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	activities := []domain.DayActivity{}
	for rows.Next() {
		var a domain.DayActivity
		if err := rows.Scan(&a.ID, &a.Day, &a.StartTime, &a.EndTime, &a.Description, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		activities = append(activities, a)
	}
	return activities, rows.Err()
}

// CreateDayActivity insere uma atividade. Timestamps são preenchidos aqui
// quando vazios, para o chamador não precisar conhecer o formato interno.
func (s *Store) CreateDayActivity(a domain.DayActivity) error {
	s.dayPlanMu.Lock()
	defer s.dayPlanMu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	if a.CreatedAt == "" {
		a.CreatedAt = now
	}
	if a.UpdatedAt == "" {
		a.UpdatedAt = a.CreatedAt
	}

	_, err := s.DB.ExecContext(s.queryCtx(),
		`INSERT INTO day_activities (`+dayActivityColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Day, a.StartTime, a.EndTime, a.Description, a.CreatedAt, a.UpdatedAt)
	return err
}

// UpdateDayActivity atualiza uma atividade pelo id. Devolve sql.ErrNoRows
// quando o id não existe, para o handler responder 404.
func (s *Store) UpdateDayActivity(a domain.DayActivity) error {
	s.dayPlanMu.Lock()
	defer s.dayPlanMu.Unlock()

	if a.UpdatedAt == "" {
		a.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	res, err := s.DB.ExecContext(s.queryCtx(),
		`UPDATE day_activities SET day = ?, start_time = ?, end_time = ?, description = ?, updated_at = ? WHERE id = ?`,
		a.Day, a.StartTime, a.EndTime, a.Description, a.UpdatedAt, a.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteDayActivity remove uma atividade pelo id. Devolve sql.ErrNoRows
// quando o id não existe.
func (s *Store) DeleteDayActivity(id string) error {
	s.dayPlanMu.Lock()
	defer s.dayPlanMu.Unlock()

	res, err := s.DB.ExecContext(s.queryCtx(), `DELETE FROM day_activities WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// EnsureDayCopied copia as atividades de sourceDay para targetDay **uma única
// vez**, devolvendo quantas copiou. É a cópia automática da Rotina do mesmo dia
// da semana anterior (targetDay−7): se a origem estiver vazia, nada é copiado.
//
// A marca fica em day_activity_seeds (por dia de destino), dentro da MESMA
// transação da cópia. Sem ela, cada requisição copiaria de novo (duplicando) ou
// repetiria a consulta em todo dia vazio; a marca também impede uma cópia
// retroativa se a origem ganhar atividades depois.
//
// Tudo roda sob dayPlanMu (o lock do domínio Rotina), então duas requisições
// concorrentes não copiam duas vezes. Timestamps e ids são novos: a cópia é uma
// nova rotina, independente da origem.
func (s *Store) EnsureDayCopied(targetDay, sourceDay string) (int, error) {
	if targetDay == "" || sourceDay == "" || targetDay == sourceDay {
		return 0, nil
	}

	s.dayPlanMu.Lock()
	defer s.dayPlanMu.Unlock()

	ctx := s.queryCtx()

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var processed int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM day_activity_seeds WHERE day = ?`, targetDay).Scan(&processed)
	if err == nil {
		return 0, nil // o dia já foi processado (mesmo que a origem estivesse vazia)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT start_time, end_time, description FROM day_activities WHERE day = ? ORDER BY start_time ASC, end_time ASC, id ASC`,
		sourceDay)
	if err != nil {
		return 0, err
	}

	type dayCopy struct{ start, end, description string }
	var copies []dayCopy
	for rows.Next() {
		var c dayCopy
		if err := rows.Scan(&c.start, &c.end, &c.description); err != nil {
			rows.Close()
			return 0, err
		}
		copies = append(copies, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, c := range copies {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO day_activities (`+dayActivityColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			processor.GenerateCUID2(), targetDay, c.start, c.end, c.description, now, now); err != nil {
			return 0, err
		}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO day_activity_seeds (day, source_day, copied, created_at) VALUES (?, ?, ?, ?)`,
		targetDay, sourceDay, len(copies), now); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(copies), nil
}
