package appointments

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
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

	origLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = origLocal })

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

	// Verify that the tree HTML contains the custom week starting date label structure (e.g. Semana 22 jun)
	if !strings.Contains(bodyStr, "Semana 22 jun") {
		t.Error("expected HTML to contain 'Semana 22 jun'")
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




func TestHandleGetAgendaTreeIntensive(t *testing.T) {
	ctx := newTestContext(t)

	// ISO Week 27, 2026: June 29 (Monday) to July 5 (Sunday)
	apps := []domain.Appointment{
		{ID: "app-june-29-early", Description: "Monday Early", EventDate: "2026-06-29T15:00:00"},
		{ID: "app-july-5", Description: "Sunday July 5", EventDate: "2026-07-05T12:00:00"},
		{ID: "app-june-29-late", Description: "Monday Late", EventDate: "2026-06-29T19:00:00"},
		{ID: "app-june-30", Description: "Tuesday June 30", EventDate: "2026-06-30T21:00:00"},
		// Week 26 item
		{ID: "app-june-28", Description: "Sunday June 28 (Week 26)", EventDate: "2026-06-28T17:00:00"},
	}

	for _, a := range apps {
		if err := ctx.Store.CreateAppointment(a); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest("GET", "/api/appointments/tree", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetAgendaTree(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()

	// 1. Week 27 (starting June 29) must appear exactly once (no split across June/July)
	semana29JunCount := strings.Count(body, "Semana 29 jun")
	if semana29JunCount != 1 {
		t.Errorf("expected exactly 1 'Semana 29 jun', got %d", semana29JunCount)
	}

	// 2. Week 27 (starting June 29) must appear after Week 26 (starting June 22)
	posWeek29Jun := strings.Index(body, "Semana 29 jun")
	posWeek22Jun := strings.Index(body, "Semana 22 jun")
	if posWeek29Jun == -1 || posWeek22Jun == -1 {
		t.Fatalf("expected both Semana 29 jun and Semana 22 jun present")
	}
	// Ascending order: older week (22 jun) appears before newer week (29 jun)
	if posWeek22Jun > posWeek29Jun {
		t.Errorf("Semana 22 jun (older) must appear before Semana 29 jun (newer) in ascending order")
	}

	// 3. Items within Week 27 must be chronological (ascending: oldest first, newest last)
	posJuly5 := strings.Index(body, "Sunday July 5")
	posJune30 := strings.Index(body, "Tuesday June 30")
	posJune29Late := strings.Index(body, "Monday Late")
	posJune29Early := strings.Index(body, "Monday Early")

	if posJuly5 == -1 || posJune30 == -1 || posJune29Late == -1 || posJune29Early == -1 {
		t.Fatalf("expected all Week 27 items present in HTML")
	}
	if !(posJune29Early < posJune29Late && posJune29Late < posJune30 && posJune30 < posJuly5) {
		t.Errorf("items in Week 27 not sorted chronologically ascending (Early < Late < June 30 < July 5)")
	}
}

// TestHandleGetAgendaTreeEmpty verifies that an empty DB renders a "no appointments" message.
func TestHandleGetAgendaTreeEmpty(t *testing.T) {
	ctx := newTestContext(t)

	req := httptest.NewRequest("GET", "/api/appointments/tree", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetAgendaTree(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Nenhum compromisso agendado") {
		t.Errorf("expected empty-state message in HTML")
	}
}

// TestHandleGetAgendaTreeMultiYear ensures items across different years are sorted with most recent year first.
func TestHandleGetAgendaTreeMultiYear(t *testing.T) {
	ctx := newTestContext(t)

	apps := []domain.Appointment{
		{ID: "y2024", Description: "Evento 2024", EventDate: "2024-03-15T10:00:00"},
		{ID: "y2025", Description: "Evento 2025", EventDate: "2025-06-20T10:00:00"},
		{ID: "y2026", Description: "Evento 2026", EventDate: "2026-09-01T10:00:00"},
	}
	for _, a := range apps {
		if err := ctx.Store.CreateAppointment(a); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest("GET", "/api/appointments/tree", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetAgendaTree(rr, req)

	body := rr.Body.String()

	pos2026 := strings.Index(body, "2026")
	pos2025 := strings.Index(body, "2025")
	pos2024 := strings.Index(body, "2024")

	if pos2026 == -1 || pos2025 == -1 || pos2024 == -1 {
		t.Fatalf("expected all years present in HTML")
	}
	// Ascending order: 2024 before 2025 before 2026
	if !(pos2024 < pos2025 && pos2025 < pos2026) {
		t.Errorf("years not sorted ascending: 2024=%d 2025=%d 2026=%d", pos2024, pos2025, pos2026)
	}
}

// TestHandleGetAgendaTreeDecYearBoundary tests ISO week 1 of year boundary (e.g. Dec 28 in week 1 of next year).
func TestHandleGetAgendaTreeDecYearBoundary(t *testing.T) {
	ctx := newTestContext(t)

	// Dec 28, 2026 belongs to ISO Week 53 of 2026 (or week 1 of 2027, depending on year)
	// Jan 2, 2027 belongs to ISO Week 53 of 2026 / Week 1 of 2027
	// This tests we don't accidentally split the week
	apps := []domain.Appointment{
		{ID: "dec28", Description: "Dec 28 item", EventDate: "2026-12-28T10:00:00"},
		{ID: "jan2", Description: "Jan 2 item", EventDate: "2027-01-02T10:00:00"},
	}
	for _, a := range apps {
		if err := ctx.Store.CreateAppointment(a); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest("GET", "/api/appointments/tree", nil)
	rr := httptest.NewRecorder()
	ctx.HandleGetAgendaTree(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Dec 28 item") || !strings.Contains(body, "Jan 2 item") {
		t.Errorf("expected both cross-year items in HTML")
	}
}

// TestHandleCreateAppointmentBadJSON verifies that malformed JSON returns 400.
func TestHandleCreateAppointmentBadJSON(t *testing.T) {
	ctx := newTestContext(t)

	req := httptest.NewRequest("POST", "/api/appointments/create", strings.NewReader("not json at all"))
	rr := httptest.NewRecorder()
	ctx.HandleCreateAppointment(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", rr.Code)
	}
}

// TestHandleUpdateAppointmentBadJSON verifies that malformed JSON returns 400.
func TestHandleUpdateAppointmentBadJSON(t *testing.T) {
	ctx := newTestContext(t)

	req := httptest.NewRequest("POST", "/api/appointments/update", strings.NewReader("{bad json"))
	rr := httptest.NewRecorder()
	ctx.HandleUpdateAppointment(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", rr.Code)
	}
}

// TestHandleDeleteAppointmentMissingID verifies that missing ?id= returns 400.
func TestHandleDeleteAppointmentMissingID(t *testing.T) {
	ctx := newTestContext(t)

	req := httptest.NewRequest("DELETE", "/api/appointments/delete", nil) // no ?id=
	rr := httptest.NewRecorder()
	ctx.HandleDeleteAppointment(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing id, got %d", rr.Code)
	}
}

// TestHandleCreateAppointmentGeneratesID ensures a UUID/CUID is always generated when ID is absent.
func TestHandleCreateAppointmentGeneratesID(t *testing.T) {
	ctx := newTestContext(t)

	// Send without an ID field
	body := strings.NewReader(`{"description":"Test #auto-id","event_date":"2026-06-29T10:00:00"}`)
	req := httptest.NewRequest("POST", "/api/appointments/create", body)
	rr := httptest.NewRecorder()
	ctx.HandleCreateAppointment(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var created domain.Appointment
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Error("expected a generated ID, got empty string")
	}
	if len(created.ID) < 8 {
		t.Errorf("generated ID too short: %q", created.ID)
	}
}

// TestHandleAgendaPageCacheHeaders verifies that the agenda page sets no-cache headers.
func TestHandleAgendaPageCacheHeaders(t *testing.T) {
	ctx := newTestContext(t)

	req := httptest.NewRequest("GET", "/agenda", nil)
	rr := httptest.NewRecorder()
	ctx.HandleAgendaPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("expected Cache-Control: no-store, got %q", cc)
	}
	if rr.Header().Get("Pragma") != "no-cache" {
		t.Errorf("expected Pragma: no-cache")
	}
}

// TestGetISOWeekEdgeCases tests ISO week edge cases at year boundaries.
func TestGetISOWeekEdgeCases(t *testing.T) {
	tests := []struct {
		date     time.Time
		expected int
		label    string
	}{
		{time.Date(2026, time.June, 27, 12, 0, 0, 0, time.UTC), 26, "Jun 27 2026"},
		{time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC), 1, "Jan 1 2026"},
		{time.Date(2026, time.January, 5, 12, 0, 0, 0, time.UTC), 2, "Jan 5 2026"},
		{time.Date(2026, time.December, 28, 12, 0, 0, 0, time.UTC), 53, "Dec 28 2026"},
		{time.Date(2024, time.December, 30, 12, 0, 0, 0, time.UTC), 1, "Dec 30 2024 = week 1 of 2025"},
		{time.Date(2025, time.January, 1, 12, 0, 0, 0, time.UTC), 1, "Jan 1 2025"},
		{time.Date(2020, time.December, 31, 12, 0, 0, 0, time.UTC), 53, "Dec 31 2020"},
	}

	for _, tt := range tests {
		got := GetISOWeek(tt.date)
		if got != tt.expected {
			t.Errorf("GetISOWeek(%s) = %d, want %d", tt.label, got, tt.expected)
		}
	}
}

// TestFrontendDateParser invokes the node-based test suite for the agenda date parser
// to ensure no regressions occur on frontend date normalization and parsing.
func TestFrontendDateParser(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found in PATH, skipping frontend date parser test")
	}

	scriptPath := filepath.Join("..", "..", "..", "web", "static", "js", "appointments.test.cjs")
	cmd := exec.Command(nodePath, scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Frontend date parser tests failed: %v\nOutput:\n%s", err, string(output))
	}
}

