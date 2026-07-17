package api

// RD-1149: opaque keyset cursor pass-through for the by-address feeds.
//
// The block-explorer BFF treats the cursor as opaque: it forwards ?cursor= to
// privacy-proxy (preferring it over the legacy ?before= block bound) and surfaces
// the proxy's X-Next-Cursor response header as the next-page cursor ("" when the
// header is absent = feed exhausted). These tests stand up a mock proxy that
// records the outbound query and controls the response header.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"explorer/internal/rpc"
)

// cursorProxy stands up a mock privacy-proxy that records the last request's
// parsed query and echoes a fixed X-Next-Cursor header (empty = omit the header).
func cursorProxy(t *testing.T, body, nextCursor string) (*ProxyDataProvider, context.Context, *url.Values) {
	t.Helper()
	var lastQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastQuery = r.URL.Query()
		if nextCursor != "" {
			w.Header().Set("X-Next-Cursor", nextCursor)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	ctx := context.WithValue(context.Background(), rpc.ContextKeyAuthToken, "viewer-jwt")
	return NewProxyDataProvider(srv.URL), ctx, &lastQuery
}

func TestProxyProvider_ByAddressTx_SendsCursorAndSurfacesHeader(t *testing.T) {
	p, ctx, lastQuery := cursorProxy(t, `[{"hash":"0x1"},{"hash":"0x2"}]`, "next-abc")

	txs, next, err := p.GetTransactionsByAddress(ctx, "0xabc", 25, "cur-123", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(txs) != 2 {
		t.Errorf("len(txs) = %d, want 2", len(txs))
	}
	if next != "next-abc" {
		t.Errorf("next cursor = %q, want %q (from X-Next-Cursor header)", next, "next-abc")
	}
	// cursor must be present; ?before= must be absent when a cursor is set.
	if got := lastQuery.Get("cursor"); got != "cur-123" {
		t.Errorf("outbound cursor = %q, want %q", got, "cur-123")
	}
	if lastQuery.Has("before") {
		t.Errorf("outbound query must not carry ?before= when a cursor is set: %v", *lastQuery)
	}
}

func TestProxyProvider_ByAddressTransfers_SendsCursorAndSurfacesHeader(t *testing.T) {
	p, ctx, lastQuery := cursorProxy(t, `[{"txHash":"0x1"}]`, "next-xfer")

	xfers, next, err := p.GetTransfersByAddress(ctx, "0xabc", 25, "cur-xfer", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(xfers) != 1 {
		t.Errorf("len(xfers) = %d, want 1", len(xfers))
	}
	if next != "next-xfer" {
		t.Errorf("next cursor = %q, want %q", next, "next-xfer")
	}
	if got := lastQuery.Get("cursor"); got != "cur-xfer" {
		t.Errorf("outbound cursor = %q, want %q", got, "cur-xfer")
	}
}

// No X-Next-Cursor header on the response => exhausted feed => "" (and HasMore is
// false once the handler wraps it).
func TestProxyProvider_ByAddress_NoHeaderMeansExhausted(t *testing.T) {
	p, ctx, _ := cursorProxy(t, `[{"hash":"0x1"}]`, "")
	_, next, err := p.GetTransactionsByAddress(ctx, "0xabc", 25, "cur-1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if next != "" {
		t.Errorf("next cursor = %q, want empty (no X-Next-Cursor header = exhausted)", next)
	}
}

// When no cursor is supplied the provider falls back to the legacy ?before=
// block-exclusive bound (older-proxy compatibility path).
func TestProxyProvider_ByAddress_FallsBackToBefore(t *testing.T) {
	p, ctx, lastQuery := cursorProxy(t, `[]`, "")
	before := uint64(100)
	if _, _, err := p.GetTransactionsByAddress(ctx, "0xabc", 25, "", &before); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := lastQuery.Get("before"); got != "100" {
		t.Errorf("outbound before = %q, want %q (fallback path)", got, "100")
	}
	if lastQuery.Has("cursor") {
		t.Errorf("outbound query must not carry ?cursor= when none is set: %v", *lastQuery)
	}
}
