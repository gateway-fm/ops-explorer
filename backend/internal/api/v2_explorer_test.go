//go:build privacy

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestV2ReadClosedQueryAndRouteSurface(t *testing.T) {
	if len(v2ReadRoutes) != 36 {
		t.Fatal("ordinary route count")
	}
	for _, x := range []struct{ kind, raw string }{
		{"transactions", "offset=1"}, {"page", "limit=01"}, {"page", "limit=1001"}, {"page", "limit=1&limit=2"}, {"", "limit=1"}, {"logs", "from=9&to=1"}, {"logs", "topic0=0x1234"}, {"tokens", "type=other"}, {"search", "q=bank&limit=21"}, {"search", "q=%00"}, {"chart", "interval=year"}, {"page", "cursor=%0A"},
	} {
		if _, e := v2ReadQuery(x.raw, x.kind); e == nil {
			t.Fatalf("invalid query accepted: %s %s", x.kind, x.raw)
		}
	}
	for _, x := range []struct{ kind, raw string }{{"page", "limit=1000&cursor=opaque.A_-"}, {"transactions", "category=token_transfer"}, {"logs", "from=0&to=9223372036854775807"}, {"search", "q=Bank+One&limit=20"}, {"chart", "interval=week&limit=365"}} {
		if _, e := v2ReadQuery(x.raw, x.kind); e != nil {
			t.Fatalf("legal query rejected: %s", x.kind)
		}
	}
}

// Actual restricted BFF storage/cookie capture and TLS transport; upstream DTOs
// are protocol fixtures here, not qualification of the owned OPS data engine.
func TestV2ReadActualSessionLosslessWireAndChangedCapture(t *testing.T) {
	f := newWalletAppFixture(t)
	base := http.DefaultTransport
	f.server.v2Reads.client.Transport = walletRoundTrip(func(r *http.Request) (*http.Response, error) {
		if d, ok := r.Context().Deadline(); !ok || time.Until(d) > 25*time.Second {
			t.Error("read request deadline")
		}
		return base.RoundTrip(r)
	})
	const body = `{"items":[{"hash":"0x1111111111111111111111111111111111111111111111111111111111111111","value":"9007199254740993123456789"}],"page":{"next":"opaque-next","hasMore":null,"scanLimited":true,"population":"LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS"}}`
	f.onWallet = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/explorer/transactions" || r.URL.RawQuery != "limit=25" {
			t.Error("fixed selected upstream route/query")
		}
		w.Header().Set("Set-Cookie", "not-forwarded=1")
		_, _ = w.Write([]byte(body))
	}
	response := f.call("GET", "/api/v2/explorer/transactions?limit=25", "")
	if response.Code != 200 || response.Body.String() != body || response.Header().Get("Set-Cookie") != "" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("lossless checked source bytes not preserved", response.Code)
	}
	before := f.effects.Load()
	for _, path := range []string{"/api/transactions", "/api/v1/transactions", "/api/v2/explorer/tokens/0x11/inventory", "/api/v2/explorer/transactions?offset=1", "/api/v2/explorer/transactions?limit=1&limit=2", "/api/v2/explorer/transactions/0x123"} {
		if got := f.call("GET", path, ""); got.Code < 400 {
			t.Fatal("extra or malformed read route", got.Code)
		}
	}
	if f.effects.Load() != before {
		t.Fatal("invalid route/query reached upstream")
	}
	f.onWallet = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"error":{"code":"cursor_stale"}}`))
	}
	if got := f.call("GET", "/api/v2/explorer/transactions?cursor=old", ""); got.Code != 409 || !strings.Contains(got.Body.String(), "cursor_stale") {
		t.Fatal("source cursor conflict not preserved", got.Code)
	}
	if got := f.call("GET", "/api/auth/status", ""); got.Code != 200 || !strings.Contains(got.Body.String(), `"authenticated":true`) {
		t.Fatal("cursor conflict revoked app context")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	defer closeIfOpen(release)
	f.onWallet = func(w http.ResponseWriter, r *http.Request) { close(entered); <-release; _, _ = w.Write([]byte(body)) }
	out := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.server.Handler().ServeHTTP(out, f.request("GET", "/api/v2/explorer/transactions", nil))
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("held read did not reach upstream")
	}
	if got := f.call("POST", "/api/auth/logout", "{}"); got.Code != 200 {
		t.Fatal("actual durable logout", got.Code)
	}
	close(release)
	<-done
	if out.Code != 409 || strings.Contains(out.Body.String(), "9007199254740993123456789") {
		t.Fatal("old private data escaped final capture gate", out.Code)
	}
}
