package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"explorer/internal/log"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	send          chan []byte
	topics        map[string]bool
	config        *Config
	mu            sync.Mutex
	closed        bool
	Authenticated bool
}

type ClientMessage struct {
	Action string   `json:"action"` // subscribe, unsubscribe
	Topics []string `json:"topics"`
}

func NewClient(hub *Hub, conn *websocket.Conn, config *Config) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		topics: make(map[string]bool),
		config: config,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
	}()

	c.conn.SetReadLimit(c.config.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warn("websocket error", "error", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if !ok {
						c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(message []byte) {
	var msg ClientMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Warn("invalid client message", "error", err)
		c.sendError("invalid message format")
		return
	}

	switch msg.Action {
	case "subscribe":
		for _, topic := range msg.Topics {
			if !isValidTopic(topic) {
				c.sendError("invalid topic: " + topic)
				continue
			}
			// Block unauthenticated clients from subscribing to sensitive topics
			if !c.Authenticated && c.hub.privacyEnabled && isPrivacyGatedTopic(topic) {
				c.sendError("authentication required for topic: " + topic)
				continue
			}
			c.hub.Subscribe(c, topic)
			c.sendSuccess("subscribed", topic)
		}

	case "unsubscribe":
		for _, topic := range msg.Topics {
			c.hub.Unsubscribe(c, topic)
			c.sendSuccess("unsubscribed", topic)
		}

	default:
		c.sendError("unknown action: " + msg.Action)
	}
}

func isValidTopic(topic string) bool {
	validTopics := map[string]bool{
		"blocks":       true,
		"transactions": true,
		"price":        true,
		"sync":         true,
	}

	if validTopics[topic] {
		return true
	}

	// address-specific topics (address:0x...)
	if len(topic) > 8 && topic[:8] == "address:" {
		return true
	}

	return false
}

func (c *Client) sendError(message string) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type":    "error",
		"message": message,
	})
	c.send <- msg
}

func (c *Client) sendSuccess(action, topic string) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type":   "success",
		"action": action,
		"topic":  topic,
	})
	c.send <- msg
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}
	c.closed = true

	close(c.send)
}

func (c *Client) Topics() []string {
	topics := make([]string, 0, len(c.topics))
	for topic := range c.topics {
		topics = append(topics, topic)
	}
	return topics
}

// isPrivacyGatedTopic returns true for topics that require authentication when privacy is enabled.
func isPrivacyGatedTopic(topic string) bool {
	if topic == "transactions" {
		return true
	}
	if len(topic) > 8 && topic[:8] == "address:" {
		return true
	}
	return false
}
