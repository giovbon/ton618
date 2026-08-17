package services

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"ton618/core/internal/core/db"
	"ton618/core/internal/core/domain"
)

func newTestStore(t *testing.T) (*db.Store, func()) {
	// Cria banco de dados temporário
	dbPath := fmt.Sprintf("test_ntfy_%d.db", time.Now().UnixNano())
	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Remove(dbPath)
	}

	return store, cleanup
}

func TestNtfyService(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	// Mock do servidor Ntfy
	var mu sync.Mutex
	var receivedRequests []struct {
		Body     string
		Headers  http.Header
		Endpoint string
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		mu.Lock()
		receivedRequests = append(receivedRequests, struct {
			Body     string
			Headers  http.Header
			Endpoint string
		}{
			Body:     string(bodyBytes),
			Headers:  r.Header,
			Endpoint: r.URL.Path,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Configura as credenciais de teste no banco
	store.SetSetting("ntfy_url", server.URL)
	store.SetSetting("ntfy_topic", "test_topic")
	store.SetSetting("ntfy_user", "test_user")
	store.SetSetting("ntfy_pass", "test_pass")

	svc := NewNtfyService(store)

	// Adiciona agendamentos
	// Data de referência de teste: 2026-07-07T15:00 (dentro da janela de lead
	// de 24h do app1, que é 07-07T14:30 → 07-08T14:30)
	refTime := time.Date(2026, 7, 7, 15, 0, 0, 0, time.UTC)

	// Agendamento 1: 2026-07-08T14:30 -> Deve disparar (dentro da janela de 24h antes)
	app1 := domain.Appointment{
		ID:          "app_tomorrow",
		Description: "Reunião de Alinhamento",
		EventDate:   "2026-07-08T14:30:00",
	}
	// Agendamento 2: 2026-07-09T09:00 -> Não deve disparar (janela começa 07-08T09:00)
	app2 := domain.Appointment{
		ID:          "app_after_tomorrow",
		Description: "Dentista",
		EventDate:   "2026-07-09T09:00:00",
	}

	store.CreateAppointment(app1)
	store.CreateAppointment(app2)

	// --- 1. Teste de Lembrete por lead time (24h antes do evento) ---
	svc.checkAndSendEventRemindersAt(refTime)

	mu.Lock()
	reqCount := len(receivedRequests)
	mu.Unlock()

	if reqCount != 1 {
		t.Fatalf("esperava 1 requisição no ntfy, obteve %d", reqCount)
	}

	mu.Lock()
	req := receivedRequests[0]
	mu.Unlock()

	if req.Endpoint != "/test_topic" {
		t.Errorf("endpoint incorreto: %s", req.Endpoint)
	}
	authHeader := req.Headers.Get("Authorization")
	if authHeader == "" {
		t.Errorf("esperava header Authorization de Basic Auth")
	}
	if req.Headers.Get("Priority") != "default" {
		t.Errorf("esperava prioridade default (média) para lembretes, obteve %s", req.Headers.Get("Priority"))
	}
	if req.Headers.Get("Tags") != "calendar" {
		t.Errorf("esperava tag calendar, obteve %s", req.Headers.Get("Tags"))
	}

	// Verifica se registrou no banco
	sent, err := store.HasNotificationBeenSent("lead_24h_app_tomorrow_20260708_1430")
	if err != nil {
		t.Fatalf("erro ao checar banco: %v", err)
	}
	if !sent {
		t.Error("esperava que a notificação estivesse registrada como enviada no banco de dados")
	}

	// Tenta rodar novamente para ver se bloqueia envio duplicado
	svc.checkAndSendEventRemindersAt(refTime)
	
	mu.Lock()
	reqCountAfter := len(receivedRequests)
	mu.Unlock()

	if reqCountAfter != 1 {
		t.Errorf("esperava que não enviasse novamente, mas enviou. total de requisições: %d", reqCountAfter)
	}

	// --- 2. Teste do Resumo Semanal ---
	// Data de referência de teste para domingo: 2026-07-05 (Domingo)
	// A próxima semana (segunda a domingo) vai de 2026-07-06 a 2026-07-12
	sundayRef := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)

	// Reseta requisições recebidas
	mu.Lock()
	receivedRequests = nil
	mu.Unlock()

	svc.checkAndSendWeeklySummaryAt(sundayRef)

	mu.Lock()
	reqCountWeekly := len(receivedRequests)
	mu.Unlock()

	if reqCountWeekly != 1 {
		t.Fatalf("esperava 1 requisição para resumo semanal, obteve %d", reqCountWeekly)
	}

	mu.Lock()
	reqW := receivedRequests[0]
	mu.Unlock()

	if reqW.Headers.Get("Priority") != "high" {
		t.Errorf("esperava prioridade high (alta) para resumo semanal, obteve %s", reqW.Headers.Get("Priority"))
	}
	if reqW.Headers.Get("Tags") != "calendar,clipboard" {
		t.Errorf("esperava tags calendar,clipboard, obteve %s", reqW.Headers.Get("Tags"))
	}

	// O corpo deve listar os dois agendamentos (Reunião e Dentista) que caem na semana de 06 a 12
	if !contains(reqW.Body, "Reunião de Alinhamento") || !contains(reqW.Body, "Dentista") {
		t.Errorf("resumo semanal não continha as reuniões esperadas. corpo obtido: %s", reqW.Body)
	}

	// Verifica se registrou a semana no banco
	// ISO Week de 2026-07-06 (Segunda) é a semana 28
	sentW, err := store.HasNotificationBeenSent("weekly_2026_28")
	if err != nil {
		t.Fatalf("erro ao checar banco para resumo semanal: %v", err)
	}
	if !sentW {
		t.Error("esperava que o resumo semanal estivesse registrado no banco de dados")
	}
}

func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || (len(substr) > 0 && (str[:len(substr)] == substr || contains(str[1:], substr))))
}

// TestNtfy_LeadTimePrevisivel garante a previsibilidade dos lembretes:
// o evento é notificado na janela [evento − lead, evento), nunca antes nem
// depois, com dedup e lead configurável (agenda_notify_hours).
func TestNtfy_LeadTimePrevisivel(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		mu.Lock()
		requests++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store.SetSetting("ntfy_url", server.URL)
	store.SetSetting("ntfy_topic", "test_topic")

	svc := NewNtfyService(store)

	app := domain.Appointment{
		ID:          "app_lead",
		Description: "Consulta",
		EventDate:   "2026-07-08T14:30:00",
	}
	store.CreateAppointment(app)

	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return requests
	}

	// 1. Antes da janela (evento − 24h = 07-07T14:30) → NÃO dispara
	svc.checkAndSendEventRemindersAt(time.Date(2026, 7, 7, 14, 0, 0, 0, time.UTC))
	if c := count(); c != 0 {
		t.Fatalf("antes da janela deveria ter 0 notificações, got %d", c)
	}

	// 2. No início da janela (07-07T14:30 = evento − 24h) → dispara
	svc.checkAndSendEventRemindersAt(time.Date(2026, 7, 7, 14, 30, 0, 0, time.UTC))
	if c := count(); c != 1 {
		t.Fatalf("na janela deveria ter 1 notificação, got %d", c)
	}

	// 3. Dedup: chamadas seguintes dentro da janela não re-enviam
	svc.checkAndSendEventRemindersAt(time.Date(2026, 7, 7, 15, 0, 0, 0, time.UTC))
	if c := count(); c != 1 {
		t.Fatalf("dedup falhou: got %d", c)
	}

	// 4. Depois do evento → NÃO dispara (sem lembrete atrasado)
	svc.checkAndSendEventRemindersAt(time.Date(2026, 7, 8, 15, 0, 0, 0, time.UTC))
	if c := count(); c != 1 {
		t.Fatalf("depois do evento não deveria enviar, got %d", c)
	}

	// 5. Lead configurável (2h): evento 2026-07-09T10:00 → janela [08:00, 10:00)
	store.SetSetting("agenda_notify_hours", "2")
	app2 := domain.Appointment{ID: "app_2h", Description: "Reunião 2h", EventDate: "2026-07-09T10:00:00"}
	store.CreateAppointment(app2)

	svc.checkAndSendEventRemindersAt(time.Date(2026, 7, 9, 7, 59, 0, 0, time.UTC))
	if c := count(); c != 1 {
		t.Fatalf("2h: antes da janela deveria ter 1 (só o 1º evento), got %d", c)
	}

	svc.checkAndSendEventRemindersAt(time.Date(2026, 7, 9, 8, 30, 0, 0, time.UTC))
	if c := count(); c != 2 {
		t.Fatalf("2h: na janela deveria ter 2 notificações, got %d", c)
	}

	svc.checkAndSendEventRemindersAt(time.Date(2026, 7, 9, 9, 0, 0, 0, time.UTC))
	if c := count(); c != 2 {
		t.Fatalf("2h: dedup falhou, got %d", c)
	}
}

// TestNtfy_LeadHours_PadraoEConfiguravel valida o leitor da configuração de lead.
func TestNtfy_LeadHours_PadraoEConfiguravel(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	svc := NewNtfyService(store)

	// Sem configuração → padrão 24
	if got := svc.leadHours(); got != 24 {
		t.Fatalf("padrão deveria ser 24, got %d", got)
	}

	store.SetSetting("agenda_notify_hours", "6")
	if got := svc.leadHours(); got != 6 {
		t.Fatalf("esperava 6, got %d", got)
	}

	store.SetSetting("agenda_notify_hours", "0")
	if got := svc.leadHours(); got != 24 {
		t.Fatalf("valor inválido (0) deveria cair para 24, got %d", got)
	}

	store.SetSetting("agenda_notify_hours", "abc")
	if got := svc.leadHours(); got != 24 {
		t.Fatalf("valor inválido deveria cair para 24, got %d", got)
	}
}
