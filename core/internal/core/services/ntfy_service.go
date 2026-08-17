package services

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
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

func (s *NtfyService) CheckAndSendEventReminders() {
	s.checkAndSendEventRemindersAt(time.Now())
}

// checkAndSendEventRemindersAt envia lembretes previsíveis para eventos da
// agenda: cada evento recebe uma notificação LEAD horas antes do seu horário
// (padrão 24h, configurável via setting "agenda_notify_hours").
//
// Janela de envio: evento − lead <= agora < evento. Assim o lembrete chega em
// um intervalo previsível (limitado pelo intervalo do poll do ntfy — ver
// NTFY_POLL_INTERVAL_SEC) e NUNCA após o evento ter começado.
func (s *NtfyService) checkAndSendEventRemindersAt(now time.Time) {
	apps, err := s.store.GetAppointments()
	if err != nil || len(apps) == 0 {
		return
	}

	leadHours := s.leadHours()
	lead := time.Duration(leadHours) * time.Hour

	// Usa o timezone configurado pelo usuário (mesmo do frontend da agenda)
	// para que "evento − lead" seja calculado no horário local correto.
	loc := s.userLocation()
	nowLocal := now.In(loc)

	for _, a := range apps {
		t, err := timeutil.ParseFloatingTime(a.EventDate)
		if err != nil {
			continue
		}
		// Interpreta o horário da nota no mesmo timezone do usuário
		tLocal := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)

		notifyAt := tLocal.Add(-lead)
		if notifyAt.After(nowLocal) || !tLocal.After(nowLocal) {
			// Ainda não chegou no ponto de notificação OU o evento já passou.
			continue
		}

		// Dedup: uma notificação por lead + evento + horário. Incluir o horário
		// faz com que mover o evento re-dispare o lembrete no novo horário.
		logID := fmt.Sprintf("lead_%dh_%s_%s", leadHours, a.ID, tLocal.Format("20060102_1504"))
		sent, _ := s.store.HasNotificationBeenSent(logID)
		if sent {
			continue
		}

		title := fmt.Sprintf("Lembrete: %s", a.Description)
		msg := fmt.Sprintf("Faltam %d hora(s) para:\n%s\n%s", leadHours, a.Description, tLocal.Format("02/01 15:04"))

		if err := s.SendNotification(title, msg, "default", "calendar"); err != nil {
			slog.Error("ntfy lead send failed", "error", err)
		} else {
			s.store.RecordNotificationSent(logID, "lead", now.Format(time.RFC3339))
		}
	}
}

// leadHours retorna o lead time (horas antes do evento) configurado.
// Padrão 24h. Valores inválidos ou <= 0 caem para o padrão.
func (s *NtfyService) leadHours() int {
	hours := 24
	if v, err := s.store.GetSetting("agenda_notify_hours"); err == nil {
		v = strings.TrimSpace(v)
		if v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				hours = parsed
			}
		}
	}
	return hours
}

// userLocation retorna o timezone configurado pelo usuário (default UTC).
func (s *NtfyService) userLocation() *time.Location {
	loc := time.UTC
	if tzName, err := s.store.GetSetting("agenda_timezone"); err == nil && tzName != "" {
		if parsed, err := time.LoadLocation(tzName); err == nil {
			loc = parsed
		}
	}
	return loc
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

