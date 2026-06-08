package price

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// capturingRT records the URL of every outbound request and returns a canned
// CoinGecko-shaped response so fetchAndCache succeeds.
type capturingRT struct {
	count   atomic.Int64
	lastURL atomic.Value // string
	body    string
}

func (rt *capturingRT) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.count.Add(1)
	rt.lastURL.Store(r.URL.String())
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(rt.body)),
		Header:     make(http.Header),
	}, nil
}

// P-4: the configured coin id must appear in the outbound CoinGecko request URL
// (the egress target must follow PRICE_COIN_ID, not a hardcoded "ethereum").
func TestPrice_ConfiguredCoinIDAppearsInRequestURL(t *testing.T) {
	rt := &capturingRT{body: `{"matic-network":{"usd":1.23,"usd_24h_change":0.5}}`}
	s := NewService("matic-network", "eur", time.Minute)
	s.httpClient = &http.Client{Transport: rt}

	if _, err := s.GetPrice(context.Background()); err != nil {
		t.Fatalf("GetPrice: %v", err)
	}

	gotURL, _ := rt.lastURL.Load().(string)
	if !strings.Contains(gotURL, "ids=matic-network") {
		t.Errorf("request URL %q does not carry the configured coin id (ids=matic-network)", gotURL)
	}
	if !strings.Contains(gotURL, "vs_currencies=eur") {
		t.Errorf("request URL %q does not carry the configured currency (vs_currencies=eur)", gotURL)
	}
}

// P-4: when price is disabled, StartBackgroundRefresh is simply never invoked,
// and the service must make NO outbound egress on construction alone — so a
// privacy/air-gapped deployment that leaves ENABLE_PRICE off never hits
// CoinGecko. (The main wiring only calls StartBackgroundRefresh when
// cfg.EnablePrice; this asserts the service honours that by not egressing on
// its own.)
func TestPrice_NoEgressWithoutRefresh(t *testing.T) {
	rt := &capturingRT{body: `{"ethereum":{"usd":1.0}}`}
	s := NewService("ethereum", "usd", time.Minute)
	s.httpClient = &http.Client{Transport: rt}

	// Construct only; do NOT call StartBackgroundRefresh or GetPrice.
	time.Sleep(20 * time.Millisecond) // give any (erroneous) background goroutine a chance

	if n := rt.count.Load(); n != 0 {
		t.Errorf("expected zero outbound requests when refresh is not started, got %d", n)
	}
}

// P-4 (positive control): starting the background refresher DOES egress, so the
// disabled-case assertion above is meaningful (not vacuously true).
func TestPrice_RefreshEgressesWhenStarted(t *testing.T) {
	done := make(chan struct{}, 1)
	rt := &signallingRT{body: `{"ethereum":{"usd":1.0}}`, done: done}
	s := NewService("ethereum", "usd", time.Minute)
	s.httpClient = &http.Client{Transport: rt}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.StartBackgroundRefresh(ctx)

	select {
	case <-done:
		// got the expected egress
	case <-time.After(2 * time.Second):
		t.Fatal("expected an outbound request after StartBackgroundRefresh, got none")
	}
}

type signallingRT struct {
	body string
	done chan struct{}
}

func (rt *signallingRT) RoundTrip(r *http.Request) (*http.Response, error) {
	select {
	case rt.done <- struct{}{}:
	default:
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(rt.body)),
		Header:     make(http.Header),
	}, nil
}
