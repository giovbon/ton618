package dayplan

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

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
	if !strings.Contains(rec.Body.String(), "c0ea5e9-1") {
		t.Fatal("fragmento criado não contém a fonte Mermaid")
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
