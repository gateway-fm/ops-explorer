package publicapi

// Contract under test: the public-api HTTP route layer wiring (audit T: public
// surface incl. handlers). These assert the documented per-route status
// contract (200 on success, 400 on a malformed path param, 404 on a nil result,
// 503 when the data source is down, and the rate-limit headers the middleware
// promises) — derived from the handler/route contract, not harvested output.
//
// Tests drive the full server via NewServer(...).router so chi routing, the
// CORS/timeout middleware, and the (W-4-fixed) rate limiter are all in the path.
// The provider is a stub embedding api.DataProvider; only the methods a given
// route needs are implemented. Helpers use the `tc` prefix.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"explorer/internal/api"
	"explorer/internal/types"
)

// tcProvider is a stub api.DataProvider. The embedded nil interface supplies
// the full method set; only the fields/methods exercised here are overridden.
// A method left at its embedded nil would panic if called — which keeps the
// tests honest about exactly which provider calls each route makes.
type tcProvider struct {
	api.DataProvider

	latestBlockNum uint64
	latestBlockErr error

	block      *types.Block
	blockErr   error
	blockTxs   []types.Transaction
	blockTxErr error
}

func (p *tcProvider) GetLatestBlockNumber(context.Context) (uint64, error) {
	return p.latestBlockNum, p.latestBlockErr
}
func (p *tcProvider) GetBlock(_ context.Context, _ uint64) (*types.Block, error) {
	return p.block, p.blockErr
}
func (p *tcProvider) GetTransactionsByBlock(_ context.Context, _ uint64) ([]types.Transaction, error) {
	return p.blockTxs, p.blockTxErr
}

// tcServer builds a public-api Server with a generous rate limit so route tests
// are not throttled, wired to the given provider.
func tcServer(p api.DataProvider) *Server {
	return NewServer(p, nil, nil, 0, 100000, time.Minute)
}

// tcReq sends a GET through the server router and returns the recorder. A unique
// RemoteAddr keeps each test in its own rate-limit bucket.
func tcReq(s *Server, path, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func TestHealthCheck_Healthy(t *testing.T) {
	s := tcServer(&tcProvider{latestBlockNum: 100})
	w := tcReq(s, "/health", "203.0.113.10:1")

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "healthy" {
		t.Fatalf("status = %v, want healthy", body["status"])
	}
}

func TestHealthCheck_Unhealthy(t *testing.T) {
	// Contract: a provider error -> 503 with status:unhealthy.
	s := tcServer(&tcProvider{latestBlockErr: errors.New("indexer down")})
	w := tcReq(s, "/health", "203.0.113.10:2")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "unhealthy" {
		t.Fatalf("status = %v, want unhealthy", body["status"])
	}
}

func TestGetBlock_Success(t *testing.T) {
	s := tcServer(&tcProvider{
		block:    &types.Block{Number: 42, Hash: "0xabc"},
		blockTxs: []types.Transaction{},
	})
	w := tcReq(s, "/api/v1/blocks/42", "203.0.113.11:1")

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var body struct {
		Block types.Block `json:"block"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Block.Number != 42 {
		t.Fatalf("block.number = %d, want 42", body.Block.Number)
	}
}

func TestGetBlock_InvalidNumber(t *testing.T) {
	// Contract: a non-numeric {number} path param -> 400.
	s := tcServer(&tcProvider{})
	w := tcReq(s, "/api/v1/blocks/not-a-number", "203.0.113.11:2")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", w.Code)
	}
}

func TestGetBlock_NotFound(t *testing.T) {
	// Contract: a nil block (no error) -> 404.
	s := tcServer(&tcProvider{block: nil})
	w := tcReq(s, "/api/v1/blocks/999", "203.0.113.11:3")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", w.Code)
	}
}

func TestRateLimitHeadersPresent(t *testing.T) {
	// Contract: the limiter middleware sets X-RateLimit-* on every response.
	s := tcServer(&tcProvider{latestBlockNum: 1})
	w := tcReq(s, "/health", "203.0.113.12:1")

	for _, h := range []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		if w.Header().Get(h) == "" {
			t.Errorf("missing %s header", h)
		}
	}
}

func TestUnknownRoute404(t *testing.T) {
	s := tcServer(&tcProvider{})
	w := tcReq(s, "/api/v1/does-not-exist", "203.0.113.13:1")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404 for unknown route", w.Code)
	}
}
