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

		// Reset window if expired.
		if now.After(entry.windowEnd) {
			entry.count = 0
			entry.windowEnd = now.Add(rl.window)
		}

		entry.count++
		remaining := rl.limit - entry.count
		if remaining < 0 {
			remaining = 0
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(entry.windowEnd.Unix(), 10))

		if entry.count > rl.limit {
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
			if now.After(entry.windowEnd) {
				rl.store.Delete(key)
			}
			return true
		})
	}
}
