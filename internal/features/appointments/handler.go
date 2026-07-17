package appointments

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
	"ton618/internal/core/config"
	"ton618/internal/core/db"
	"ton618/internal/core/domain"
	"ton618/internal/core/timeutil"
	"ton618/internal/httputil"
	"ton618/internal/processor"
)

type HandlerContext struct {
	Cfg   *config.AppConfig
	Store *db.Store
}

func NewHandlerContext(cfg *config.AppConfig, store *db.Store) *HandlerContext {
	return &HandlerContext{
		Cfg:   cfg,
		Store: store,
	}
}

// HandleGetAppointments returns all appointments in JSON format.
func (ctx *HandlerContext) HandleGetAppointments(w http.ResponseWriter, r *http.Request) {
	apps, err := ctx.Store.GetAppointments()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if apps == nil {
		apps = []domain.Appointment{}
	}
	httputil.WriteJSON(w, apps)
}

// HandleCreateAppointment creates a new appointment.
func (ctx *HandlerContext) HandleCreateAppointment(w http.ResponseWriter, r *http.Request) {
	var a domain.Appointment
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if a.ID == "" {
		a.ID = processor.GenerateCUID2()
	}


	if err := ctx.Store.CreateAppointment(a); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	httputil.WriteJSONStatus(w, http.StatusCreated, a)
}

// HandleUpdateAppointment updates an existing appointment.
func (ctx *HandlerContext) HandleUpdateAppointment(w http.ResponseWriter, r *http.Request) {
	var a domain.Appointment
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := ctx.Store.UpdateAppointment(a); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	httputil.WriteJSONStatus(w, http.StatusOK, a)
}

// HandleDeleteAppointment removes an appointment.
func (ctx *HandlerContext) HandleDeleteAppointment(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	if err := ctx.Store.DeleteAppointment(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandlePurgeOldAppointments deletes all appointments whose event_date is older than 7 days.
func (ctx *HandlerContext) HandlePurgeOldAppointments(w http.ResponseWriter, r *http.Request) {
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	if err := ctx.Store.DeleteOldAppointments(cutoff); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// HandleAgendaPage renders the agenda layout.
func (ctx *HandlerContext) HandleAgendaPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	newCtx := context.WithValue(r.Context(), "is_full_width", true)
	Agenda("📅 Agenda").Render(newCtx, w)
}

// parseEventDate parses the appointment event date string safely using the shared timeutil package.
func parseEventDate(dateStr string) time.Time {
	t, err := timeutil.ParseFloatingTime(dateStr)
	if err != nil {
		return time.Time{}
	}
	return t
}

// GetISOWeek returns the ISO week number for a given date
func GetISOWeek(date time.Time) int {
	_, week := date.ISOWeek()
	return week
}

// HandleGetAgendaTree renders the agenda tree view HTML via Templ.
// Supports lazy loading via ?offset=N&limit=M query parameters.
// Default: offset=0, limit=8 (weeks).
func (ctx *HandlerContext) HandleGetAgendaTree(w http.ResponseWriter, r *http.Request) {
	apps, err := ctx.Store.GetAppointments()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if apps == nil {
		apps = []domain.Appointment{}
	}

	// Parse pagination params
	offsetStr := r.URL.Query().Get("offset")
	limitStr  := r.URL.Query().Get("limit")
	offset, limit := 0, 8
	if v, err2 := parseInt(offsetStr); err2 == nil && v >= 0 {
		offset = v
	}
	if v, err2 := parseInt(limitStr); err2 == nil && v > 0 {
		limit = v
	}

	type groupKey struct {
		year int
		week int
	}

	groupsMap := make(map[groupKey][]domain.Appointment)
	var keys []groupKey

	for _, a := range apps {
		dt   := parseEventDate(a.EventDate)
		year := dt.Year()
		week := GetISOWeek(dt)

		key := groupKey{year: year, week: week}
		if _, ok := groupsMap[key]; !ok {
			keys = append(keys, key)
		}
		groupsMap[key] = append(groupsMap[key], a)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].year != keys[j].year {
			return keys[i].year < keys[j].year
		}
		return keys[i].week < keys[j].week
	})

	monthsPt := []string{"Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho", "Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro"}
	var allGroups []WeekGroup

	for _, k := range keys {
		appsInGroup := groupsMap[k]
		sort.Slice(appsInGroup, func(i, j int) bool {
			return appsInGroup[i].EventDate < appsInGroup[j].EventDate
		})

		repMonth := 1
		repYear  := k.year
		if len(appsInGroup) > 0 {
			dtRep    := parseEventDate(appsInGroup[0].EventDate)
			repMonth  = int(dtRep.Month())
			repYear   = dtRep.Year()
		}

		mName := "Desconhecido"
		if repMonth >= 1 && repMonth <= 12 {
			mName = monthsPt[repMonth-1]
		}

		allGroups = append(allGroups, WeekGroup{
			Year:         repYear,
			Month:        repMonth,
			MonthName:    mName,
			WeekNumber:   k.week,
			Appointments: appsInGroup,
		})
	}

	// Paginate
	total    := len(allGroups)
	end      := offset + limit
	if end > total {
		end = total
	}
	var page []WeekGroup
	if offset < total {
		page = allGroups[offset:end]
	}
	hasMore  := end < total
	nextOffset := end

	AgendaTree(page, hasMore, nextOffset, limit).Render(r.Context(), w)
}

// parseInt parses a string to int, returning an error if blank or invalid.
func parseInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

