package appointments

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ton618/internal/core/config"
	"ton618/internal/core/db"
	"ton618/internal/core/domain"
)

func newTestContext(t *testing.T) *HandlerContext {
	t.Helper()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test.db")

	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := &config.AppConfig{}

	return NewHandlerContext(cfg, store)
}

func TestGetISOWeek(t *testing.T) {
	// Test known dates
	tests := []struct {
		date     time.Time
		expected int
	}{
		{time.Date(2026, time.June, 27, 12, 0, 0, 0, time.UTC), 26},
		{time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC), 1}, // ISO week 1 of 2026
		{time.Date(2026, time.January, 5, 12, 0, 0, 0, time.UTC), 2},
	}

	for _, tt := range tests {
		got := GetISOWeek(tt.date)
		if got != tt.expected {
			t.Errorf("GetISOWeek(%s) = %d, want %d", tt.date, got, tt.expected)
		}
	}
}

func TestHandleGetAppointments(t *testing.T) {
	ctx := newTestContext(t)

	// Create test appointments
	app1 := domain.Appointment{
		ID:          "app-1",
		Description: "First appointment",
		EventDate:   time.Now().Format(time.RFC3339),
	}
	app2 := domain.Appointment{
		ID:          "app-2",
		Description: "Second appointment",
		EventDate:   time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}

	if err := ctx.Store.CreateAppointment(app1); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Store.CreateAppointment(app2); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/appointments", nil)
	rr := httptest.NewRecorder()

	ctx.HandleGetAppointments(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var got []domain.Appointment
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 {
		t.Errorf("expected 2 appointments, got %d", len(got))
	}
}

func TestHandleCreateAppointment(t *testing.T) {
	ctx := newTestContext(t)

	app := domain.Appointment{
		Description: "New appointment #ueeepa",
		EventDate:   time.Now().Format(time.RFC3339),
		Year:        2026,
		Month:       6,
		WeekNumber:  26,
	}

	body, err := json.Marshal(app)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/api/appointments/create", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	ctx.HandleCreateAppointment(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	var created domain.Appointment
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	if created.ID == "" {
		t.Error("expected CUID2 ID to be generated and returned")
	}

	// Verify it exists in db
	dbApps, err := ctx.Store.GetAppointments()
	if err != nil {
		t.Fatal(err)
	}
	if len(dbApps) != 1 || dbApps[0].Description != "New appointment #ueeepa" {
		t.Errorf("expected created appointment in DB, got: %v", dbApps)
	}
}

func TestHandleUpdateAppointment(t *testing.T) {
	ctx := newTestContext(t)

	app := domain.Appointment{
		ID:          "test-app-id",
		Description: "Old Description",
		EventDate:   time.Now().Format(time.RFC3339),
	}
	if err := ctx.Store.CreateAppointment(app); err != nil {
		t.Fatal(err)
	}

	app.Description = "New Description #updated"
	body, err := json.Marshal(app)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/api/appointments/update", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	ctx.HandleUpdateAppointment(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	dbApps, err := ctx.Store.GetAppointments()
	if err != nil {
		t.Fatal(err)
	}
	if len(dbApps) != 1 || dbApps[0].Description != "New Description #updated" {
		t.Errorf("expected updated description in DB, got: %v", dbApps)
	}
}

func TestHandleDeleteAppointment(t *testing.T) {
	ctx := newTestContext(t)

	app := domain.Appointment{
		ID:          "test-app-to-delete",
		Description: "To be deleted",
		EventDate:   time.Now().Format(time.RFC3339),
	}
	if err := ctx.Store.CreateAppointment(app); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("DELETE", "/api/appointments/delete?id=test-app-to-delete", nil)
	rr := httptest.NewRecorder()

	ctx.HandleDeleteAppointment(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	dbApps, err := ctx.Store.GetAppointments()
	if err != nil {
		t.Fatal(err)
	}
	if len(dbApps) != 0 {
		t.Errorf("expected DB to be empty, got: %v", dbApps)
	}
}

func TestHandlePurgeOldAppointments(t *testing.T) {
	ctx := newTestContext(t)

	recentApp := domain.Appointment{
		ID:          "recent",
		Description: "Recent App",
		EventDate:   time.Now().Format(time.RFC3339),
	}
	oldApp := domain.Appointment{
		ID:          "old",
		Description: "Old App",
		EventDate:   time.Now().Add(-10 * 24 * time.Hour).Format(time.RFC3339),
	}

	if err := ctx.Store.CreateAppointment(recentApp); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Store.CreateAppointment(oldApp); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("DELETE", "/api/appointments/purge-old", nil)
	rr := httptest.NewRecorder()

	ctx.HandlePurgeOldAppointments(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	dbApps, err := ctx.Store.GetAppointments()
	if err != nil {
		t.Fatal(err)
	}
	if len(dbApps) != 1 || dbApps[0].ID != "recent" {
		t.Errorf("expected only recent app to remain, got: %v", dbApps)
	}
}

func TestHandleGetAgendaTree(t *testing.T) {
	ctx := newTestContext(t)

	// Create test appointments
	// Friday 26th June 2026 is week 26
	app1 := domain.Appointment{
		ID:          "tree-app-1",
		Description: "Friday item #ueeepa",
		EventDate:   "2026-06-26T15:52:20.533Z",
	}
	// Saturday 27th June 2026 is week 26
	app2 := domain.Appointment{
		ID:          "tree-app-2",
		Description: "Saturday item #ueepa",
		EventDate:   "2026-06-27T20:00:00.000Z",
	}

	if err := ctx.Store.CreateAppointment(app1); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Store.CreateAppointment(app2); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/appointments/tree", nil)
	rr := httptest.NewRecorder()

	ctx.HandleGetAgendaTree(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	bodyStr := rr.Body.String()

	// Verify that the tree HTML contains Year, Month (Junho) and Week (Semana 26) structure
	if !strings.Contains(bodyStr, "Ano 2026") {
		t.Error("expected HTML to contain 'Ano 2026'")
	}
	if !strings.Contains(bodyStr, "Junho") {
		t.Error("expected HTML to contain 'Junho'")
	}
	if !strings.Contains(bodyStr, "Semana 26") {
		t.Error("expected HTML to contain 'Semana 26'")
	}
	if !strings.Contains(bodyStr, "Friday item") {
		t.Error("expected HTML to contain 'Friday item'")
	}
	if !strings.Contains(bodyStr, "Saturday item") {
		t.Error("expected HTML to contain 'Saturday item'")
	}
}

func TestHandleAgendaPage(t *testing.T) {
	ctx := newTestContext(t)

	req := httptest.NewRequest("GET", "/agenda", nil)
	rr := httptest.NewRecorder()

	ctx.HandleAgendaPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "id=\"agenda-timeline\"") {
		t.Error("expected agenda-timeline container in page output")
	}
}
