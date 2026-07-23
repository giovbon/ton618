package services

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"ton618/core/internal/core/db"
	"ton618/core/internal/core/timeutil"
)

type NtfyService struct {
	store *db.Store
}

func NewNtfyService(store *db.Store) *NtfyService {
	return &NtfyService{
		store: store,
	}
}

func (s *NtfyService) isConfigured() (string, string, string, string, bool) {
	url, _ := s.store.GetSetting("ntfy_url")
	topic, _ := s.store.GetSetting("ntfy_topic")
	user, _ := s.store.GetSetting("ntfy_user")
	pass, _ := s.store.GetSetting("ntfy_pass")

	url = strings.TrimSpace(url)
	topic = strings.TrimSpace(topic)
	if url == "" || topic == "" {
		return "", "", "", "", false
	}
	// Trim trailing slash from url if any
	url = strings.TrimRight(url, "/")
	return url, topic, user, pass, true
}

func (s *NtfyService) SendNotification(title, message, priority, tags string) error {
	url, topic, user, pass, configured := s.isConfigured()
	if !configured {
		return nil // Ntfy not configured, silent ignore
	}

	endpoint := fmt.Sprintf("%s/%s", url, topic)
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("create ntfy req: %w", err)
	}

	if title != "" {
		req.Header.Set("Title", title)
	}
	if priority != "" {
		req.Header.Set("Priority", priority)
	}
	if tags != "" {
		req.Header.Set("Tags", tags)
	}
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send ntfy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *NtfyService) CheckAndSendDailyAppointments() {
	s.checkAndSendDailyAppointmentsAt(time.Now())
}

func (s *NtfyService) checkAndSendDailyAppointmentsAt(now time.Time) {
	apps, err := s.store.GetAppointments()
	if err != nil || len(apps) == 0 {
		return
	}

	// Usa o timezone configurado pelo usuário (mesmo do frontend da agenda)
	// para que "amanhã" seja calculado no horário local correto.
	loc := time.UTC
	if tzName, err2 := s.store.GetSetting("agenda_timezone"); err2 == nil && tzName != "" {
		if parsed, err3 := time.LoadLocation(tzName); err3 == nil {
			loc = parsed
		}
	}

	nowLocal := now.In(loc)
	// "Amanhã" começa à meia-noite do próximo dia no fuso do usuário
	tomorrowLocal := nowLocal.AddDate(0, 0, 1)
	tomorrowStart := time.Date(tomorrowLocal.Year(), tomorrowLocal.Month(), tomorrowLocal.Day(), 0, 0, 0, 0, loc)
	tomorrowEnd := tomorrowStart.Add(24 * time.Hour)

	for _, a := range apps {
		t, err := timeutil.ParseFloatingTime(a.EventDate)
		if err != nil {
			continue
		}
		// Interpreta o horário da nota no mesmo timezone do usuário
		tLocal := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)

		if tLocal.After(tomorrowStart) && tLocal.Before(tomorrowEnd) {
			logID := "daily_" + a.ID
			sent, _ := s.store.HasNotificationBeenSent(logID)
			if !sent {
				title := "Lembrete: Agendamento Amanhã"
				msg := fmt.Sprintf("Você tem um agendamento amanhã às %s:\n%s", t.Format("15:04"), a.Description)

				err := s.SendNotification(title, msg, "default", "calendar")
				if err != nil {
					slog.Error("ntfy daily send failed", "error", err)
				} else {
					s.store.RecordNotificationSent(logID, "daily", now.Format(time.RFC3339))
				}
			}
		}
	}
}

func (s *NtfyService) CheckAndSendWeeklySummary() {
	s.checkAndSendWeeklySummaryAt(time.Now())
}

func (s *NtfyService) checkAndSendWeeklySummaryAt(now time.Time) {
	// Somente aos domingos
	if now.Weekday() != time.Sunday {
		return
	}

	// Identificar próxima semana (segunda a domingo)
	monday := now.AddDate(0, 0, 1)
	mondayStart := time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, monday.Location())
	nextSundayEnd := mondayStart.AddDate(0, 0, 7) // até segunda que vem as 00:00

	year, week := mondayStart.ISOWeek()
	logID := fmt.Sprintf("weekly_%d_%d", year, week)

	sent, _ := s.store.HasNotificationBeenSent(logID)
	if sent {
		return
	}

	apps, err := s.store.GetAppointments()
	if err != nil || len(apps) == 0 {
		return
	}

	var upcoming []string
	for _, a := range apps {
		t, err := timeutil.ParseFloatingTime(a.EventDate)
		if err != nil {
			continue
		}

		if (t.Equal(mondayStart) || t.After(mondayStart)) && t.Before(nextSundayEnd) {
			upcoming = append(upcoming, fmt.Sprintf("- %s: %s", t.Format("02/01 15:04"), a.Description))
		}
	}

	if len(upcoming) == 0 {
		// Mesmo sem agendamentos, registrar para não checar novamente hoje
		s.store.RecordNotificationSent(logID, "weekly", now.Format(time.RFC3339))
		return
	}

	title := fmt.Sprintf("Resumo da Semana %d", week)
	msg := fmt.Sprintf("Você tem %d agendamento(s) para a próxima semana:\n\n%s", len(upcoming), strings.Join(upcoming, "\n"))

	err = s.SendNotification(title, msg, "high", "calendar,clipboard")
	if err != nil {
		slog.Error("ntfy weekly send failed", "error", err)
	} else {
		s.store.RecordNotificationSent(logID, "weekly", now.Format(time.RFC3339))
	}
}

