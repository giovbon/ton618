package events

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
)

// Event representa uma mensagem SSE
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Hub gerencia as conexões de clientes SSE
type Hub struct {
	mu      sync.RWMutex
	clients map[chan Event]bool
}

var globalHub *Hub
var once sync.Once

// GetHub retorna a instância única do Hub (Singleton)
func GetHub() *Hub {
	once.Do(func() {
		globalHub = &Hub{
			clients: make(map[chan Event]bool),
		}
	})
	return globalHub
}

// Register adiciona um novo cliente ao Hub
func (h *Hub) Register() chan Event {
	ch := make(chan Event, 10)
	h.mu.Lock()
	h.clients[ch] = true
	h.mu.Unlock()
	return ch
}

// Unregister remove um cliente do Hub
func (h *Hub) Unregister(ch chan Event) {
	h.mu.Lock()
	delete(h.clients, ch)
	close(ch)
	h.mu.Unlock()
}

// Broadcast envia um evento para todos os clientes conectados
func (h *Hub) Broadcast(eventType string, data interface{}) {
	event := Event{
		Type: eventType,
		Data: data,
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.clients) == 0 {
		return
	}

	log.Printf("[SSE] Broadfcasting: %s\n", eventType)

	for ch := range h.clients {
		select {
		case ch <- event:
		default:
			// Se o canal estiver cheio, ignoramos este cliente específico (ou poderíamos removê-lo)
		}
	}
}

// FormatSSE formata o evento para o protocolo SSE
func FormatSSE(event Event) ([]byte, error) {
	payload, err := json.Marshal(event.Data)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event.Type, string(payload))), nil
}
