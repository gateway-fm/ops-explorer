package events

import (
	"encoding/json"
	"sync"
)

// EventType represents the type of event
type EventType string

const (
	EventBlockNew      EventType = "block:new"
	EventTxNew         EventType = "tx:new"
	EventAddressActivity EventType = "address:activity"
	EventPriceUpdate   EventType = "price:update"
	EventSyncStatus    EventType = "sync:status"
)

// Event represents an event in the system
type Event struct {
	Type EventType       `json:"type"`
	Data json.RawMessage `json:"data"`
}

// NewEvent creates a new event with the given type and data
func NewEvent(eventType EventType, data interface{}) (*Event, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &Event{
		Type: eventType,
		Data: jsonData,
	}, nil
}

// Subscriber is a function that handles events
type Subscriber func(event *Event)

// Bus is a publish/subscribe event bus
type Bus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]chan *Event
	closed      bool
}

// NewBus creates a new event bus
func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[EventType][]chan *Event),
	}
}

// Subscribe subscribes to events of a specific type
// Returns a channel that will receive events
func (b *Bus) Subscribe(eventType EventType) <-chan *Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan *Event, 100) // Buffer to prevent blocking publishers
	b.subscribers[eventType] = append(b.subscribers[eventType], ch)
	return ch
}

// SubscribeAll subscribes to all event types
func (b *Bus) SubscribeAll() <-chan *Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan *Event, 100)

	// Subscribe to all known event types
	eventTypes := []EventType{
		EventBlockNew,
		EventTxNew,
		EventAddressActivity,
		EventPriceUpdate,
		EventSyncStatus,
	}

	for _, et := range eventTypes {
		b.subscribers[et] = append(b.subscribers[et], ch)
	}

	return ch
}

// Unsubscribe removes a subscriber channel
func (b *Bus) Unsubscribe(ch <-chan *Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for eventType, subs := range b.subscribers {
		for i, sub := range subs {
			if sub == ch {
				b.subscribers[eventType] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
}

// Publish sends an event to all subscribers of that event type
func (b *Bus) Publish(event *Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	subs, ok := b.subscribers[event.Type]
	if !ok {
		return
	}

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// Channel is full, skip to avoid blocking
		}
	}
}

// PublishNewBlock publishes a new block event
func (b *Bus) PublishNewBlock(block interface{}) error {
	event, err := NewEvent(EventBlockNew, block)
	if err != nil {
		return err
	}
	b.Publish(event)
	return nil
}

// PublishNewTransaction publishes a new transaction event
func (b *Bus) PublishNewTransaction(tx interface{}) error {
	event, err := NewEvent(EventTxNew, tx)
	if err != nil {
		return err
	}
	b.Publish(event)
	return nil
}

// PublishAddressActivity publishes an address activity event
func (b *Bus) PublishAddressActivity(address string, activity interface{}) error {
	data := map[string]interface{}{
		"address":  address,
		"activity": activity,
	}
	event, err := NewEvent(EventAddressActivity, data)
	if err != nil {
		return err
	}
	b.Publish(event)
	return nil
}

// PublishPriceUpdate publishes a price update event
func (b *Bus) PublishPriceUpdate(price interface{}) error {
	event, err := NewEvent(EventPriceUpdate, price)
	if err != nil {
		return err
	}
	b.Publish(event)
	return nil
}

// PublishSyncStatus publishes a sync status update event
func (b *Bus) PublishSyncStatus(status interface{}) error {
	event, err := NewEvent(EventSyncStatus, status)
	if err != nil {
		return err
	}
	b.Publish(event)
	return nil
}

// Close closes all subscriber channels
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true

	// Close all channels
	closedChans := make(map[chan *Event]bool)
	for _, subs := range b.subscribers {
		for _, ch := range subs {
			if !closedChans[ch] {
				close(ch)
				closedChans[ch] = true
			}
		}
	}

	b.subscribers = make(map[EventType][]chan *Event)
}
