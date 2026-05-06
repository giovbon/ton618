package api

import (
	"etl/internal/events"
	"log"
	"net/http"
)

// HandleEvents gerencia conexões SSE (Server-Sent Events)
func (ctx *HandlerContext) HandleEvents(w http.ResponseWriter, r *http.Request) {
	// 1. Headers necessários para SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	hub := events.GetHub()
	clientChan := hub.Register()
	defer hub.Unregister(clientChan)

	// Garante que o flush funciona (enviar dados imediatamente)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming não suportado", http.StatusInternalServerError)
		return
	}

	// Notificar conexão estabelecida
	initialMsg, _ := events.FormatSSE(events.Event{Type: "connected", Data: "Welcome to TON-618"})
	w.Write(initialMsg)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-clientChan:
			msg, err := events.FormatSSE(event)
			if err != nil {
				log.Printf("[SSE] Erro ao formatar evento: %v\n", err)
				continue
			}
			_, err = w.Write(msg)
			if err != nil {
				log.Printf("[SSE] Erro ao enviar dado (cliente provavelmente fechou): %v\n", err)
				return
			}
			flusher.Flush()
		}
	}
}
