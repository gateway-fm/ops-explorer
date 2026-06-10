package api

// Counter / total consistency — PRIVACY pass-through (plan §4.2).
//
// In privacy mode the explorer is a thin BFF over privacy-proxy. The proxy
// intentionally returns total = the count of VISIBLE/redacted rows, which may
// differ from the raw DB total (e.g. "2 of 50"). ProxyDataProvider must
// faithfully surface the proxy's total — it must NOT "correct" it to len(data).
// These tests stand up a mock proxy and assert the verbatim pass-through,
// including the redacted {data:[2], total:50} case.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"explorer/internal/rpc"
	"explorer/internal/types"
)

// ppProxy stands up a mock privacy-proxy returning `body` (JSON) for any path,
// and a ProxyDataProvider pointed at it. The auth token is injected via ctx.
func ppProxy(t *testing.T, body string) (*ProxyDataProvider, context.Context) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The provider forwards the caller's Bearer; assert it travels.
		if r.Header.Get("Authorization") == "" {
			t.Errorf("proxy request missing Authorization Bearer (path %s)", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	ctx := context.WithValue(context.Background(), rpc.ContextKeyAuthToken, "viewer-jwt")
	return NewProxyDataProvider(srv.URL), ctx
}

func TestProxyProvider_PaginatedTotalPassthrough_Equal(t *testing.T) {
	// {data:[3 rows], total:3} -> surface 3 rows, total 3.
	p, ctx := ppProxy(t, `{"data":[{"hash":"0x1"},{"hash":"0x2"},{"hash":"0x3"}],"total":3}`)
	data, total, err := p.GetTransactionsPaginated(ctx, 1, 25)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(data) != 3 || total != 3 {
		t.Errorf("got len=%d total=%d, want 3/3", len(data), total)
	}
}

func TestProxyProvider_PaginatedTotalPassthrough_Redacted(t *testing.T) {
	// THE headline case: proxy returns only 2 visible rows but total=50 (the raw
	// DB total). The explorer must surface BOTH unchanged — visibility is the
	// proxy's call, not the explorer's. A naive len(data) "correction" would
	// report total=2 and is forbidden.
	p, ctx := ppProxy(t, `{"data":[{"hash":"0x1"},{"hash":"0x2"}],"total":50}`)

	check := func(name string, data []types.Transaction, total int64, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s err: %v", name, err)
		}
		if len(data) != 2 {
			t.Errorf("%s: len(data) = %d, want 2 (visible rows)", name, len(data))
		}
		if total != 50 {
			t.Errorf("%s: total = %d, want 50 unchanged (explorer must NOT correct to len(data)=2)", name, total)
		}
	}

	d, tot, err := p.GetTransactionsPaginated(ctx, 1, 25)
	check("GetTransactionsPaginated", d, tot, err)

	d2, tot2, err2 := p.GetTransactionsPaginatedWithCategories(ctx, 1, 25)
	check("GetTransactionsPaginatedWithCategories", d2, tot2, err2)
}

func TestProxyProvider_AccountsTotalPassthrough_Redacted(t *testing.T) {
	// Same pass-through contract for accounts: 1 visible of 99.
	p, ctx := ppProxy(t, `{"data":[{"address":"0xabc"}],"total":99}`)
	data, total, err := p.GetAccountsPaginated(ctx, 1, 25)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(data) != 1 || total != 99 {
		t.Errorf("got len=%d total=%d, want 1/99 (proxy total unchanged)", len(data), total)
	}
}

func TestProxyProvider_TokensTotalPassthrough_Redacted(t *testing.T) {
	p, ctx := ppProxy(t, `{"data":[{"address":"0xtok"}],"total":1000}`)
	data, total, err := p.GetTokens(ctx, 25, 0, "", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(data) != 1 || total != 1000 {
		t.Errorf("got len=%d total=%d, want 1/1000", len(data), total)
	}
}

// Sanity: the auth Bearer is required (the proxy enforces RBAC per caller). Also
// confirms a missing data array decodes to an empty slice, not a panic.
func TestProxyProvider_EmptyData(t *testing.T) {
	p, ctx := ppProxy(t, `{"data":[],"total":0}`)
	data, total, err := p.GetTransactionsPaginated(ctx, 1, 25)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(data) != 0 || total != 0 {
		t.Errorf("got len=%d total=%d, want 0/0", len(data), total)
	}
	// JSON round-trips deterministically.
	if b, _ := json.Marshal(data); string(b) != "null" && string(b) != "[]" {
		t.Errorf("unexpected data marshal %s", b)
	}
}
