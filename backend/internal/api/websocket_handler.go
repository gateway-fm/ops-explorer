package api

import (
	"net/http"

	"explorer/internal/log"
	ws "explorer/internal/websocket"

	gorillaWS "github.com/gorilla/websocket"
)

var upgrader = gorillaWS.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// TODO: restrict origins in production
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

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
	client.Authenticated = s.isPrivacyAuthenticated(r)

	s.wsHub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}
