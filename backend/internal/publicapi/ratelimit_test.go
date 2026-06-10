package publicapi

// TDD for the public-api rate limiter (audit W-4, counter-race half).
//
// These tests assert the CORRECT behavior of the limiter:
//   - enforcement: the first `limit` requests in a window pass; the next is 429.
//   - window reset: once the window elapses, the bucket resets and requests pass.
//   - per-key isolation: distinct RemoteAddr values get INDEPENDENT buckets
//     (driven via RemoteAddr, NOT via X-Forwarded-For — we deliberately do not
//     assert "leftmost XFF is trusted", which would lock in the W-4 spoofing bug;
//     the XFF-trust half is left to the dedicated W-4 work).
//   - concurrency safety: many concurrent requests for the SAME key must be
//     counted without a data race.
//
// The concurrency test is RED under `go test -race` against the original
// ratelimit.go because entry.count++ / entry.count = 0 / entry.windowEnd are
// unsynchronized read-modify-writes on a *clientEntry shared via sync.Map (and
// the cleanup goroutine reads windowEnd concurrently). The minimal fix — guard
// the per-entry mutation with a mutex — turns it green. Helpers use the `tc`
// prefix per the coexistence contract.

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// tcOKHandler is a trivial 200 handler used as the limiter's `next`.
func tcOKHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// tcDoReq sends one request through the limiter middleware with the given
// RemoteAddr and returns the response recorder.
func tcDoReq(h http.Handler, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/blocks", nil)
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestRateLimiterEnforcesLimit(t *testing.T) {
	// Contract: with limit=3, requests 1..3 pass (200) and request 4 is 429.
	rl := NewRateLimiter(3, time.Minute)
	h := rl.Middleware(tcOKHandler())

	const addr = "203.0.113.7:5555"
	for i := 1; i <= 3; i++ {
		if w := tcDoReq(h, addr); w.Code != http.StatusOK {
			t.Fatalf("request %d: status %d, want 200 (within limit)", i, w.Code)
		}
	}
	if w := tcDoReq(h, addr); w.Code != http.StatusTooManyRequests {
		t.Fatalf("request 4: status %d, want 429 (over limit)", w.Code)
	}
}

func TestRateLimiterWindowReset(t *testing.T) {
	// Contract: once the window elapses, the bucket resets and requests pass
	// again. Use a short window and sleep past it.
	const window = 40 * time.Millisecond
	rl := NewRateLimiter(1, window)
	h := rl.Middleware(tcOKHandler())

	const addr = "203.0.113.8:5555"
	if w := tcDoReq(h, addr); w.Code != http.StatusOK {
		t.Fatalf("first request: status %d, want 200", w.Code)
	}
	if w := tcDoReq(h, addr); w.Code != http.StatusTooManyRequests {
		t.Fatalf("second request within window: status %d, want 429", w.Code)
	}

	time.Sleep(window + 30*time.Millisecond)

	if w := tcDoReq(h, addr); w.Code != http.StatusOK {
		t.Fatalf("post-window request: status %d, want 200 (window reset)", w.Code)
	}
}

func TestRateLimiterPerKeyIsolation(t *testing.T) {
	// Contract: distinct RemoteAddr values get independent buckets. Exhausting
	// one key's bucket must not affect another key.
	rl := NewRateLimiter(1, time.Minute)
	h := rl.Middleware(tcOKHandler())

	const addrA = "198.51.100.1:1111"
	const addrB = "198.51.100.2:2222"

	if w := tcDoReq(h, addrA); w.Code != http.StatusOK {
		t.Fatalf("A first: status %d, want 200", w.Code)
	}
	if w := tcDoReq(h, addrA); w.Code != http.StatusTooManyRequests {
		t.Fatalf("A second: status %d, want 429 (A exhausted)", w.Code)
	}
	// B must still have its own fresh bucket.
	if w := tcDoReq(h, addrB); w.Code != http.StatusOK {
		t.Fatalf("B first: status %d, want 200 (independent bucket)", w.Code)
	}
}

func TestRateLimiterConcurrentSameKey(t *testing.T) {
	// W-4 counter-race exercise: many concurrent requests for the SAME key.
	// Under -race this trips the data race on entry.count until the counter is
	// synchronized. Functionally: with limit=N and M>N concurrent requests, the
	// number of 200s must not EXCEED the limit (a lost-update race would let
	// extra requests through). We assert <= limit (and, since all requests fall
	// in one window, exactly limit succeed once the counter is correct).
	const limit = 50
	const concurrent = 400
	rl := NewRateLimiter(limit, time.Minute)
	h := rl.Middleware(tcOKHandler())

	const addr = "192.0.2.50:9999"
	var ok200, got429 int64

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			w := tcDoReq(h, addr)
			switch w.Code {
			case http.StatusOK:
				atomic.AddInt64(&ok200, 1)
			case http.StatusTooManyRequests:
				atomic.AddInt64(&got429, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	total := atomic.LoadInt64(&ok200) + atomic.LoadInt64(&got429)
	if total != concurrent {
		t.Fatalf("accounted %d responses, want %d", total, concurrent)
	}
	if ok200 := atomic.LoadInt64(&ok200); ok200 > limit {
		t.Fatalf("allowed %d requests, want <= limit %d (counter lost updates under concurrency)", ok200, limit)
	}
	// With a correct counter, exactly `limit` requests succeed in the single
	// window and the rest are throttled.
	if ok200 := atomic.LoadInt64(&ok200); ok200 != limit {
		t.Fatalf("allowed %d requests, want exactly limit %d", ok200, limit)
	}
}

func TestRateLimiterConcurrentDistinctKeys(t *testing.T) {
	// -race exercise across DISTINCT keys: concurrent requests for many
	// different RemoteAddr values must each get their own bucket without racing
	// the sync.Map / per-entry state.
	const limit = 1
	rl := NewRateLimiter(limit, time.Minute)
	h := rl.Middleware(tcOKHandler())

	const keys = 200
	var ok200 int64
	var wg sync.WaitGroup
	for i := 0; i < keys; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			addr := tcAddrForIndex(idx)
			if w := tcDoReq(h, addr); w.Code == http.StatusOK {
				atomic.AddInt64(&ok200, 1)
			}
		}(i)
	}
	wg.Wait()

	// Each distinct key has its own fresh bucket (limit=1), so all `keys`
	// first-requests should succeed.
	if got := atomic.LoadInt64(&ok200); got != keys {
		t.Fatalf("allowed %d of %d distinct-key first requests, want all (per-key isolation)", got, keys)
	}
}

// tcAddrForIndex produces a unique RemoteAddr per index without colliding.
func tcAddrForIndex(i int) string {
	return "10." + itoa3(i>>16&0xff) + "." + itoa3(i>>8&0xff) + "." + itoa3(i&0xff) + ":1234"
}

func itoa3(n int) string {
	if n == 0 {
		return "0"
	}
	var b [3]byte
	pos := len(b)
	for n > 0 {
		pos--
		b[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(b[pos:])
}
