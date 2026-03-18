package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"explorer/internal/events"
	"explorer/internal/log"
)

type Hub struct {
	clients        map[*Client]bool
	topics         map[string]map[*Client]bool
	register       chan *Client
	unregister     chan *Client
	broadcast      chan *BroadcastMessage
	eventBus       *events.Bus
	maxConnections int
	privacyEnabled bool
	mu             sync.RWMutex
	done           chan struct{}
}

type BroadcastMessage struct {
	Topic   string
	Message []byte
}

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

// SetPrivacyEnabled configures whether the hub enforces privacy restrictions.
func (h *Hub) SetPrivacyEnabled(enabled bool) {
	h.privacyEnabled = enabled
}

func (h *Hub) Run(ctx context.Context) {
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

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.clients) >= h.maxConnections {
		log.Warn("max connections reached, rejecting client")
		client.Close()
		return
	}

	h.clients[client] = true
	log.Debug("client registered", "total", len(h.clients))
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)

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

func (h *Hub) Subscribe(client *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.topics[topic]; !ok {
		h.topics[topic] = make(map[*Client]bool)
	}
	h.topics[topic][client] = true
	client.topics[topic] = true
}

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

func (h *Hub) broadcastToTopic(topic string, message []byte) {
	h.mu.RLock()
	clients, ok := h.topics[topic]
	if !ok {
		h.mu.RUnlock()
		return
	}

	// Copy to avoid holding lock during send
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

	switch event.Type {
	case events.EventBlockNew:
		if h.privacyEnabled {
			h.broadcastBlockEventWithPrivacy(event.Data, msg)
		} else {
			h.broadcastToTopic("blocks", msg)
		}
	case events.EventTxNew:
		if h.privacyEnabled {
			h.broadcastToTopicAuthenticated("transactions", msg)
		} else {
			h.broadcastToTopic("transactions", msg)
		}
	case events.EventPriceUpdate:
		h.broadcastToTopic("price", msg)
	case events.EventSyncStatus:
		h.broadcastToTopic("sync", msg)
	case events.EventAddressActivity:
		var data struct {
			Address string `json:"address"`
		}
		if err := json.Unmarshal(event.Data, &data); err == nil {
			if h.privacyEnabled {
				h.broadcastToTopicAuthenticated("address:"+data.Address, msg)
			} else {
				h.broadcastToTopic("address:"+data.Address, msg)
			}
		}
	}
}

// broadcastToTopicAuthenticated sends messages only to authenticated clients subscribed to a topic.
func (h *Hub) broadcastToTopicAuthenticated(topic string, message []byte) {
	h.mu.RLock()
	clients, ok := h.topics[topic]
	if !ok {
		h.mu.RUnlock()
		return
	}

	clientsCopy := make([]*Client, 0, len(clients))
	for client := range clients {
		if client.Authenticated {
			clientsCopy = append(clientsCopy, client)
		}
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

// broadcastBlockEventWithPrivacy sends full block data to authenticated clients
// and stripped block data (number, hash, timestamp only) to unauthenticated clients.
func (h *Hub) broadcastBlockEventWithPrivacy(eventData json.RawMessage, fullMsg []byte) {
	h.mu.RLock()
	clients, ok := h.topics["blocks"]
	if !ok {
		h.mu.RUnlock()
		return
	}

	var authClients, unauthClients []*Client
	for client := range clients {
		if client.Authenticated {
			authClients = append(authClients, client)
		} else {
			unauthClients = append(unauthClients, client)
		}
	}
	h.mu.RUnlock()

	// Send full data to authenticated clients
	for _, client := range authClients {
		select {
		case client.send <- fullMsg:
		default:
			go h.Unregister(client)
		}
	}

	// Build stripped message for unauthenticated clients
	if len(unauthClients) > 0 {
		var blockData struct {
			Number    uint64 `json:"number"`
			Hash      string `json:"hash"`
			Timestamp uint64 `json:"timestamp"`
		}
		if err := json.Unmarshal(eventData, &blockData); err != nil {
			return
		}
		strippedMsg, err := json.Marshal(map[string]interface{}{
			"type":  "event",
			"topic": "blocks",
			"data":  blockData,
		})
		if err != nil {
			return
		}
		for _, client := range unauthClients {
			select {
			case client.send <- strippedMsg:
			default:
				go h.Unregister(client)
			}
		}
	}
}

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

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) TopicCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.topics)
}

type Config struct {
	MaxConnections int
	PingInterval   time.Duration
	WriteWait      time.Duration
	PongWait       time.Duration
	MaxMessageSize int64
}

func DefaultConfig() *Config {
	return &Config{
		MaxConnections: 10000,
		PingInterval:   30 * time.Second,
		WriteWait:      10 * time.Second,
		PongWait:       60 * time.Second,
		MaxMessageSize: 512,
	}
}
