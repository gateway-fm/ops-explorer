package publicapi

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
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

// DefaultTrustedHops is the number of proxies in front of the public API that
// append the caller's address to X-Forwarded-For: one, the ingress (for
// example an Istio/Envoy gateway, or the frontend's nginx).
const DefaultTrustedHops = 1

// RateLimiter implements an IP-based sliding window rate limiter.
type RateLimiter struct {
	limit  int
	window time.Duration
	store  sync.Map
	// trustedHops is how many proxies in front of this service append to
	// X-Forwarded-For. The client is the entry the outermost of them appended:
	// the trustedHops-th from the RIGHT. Entries left of it were sent by the
	// client and are never trusted. 0 means no proxy: key on the TCP peer.
	trustedHops int
}

// NewRateLimiter creates a rate limiter with the given request limit per
// window duration, trusting DefaultTrustedHops proxies.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limit:       limit,
		window:      window,
		trustedHops: DefaultTrustedHops,
	}
	go rl.cleanup()
	return rl
}

// SetTrustedHops sets how many proxies in front of the service append to
// X-Forwarded-For (0 = clients connect directly). Negative values become 0.
// Call before serving.
func (rl *RateLimiter) SetTrustedHops(n int) {
	if n < 0 {
		n = 0
	}
	rl.trustedHops = n
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

// clientIP is the rate-limit key. Behind trustedHops proxies it is the
// address the outermost trusted proxy appended to X-Forwarded-For (the
// trustedHops-th entry from the right); a client can prepend anything it
// likes to the header, so entries left of that one are never used. When
// the header has fewer entries, or that entry is not an IP, the TCP peer is
// used.
//
// The limiter cannot tell a proxy from a client: a caller that reaches the
// service directly while trustedHops > 0 can send its own header and choose
// its key. Where the API is reachable both directly and through a proxy,
// only the proxied path is protected; set trustedHops to 0 if direct access
// is the one that matters.
func (rl *RateLimiter) clientIP(r *http.Request) string {
	if rl.trustedHops > 0 {
		var hops []string
		for _, v := range r.Header.Values("X-Forwarded-For") {
			for _, h := range strings.Split(v, ",") {
				hops = append(hops, strings.TrimSpace(h))
			}
		}
		if i := len(hops) - rl.trustedHops; i >= 0 && i < len(hops) {
			if ip := net.ParseIP(hops[i]); ip != nil {
				return ip.String()
			}
		}
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
