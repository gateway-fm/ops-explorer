package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"explorer/internal/events"
	"explorer/internal/log"
)

// Hub maintains the set of active clients and broadcasts messages to clients
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Clients indexed by subscription topic
	topics map[string]map[*Client]bool

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Broadcast channel for topic-based messages
	broadcast chan *BroadcastMessage

	// Event bus for receiving events
	eventBus *events.Bus

	// Max connections
	maxConnections int

	// Mutex for client map
	mu sync.RWMutex

	// Shutdown signal
	done chan struct{}
}

// BroadcastMessage is a message to broadcast to a specific topic
type BroadcastMessage struct {
	Topic   string
	Message []byte
}

// NewHub creates a new Hub
func NewHub(eventBus *events.Bus, maxConnections int) *Hub {
	return &Hub{
		clients:        make(map[*Client]bool),
		topics:         make(map[string]map[*Client]bool),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		broadcast:      make(chan *BroadcastMessage, 256),
		eventBus:       eventBus,
		maxConnections: maxConnections,
		done:           make(chan struct{}),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run(ctx context.Context) {
	// Subscribe to events
	eventChan := h.eventBus.SubscribeAll()

	for {
		select {
		case <-ctx.Done():
			close(h.done)
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastToTopic(message.Topic, message.Message)

		case event := <-eventChan:
			h.handleEvent(event)
		}
	}
}

// registerClient adds a client to the hub
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check max connections
	if len(h.clients) >= h.maxConnections {
		log.Warn("max connections reached, rejecting client")
		client.Close()
		return
	}

	h.clients[client] = true
	log.Debug("client registered", "total", len(h.clients))
}

// unregisterClient removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)

		// Remove from all topics
		for topic, clients := range h.topics {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.topics, topic)
			}
		}

		client.Close()
		log.Debug("client unregistered", "total", len(h.clients))
	}
}

// Subscribe adds a client to a topic
func (h *Hub) Subscribe(client *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.topics[topic]; !ok {
		h.topics[topic] = make(map[*Client]bool)
	}
	h.topics[topic][client] = true
	client.topics[topic] = true
}

// Unsubscribe removes a client from a topic
func (h *Hub) Unsubscribe(client *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.topics[topic]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.topics, topic)
		}
	}
	delete(client.topics, topic)
}

// broadcastToTopic sends a message to all clients subscribed to a topic
func (h *Hub) broadcastToTopic(topic string, message []byte) {
	h.mu.RLock()
	clients, ok := h.topics[topic]
	if !ok {
		h.mu.RUnlock()
		return
	}

	// Copy clients to avoid holding lock during send
	clientsCopy := make([]*Client, 0, len(clients))
	for client := range clients {
		clientsCopy = append(clientsCopy, client)
	}
	h.mu.RUnlock()

	for _, client := range clientsCopy {
		select {
		case client.send <- message:
		default:
			// Client's buffer is full, mark for closure
			go h.Unregister(client)
		}
	}
}

// broadcastToAll sends a message to all connected clients
func (h *Hub) broadcastToAll(message []byte) {
	h.mu.RLock()
	clientsCopy := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clientsCopy = append(clientsCopy, client)
	}
	h.mu.RUnlock()

	for _, client := range clientsCopy {
		select {
		case client.send <- message:
		default:
			go h.Unregister(client)
		}
	}
}

// handleEvent processes events from the event bus and broadcasts to appropriate topics
func (h *Hub) handleEvent(event *events.Event) {
	msg, err := json.Marshal(map[string]interface{}{
		"type":  "event",
		"topic": eventTypeToTopic(event.Type),
		"data":  json.RawMessage(event.Data),
	})
	if err != nil {
		log.Error("failed to marshal event", "error", err)
		return
	}

	// Determine which topics to broadcast to based on event type
	switch event.Type {
	case events.EventBlockNew:
		h.broadcastToTopic("blocks", msg)
	case events.EventTxNew:
		h.broadcastToTopic("transactions", msg)
	case events.EventPriceUpdate:
		h.broadcastToTopic("price", msg)
	case events.EventSyncStatus:
		h.broadcastToTopic("sync", msg)
	case events.EventAddressActivity:
		// Parse the address from the event data
		var data struct {
			Address string `json:"address"`
		}
		if err := json.Unmarshal(event.Data, &data); err == nil {
			h.broadcastToTopic("address:"+data.Address, msg)
		}
	}
}

// eventTypeToTopic converts an event type to a topic name
func eventTypeToTopic(et events.EventType) string {
	switch et {
	case events.EventBlockNew:
		return "blocks"
	case events.EventTxNew:
		return "transactions"
	case events.EventPriceUpdate:
		return "price"
	case events.EventSyncStatus:
		return "sync"
	case events.EventAddressActivity:
		return "address"
	default:
		return string(et)
	}
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// TopicCount returns the number of active topics
func (h *Hub) TopicCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.topics)
}

// Config holds WebSocket hub configuration
type Config struct {
	MaxConnections int
	PingInterval   time.Duration
	WriteWait      time.Duration
	PongWait       time.Duration
	MaxMessageSize int64
}

// DefaultConfig returns the default hub configuration
func DefaultConfig() *Config {
	return &Config{
		MaxConnections: 10000,
		PingInterval:   30 * time.Second,
		WriteWait:      10 * time.Second,
		PongWait:       60 * time.Second,
		MaxMessageSize: 512,
	}
}
