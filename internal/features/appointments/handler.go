package appointments

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"
	"ton618/internal/core/config"
	"ton618/internal/core/db"
	"ton618/internal/core/domain"
	"ton618/internal/processor"
	"ton618/web/layout"
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apps)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(a)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(a)
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
	layout.Agenda("📅 Agenda").Render(newCtx, w)
}

// GetISOWeek returns the ISO week number for a given date
func GetISOWeek(date time.Time) int {
	_, week := date.ISOWeek()
	return week
}

// HandleGetAgendaTree renders the agenda tree view HTML via Templ
func (ctx *HandlerContext) HandleGetAgendaTree(w http.ResponseWriter, r *http.Request) {
	apps, err := ctx.Store.GetAppointments()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if apps == nil {
		apps = []domain.Appointment{}
	}

	type groupKey struct {
		year  int
		month int
		week  int
	}

	groupsMap := make(map[groupKey][]domain.Appointment)
	var keys []groupKey

	for _, a := range apps {
		dt, _ := time.Parse(time.RFC3339, a.EventDate)
		dt = dt.Local()
		year := dt.Year()
		month := int(dt.Month())
		week := GetISOWeek(dt)

		key := groupKey{year: year, month: month, week: week}
		if _, ok := groupsMap[key]; !ok {
			keys = append(keys, key)
		}
		groupsMap[key] = append(groupsMap[key], a)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].year != keys[j].year {
			return keys[i].year > keys[j].year
		}
		if keys[i].month != keys[j].month {
			return keys[i].month > keys[j].month
		}
		return keys[i].week > keys[j].week
	})

	monthsPt := []string{"Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho", "Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro"}
	var groups []layout.WeekGroup

	for _, k := range keys {
		mName := "Desconhecido"
		if k.month >= 1 && k.month <= 12 {
			mName = monthsPt[k.month-1]
		}
		groups = append(groups, layout.WeekGroup{
			Year:         k.year,
			Month:        k.month,
			MonthName:    mName,
			WeekNumber:   k.week,
			Appointments: groupsMap[k],
		})
	}

	layout.AgendaTree(groups).Render(r.Context(), w)
}

