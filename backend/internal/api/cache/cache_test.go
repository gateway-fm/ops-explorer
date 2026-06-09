package cache

// Contract under test: the package doc comment for internal/api/cache
// (cache.go:1-3): "Package cache provides a TTL-bounded LRU with singleflight
// deduplication. Errors are not cached — failed fetches leave the slot empty so
// the next caller retries instead of seeing a stale 'not found'."
//
// These are contract/characterization tests: every expected value below is
// derived from that documented contract (and the semantics of the underlying
// expirable LRU + golang.org/x/sync/singleflight types), NOT from observing the
// current implementation's output. If the implementation ever contradicts the
// contract, the test is the source of truth and the code should be fixed.
//
// Helpers are prefixed `tc` per the cross-branch file-ownership coexistence
// contract (privacy-hardening branch uses `ph`).

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// tcErrBoom is a sentinel fetch error used to assert the "errors are not
// cached" contract.
var tcErrBoom = errors.New("boom")

// tcBlockingFetch returns a fetch func that blocks until `release` is closed,
// counting every invocation atomically. Used to prove singleflight dedup.
func tcBlockingFetch(counter *int64, release <-chan struct{}, value string) func() (string, error) {
	return func() (string, error) {
		atomic.AddInt64(counter, 1)
		<-release
		return value, nil
	}
}

// TestTTLCacheGetMiss covers the contract that a miss invokes fetch exactly once
// and returns the fetched value.
func TestTTLCacheGetMiss(t *testing.T) {
	c := New[string](8, time.Minute)

	var calls int64
	got, err := c.Get("k", func() (string, error) {
		atomic.AddInt64(&calls, 1)
		return "v", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "v" {
		t.Fatalf("got %q, want %q", got, "v")
	}
	if calls != 1 {
		t.Fatalf("fetch called %d times, want exactly 1 on a cold miss", calls)
	}
}

// TestTTLCacheGetHit covers the contract that a populated key is served from the
// cache WITHOUT re-invoking fetch.
func TestTTLCacheGetHit(t *testing.T) {
	c := New[string](8, time.Minute)

	var calls int64
	fetch := func() (string, error) {
		atomic.AddInt64(&calls, 1)
		return "v", nil
	}
	if _, err := c.Get("k", fetch); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	got, err := c.Get("k", fetch)
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if got != "v" {
		t.Fatalf("got %q, want %q", got, "v")
	}
	if calls != 1 {
		t.Fatalf("fetch called %d times across two Gets, want exactly 1 (second served from cache)", calls)
	}
}

// TestTTLCacheErrorsNotCached encodes the explicit doc contract: "Errors are not
// cached — failed fetches leave the slot empty so the next caller retries
// instead of seeing a stale 'not found'." The first fetch fails; the second
// must re-invoke fetch and succeed.
func TestTTLCacheErrorsNotCached(t *testing.T) {
	c := New[string](8, time.Minute)

	var calls int64
	got, err := c.Get("k", func() (string, error) {
		atomic.AddInt64(&calls, 1)
		return "", tcErrBoom
	})
	if !errors.Is(err, tcErrBoom) {
		t.Fatalf("first Get error = %v, want %v", err, tcErrBoom)
	}
	if got != "" {
		t.Fatalf("first Get value = %q, want zero value on error", got)
	}

	// Second call: per contract the slot must be empty, so fetch runs again and
	// the (now succeeding) result is returned — no stale error is cached.
	got, err = c.Get("k", func() (string, error) {
		atomic.AddInt64(&calls, 1)
		return "recovered", nil
	})
	if err != nil {
		t.Fatalf("second Get error = %v, want nil (error must not be cached)", err)
	}
	if got != "recovered" {
		t.Fatalf("second Get value = %q, want %q (retry after non-cached error)", got, "recovered")
	}
	if calls != 2 {
		t.Fatalf("fetch called %d times, want 2 (error not cached ⇒ retry)", calls)
	}
}

// TestTTLCacheExpiry encodes the TTL contract: a value cached at time T is no
// longer served after the TTL elapses, so fetch is invoked again. Uses a short
// TTL and a real sleep with margin.
func TestTTLCacheExpiry(t *testing.T) {
	const ttl = 30 * time.Millisecond
	c := New[string](8, ttl)

	var calls int64
	fetch := func() (string, error) {
		n := atomic.AddInt64(&calls, 1)
		if n == 1 {
			return "first", nil
		}
		return "second", nil
	}

	got, err := c.Get("k", fetch)
	if err != nil || got != "first" {
		t.Fatalf("initial Get = (%q, %v), want (first, nil)", got, err)
	}

	// Within TTL: still served from cache.
	got, err = c.Get("k", fetch)
	if err != nil || got != "first" {
		t.Fatalf("within-TTL Get = (%q, %v), want (first, nil)", got, err)
	}
	if calls != 1 {
		t.Fatalf("fetch called %d times within TTL, want 1", calls)
	}

	// After TTL: entry expired, fetch runs again.
	time.Sleep(ttl + 50*time.Millisecond)
	got, err = c.Get("k", fetch)
	if err != nil {
		t.Fatalf("post-TTL Get error = %v", err)
	}
	if got != "second" {
		t.Fatalf("post-TTL Get value = %q, want %q (entry should have expired)", got, "second")
	}
	if calls != 2 {
		t.Fatalf("fetch called %d times after TTL, want 2", calls)
	}
}

// TestTTLCacheSingleflightDedup encodes the "singleflight deduplication"
// contract: N concurrent Get calls for the SAME key, while fetch is blocked,
// must collapse to exactly ONE fetch invocation, and all callers must receive
// the same value. Runs meaningfully under `go test -race`.
func TestTTLCacheSingleflightDedup(t *testing.T) {
	c := New[string](8, time.Minute)

	const goroutines = 50
	var fetchCount int64
	release := make(chan struct{})
	fetch := tcBlockingFetch(&fetchCount, release, "shared")

	var wg sync.WaitGroup
	results := make([]string, goroutines)
	errs := make([]error, goroutines)
	started := make(chan struct{}, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			started <- struct{}{}
			results[idx], errs[idx] = c.Get("k", fetch)
		}(i)
	}

	// Wait until all goroutines have at least entered Get before releasing the
	// single in-flight fetch, maximizing the window for dedup to take effect.
	for i := 0; i < goroutines; i++ {
		<-started
	}
	// Give the scheduler a moment so contenders are parked in singleflight.
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := atomic.LoadInt64(&fetchCount); got != 1 {
		t.Fatalf("fetch invoked %d times for %d concurrent Gets, want exactly 1 (singleflight dedup)", got, goroutines)
	}
	for i := 0; i < goroutines; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d got error %v, want nil", i, errs[i])
		}
		if results[i] != "shared" {
			t.Fatalf("goroutine %d got %q, want %q (all callers share the single fetch result)", i, results[i], "shared")
		}
	}
}

// TestTTLCacheDistinctKeysIndependent confirms distinct keys do not collide:
// each key gets its own fetch + its own cached value.
func TestTTLCacheDistinctKeysIndependent(t *testing.T) {
	c := New[string](8, time.Minute)

	v1, err := c.Get("a", func() (string, error) { return "A", nil })
	if err != nil || v1 != "A" {
		t.Fatalf("Get(a) = (%q, %v), want (A, nil)", v1, err)
	}
	v2, err := c.Get("b", func() (string, error) { return "B", nil })
	if err != nil || v2 != "B" {
		t.Fatalf("Get(b) = (%q, %v), want (B, nil)", v2, err)
	}

	// Re-reading each key serves its own cached value.
	v1, _ = c.Get("a", func() (string, error) { return "SHOULD-NOT-RUN", nil })
	v2, _ = c.Get("b", func() (string, error) { return "SHOULD-NOT-RUN", nil })
	if v1 != "A" || v2 != "B" {
		t.Fatalf("cached values crossed: a=%q b=%q, want a=A b=B", v1, v2)
	}
}
