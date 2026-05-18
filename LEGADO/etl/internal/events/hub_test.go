package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestHubSingleton(t *testing.T) {
	h1 := GetHub()
	h2 := GetHub()

	if h1 != h2 {
		t.Error("GetHub should return the same instance")
	}
}

func TestHubRegister(t *testing.T) {
	hub := GetHub()
	ch := hub.Register()

	if ch == nil {
		t.Error("Register should return a non-nil channel")
	}

	hub.mu.RLock()
	registered := hub.clients[ch]
	hub.mu.RUnlock()

	if !registered {
		t.Error("channel should be registered in clients map")
	}

	hub.Unregister(ch)
}

func TestHubUnregister(t *testing.T) {
	hub := GetHub()
	ch := hub.Register()
	hub.Unregister(ch)

	hub.mu.RLock()
	_, exists := hub.clients[ch]
	hub.mu.RUnlock()

	if exists {
		t.Error("Unregister should remove the client")
	}

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after unregister")
		}
	default:
		// Expected: closed channel returns zero value with ok=false
		if _, ok := <-ch; ok {
			t.Error("channel should be closed")
		}
	}
}

func TestHubBroadcast(t *testing.T) {
	hub := GetHub()

	// Limpar clientes anteriores
	hub.mu.Lock()
	for ch := range hub.clients {
		delete(hub.clients, ch)
		close(ch)
	}
	hub.mu.Unlock()

	ch1 := hub.Register()
	ch2 := hub.Register()
	defer hub.Unregister(ch1)
	defer hub.Unregister(ch2)

	hub.Broadcast("test:event", map[string]string{"msg": "hello"})

	timeout := time.After(100 * time.Millisecond)

	for i := 0; i < 2; i++ {
		select {
		case evt := <-ch1:
			if evt.Type != "test:event" {
				t.Errorf("expected event type 'test:event', got %q", evt.Type)
			}
			data, _ := json.Marshal(evt.Data)
			if string(data) != `{"msg":"hello"}` {
				t.Errorf("expected data '{\"msg\":\"hello\"}', got %s", string(data))
			}
			// ch1 recebeu, verifica que ch2 também recebeu
		case evt := <-ch2:
			if evt.Type != "test:event" {
				t.Errorf("expected event type 'test:event', got %q", evt.Type)
			}
		case <-timeout:
			t.Error("timeout waiting for broadcast event")
		}
	}
}

func TestHubBroadcastNoClients(t *testing.T) {
	hub := &Hub{
		clients: make(map[chan Event]bool),
	}

	// Should not panic with no clients
	hub.Broadcast("test", "data")
}

func TestHubBroadcastSlowClient(t *testing.T) {
	hub := &Hub{
		clients: make(map[chan Event]bool),
	}

	// Canal com buffer 0 (sem buffer) → vai encher rapidamente
	ch := make(chan Event)
	hub.mu.Lock()
	hub.clients[ch] = true
	hub.mu.Unlock()
	defer func() {
		hub.mu.Lock()
		delete(hub.clients, ch)
		hub.mu.Unlock()
	}()

	// Broadcast deve funcionar mesmo com canal cheio (usa select default)
	for i := 0; i < 100; i++ {
		hub.Broadcast("test", "data") // não deve bloquear
	}
}

func TestFormatSSE(t *testing.T) {
	evt := Event{
		Type: "sync:finished",
		Data: map[string]interface{}{"new_docs": 5, "tags": 10},
	}

	result, err := FormatSSE(evt)
	if err != nil {
		t.Fatalf("FormatSSE should not error: %v", err)
	}

	expected := "event: sync:finished\ndata: {\"new_docs\":5,\"tags\":10}\n\n"
	if string(result) != expected {
		t.Errorf("FormatSSE mismatch.\nexpected: %q\ngot:      %q", expected, string(result))
	}
}

func TestFormatSSEStringData(t *testing.T) {
	evt := Event{
		Type: "ocr:started",
		Data: "processando imagem.png",
	}

	result, err := FormatSSE(evt)
	if err != nil {
		t.Fatalf("FormatSSE should not error: %v", err)
	}

	expected := "event: ocr:started\ndata: \"processando imagem.png\"\n\n"
	if string(result) != expected {
		t.Errorf("FormatSSE mismatch.\nexpected: %q\ngot:      %q", expected, string(result))
	}
}
