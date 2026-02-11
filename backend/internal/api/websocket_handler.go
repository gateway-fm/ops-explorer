package api

import (
	"net/http"

	"explorer/internal/log"
	ws "explorer/internal/websocket"

	gorillaWS "github.com/gorilla/websocket"
)

// Upgrader specifies parameters for upgrading an HTTP connection to a WebSocket connection
var upgrader = gorillaWS.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development
		// In production, you should check the origin
		return true
	},
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if s.wsHub == nil {
		http.Error(w, "WebSocket not available", http.StatusServiceUnavailable)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("websocket upgrade failed", "error", err)
		return
	}

	client := ws.NewClient(s.wsHub, conn, s.wsConfig)

	s.wsHub.Register(client)

	// Start read and write pumps in separate goroutines
	go client.WritePump()
	go client.ReadPump()
}
