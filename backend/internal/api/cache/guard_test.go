package cache

import (
	"testing"

	"explorer/internal/api"
)

// callerScopedFake is a DataProvider that is caller-scoped — it returns
// per-caller (RBAC-redacted) data, exactly like the privacy-mode
// ProxyDataProvider. It embeds DirectDBProvider only to satisfy the full
// DataProvider method set; the load-bearing part is the CallerScoped() marker.
type callerScopedFake struct {
	*api.DirectDBProvider
}

func (callerScopedFake) CallerScoped() {}

// scopedDecorator wraps a caller-scoped provider and PROMOTES its CallerScoped()
// marker by embedding it. This models the audit's concern that the marker is
// only structural if propagated: a decorator around a caller-scoped provider
// must itself still be detected as caller-scoped.
type scopedDecorator struct {
	callerScopedFake
}

// plainFake is a non-caller-scoped provider (same value for the same args
// regardless of caller) — the indexerclient / DirectDBProvider shape that is
// safe to cache.
type plainFake struct {
	*api.DirectDBProvider
}

// P-1: cache.NewProvider must REFUSE (panic / log.Fatal) a caller-scoped
// provider so a future refactor or one-line copy-paste cannot silently wrap the
// privacy provider in a shared cache and leak one user's redacted view to
// another (PROD_READINESS_AUDIT §P-1).
func TestNewProvider_RejectsCallerScopedProvider(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected NewProvider to panic/refuse when wrapping a caller-scoped provider, but it returned normally")
		}
	}()
	_ = NewProvider(callerScopedFake{})
}

// P-1: a decorator that wraps (and promotes the marker of) a caller-scoped
// provider must ALSO be rejected — the structural guard has to follow the
// promoted marker, not just the concrete ProxyDataProvider type.
func TestNewProvider_RejectsDecoratedCallerScopedProvider(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected NewProvider to panic/refuse when wrapping a decorator that promotes CallerScoped(), but it returned normally")
		}
	}()
	_ = NewProvider(scopedDecorator{})
}

// P-1: a plain (non-caller-scoped) provider must be cached normally — no panic.
func TestNewProvider_AllowsPlainProvider(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewProvider must NOT panic for a non-caller-scoped provider, got panic: %v", r)
		}
	}()
	cp := NewProvider(plainFake{})
	if cp == nil {
		t.Fatal("expected a non-nil CachingProvider for a plain provider")
	}
}
