package publicapi

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type clientEntry struct {
	// mu guards count and windowEnd. The entry is shared across request
	// goroutines via the sync.Map, and the cleanup goroutine also reads
	// windowEnd, so every read-modify-write of these fields must hold mu.
	// (Audit W-4, counter-race half: the previous unguarded entry.count++ was a
	// data race that undercounted under load, weakening the only DoS control.)
	mu        sync.Mutex
	count     int
	windowEnd time.Time
}

// RateLimiter implements an IP-based sliding window rate limiter.
type RateLimiter struct {
	limit  int
	window time.Duration
	store  sync.Map
}

// NewRateLimiter creates a rate limiter with the given request limit per window duration.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limit:  limit,
		window: window,
	}
	go rl.cleanup()
	return rl
}

// Middleware returns an HTTP middleware that enforces rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.clientIP(r)
		now := time.Now()

		val, _ := rl.store.LoadOrStore(ip, &clientEntry{
			count:     0,
			windowEnd: now.Add(rl.window),
		})
		entry := val.(*clientEntry)

		// Mutate count/windowEnd under the entry lock and snapshot the values
		// for use after unlocking. Without this the read-modify-write races
		// across concurrent requests for the same key (W-4).
		entry.mu.Lock()
		// Reset window if expired.
		if now.After(entry.windowEnd) {
			entry.count = 0
			entry.windowEnd = now.Add(rl.window)
		}
		entry.count++
		count := entry.count
		windowEnd := entry.windowEnd
		entry.mu.Unlock()

		remaining := rl.limit - count
		if remaining < 0 {
			remaining = 0
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(windowEnd.Unix(), 10))

		if count > rl.limit {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "rate limit exceeded",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) clientIP(r *http.Request) string {
	// Check X-Forwarded-For first for proxy support.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list.
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rl.store.Range(func(key, value any) bool {
			entry := value.(*clientEntry)
			entry.mu.Lock()
			expired := now.After(entry.windowEnd)
			entry.mu.Unlock()
			if expired {
				rl.store.Delete(key)
			}
			return true
		})
	}
}
