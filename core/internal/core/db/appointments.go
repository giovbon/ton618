package db

import (
	"database/sql"
	"time"
	"ton618/core/internal/core/domain"
	"ton618/core/internal/core/db/generated"
)

// CreateAppointment inserts a new appointment into the database.
func (s *Store) CreateAppointment(a domain.Appointment) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	if a.CreatedAt == "" {
		a.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	return s.Q.CreateAppointment(s.queryCtx(), dbgen.CreateAppointmentParams{
		ID:          a.ID,
		Description: sql.NullString{String: a.Description, Valid: true},
		EventDate:   sql.NullString{String: a.EventDate, Valid: true},
		Year:        sql.NullInt64{Int64: int64(a.Year), Valid: true},
		Month:       sql.NullInt64{Int64: int64(a.Month), Valid: true},
		WeekNumber:  sql.NullInt64{Int64: int64(a.WeekNumber), Valid: true},
		CreatedAt:   sql.NullString{String: a.CreatedAt, Valid: true},
	})
}

// UpdateAppointment updates an existing appointment.
func (s *Store) UpdateAppointment(a domain.Appointment) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	return s.Q.UpdateAppointment(s.queryCtx(), dbgen.UpdateAppointmentParams{
		Description: sql.NullString{String: a.Description, Valid: true},
		EventDate:   sql.NullString{String: a.EventDate, Valid: true},
		Year:        sql.NullInt64{Int64: int64(a.Year), Valid: true},
		Month:       sql.NullInt64{Int64: int64(a.Month), Valid: true},
		WeekNumber:  sql.NullInt64{Int64: int64(a.WeekNumber), Valid: true},
		ID:          a.ID,
	})
}

// DeleteAppointment removes an appointment by ID.
func (s *Store) DeleteAppointment(id string) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	return s.Q.DeleteAppointment(s.queryCtx(), id)
}

// DeleteOldAppointments removes all appointments whose event_date is before the given time.
func (s *Store) DeleteOldAppointments(before time.Time) error {
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	return s.Q.DeleteOldAppointments(s.queryCtx(), sql.NullString{
		String: before.UTC().Format(time.RFC3339),
		Valid:  true,
	})
}

// GetAppointments returns all appointments, optionally filtered or ordered.
func (s *Store) GetAppointments() ([]domain.Appointment, error) {
	rows, err := s.Q.GetAppointments(s.queryCtx())
	if err != nil {
		return nil, err
	}

	var apps []domain.Appointment
	for _, a := range rows {
		apps = append(apps, domain.Appointment{
			ID:          a.ID,
			Description: a.Description.String,
			EventDate:   a.EventDate.String,
			Year:        int(a.Year.Int64),
			Month:       int(a.Month.Int64),
			WeekNumber:  int(a.WeekNumber.Int64),
			CreatedAt:   a.CreatedAt.String,
		})
	}
	return apps, nil
}
