package api

// RD-1149: opaque keyset cursor pass-through for the by-address feeds.
//
// The block-explorer BFF treats the cursor as opaque: it forwards ?cursor= to
// privacy-proxy (preferring it over the legacy ?before= block bound) and reads
// the proxy's next_cursor from the JSON response body ("" / absent = feed
// exhausted). These tests stand up a mock proxy that records the outbound query
// and controls the response body envelope.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"explorer/internal/rpc"
)

// cursorProxy stands up a mock privacy-proxy that records the last request's
// parsed query and returns the given JSON body verbatim.
func cursorProxy(t *testing.T, body string) (*ProxyDataProvider, context.Context, *url.Values) {
	t.Helper()
	var lastQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	ctx := context.WithValue(context.Background(), rpc.ContextKeyAuthToken, "viewer-jwt")
	return NewProxyDataProvider(srv.URL), ctx, &lastQuery
}

func TestProxyProvider_ByAddressTx_SendsCursorAndReadsBodyNextCursor(t *testing.T) {
	p, ctx, lastQuery := cursorProxy(t, `{"transactions":[{"hash":"0x1"},{"hash":"0x2"}],"next_cursor":"next-abc"}`)

	txs, next, err := p.GetTransactionsByAddress(ctx, "0xabc", 25, "cur-123", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(txs) != 2 {
		t.Errorf("len(txs) = %d, want 2", len(txs))
	}
	if next != "next-abc" {
		t.Errorf("next cursor = %q, want %q (from body next_cursor)", next, "next-abc")
	}
	// cursor must be present; ?before= must be absent when a cursor is set.
	if got := lastQuery.Get("cursor"); got != "cur-123" {
		t.Errorf("outbound cursor = %q, want %q", got, "cur-123")
	}
	if lastQuery.Has("before") {
		t.Errorf("outbound query must not carry ?before= when a cursor is set: %v", *lastQuery)
	}
}

func TestProxyProvider_ByAddressTransfers_SendsCursorAndReadsBodyNextCursor(t *testing.T) {
	p, ctx, lastQuery := cursorProxy(t, `{"transfers":[{"txHash":"0x1"}],"next_cursor":"next-xfer"}`)

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

// No next_cursor in the body => exhausted feed => "" (and HasMore is false once
// the handler wraps it via cursorPage).
func TestProxyProvider_ByAddress_NoBodyCursorMeansExhausted(t *testing.T) {
	p, ctx, _ := cursorProxy(t, `{"transactions":[{"hash":"0x1"}]}`)
	_, next, err := p.GetTransactionsByAddress(ctx, "0xabc", 25, "cur-1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if next != "" {
		t.Errorf("next cursor = %q, want empty (no body next_cursor = exhausted)", next)
	}
}

// When no cursor is supplied the provider falls back to the legacy ?before=
// block-exclusive bound (older-proxy compatibility path).
func TestProxyProvider_ByAddress_FallsBackToBefore(t *testing.T) {
	p, ctx, lastQuery := cursorProxy(t, `{"transactions":[]}`)
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
