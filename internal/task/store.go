package task

import (
	"database/sql"
	"time"

	"ton618/internal/db"
)

// Store gerencia as operações de tarefas no banco.
type Store struct {
	db *db.Store
}

// NewStore cria um Store de tarefas.
func NewStore(store *db.Store) *Store {
	return &Store{db: store}
}

// ── CRUD ──

func (s *Store) CreateTask(t Task) error {
	t.CreatedAt = time.Now()
	t.UpdatedAt = t.CreatedAt
	if t.Color == "" {
		t.Color = CategoryColor(t.Category)
	}
	_, err := s.db.DB.Exec(`
		INSERT INTO tasks (id, title, description, status, priority, category, start_time, end_time, all_day, color, recurrence_id, note_link, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, t.Description, t.Status, t.Priority, t.Category,
		t.StartTime.Format(time.RFC3339), t.EndTime.Format(time.RFC3339),
		boolToInt(t.AllDay), t.Color, t.RecurrenceID, t.NoteLink,
		t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *Store) UpdateTask(t Task) error {
	t.UpdatedAt = time.Now()
	_, err := s.db.DB.Exec(`
		UPDATE tasks SET title=?, description=?, status=?, priority=?, category=?, start_time=?, end_time=?, all_day=?, color=?, recurrence_id=?, note_link=?, updated_at=?
		WHERE id=?`,
		t.Title, t.Description, t.Status, t.Priority, t.Category,
		t.StartTime.Format(time.RFC3339), t.EndTime.Format(time.RFC3339),
		boolToInt(t.AllDay), t.Color, t.RecurrenceID, t.NoteLink,
		t.UpdatedAt.Format(time.RFC3339), t.ID,
	)
	return err
}

func (s *Store) DeleteTask(id string) error {
	_, err := s.db.DB.Exec("DELETE FROM tasks WHERE id = ?", id)
	return err
}

func (s *Store) GetTask(id string) (*Task, error) {
	row := s.db.DB.QueryRow(`
		SELECT id, title, description, status, priority, category, start_time, end_time, all_day, color, recurrence_id, note_link, created_at, updated_at
		FROM tasks WHERE id = ?`, id)
	return scanTask(row)
}

// ListTasks retorna tarefas num intervalo de tempo. Inclui tarefas que cruzam o intervalo.
func (s *Store) ListTasks(from, to time.Time) ([]Task, error) {
	rows, err := s.db.DB.Query(`
		SELECT id, title, description, status, priority, category, start_time, end_time, all_day, color, recurrence_id, note_link, created_at, updated_at
		FROM tasks
		WHERE start_time < ? AND end_time > ?
		ORDER BY start_time ASC
	`, to.Format(time.RFC3339), from.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

// GetUpcomingTasks retorna as próximas N tarefas pendentes.
func (s *Store) GetUpcomingTasks(limit int) ([]Task, error) {
	rows, err := s.db.DB.Query(`
		SELECT id, title, description, status, priority, category, start_time, end_time, all_day, color, recurrence_id, note_link, created_at, updated_at
		FROM tasks
		WHERE start_time >= ? AND status != 'cancelled' AND status != 'done'
		ORDER BY start_time ASC
		LIMIT ?
	`, time.Now().Format(time.RFC3339), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (s *Store) GetAllTasks() ([]Task, error) {
	rows, err := s.db.DB.Query(`
		SELECT id, title, description, status, priority, category, start_time, end_time, all_day, color, recurrence_id, note_link, created_at, updated_at
		FROM tasks
		ORDER BY start_time ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

// ── Recorrência ──

func (s *Store) CreateRecurrence(r TaskRecurrence) error {
	_, err := s.db.DB.Exec(`
		INSERT INTO task_recurrence (id, rule, days_of_week, days_of_month, interval, end_date)
		VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.Rule, r.DaysOfWeek, r.DaysOfMonth, r.Interval, r.EndDate.Format(time.RFC3339),
	)
	return err
}

func (s *Store) GetRecurrence(id string) (*TaskRecurrence, error) {
	row := s.db.DB.QueryRow(`
		SELECT id, rule, days_of_week, days_of_month, interval, end_date
		FROM task_recurrence WHERE id = ?`, id)
	var r TaskRecurrence
	var endDate string
	err := row.Scan(&r.ID, &r.Rule, &r.DaysOfWeek, &r.DaysOfMonth, &r.Interval, &endDate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.EndDate, _ = time.Parse(time.RFC3339, endDate)
	return &r, nil
}

// ── Helpers de scan ──

type scanner interface {
	Scan(dest ...any) error
}

func scanTask(s scanner) (*Task, error) {
	var t Task
	var startStr, endStr, createdStr, updatedStr string
	var allDay int
	err := s.Scan(&t.ID, &t.Title, &t.Description, (*string)(&t.Status), (*string)(&t.Priority), &t.Category,
		&startStr, &endStr, &allDay, &t.Color, &t.RecurrenceID, &t.NoteLink,
		&createdStr, &updatedStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.StartTime, _ = time.Parse(time.RFC3339, startStr)
	t.EndTime, _ = time.Parse(time.RFC3339, endStr)
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	t.AllDay = allDay == 1
	return &t, nil
}

func scanTasks(rows *sql.Rows) ([]Task, error) {
	var tasks []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		if t != nil {
			tasks = append(tasks, *t)
		}
	}
	return tasks, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
