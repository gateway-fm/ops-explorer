package api

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemoryImpersonationStore_MintAndLookup(t *testing.T) {
	s := NewMemoryImpersonationStoreNoGC()
	defer s.Stop()

	tok, exp, err := s.Mint(context.Background(), ImpersonationSession{
		AdminDID:  "did:p:admin",
		TargetDID: "did:p:user",
		OrgID:     "org-7",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if tok == "" {
		t.Fatal("Mint returned empty token")
	}
	if exp.Before(time.Now()) {
		t.Fatalf("Mint returned past expiry: %v", exp)
	}

	got, err := s.Lookup(context.Background(), tok)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got.AdminDID != "did:p:admin" || got.TargetDID != "did:p:user" || got.OrgID != "org-7" {
		t.Fatalf("unexpected session: %+v", got)
	}
	if !got.ExpiresAt.Equal(exp) {
		t.Fatalf("expiry mismatch: got %v want %v", got.ExpiresAt, exp)
	}
}

func TestMemoryImpersonationStore_LookupMissing(t *testing.T) {
	s := NewMemoryImpersonationStoreNoGC()
	defer s.Stop()

	_, err := s.Lookup(context.Background(), "nope")
	if err != ErrImpersonationNotFound {
		t.Fatalf("expected ErrImpersonationNotFound, got %v", err)
	}
}

func TestMemoryImpersonationStore_Expiry(t *testing.T) {
	s := NewMemoryImpersonationStoreNoGC()
	defer s.Stop()

	frozen := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return frozen }

	tok, _, err := s.Mint(context.Background(), ImpersonationSession{
		AdminDID:  "did:p:a",
		TargetDID: "did:p:b",
		OrgID:     "org-7",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	// Within TTL → present.
	if _, err := s.Lookup(context.Background(), tok); err != nil {
		t.Fatalf("expected lookup to succeed within TTL: %v", err)
	}

	// Advance past TTL → drops.
	s.now = func() time.Time { return frozen.Add(2 * time.Minute) }
	if _, err := s.Lookup(context.Background(), tok); err != ErrImpersonationNotFound {
		t.Fatalf("expected expired lookup to return ErrImpersonationNotFound, got %v", err)
	}

	// The entry should be evicted from the underlying map as a side effect
	// of the Lookup that observed expiry. sweepExpired() with the advanced
	// clock should find nothing to do.
	s.sweepExpired()
	s.mu.RLock()
	count := len(s.sessions)
	s.mu.RUnlock()
	if count != 0 {
		t.Fatalf("expected store empty after expiry lookup, got %d", count)
	}
}

func TestMemoryImpersonationStore_Revoke(t *testing.T) {
	s := NewMemoryImpersonationStoreNoGC()
	defer s.Stop()

	tok, _, err := s.Mint(context.Background(), ImpersonationSession{
		AdminDID:  "did:p:a",
		TargetDID: "did:p:b",
		OrgID:     "org-7",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	if err := s.Revoke(context.Background(), tok); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := s.Lookup(context.Background(), tok); err != ErrImpersonationNotFound {
		t.Fatalf("expected revoked token to be absent, got %v", err)
	}

	// Idempotent: revoking again is fine.
	if err := s.Revoke(context.Background(), tok); err != nil {
		t.Fatalf("second Revoke: %v", err)
	}
}

func TestMemoryImpersonationStore_Sweep(t *testing.T) {
	s := NewMemoryImpersonationStoreNoGC()
	defer s.Stop()

	frozen := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return frozen }

	// Mint two: one short, one long.
	short, _, err := s.Mint(context.Background(), ImpersonationSession{
		AdminDID: "a", TargetDID: "b", OrgID: "org-7",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Mint short: %v", err)
	}
	long, _, err := s.Mint(context.Background(), ImpersonationSession{
		AdminDID: "a", TargetDID: "c", OrgID: "org-7",
	}, 1*time.Hour)
	if err != nil {
		t.Fatalf("Mint long: %v", err)
	}

	// Advance past the short one's expiry only.
	s.now = func() time.Time { return frozen.Add(2 * time.Minute) }
	s.sweepExpired()

	if _, err := s.Lookup(context.Background(), short); err != ErrImpersonationNotFound {
		t.Fatalf("expected short token to be evicted, got %v", err)
	}
	if _, err := s.Lookup(context.Background(), long); err != nil {
		t.Fatalf("expected long token to survive sweep: %v", err)
	}
}

func TestMemoryImpersonationStore_MintValidation(t *testing.T) {
	s := NewMemoryImpersonationStoreNoGC()
	defer s.Stop()

	tests := []struct {
		name string
		sess ImpersonationSession
	}{
		{name: "missing admin", sess: ImpersonationSession{TargetDID: "x", OrgID: "o"}},
		{name: "missing target", sess: ImpersonationSession{AdminDID: "x", OrgID: "o"}},
		{name: "missing org", sess: ImpersonationSession{AdminDID: "x", TargetDID: "y"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := s.Mint(context.Background(), tt.sess, time.Minute); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestMemoryImpersonationStore_Concurrent(t *testing.T) {
	s := NewMemoryImpersonationStoreNoGC()
	defer s.Stop()

	const n = 50
	var wg sync.WaitGroup
	tokens := make([]string, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			tok, _, err := s.Mint(context.Background(), ImpersonationSession{
				AdminDID:  "admin",
				TargetDID: "target",
				OrgID:     "org-7",
			}, time.Minute)
			if err != nil {
				t.Errorf("Mint: %v", err)
				return
			}
			tokens[i] = tok
		}(i)
	}
	wg.Wait()

	seen := make(map[string]struct{}, n)
	for _, tok := range tokens {
		if tok == "" {
			t.Fatal("got empty token in concurrent mint")
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("duplicate token in concurrent mint: %s", tok)
		}
		seen[tok] = struct{}{}
	}
}
