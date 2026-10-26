package dayplan

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ton618/core/internal/core/config"
	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
	"ton618/core/internal/core/featureflags"
)

func newTestContext(t *testing.T) *HandlerContext {
	t.Helper()
	store, err := db.NewStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	return NewHandlerContext(&config.AppConfig{}, store, featureflags.NewManager(store))
}

func formRequest(method, target string, values url.Values) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestHandleRotinaPage(t *testing.T) {
	ctx := newTestContext(t)
	rec := httptest.NewRecorder()

	ctx.HandleRotinaPage(rec, httptest.NewRequest(http.MethodGet, "/rotina?date=2026-10-25", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"rotina-schedule", "Resumo do dia", "Linha do tempo", "Sem atividades"} {
		if !strings.Contains(body, want) {
			t.Errorf("página não contém %q", want)
		}
	}
	if !strings.Contains(body, "2026-10-25") {
		t.Error("data não aparece na página")
	}
}

func TestHandleRotinaPage_DesabilitadaRedireciona(t *testing.T) {
	ctx := newTestContext(t)
	if err := ctx.Flags.SetRotinaEnabled(false); err != nil {
		t.Fatalf("SetRotinaEnabled: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx.HandleRotinaPage(rec, httptest.NewRequest(http.MethodGet, "/rotina", nil))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("Location = %q, want /", loc)
	}
}

func TestCreateUpdateDeleteFlow(t *testing.T) {
	ctx := newTestContext(t)

	// Criar.
	rec := httptest.NewRecorder()
	ctx.HandleCreateDayActivity(rec, formRequest(http.MethodPost, "/api/rotina/create", url.Values{
		"date":        {"2026-10-25"},
		"start_time":  {"09:00"},
		"end_time":    {"10:30"},
		"description": {"Academia"},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Academia") {
		t.Fatal("fragmento criado não contém a descrição")
	}
	if !strings.Contains(rec.Body.String(), "rotina-timeline-data") {
		t.Fatal("fragmento criado não contém o JSON do renderer nativo")
	}

	activities, err := ctx.Store.GetDayActivities("2026-10-25")
	if err != nil || len(activities) != 1 {
		t.Fatalf("atividades = %d, err=%v", len(activities), err)
	}

	// Editar.
	rec = httptest.NewRecorder()
	ctx.HandleUpdateDayActivity(rec, formRequest(http.MethodPost, "/api/rotina/update", url.Values{
		"id":          {activities[0].ID},
		"date":        {"2026-10-25"},
		"start_time":  {"09:15"},
		"end_time":    {"11:00"},
		"description": {"Academia atualizada"},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Academia atualizada") {
		t.Fatal("fragmento atualizado não contém a nova descrição")
	}

	// Excluir.
	rec = httptest.NewRecorder()
	ctx.HandleDeleteDayActivity(rec, httptest.NewRequest(http.MethodDelete,
		"/api/rotina/delete?id="+activities[0].ID+"&date=2026-10-25", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Sem atividades") {
		t.Fatal("fragmento após excluir deveria mostrar o estado vazio")
	}
	activities, _ = ctx.Store.GetDayActivities("2026-10-25")
	if len(activities) != 0 {
		t.Fatalf("esperava 0 atividades, veio %d", len(activities))
	}
}

func TestCreate_Validacao(t *testing.T) {
	ctx := newTestContext(t)

	cases := []struct {
		name   string
		form   url.Values
		status int
	}{
		{
			name:   "fim antes do início",
			form:   url.Values{"date": {"2026-10-25"}, "start_time": {"10:00"}, "end_time": {"09:00"}, "description": {"x"}},
			status: http.StatusBadRequest,
		},
		{
			name:   "horário inválido",
			form:   url.Values{"date": {"2026-10-25"}, "start_time": {"25:00"}, "end_time": {"26:00"}, "description": {"x"}},
			status: http.StatusBadRequest,
		},
		{
			name:   "descrição vazia",
			form:   url.Values{"date": {"2026-10-25"}, "start_time": {"09:00"}, "end_time": {"10:00"}, "description": {"   "}},
			status: http.StatusBadRequest,
		},
		{
			name:   "data inválida",
			form:   url.Values{"date": {"25/10/2026"}, "start_time": {"09:00"}, "end_time": {"10:00"}, "description": {"x"}},
			status: http.StatusBadRequest,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx.HandleCreateDayActivity(rec, formRequest(http.MethodPost, "/api/rotina/create", c.form))
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, c.status, rec.Body.String())
			}
		})
	}
}

func TestUpdateDelete_NotFound(t *testing.T) {
	ctx := newTestContext(t)

	rec := httptest.NewRecorder()
	ctx.HandleUpdateDayActivity(rec, formRequest(http.MethodPost, "/api/rotina/update", url.Values{
		"id": {"nao-existe"}, "date": {"2026-10-25"}, "start_time": {"09:00"}, "end_time": {"10:00"}, "description": {"x"},
	}))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update inexistente: status = %d, want 404", rec.Code)
	}

	rec = httptest.NewRecorder()
	ctx.HandleDeleteDayActivity(rec, httptest.NewRequest(http.MethodDelete,
		"/api/rotina/delete?id=nao-existe&date=2026-10-25", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete inexistente: status = %d, want 404", rec.Code)
	}
}

func TestHandleRotinaSchedule(t *testing.T) {
	ctx := newTestContext(t)
	if err := ctx.Store.CreateDayActivity(domainActivity("2026-10-25", "08:00", "09:00", "Mercado")); err != nil {
		t.Fatalf("CreateDayActivity: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx.HandleRotinaSchedule(rec, httptest.NewRequest(http.MethodGet, "/api/rotina/schedule?date=2026-10-25", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Mercado") {
		t.Fatal("fragmento não contém a atividade")
	}
	if !strings.Contains(rec.Body.String(), "rotina-timeline-data") {
		t.Fatal("fragmento deveria trazer o JSON do renderer nativo")
	}
}

// TestScheduleFor_TimelineRenderer garante que o Mermaid só é gerado quando a
// flag de comparação está desligada (economia de trabalho e de 3,2 MB no page).
func TestScheduleFor_TimelineRenderer(t *testing.T) {
	ctx := newTestContext(t)
	if err := ctx.Store.CreateDayActivity(domainActivity("2026-10-25", "08:00", "09:00", "Mercado")); err != nil {
		t.Fatalf("CreateDayActivity: %v", err)
	}

	data, err := ctx.scheduleFor("2026-10-25")
	if err != nil {
		t.Fatalf("scheduleFor: %v", err)
	}
	if !data.TimelineNative {
		t.Fatal("o renderer nativo deveria ser o padrão")
	}
	if data.MermaidSrc != "" {
		t.Fatalf("modo nativo não deveria gerar Mermaid: %q", data.MermaidSrc)
	}

	if err := ctx.Flags.SetRotinaNativeTimeline(false); err != nil {
		t.Fatalf("SetRotinaNativeTimeline: %v", err)
	}
	data, err = ctx.scheduleFor("2026-10-25")
	if err != nil {
		t.Fatalf("scheduleFor (mermaid): %v", err)
	}
	if data.TimelineNative {
		t.Fatal("flag desligada deveria usar o Mermaid")
	}
	if !strings.Contains(data.MermaidSrc, "gantt") {
		t.Fatalf("fonte Mermaid ausente: %q", data.MermaidSrc)
	}
}

func TestHandleRotinaSetting_TimelineNative(t *testing.T) {
	ctx := newTestContext(t)

	rec := httptest.NewRecorder()
	ctx.HandlePostRotinaSetting(rec, formRequest(http.MethodPost, "/api/settings/rotina", url.Values{
		"native": {"0"},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if ctx.Flags.Get().RotinaNativeTimeline {
		t.Fatal("POST native=0 deveria desligar o renderer nativo")
	}

	rec = httptest.NewRecorder()
	ctx.HandleGetRotinaSetting(rec, httptest.NewRequest(http.MethodGet, "/api/settings/rotina", nil))
	var body struct {
		Enabled        bool `json:"enabled"`
		TimelineNative bool `json:"timeline_native"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("JSON inválido: %v; body=%s", err, rec.Body.String())
	}
	if body.TimelineNative {
		t.Fatal("GET deveria refletir native=0")
	}
	if !body.Enabled {
		t.Fatal("GET não deveria desligar a Rotina inteira")
	}
}

// TestSeedDay_CopiaSemanaAnterior cobre a cópia automática do dia de destino
// a partir do mesmo dia da semana anterior, sem duplicar em chamadas repetidas.
func TestSeedDay_CopiaSemanaAnterior(t *testing.T) {
	ctx := newTestContext(t)

	for _, a := range []domain.DayActivity{
		domainActivity("2026-10-18", "08:00", "08:30", "Café"),
		domainActivity("2026-10-18", "09:00", "10:00", "Academia"),
	} {
		if err := ctx.Store.CreateDayActivity(a); err != nil {
			t.Fatalf("CreateDayActivity: %v", err)
		}
	}

	ctx.seedDay("2026-10-25")

	got, err := ctx.Store.GetDayActivities("2026-10-25")
	if err != nil {
		t.Fatalf("GetDayActivities: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("destino tem %d atividades, want 2", len(got))
	}
	if got[0].Description != "Café" || got[1].Description != "Academia" {
		t.Fatalf("cópia inesperada: %+v", got)
	}

	// Chamar de novo no mesmo dia não duplica.
	ctx.seedDay("2026-10-25")
	if got, _ = ctx.Store.GetDayActivities("2026-10-25"); len(got) != 2 {
		t.Fatalf("cópia repetida duplicou: %d atividades", len(got))
	}
}

// TestSeedDay_SemOrigemNaoCopia garante que um dia anterior vazio não gera
// atividades no dia atual.
func TestSeedDay_SemOrigemNaoCopia(t *testing.T) {
	ctx := newTestContext(t)

	ctx.seedDay("2026-10-25")

	if got, _ := ctx.Store.GetDayActivities("2026-10-25"); len(got) != 0 {
		t.Fatalf("origem vazia criou %d atividades", len(got))
	}
}

// TestSeedDay_FlagDesligadaNaoCopia garante que a cópia respeita o desligamento
// da Rotina nas Configurações.
func TestSeedDay_FlagDesligadaNaoCopia(t *testing.T) {
	ctx := newTestContext(t)
	if err := ctx.Flags.SetRotinaEnabled(false); err != nil {
		t.Fatalf("SetRotinaEnabled: %v", err)
	}
	if err := ctx.Store.CreateDayActivity(domainActivity("2026-10-18", "09:00", "10:00", "Academia")); err != nil {
		t.Fatalf("CreateDayActivity: %v", err)
	}

	ctx.seedDay("2026-10-25")

	if got, _ := ctx.Store.GetDayActivities("2026-10-25"); len(got) != 0 {
		t.Fatalf("flag desligada copiou %d atividades", len(got))
	}
}

// TestSeedToday_CopiaSemanaAnterior usa a data real de hoje para garantir que
// o gancho chamado pelos handlers copia exatamente hoje − 7 dias.
func TestSeedToday_CopiaSemanaAnterior(t *testing.T) {
	ctx := newTestContext(t)

	today := time.Now().Format(dateLayout)
	source := shiftDay(today, -7)
	if err := ctx.Store.CreateDayActivity(domainActivity(source, "09:00", "10:00", "Rotina da semana passada")); err != nil {
		t.Fatalf("CreateDayActivity: %v", err)
	}

	ctx.SeedToday()

	got, err := ctx.Store.GetDayActivities(today)
	if err != nil {
		t.Fatalf("GetDayActivities: %v", err)
	}
	if len(got) != 1 || got[0].Description != "Rotina da semana passada" {
		t.Fatalf("hoje não recebeu a rotina da semana anterior: %+v", got)
	}
}

func TestSettingsRotina(t *testing.T) {
	ctx := newTestContext(t)

	rec := httptest.NewRecorder()
	ctx.HandleGetRotinaSetting(rec, httptest.NewRequest(http.MethodGet, "/api/settings/rotina", nil))
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if got["enabled"] != true {
		t.Fatalf("enabled inicial = %v, want true", got["enabled"])
	}

	rec = httptest.NewRecorder()
	ctx.HandlePostRotinaSetting(rec, formRequest(http.MethodPost, "/api/settings/rotina", url.Values{"enabled": {"0"}}))
	if rec.Code != http.StatusOK {
		t.Fatalf("post status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if ctx.Flags.RotinaEnabled() {
		t.Fatal("flag deveria estar desligada")
	}

	rec = httptest.NewRecorder()
	ctx.HandleGetRotinaSetting(rec, httptest.NewRequest(http.MethodGet, "/api/settings/rotina", nil))
	got = map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["enabled"] != false {
		t.Fatalf("enabled após desligar = %v, want false", got["enabled"])
	}
}

// domainActivity evita repetir o literal em vários testes.
func domainActivity(day, start, end, desc string) (a domain.DayActivity) {
	a.Day = day
	a.StartTime = start
	a.EndTime = end
	a.Description = desc
	a.ID = "test-" + start
	return a
}
