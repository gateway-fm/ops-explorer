//go:build privacy

package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"explorer/internal/auth"
	"explorer/internal/privacy"
	"explorer/internal/rpc"
	"explorer/pkg/log"

	"github.com/go-chi/chi/v5"
)

// capturingHandler is a slog.Handler that records every attribute key it sees
// across all log records, so a test can assert that privacy-sensitive
// identifiers are never emitted.
type capturingHandler struct {
	mu    sync.Mutex
	keys  map[string]string // key -> stringified value (last seen)
	attrs []slog.Attr
}

func newCapturingHandler() *capturingHandler {
	return &capturingHandler{keys: map[string]string{}}
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, a := range h.attrs {
		h.keys[a.Key] = a.Value.String()
	}
	r.Attrs(func(a slog.Attr) bool {
		h.keys[a.Key] = a.Value.String()
		return true
	})
	return nil
}

func (h *capturingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	nh := &capturingHandler{keys: h.keys}
	nh.attrs = append(nh.attrs, h.attrs...)
	nh.attrs = append(nh.attrs, attrs...)
	return nh
}

func (h *capturingHandler) WithGroup(string) slog.Handler { return h }

func (h *capturingHandler) sawKey(key string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, ok := h.keys[key]
	return v, ok
}

// errHTTPClient is an http.Client whose RoundTripper always fails, forcing the
// privacy/eth-link handlers down their log.Warn error branches.
func errRoundTripper(*http.Request) (*http.Response, error) {
	return nil, errors.New("simulated proxy failure")
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestPrivacyHandlers_NoPIIInLogs is the P-3 lint test (privacy build): it
// drives the privacy / eth-link handlers and the impersonation middleware down
// their logging paths with real on-chain identifiers and asserts that no log
// record ever carries a key that would let log/SIEM access correlate a DID with
// the addresses/grants it viewed: viewer_did, grant_id, address_id, caller,
// admin, address.
func TestPrivacyHandlers_NoPIIInLogs(t *testing.T) {
	cap := newCapturingHandler()
	prev := log.Default()
	log.SetDefault(slog.New(cap))
	t.Cleanup(func() { log.SetDefault(prev) })

	errClient := &http.Client{Transport: rtFunc(errRoundTripper)}
	pc := privacy.NewClientWithHTTP("https://proxy.invalid", errClient)

	s := &Server{
		privacyClient: pc,
		provider:      NewProxyDataProvider("https://proxy.invalid"),
	}

	const (
		did     = "did:privado:test-viewer-123"
		addr    = "0x52908400098527886E0F7030069857D2E4169EE7"
		grantID = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
		addrID  = "9b7e4f5a-1c2d-4e3f-8a9b-0c1d2e3f4a5b"
	)

	// Build a request whose auth cookie carries a DID claim so getViewerIdentity
	// resolves a viewer (drives handlers past the auth gate into the proxy call).
	authedReq := func(method, target string) *http.Request {
		r := httptest.NewRequest(method, target, nil)
		r.AddCookie(&http.Cookie{Name: AuthCookieName, Value: makeJWTForDID(did)})
		// Attach the auth token to context as authContextMiddleware would.
		r = r.WithContext(context.WithValue(r.Context(), rpc.ContextKeyAuthToken, "tok"))
		return r
	}

	// 1. handleGetViewableAddresses -> log.Warn (viewer_did)
	{
		r := authedReq(http.MethodGet, "/api/privacy/viewable-addresses")
		s.handleGetViewableAddresses(httptest.NewRecorder(), r)
	}

	// 2. handleGetGrantedAddress -> log.Warn (grant_id, address_id)
	{
		r := authedReq(http.MethodGet, "/api/privacy/grant/"+grantID+"/"+addrID)
		r = withChiURLParams(r, map[string]string{"grantId": grantID, "addressId": addrID})
		s.handleGetGrantedAddress(httptest.NewRecorder(), r)
	}

	// 3. handleGetGrantedAddressTransactions -> log.Warn (grant_id, address_id)
	{
		r := authedReq(http.MethodGet, "/api/privacy/grant/"+grantID+"/"+addrID+"/transactions")
		r = withChiURLParams(r, map[string]string{"grantId": grantID, "addressId": addrID})
		s.handleGetGrantedAddressTransactions(httptest.NewRecorder(), r)
	}

	// 4. handleGetGrantActivityLogs -> log.Warn (grant_id)
	{
		r := authedReq(http.MethodGet, "/api/privacy/grant/"+grantID+"/activity")
		r = withChiURLParams(r, map[string]string{"grantId": grantID})
		s.handleGetGrantActivityLogs(httptest.NewRecorder(), r)
	}

	// 5. handleVerifyLink -> log.Warn (address)
	{
		body := strings.NewReader(`{"nonce":"n","address":"` + addr + `","signature":"0xsig"}`)
		r := httptest.NewRequest(http.MethodPost, "/api/eth/link/verify", body)
		r.AddCookie(&http.Cookie{Name: AuthCookieName, Value: makeJWTForDID(did)})
		r = r.WithContext(context.WithValue(r.Context(), rpc.ContextKeyAuthToken, "tok"))
		s.handleVerifyLink(httptest.NewRecorder(), r)
	}

	// 6. handleUnlinkAddress -> log.Warn (address)
	{
		r := httptest.NewRequest(http.MethodDelete, "/api/eth/addresses/"+addr, nil)
		r.AddCookie(&http.Cookie{Name: AuthCookieName, Value: makeJWTForDID(did)})
		r = r.WithContext(context.WithValue(r.Context(), rpc.ContextKeyAuthToken, "tok"))
		r = withChiURLParams(r, map[string]string{"address": addr})
		s.handleUnlinkAddress(httptest.NewRecorder(), r)
	}

	// 7. impersonationMiddleware caller/admin mismatch -> log.Warn (caller, admin)
	{
		store := NewMemoryImpersonationStoreNoGC()
		tok, _, err := store.Mint(context.Background(), ImpersonationSession{
			AdminDID:  "did:privado:admin-999",
			TargetDID: "did:privado:target-1",
			OrgID:     "org-1",
		}, 0)
		if err != nil {
			t.Fatalf("mint: %v", err)
		}
		s2 := &Server{impersonations: store}
		mw := s2.impersonationMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		r := httptest.NewRequest(http.MethodGet, "/api/v1/blocks", nil)
		r.Header.Set(ImpersonateHeader, tok)
		// Caller DID (from cookie) deliberately differs from the admin DID.
		r.AddCookie(&http.Cookie{Name: AuthCookieName, Value: makeJWTForDID("did:privado:someone-else")})
		mw.ServeHTTP(httptest.NewRecorder(), r)
	}

	forbidden := []string{"viewer_did", "grant_id", "address_id", "caller", "admin", "address"}
	for _, key := range forbidden {
		if v, ok := cap.sawKey(key); ok {
			t.Errorf("privacy-build log emitted forbidden key %q (value=%q) — PII must not reach logs (P-3)", key, v)
		}
	}
}

// makeJWTForDID builds a display-only JWT carrying the given DID as sub,
// reusing the shared makeTestJWT fixture.
func makeJWTForDID(did string) string {
	return makeTestJWT(did, time.Now().Add(time.Hour))
}

// withChiURLParams attaches chi URL params to a request's context so handlers
// that call chi.URLParam resolve them.
func withChiURLParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

var _ = auth.JWTClaims{} // keep auth import even if fixtures change
var _ = io.Discard
