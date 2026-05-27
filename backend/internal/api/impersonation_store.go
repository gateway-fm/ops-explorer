package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ImpersonationSession holds the metadata mapped to an opaque session token
// minted when a tier-2 admin starts a "View as user" diagnostic view.
//
// The token (not the target DID) is what the browser carries in the URL as
// `?as=<token>`, so DIDs do not leak through URL history, copy-paste, or
// referrer headers. The store is the only thing that can translate the token
// back into the target DID for outbound calls.
type ImpersonationSession struct {
	// AdminDID is the DID of the admin who started the impersonation.
	// On every impersonated request the BFF middleware verifies the caller's
	// auth-cookie subject still matches this value, so a leaked token cannot
	// be replayed by a different user.
	AdminDID string
	// TargetDID is the DID being viewed-as. All proxied requests are routed
	// under /api/v1/admin/impersonate/<TargetDID>/... on privacy-proxy.
	TargetDID string
	// ExpiresAt is when this token stops being honored. The store also runs a
	// periodic GC to drop entries past their expiry, but the per-lookup check
	// is the authoritative one in case GC has not yet swept.
	ExpiresAt time.Time
}

// ImpersonationStore is the small interface the rest of the codebase depends
// on. The default implementation is in-memory; a Redis-backed implementation
// can be swapped in for multi-instance deployments without touching callers.
type ImpersonationStore interface {
	// Mint issues a new token for the given session and stores it. The session
	// is mutated (or its returned copy populated) with the chosen ExpiresAt so
	// callers can return it directly to the client. Returns the opaque token.
	Mint(ctx context.Context, session ImpersonationSession, ttl time.Duration) (token string, expiresAt time.Time, err error)
	// Lookup fetches a session by token. Returns ErrImpersonationNotFound when
	// the token is unknown OR expired (callers should treat both the same as
	// "session does not exist").
	Lookup(ctx context.Context, token string) (ImpersonationSession, error)
	// Revoke removes a token. Idempotent — revoking an unknown token is not an
	// error so that the DELETE endpoint can return 204 either way.
	Revoke(ctx context.Context, token string) error
}

// ErrImpersonationNotFound is returned by Lookup when the token is missing or
// past its expiry. Callers should map this to 401 on inbound requests and to
// a non-error path on revocation.
var ErrImpersonationNotFound = errors.New("impersonation: session not found or expired")

// DefaultImpersonationTTL is the lifetime applied to newly minted tokens.
// Chosen short enough that a leaked token has bounded blast radius, long
// enough that an admin can complete a diagnostic browsing session without
// constantly re-authorising.
const DefaultImpersonationTTL = 1 * time.Hour

// impersonationTokenBytes is the random material used to mint tokens. 32
// bytes (256 bits) hex-encoded is consistent with privacy-proxy session id
// generation and resistant to brute force.
const impersonationTokenBytes = 32

// memoryImpersonationStore is the in-memory implementation of
// ImpersonationStore. It is safe for concurrent use. A goroutine started in
// NewMemoryImpersonationStore periodically removes expired entries; callers
// who need a controlled lifecycle (tests) can use NewMemoryImpersonationStoreNoGC.
type memoryImpersonationStore struct {
	mu       sync.RWMutex
	sessions map[string]ImpersonationSession

	now      func() time.Time
	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewMemoryImpersonationStore returns a production-ready in-memory store with
// a background GC goroutine that sweeps expired entries every gcInterval.
// Pass 0 to use the default sweep cadence.
func NewMemoryImpersonationStore(gcInterval time.Duration) *memoryImpersonationStore {
	if gcInterval <= 0 {
		gcInterval = 5 * time.Minute
	}
	s := &memoryImpersonationStore{
		sessions: make(map[string]ImpersonationSession),
		now:      time.Now,
		stopCh:   make(chan struct{}),
	}
	go s.runGC(gcInterval)
	return s
}

// NewMemoryImpersonationStoreNoGC returns a store with no background GC. Used
// by tests that drive time deterministically; production code should use
// NewMemoryImpersonationStore.
func NewMemoryImpersonationStoreNoGC() *memoryImpersonationStore {
	return &memoryImpersonationStore{
		sessions: make(map[string]ImpersonationSession),
		now:      time.Now,
		stopCh:   make(chan struct{}),
	}
}

// Mint generates a fresh random token, stamps the session with its expiry,
// stores it, and returns the token + the resolved expiry time so the handler
// can include it in the JSON response without an extra read.
func (s *memoryImpersonationStore) Mint(_ context.Context, session ImpersonationSession, ttl time.Duration) (string, time.Time, error) {
	if session.AdminDID == "" {
		return "", time.Time{}, errors.New("impersonation: AdminDID required")
	}
	if session.TargetDID == "" {
		return "", time.Time{}, errors.New("impersonation: TargetDID required")
	}
	if ttl <= 0 {
		ttl = DefaultImpersonationTTL
	}

	buf := make([]byte, impersonationTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, fmt.Errorf("impersonation: failed to generate token: %w", err)
	}
	token := hex.EncodeToString(buf)
	expiresAt := s.now().Add(ttl)
	session.ExpiresAt = expiresAt

	s.mu.Lock()
	s.sessions[token] = session
	s.mu.Unlock()

	return token, expiresAt, nil
}

func (s *memoryImpersonationStore) Lookup(_ context.Context, token string) (ImpersonationSession, error) {
	if token == "" {
		return ImpersonationSession{}, ErrImpersonationNotFound
	}
	s.mu.RLock()
	session, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return ImpersonationSession{}, ErrImpersonationNotFound
	}
	if s.now().After(session.ExpiresAt) {
		// Drop the entry now we know it is stale. Best-effort — GC will catch
		// any contention loss.
		s.mu.Lock()
		if current, stillThere := s.sessions[token]; stillThere && current.ExpiresAt.Equal(session.ExpiresAt) {
			delete(s.sessions, token)
		}
		s.mu.Unlock()
		return ImpersonationSession{}, ErrImpersonationNotFound
	}
	return session, nil
}

func (s *memoryImpersonationStore) Revoke(_ context.Context, token string) error {
	if token == "" {
		return nil
	}
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
	return nil
}

// Stop terminates the GC goroutine. Safe to call multiple times.
func (s *memoryImpersonationStore) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
}

// sweepExpired removes entries whose ExpiresAt has passed.
// Exposed (lowercase) so tests can drive a manual sweep without sleeping.
func (s *memoryImpersonationStore) sweepExpired() {
	now := s.now()
	s.mu.Lock()
	for token, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, token)
		}
	}
	s.mu.Unlock()
}

func (s *memoryImpersonationStore) runGC(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.sweepExpired()
		}
	}
}
