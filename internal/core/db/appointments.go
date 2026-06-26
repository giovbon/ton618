package db

import (
	"time"
	"ton618/internal/core/domain"
)

// CreateAppointment inserts a new appointment into the database.
func (s *Store) CreateAppointment(a domain.Appointment) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	stmt, err := s.DB.Prepare("INSERT INTO appointments (id, description, event_date, year, month, week_number, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	if a.CreatedAt == "" {
		a.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	_, err = stmt.Exec(a.ID, a.Description, a.EventDate, a.Year, a.Month, a.WeekNumber, a.CreatedAt)
	return err
}

// UpdateAppointment updates an existing appointment.
func (s *Store) UpdateAppointment(a domain.Appointment) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	stmt, err := s.DB.Prepare("UPDATE appointments SET description = ?, event_date = ?, year = ?, month = ?, week_number = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(a.Description, a.EventDate, a.Year, a.Month, a.WeekNumber, a.ID)
	return err
}

// DeleteAppointment removes an appointment by ID.
func (s *Store) DeleteAppointment(id string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	_, err := s.DB.Exec("DELETE FROM appointments WHERE id = ?", id)
	return err
}

// DeleteOldAppointments removes all appointments whose event_date is before the given time.
func (s *Store) DeleteOldAppointments(before time.Time) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	_, err := s.DB.Exec("DELETE FROM appointments WHERE event_date < ?", before.UTC().Format(time.RFC3339))
	return err
}

// GetAppointments returns all appointments, optionally filtered or ordered.
func (s *Store) GetAppointments() ([]domain.Appointment, error) {
	rows, err := s.DB.Query("SELECT id, description, event_date, year, month, week_number, created_at FROM appointments ORDER BY year ASC, month ASC, week_number ASC, event_date ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []domain.Appointment
	for rows.Next() {
		var a domain.Appointment
		if err := rows.Scan(&a.ID, &a.Description, &a.EventDate, &a.Year, &a.Month, &a.WeekNumber, &a.CreatedAt); err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}
