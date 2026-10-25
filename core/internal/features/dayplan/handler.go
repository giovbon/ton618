package dayplan

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"ton618/core/internal/core/config"
	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
	"ton618/core/internal/core/featureflags"
	"ton618/core/internal/httputil"
	"ton618/core/internal/processor"
)

const maxDescriptionRunes = 200

// HandlerContext reúne as dependências da Rotina.
type HandlerContext struct {
	Cfg   *config.AppConfig
	Store *db.Store
	Flags *featureflags.Manager
}

func NewHandlerContext(cfg *config.AppConfig, store *db.Store, flags *featureflags.Manager) *HandlerContext {
	return &HandlerContext{Cfg: cfg, Store: store, Flags: flags}
}

// ScheduleData é o bloco trocado a cada mutação (resumo + timeline + lista +
// formulário). O JS substitui o elemento #rotina-schedule pelo HTML retornado.
type ScheduleData struct {
	Date           string
	DateLabel      string
	Weekday        string
	Activities     []domain.DayActivity
	Summary        Summary
	MermaidSrc     string
	OverlappingIDs map[string]bool
	DefaultStart   string
	DefaultEnd     string
}

// HasActivities informa se o dia tem atividades (para o estado vazio).
func (s ScheduleData) HasActivities() bool { return len(s.Activities) > 0 }

// PageData é a página completa: navegação de datas + o bloco da agenda.
type PageData struct {
	Schedule ScheduleData
	PrevDate string
	NextDate string
	Today    string
}

// HandleRotinaPage renderiza /rotina. Se a funcionalidade estiver desligada,
// redireciona para a home — o ícone do cabeçalho também não é exibido.
func (ctx *HandlerContext) HandleRotinaPage(w http.ResponseWriter, r *http.Request) {
	if !ctx.enabled() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	ctx.SeedToday()

	day := normalizeDay(r.URL.Query().Get("date"))
	data, err := ctx.scheduleFor(day)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newCtx := context.WithValue(r.Context(), "is_full_width", true)
	RotinaPage(PageData{
		Schedule: data,
		PrevDate: shiftDay(day, -1),
		NextDate: shiftDay(day, 1),
		Today:    time.Now().Format(dateLayout),
	}).Render(newCtx, w)
}

// HandleRotinaSchedule devolve só o bloco #rotina-schedule (HTML), usado para
// trocar datas e para o polling leve depois de uma mutação.
func (ctx *HandlerContext) HandleRotinaSchedule(w http.ResponseWriter, r *http.Request) {
	if !ctx.enabled() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	ctx.SeedToday()

	day := normalizeDay(r.URL.Query().Get("date"))
	data, err := ctx.scheduleFor(day)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	RotinaSchedule(data).Render(r.Context(), w)
}

// HandleCreateDayActivity cria uma atividade e devolve o bloco atualizado.
func (ctx *HandlerContext) HandleCreateDayActivity(w http.ResponseWriter, r *http.Request) {
	if !ctx.enabled() {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulário inválido", http.StatusBadRequest)
		return
	}

	activity, err := activityFromForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	activity.ID = processor.GenerateCUID2()

	if err := ctx.Store.CreateDayActivity(activity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctx.renderSchedule(w, r, activity.Day)
}

// HandleUpdateDayActivity atualiza uma atividade existente.
func (ctx *HandlerContext) HandleUpdateDayActivity(w http.ResponseWriter, r *http.Request) {
	if !ctx.enabled() {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulário inválido", http.StatusBadRequest)
		return
	}

	activity, err := activityFromForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	activity.ID = strings.TrimSpace(r.FormValue("id"))
	if activity.ID == "" {
		http.Error(w, "id obrigatório", http.StatusBadRequest)
		return
	}

	if err := ctx.Store.UpdateDayActivity(activity); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "atividade não encontrada", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctx.renderSchedule(w, r, activity.Day)
}

// HandleDeleteDayActivity exclui uma atividade. Aceita ?id= e ?date= (o JS
// envia a data visível para devolver o bloco certo).
func (ctx *HandlerContext) HandleDeleteDayActivity(w http.ResponseWriter, r *http.Request) {
	if !ctx.enabled() {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		id = strings.TrimSpace(r.FormValue("id"))
	}
	if id == "" {
		http.Error(w, "id obrigatório", http.StatusBadRequest)
		return
	}
	day := normalizeDay(firstNonEmpty(r.URL.Query().Get("date"), r.FormValue("date")))

	if err := ctx.Store.DeleteDayActivity(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "atividade não encontrada", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ctx.renderSchedule(w, r, day)
}

// HandleGetRotinaSetting devolve o estado atual da flag.
func (ctx *HandlerContext) HandleGetRotinaSetting(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, map[string]any{
		"enabled": ctx.Flags.Get().Rotina,
	})
}

// HandlePostRotinaSetting liga/desliga a Rotina. O estado em memória só muda
// depois que a escrita em `settings` tem sucesso.
func (ctx *HandlerContext) HandlePostRotinaSetting(w http.ResponseWriter, r *http.Request) {
	raw := ""
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.Enabled != nil {
			raw = featureflags.BoolString(*body.Enabled)
		}
	} else {
		_ = r.ParseForm()
		raw = r.FormValue("enabled")
	}
	if strings.TrimSpace(raw) == "" {
		http.Error(w, "enabled obrigatório", http.StatusBadRequest)
		return
	}

	enabled := featureflags.ParseBool(raw, false)
	if err := ctx.Flags.SetRotinaEnabled(enabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	httputil.WriteJSON(w, map[string]any{
		"ok":      true,
		"enabled": enabled,
	})
}

func (ctx *HandlerContext) enabled() bool {
	return ctx.Flags == nil || ctx.Flags.RotinaEnabled()
}

func (ctx *HandlerContext) scheduleFor(day string) (ScheduleData, error) {
	activities, err := ctx.Store.GetDayActivities(day)
	if err != nil {
		return ScheduleData{}, err
	}

	start, end := defaultTimes(day)
	return ScheduleData{
		Date:           day,
		DateLabel:      DateLabel(day),
		Weekday:        WeekdayLabel(day),
		Activities:     activities,
		Summary:        Summarize(activities),
		MermaidSrc:     Gantt(day, activities),
		OverlappingIDs: overlappingIDs(activities),
		DefaultStart:   start,
		DefaultEnd:     end,
	}, nil
}

func (ctx *HandlerContext) renderSchedule(w http.ResponseWriter, r *http.Request, day string) {
	data, err := ctx.scheduleFor(day)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	RotinaSchedule(data).Render(r.Context(), w)
}

// activityFromForm valida e normaliza os campos do formulário.
func activityFromForm(r *http.Request) (domain.DayActivity, error) {
	day := strings.TrimSpace(r.FormValue("date"))
	if day == "" {
		day = time.Now().Format(dateLayout)
	} else if _, err := parseDay(day); err != nil {
		return domain.DayActivity{}, errors.New("data inválida (use AAAA-MM-DD)")
	}
	start, ok := normalizeClock(r.FormValue("start_time"))
	if !ok {
		return domain.DayActivity{}, errors.New("horário de início inválido (use HH:MM)")
	}
	end, ok := normalizeClock(r.FormValue("end_time"))
	if !ok {
		return domain.DayActivity{}, errors.New("horário de fim inválido (use HH:MM)")
	}
	startMin, _ := parseClockMinutes(start)
	endMin, _ := parseClockMinutes(end)
	if endMin <= startMin {
		return domain.DayActivity{}, errors.New("o fim precisa ser depois do início")
	}

	description := strings.TrimSpace(r.FormValue("description"))
	if description == "" {
		return domain.DayActivity{}, errors.New("descrição obrigatória")
	}
	if utf8.RuneCountInString(description) > maxDescriptionRunes {
		return domain.DayActivity{}, fmt.Errorf("descrição muito longa (máximo %d caracteres)", maxDescriptionRunes)
	}

	return domain.DayActivity{
		Day:         day,
		StartTime:   start,
		EndTime:     end,
		Description: description,
	}, nil
}

// normalizeClock valida HH:MM e devolve sempre com dois dígitos.
func normalizeClock(value string) (string, bool) {
	mins, ok := parseClockMinutes(value)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%02d:%02d", mins/60, mins%60), true
}

// normalizeDay aceita só YYYY-MM-DD válido; qualquer outra coisa cai em hoje.
func normalizeDay(value string) string {
	value = strings.TrimSpace(value)
	if _, err := parseDay(value); err == nil {
		return value
	}
	return time.Now().Format(dateLayout)
}

// shiftDay soma/subtrai dias de uma data YYYY-MM-DD.
func shiftDay(day string, delta int) string {
	t, err := parseDay(day)
	if err != nil {
		return time.Now().Format(dateLayout)
	}
	return t.AddDate(0, 0, delta).Format(dateLayout)
}

// overlappingIDs marca as atividades que colidem com alguma outra, para a
// lista mostrar um aviso pontual.
func overlappingIDs(activities []domain.DayActivity) map[string]bool {
	type item struct {
		id    string
		start int
		end   int
		valid bool
	}
	items := make([]item, 0, len(activities))
	for _, a := range activities {
		s, okStart := parseClockMinutes(a.StartTime)
		e, okEnd := parseClockMinutes(a.EndTime)
		items = append(items, item{id: a.ID, start: s, end: e, valid: okStart && okEnd && e > s})
	}

	marked := map[string]bool{}
	for i := range items {
		if !items[i].valid {
			continue
		}
		for j := range items {
			if i == j || !items[j].valid {
				continue
			}
			if items[i].start < items[j].end && items[j].start < items[i].end {
				marked[items[i].id] = true
				break
			}
		}
	}
	return marked
}

// defaultTimes sugere a hora atual (dia de hoje) ou 09:00–10:00 (outro dia)
// como ponto de partida do formulário.
func defaultTimes(day string) (string, string) {
	if day != time.Now().Format(dateLayout) {
		return "09:00", "10:00"
	}
	start := time.Now().Hour()
	end := start + 1
	if end > 23 {
		start, end = 22, 23
	}
	return fmt.Sprintf("%02d:00", start), fmt.Sprintf("%02d:00", end)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
