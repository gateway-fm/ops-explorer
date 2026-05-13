package api

import (
	"net/http"

	"explorer/pkg/log"
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
		log.Error("ws: connection upgrade failed", "error", err, "remote", r.RemoteAddr)
		return
	}

	client := ws.NewClient(s.wsHub, conn, s.wsConfig)

	s.wsHub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}
