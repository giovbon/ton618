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

	"ton618/internal/core/db"
	"ton618/internal/core/domain"
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
	// Data de referência de teste: 2026-07-07T12:00:00 (Terça-feira)
	refTime := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

	// Agendamento 1: Amanhã (2026-07-08) -> Deve disparar no diário
	app1 := domain.Appointment{
		ID:          "app_tomorrow",
		Description: "Reunião de Alinhamento",
		EventDate:   "2026-07-08T14:30:00",
	}
	// Agendamento 2: Depois de amanhã (2026-07-09) -> Não deve disparar no diário de hoje
	app2 := domain.Appointment{
		ID:          "app_after_tomorrow",
		Description: "Dentista",
		EventDate:   "2026-07-09T09:00:00",
	}

	store.CreateAppointment(app1)
	store.CreateAppointment(app2)

	// --- 1. Teste de Notificações Diárias (Véspera) ---
	svc.checkAndSendDailyAppointmentsAt(refTime)

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
		t.Errorf("esperava prioridade default (média) para diários, obteve %s", req.Headers.Get("Priority"))
	}
	if req.Headers.Get("Tags") != "calendar" {
		t.Errorf("esperava tag calendar, obteve %s", req.Headers.Get("Tags"))
	}

	// Verifica se registrou no banco
	sent, err := store.HasNotificationBeenSent("daily_app_tomorrow")
	if err != nil {
		t.Fatalf("erro ao checar banco: %v", err)
	}
	if !sent {
		t.Error("esperava que a notificação estivesse registrada como enviada no banco de dados")
	}

	// Tenta rodar novamente para ver se bloqueia envio duplicado
	svc.checkAndSendDailyAppointmentsAt(refTime)
	
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
