package websocket

// Contract under test: the websocket hub/client behavior documented by the
// package's own structure and the event/topic mapping. Expected values are
// derived from the contract the code declares (the valid-topic set in
// isValidTopic, the EventType→topic mapping in eventTypeToTopic/handleEvent,
// the maxConnections guard, and the Subscribe/Unsubscribe bookkeeping), NOT by
// copying runtime output. If behavior contradicts these, the code is wrong.
//
// Strategy:
//   - Hub lifecycle (register/unregister/subscribe/unsubscribe/broadcast/event
//     routing, maxConnections) is exercised directly against the hub's internal
//     methods + helper channels. Clients on these paths never touch c.conn, so
//     a nil *websocket.Conn is safe.
//   - handleMessage paths that log via c.conn.RemoteAddr() (invalid JSON / bad
//     message) use a REAL gorilla connection established over httptest.
//
// Runs meaningfully under `go test -race`. Helpers use the `tc` prefix per the
// cross-branch coexistence contract.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"explorer/internal/events"

	gorillaWS "github.com/gorilla/websocket"
)

// tcTestConfig is a Config with tiny timeouts suitable for tests.
func tcTestConfig() *Config {
	return &Config{
		MaxConnections: 10000,
		PingInterval:   time.Hour, // never fire in a test
		WriteWait:      time.Second,
		PongWait:       time.Second,
		MaxMessageSize: 512,
	}
}

// tcNewNilConnClient builds a Client with a nil conn — valid for every hub path
// that does not read/write the socket.
func tcNewNilConnClient(h *Hub) *Client {
	return NewClient(h, nil, tcTestConfig())
}

// tcDrainSend reads up to `n` frames from a client's send channel, or fails on
// timeout. Returns the decoded JSON objects.
func tcDrainSend(t *testing.T, c *Client, n int) []map[string]any {
	t.Helper()
	out := make([]map[string]any, 0, n)
	deadline := time.After(2 * time.Second)
	for i := 0; i < n; i++ {
		select {
		case raw, ok := <-c.send:
			if !ok {
				t.Fatalf("send channel closed after %d/%d frames", i, n)
			}
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("frame %d not JSON: %v (%s)", i, err, raw)
			}
			out = append(out, m)
		case <-deadline:
			t.Fatalf("timed out waiting for frame %d/%d", i, n)
		}
	}
	return out
}

// --- pure helpers -----------------------------------------------------------

func TestEventTypeToTopic(t *testing.T) {
	// Contract: hub.go:196-211 — each known EventType maps to its topic; an
	// unknown type falls back to its raw string value.
	cases := []struct {
		et   events.EventType
		want string
	}{
		{events.EventBlockNew, "blocks"},
		{events.EventTxNew, "transactions"},
		{events.EventPriceUpdate, "price"},
		{events.EventSyncStatus, "sync"},
		{events.EventAddressActivity, "address"},
		{events.EventType("something:else"), "something:else"},
	}
	for _, c := range cases {
		if got := eventTypeToTopic(c.et); got != c.want {
			t.Errorf("eventTypeToTopic(%q) = %q, want %q", c.et, got, c.want)
		}
	}
}

func TestIsValidTopic(t *testing.T) {
	// Contract: client.go:137-155 — exactly {blocks,transactions,price,sync}
	// plus any "address:" prefixed topic are valid; everything else invalid.
	valid := []string{
		"blocks", "transactions", "price", "sync",
		"address:0x407d73d8a49eeb85d32cf465507dd71d507100c1",
		"address:anything",
	}
	for _, topic := range valid {
		if !isValidTopic(topic) {
			t.Errorf("isValidTopic(%q) = false, want true", topic)
		}
	}
	invalid := []string{
		"", "block", "Blocks", "tx", "address", "address:", // "address:" is len 8, needs >8
		"random", "addresses:0x1",
	}
	for _, topic := range invalid {
		if isValidTopic(topic) {
			t.Errorf("isValidTopic(%q) = true, want false", topic)
		}
	}
}

// --- hub lifecycle ----------------------------------------------------------

// tcRunHub starts a hub's Run loop and returns a cancel func + a wait that the
// loop has exited.
func tcRunHub(t *testing.T, h *Hub) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	// Give Run a moment to subscribe to the bus and enter its select.
	time.Sleep(10 * time.Millisecond)
	return cancel
}

func TestHubRegisterUnregister(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 10000)
	cancel := tcRunHub(t, h)
	defer cancel()

	c := tcNewNilConnClient(h)
	h.Register(c)

	// Register is async via a channel; poll for the count.
	tcWaitForCount(t, h.ClientCount, 1)

	h.Unregister(c)
	tcWaitForCount(t, h.ClientCount, 0)
}

func TestHubMaxConnectionsRejection(t *testing.T) {
	// Contract: hub.go:71-75 — when len(clients) >= maxConnections a new client
	// is rejected (closed) and NOT added.
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 2)
	cancel := tcRunHub(t, h)
	defer cancel()

	c1 := tcNewNilConnClient(h)
	c2 := tcNewNilConnClient(h)
	h.Register(c1)
	h.Register(c2)
	tcWaitForCount(t, h.ClientCount, 2)

	// Third registration must be rejected; count stays at the cap.
	c3 := tcNewNilConnClient(h)
	h.Register(c3)
	// Give the hub time to process and reject.
	time.Sleep(50 * time.Millisecond)
	if got := h.ClientCount(); got != 2 {
		t.Fatalf("ClientCount = %d after over-cap register, want 2 (rejected)", got)
	}
	// The rejected client must have been Close()d: its send channel is closed.
	select {
	case _, ok := <-c3.send:
		if ok {
			t.Fatalf("rejected client send channel delivered a value, want closed")
		}
	case <-time.After(time.Second):
		t.Fatalf("rejected client send channel not closed within 1s")
	}
}

func TestHubSubscribeBroadcastUnsubscribe(t *testing.T) {
	// Direct Subscribe/Unsubscribe + broadcastToTopic, asserting topic
	// bookkeeping and delivery. (Run loop not needed: we drive the methods.)
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 10000)

	c := tcNewNilConnClient(h)
	// Manually register so broadcastToAll-style bookkeeping is consistent.
	h.registerClient(c)

	h.Subscribe(c, "blocks")
	if got := h.TopicCount(); got != 1 {
		t.Fatalf("TopicCount = %d after one Subscribe, want 1", got)
	}
	if !c.topics["blocks"] {
		t.Fatalf("client.topics missing 'blocks' after Subscribe")
	}

	// Broadcast to the subscribed topic → client receives it.
	h.broadcastToTopic("blocks", []byte(`{"hello":"world"}`))
	frames := tcDrainSend(t, c, 1)
	if frames[0]["hello"] != "world" {
		t.Fatalf("broadcast payload = %v, want hello=world", frames[0])
	}

	// Broadcast to a topic with no subscribers → no delivery, no panic.
	h.broadcastToTopic("price", []byte(`{"x":1}`))
	select {
	case raw := <-c.send:
		t.Fatalf("received unexpected frame for unsubscribed topic: %s", raw)
	case <-time.After(100 * time.Millisecond):
	}

	// Unsubscribe → topic map cleaned up (last subscriber gone).
	h.Unsubscribe(c, "blocks")
	if got := h.TopicCount(); got != 0 {
		t.Fatalf("TopicCount = %d after Unsubscribe of last subscriber, want 0", got)
	}
	if c.topics["blocks"] {
		t.Fatalf("client.topics still has 'blocks' after Unsubscribe")
	}
}

func TestHubUnregisterRemovesTopicSubscriptions(t *testing.T) {
	// Contract: hub.go:81-98 — unregistering a client drops it from every topic
	// and removes now-empty topics.
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 10000)

	c := tcNewNilConnClient(h)
	h.registerClient(c)
	h.Subscribe(c, "blocks")
	h.Subscribe(c, "transactions")
	if got := h.TopicCount(); got != 2 {
		t.Fatalf("TopicCount = %d, want 2", got)
	}

	h.unregisterClient(c)
	if got := h.TopicCount(); got != 0 {
		t.Fatalf("TopicCount = %d after unregister, want 0 (topics cleaned)", got)
	}
	if got := h.ClientCount(); got != 0 {
		t.Fatalf("ClientCount = %d after unregister, want 0", got)
	}
}

func TestHubHandleEventRoutesByTopic(t *testing.T) {
	// Contract: hub.go:166-194 — events published on the bus are marshaled to
	// {type:"event",topic,data} and routed to the matching topic. A subscriber
	// to "blocks" gets EventBlockNew; a subscriber to "price" gets
	// EventPriceUpdate; address activity routes to address:<addr>.
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 10000)
	cancel := tcRunHub(t, h)
	defer cancel()

	blockSub := tcNewNilConnClient(h)
	priceSub := tcNewNilConnClient(h)
	addrSub := tcNewNilConnClient(h)
	h.registerClient(blockSub)
	h.registerClient(priceSub)
	h.registerClient(addrSub)
	h.Subscribe(blockSub, "blocks")
	h.Subscribe(priceSub, "price")
	const addr = "0x407d73d8a49eeb85d32cf465507dd71d507100c1"
	h.Subscribe(addrSub, "address:"+addr)

	// Publish a block event via the bus; Run picks it up and routes it.
	if err := bus.PublishNewBlock(map[string]any{"number": 123}); err != nil {
		t.Fatalf("PublishNewBlock: %v", err)
	}
	bf := tcDrainSend(t, blockSub, 1)
	if bf[0]["type"] != "event" || bf[0]["topic"] != "blocks" {
		t.Fatalf("block frame = %v, want type=event topic=blocks", bf[0])
	}
	// price/addr subscribers must NOT receive the block event.
	tcAssertNoFrame(t, priceSub)
	tcAssertNoFrame(t, addrSub)

	// Publish a price update → only the price subscriber.
	if err := bus.PublishPriceUpdate(map[string]any{"usd": 2500}); err != nil {
		t.Fatalf("PublishPriceUpdate: %v", err)
	}
	pf := tcDrainSend(t, priceSub, 1)
	if pf[0]["topic"] != "price" {
		t.Fatalf("price frame topic = %v, want price", pf[0]["topic"])
	}
	tcAssertNoFrame(t, blockSub)

	// Publish address activity → routed to address:<addr>.
	if err := bus.PublishAddressActivity(addr, map[string]any{"kind": "transfer"}); err != nil {
		t.Fatalf("PublishAddressActivity: %v", err)
	}
	af := tcDrainSend(t, addrSub, 1)
	if af[0]["topic"] != "address" {
		// handleEvent sets the message's topic field from eventTypeToTopic
		// (which yields "address" for EventAddressActivity); routing to the
		// per-address topic is separate. The frame's topic field is "address".
		t.Fatalf("address frame topic field = %v, want address", af[0]["topic"])
	}
}

func TestHubConcurrentRegisterUnregister(t *testing.T) {
	// -race exercise: many concurrent register/unregister/subscribe ops on the
	// hub must not race or deadlock; final count returns to zero.
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 10000)
	cancel := tcRunHub(t, h)
	defer cancel()

	const n = 50
	clients := make([]*Client, n)
	for i := range clients {
		clients[i] = tcNewNilConnClient(h)
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c := clients[idx]
			h.Register(c)
			// Concurrent subscribe churn on shared topics.
			h.Subscribe(c, "blocks")
			h.Subscribe(c, "price")
			h.Unsubscribe(c, "blocks")
			h.Unregister(c)
		}(i)
	}
	wg.Wait()

	tcWaitForCount(t, h.ClientCount, 0)
}

// --- handleMessage (needs a real conn for the error-log paths) --------------

// tcDialHubClient upgrades a websocket connection over httptest and returns the
// server-side *Client (wired to hub h) plus a close func. The server handler
// does NOT start ReadPump/WritePump so the test fully controls the client.
func tcDialHubClient(t *testing.T, h *Hub) (*Client, func()) {
	t.Helper()
	up := gorillaWS.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	clientCh := make(chan *Client, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		clientCh <- NewClient(h, conn, tcTestConfig())
		// Hold the handler open until the test closes the dialer.
		<-r.Context().Done()
		_ = conn.Close()
	}))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	dialer, _, err := gorillaWS.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial: %v", err)
	}

	var c *Client
	select {
	case c = <-clientCh:
	case <-time.After(2 * time.Second):
		dialer.Close()
		srv.Close()
		t.Fatalf("server did not produce a client")
	}

	return c, func() {
		dialer.Close()
		srv.Close()
	}
}

func TestClientHandleMessageSubscribe(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 10000)

	c, closeFn := tcDialHubClient(t, h)
	defer closeFn()

	// Valid subscribe → Subscribe applied + a success frame queued.
	c.handleMessage([]byte(`{"action":"subscribe","topics":["blocks"]}`))
	if !c.topics["blocks"] {
		t.Fatalf("subscribe did not register 'blocks' on client")
	}
	frames := tcDrainSend(t, c, 1)
	if frames[0]["type"] != "success" || frames[0]["action"] != "subscribed" || frames[0]["topic"] != "blocks" {
		t.Fatalf("success frame = %v, want type=success action=subscribed topic=blocks", frames[0])
	}
}

func TestClientHandleMessageInvalidTopic(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 10000)

	c, closeFn := tcDialHubClient(t, h)
	defer closeFn()

	c.handleMessage([]byte(`{"action":"subscribe","topics":["not-a-topic"]}`))
	if c.topics["not-a-topic"] {
		t.Fatalf("invalid topic was subscribed")
	}
	frames := tcDrainSend(t, c, 1)
	if frames[0]["type"] != "error" {
		t.Fatalf("frame = %v, want an error frame for an invalid topic", frames[0])
	}
}

func TestClientHandleMessageUnknownAction(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 10000)

	c, closeFn := tcDialHubClient(t, h)
	defer closeFn()

	c.handleMessage([]byte(`{"action":"frobnicate","topics":["blocks"]}`))
	frames := tcDrainSend(t, c, 1)
	if frames[0]["type"] != "error" || !strings.Contains(frames[0]["message"].(string), "unknown action") {
		t.Fatalf("frame = %v, want error 'unknown action'", frames[0])
	}
}

func TestClientHandleMessageInvalidJSON(t *testing.T) {
	// This path logs via c.conn.RemoteAddr(), hence the real connection.
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 10000)

	c, closeFn := tcDialHubClient(t, h)
	defer closeFn()

	c.handleMessage([]byte(`{not valid json`))
	frames := tcDrainSend(t, c, 1)
	if frames[0]["type"] != "error" || !strings.Contains(frames[0]["message"].(string), "invalid message format") {
		t.Fatalf("frame = %v, want error 'invalid message format'", frames[0])
	}
}

func TestClientHandleMessageUnsubscribe(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 10000)

	c, closeFn := tcDialHubClient(t, h)
	defer closeFn()

	h.Subscribe(c, "blocks")
	c.handleMessage([]byte(`{"action":"unsubscribe","topics":["blocks"]}`))
	if c.topics["blocks"] {
		t.Fatalf("unsubscribe did not remove 'blocks'")
	}
	frames := tcDrainSend(t, c, 1)
	if frames[0]["type"] != "success" || frames[0]["action"] != "unsubscribed" {
		t.Fatalf("frame = %v, want success unsubscribed", frames[0])
	}
}

func TestClientCloseIdempotent(t *testing.T) {
	// Contract: client.go:174-184 — Close() is guarded by c.closed, so calling
	// it twice must not panic (no double-close of c.send).
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 10000)
	c := tcNewNilConnClient(h)

	c.Close()
	c.Close() // must be a no-op, not a panic
}

func TestClientTopics(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()
	h := NewHub(bus, 10000)
	c := tcNewNilConnClient(h)

	h.Subscribe(c, "blocks")
	h.Subscribe(c, "price")
	got := c.Topics()
	if len(got) != 2 {
		t.Fatalf("Topics() = %v, want 2 entries", got)
	}
	set := map[string]bool{}
	for _, x := range got {
		set[x] = true
	}
	if !set["blocks"] || !set["price"] {
		t.Fatalf("Topics() = %v, want blocks+price", got)
	}
}

// --- small helpers ----------------------------------------------------------

func tcWaitForCount(t *testing.T, fn func() int, want int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if fn() == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("count = %d, want %d (timed out)", fn(), want)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func tcAssertNoFrame(t *testing.T, c *Client) {
	t.Helper()
	select {
	case raw := <-c.send:
		t.Fatalf("unexpected frame delivered: %s", raw)
	case <-time.After(100 * time.Millisecond):
	}
}
