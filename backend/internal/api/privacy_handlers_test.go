package api

// Contract under test: the privacy BFF handlers in privacy_handlers.go and the
// privacy.Client status-mapping contract they depend on. Expected values are
// derived from the per-handler contract documented in the audit (§A3) and the
// privacy.Client source contract, NOT from observing runtime output:
//
//   - handleGetGrantedAddress -> ResolveAddressID: privacy.Client collapses
//     proxy 401/403/404 to ErrNotFound (client.go:359-361); the handler maps
//     ErrNotFound -> 404 (privacy_handlers.go:85-87). Any other proxy non-200
//     -> 500.
//   - handleGetGrantedAddressTransactions / handleGetGrantActivityLogs:
//     GetGrantTransactions/GetGrantActivityLogs return (body,status,err) and the
//     handler PASSES THE PROXY STATUS THROUGH (privacy_handlers.go:185-187,
//     220-222). A proxy 403 must surface as a 403, not be collapsed.
//   - handleGetViewableAddresses: GetViewableAddressesWithIdentity errors on any
//     non-200; the handler maps that to 500 (privacy_handlers.go:39-43).
//   - Viewer identity: getViewerIdentity reads the explorer_auth cookie via
//     auth.ExtractClaims, which does NOT verify the signature — an unsigned
//     header.<b64url-json>.sig with a future exp yields a non-empty DID. No
//     cookie / expired -> empty DID -> the handler's auth-required branch.
//
// Server is built as a keyed &Server{...} literal directly (NOT api.New), per
// the coexistence contract. All helpers use the `tc` prefix.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"explorer/internal/privacy"
	"explorer/internal/types"

	"github.com/go-chi/chi/v5"
)

// --- viewer cookie -----------------------------------------------------------

// tcViewerJWT builds an UNSIGNED JWT (header.<b64url-json>.sig) carrying the
// given DID subject and a future expiry. auth.ExtractClaims base64-decodes the
// payload without signature verification, so this is sufficient for
// getViewerIdentity -> GetAuthDID to return the DID.
func tcViewerJWT(did string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims := fmt.Sprintf(`{"sub":%q,"exp":%d}`, did, time.Now().Add(time.Hour).Unix())
	payload := base64.RawURLEncoding.EncodeToString([]byte(claims))
	return header + "." + payload + ".unsigned"
}

// tcWithViewer attaches a viewer auth cookie carrying the given DID.
func tcWithViewer(r *http.Request, did string) *http.Request {
	r.AddCookie(&http.Cookie{Name: AuthCookieName, Value: tcViewerJWT(did)})
	return r
}

// tcChiReq wraps a request with chi URL params so chi.URLParam works without a
// full router.
func tcChiReq(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- minimal data provider --------------------------------------------------

// tcStubProvider implements api.DataProvider but only the three methods
// handleGetGrantedAddress exercises (GetAddressStats/GetBalance/GetCode) do
// anything; the rest are embedded from a nil interface and must not be called
// by the handlers under test. Embedding the interface gives us the full method
// set without hand-writing ~40 stubs.
type tcStubProvider struct {
	DataProvider // nil; only the overridden methods may be called
	stats        *types.AddressStats
	balance      *types.JSONString
	code         []byte
}

func (p *tcStubProvider) GetAddressStats(context.Context, string) (*types.AddressStats, error) {
	return p.stats, nil
}
func (p *tcStubProvider) GetBalance(context.Context, string) (*types.JSONString, error) {
	return p.balance, nil
}
func (p *tcStubProvider) GetCode(context.Context, string) ([]byte, error) {
	return p.code, nil
}

// --- mock privacy proxy ------------------------------------------------------

// tcProxy spins an httptest server using `h` as its handler and returns a
// Server wired with a privacy.Client pointed at it plus a stub provider.
func tcServerWithProxy(t *testing.T, h http.HandlerFunc, prov DataProvider) *Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Server{
		privacyClient: privacy.NewClient(srv.URL),
		provider:      prov,
	}
}

// =============================================================================
// handleGetGrantedAddress — ResolveAddressID status mapping
// =============================================================================

func TestHandleGetGrantedAddress_ProxyForbidden_MapsToNotFound(t *testing.T) {
	// Contract: proxy 403 -> ResolveAddressID returns ErrNotFound -> handler 404.
	// This is the ONE handler with the 401/403/404->ErrNotFound collapse.
	for _, proxyStatus := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		t.Run(http.StatusText(proxyStatus), func(t *testing.T) {
			s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
				// Resolve endpoint shape per client.go:342.
				if !strings.Contains(r.URL.Path, "/resolve/") {
					t.Errorf("unexpected proxy path %q", r.URL.Path)
				}
				w.WriteHeader(proxyStatus)
			}, &tcStubProvider{})

			req := tcChiReq(
				tcWithViewer(httptest.NewRequest(http.MethodGet, "/api/privacy/grant/g1/addr1", nil), "did:example:viewer"),
				map[string]string{"grantId": "g1", "addressId": "addr1"},
			)
			w := httptest.NewRecorder()
			s.handleGetGrantedAddress(w, req)

			if w.Code != http.StatusNotFound {
				t.Fatalf("proxy %d -> handler %d, want 404 (ErrNotFound collapse)", proxyStatus, w.Code)
			}
		})
	}
}

func TestHandleGetGrantedAddress_ProxyServerError_MapsTo500(t *testing.T) {
	// Contract: a proxy 5xx is NOT ErrNotFound; ResolveAddressID returns a
	// generic error -> handler 500.
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}, &tcStubProvider{})

	req := tcChiReq(
		tcWithViewer(httptest.NewRequest(http.MethodGet, "/api/privacy/grant/g1/addr1", nil), "did:example:viewer"),
		map[string]string{"grantId": "g1", "addressId": "addr1"},
	)
	w := httptest.NewRecorder()
	s.handleGetGrantedAddress(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("proxy 502 -> handler %d, want 500", w.Code)
	}
}

func TestHandleGetGrantedAddress_Unauthenticated(t *testing.T) {
	// Contract: no viewer cookie -> empty DID -> 401 BEFORE any proxy call.
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("proxy must NOT be called for an unauthenticated request")
	}, &tcStubProvider{})

	req := tcChiReq(
		httptest.NewRequest(http.MethodGet, "/api/privacy/grant/g1/addr1", nil),
		map[string]string{"grantId": "g1", "addressId": "addr1"},
	)
	w := httptest.NewRecorder()
	s.handleGetGrantedAddress(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no-cookie -> handler %d, want 401", w.Code)
	}
}

func TestHandleGetGrantedAddress_MissingParams(t *testing.T) {
	// Contract: empty grantId/addressId -> 400.
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("proxy must not be called when params are missing")
	}, &tcStubProvider{})

	req := tcChiReq(
		tcWithViewer(httptest.NewRequest(http.MethodGet, "/x", nil), "did:example:viewer"),
		map[string]string{"grantId": "", "addressId": ""},
	)
	w := httptest.NewRecorder()
	s.handleGetGrantedAddress(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing params -> handler %d, want 400", w.Code)
	}
}

func TestHandleGetGrantedAddress_FullDisclosure_HappyPath(t *testing.T) {
	// Contract: a "full" disclosure resolves to the real address, the handler
	// enriches with provider stats/balance/code, and emits GrantedAddressResponse
	// with display_address == real address.
	const realAddr = "0x407d73d8a49eeb85d32cf465507dd71d507100c1"
	resolve := privacy.ResolveAddressResponse{
		RealAddress:     realAddr,
		DisclosureLevel: "full",
		GrantID:         "g1",
		ScopeMethods:    []string{"transaction_history"},
	}
	bal := types.JSONString("12345")
	prov := &tcStubProvider{
		stats:   &types.AddressStats{TxCount: 7},
		balance: &bal,
		code:    []byte{0x60, 0x60}, // non-empty => IsContract true
	}
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resolve)
	}, prov)

	req := tcChiReq(
		tcWithViewer(httptest.NewRequest(http.MethodGet, "/api/privacy/grant/g1/addr1", nil), "did:example:viewer"),
		map[string]string{"grantId": "g1", "addressId": "addr1"},
	)
	w := httptest.NewRecorder()
	s.handleGetGrantedAddress(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("happy path -> %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var got GrantedAddressResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.DisplayAddress != realAddr {
		t.Errorf("display_address = %q, want real address %q (full disclosure)", got.DisplayAddress, realAddr)
	}
	if got.DisclosureLevel != "full" {
		t.Errorf("disclosure_level = %q, want full", got.DisclosureLevel)
	}
	if got.TxCount != 7 {
		t.Errorf("tx_count = %d, want 7", got.TxCount)
	}
	if got.Balance != "12345" {
		t.Errorf("balance = %q, want 12345", got.Balance)
	}
	if !got.IsContract {
		t.Errorf("is_contract = false, want true (non-empty code)")
	}
}

func TestHandleGetGrantedAddress_RedactedNeverExposesRealAddress(t *testing.T) {
	// SECURITY contract (privacy_handlers.go:106-121): for a non-full disclosure
	// the real address must NEVER appear in the response. "redacted" -> the
	// literal "[REDACTED]"; an UNKNOWN level fails safe to "[REDACTED]" too.
	const realAddr = "0x407d73d8a49eeb85d32cf465507dd71d507100c1"
	cases := []struct {
		level string
		want  string
	}{
		{"redacted", "[REDACTED]"},
		{"totally-unknown-level", "[REDACTED]"}, // fail-safe default
	}
	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			resolve := privacy.ResolveAddressResponse{
				RealAddress:     realAddr,
				DisclosureLevel: tc.level,
				GrantID:         "g1",
			}
			s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(resolve)
			}, &tcStubProvider{})

			req := tcChiReq(
				tcWithViewer(httptest.NewRequest(http.MethodGet, "/x", nil), "did:example:viewer"),
				map[string]string{"grantId": "g1", "addressId": "addr1"},
			)
			w := httptest.NewRecorder()
			s.handleGetGrantedAddress(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status %d, want 200", w.Code)
			}
			body := w.Body.String()
			if strings.Contains(strings.ToLower(body), strings.ToLower(realAddr)) {
				t.Fatalf("response leaked the real address for level %q: %s", tc.level, body)
			}
			var got GrantedAddressResponse
			_ = json.Unmarshal(w.Body.Bytes(), &got)
			if got.DisplayAddress != tc.want {
				t.Errorf("display_address = %q, want %q", got.DisplayAddress, tc.want)
			}
		})
	}
}

// =============================================================================
// handleGetGrantedAddressTransactions — proxy status passthrough
// =============================================================================

func TestHandleGetGrantedAddressTransactions_StatusPassthrough(t *testing.T) {
	// Contract: the proxy status is passed through verbatim. A 403 must surface
	// as 403 (not collapsed to 404/500), and the proxy body is forwarded as-is.
	const proxyBody = `{"transactions":[],"detail":"forbidden by proxy"}`
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/transactions") {
			t.Errorf("unexpected proxy path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(proxyBody))
	}, &tcStubProvider{})

	req := tcChiReq(
		tcWithViewer(httptest.NewRequest(http.MethodGet, "/x", nil), "did:example:viewer"),
		map[string]string{"grantId": "g1", "addressId": "addr1"},
	)
	w := httptest.NewRecorder()
	s.handleGetGrantedAddressTransactions(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("proxy 403 -> handler %d, want 403 (passthrough)", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != proxyBody {
		t.Fatalf("body = %q, want proxy body forwarded as-is", w.Body.String())
	}
}

func TestHandleGetGrantedAddressTransactions_Success(t *testing.T) {
	const proxyBody = `{"transactions":[{"hash":"0xabc"}]}`
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(proxyBody))
	}, &tcStubProvider{})

	req := tcChiReq(
		tcWithViewer(httptest.NewRequest(http.MethodGet, "/x?limit=10", nil), "did:example:viewer"),
		map[string]string{"grantId": "g1", "addressId": "addr1"},
	)
	w := httptest.NewRecorder()
	s.handleGetGrantedAddressTransactions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != proxyBody {
		t.Fatalf("body = %q, want proxy body forwarded", w.Body.String())
	}
}

func TestHandleGetGrantedAddressTransactions_Unauthenticated(t *testing.T) {
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("proxy must not be called when unauthenticated")
	}, &tcStubProvider{})

	req := tcChiReq(
		httptest.NewRequest(http.MethodGet, "/x", nil),
		map[string]string{"grantId": "g1", "addressId": "addr1"},
	)
	w := httptest.NewRecorder()
	s.handleGetGrantedAddressTransactions(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no-cookie -> %d, want 401", w.Code)
	}
}

// =============================================================================
// handleGetGrantActivityLogs — proxy status passthrough
// =============================================================================

func TestHandleGetGrantActivityLogs_StatusPassthrough(t *testing.T) {
	// Contract: proxy 403 passes through as 403.
	const proxyBody = `{"error":"not the grant holder"}`
	var gotAuth string
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if !strings.Contains(r.URL.Path, "/activity") {
			t.Errorf("unexpected proxy path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(proxyBody))
	}, &tcStubProvider{})

	const did = "did:example:viewer"
	req := tcChiReq(
		tcWithViewer(httptest.NewRequest(http.MethodGet, "/x", nil), did),
		map[string]string{"grantId": "g1"},
	)
	w := httptest.NewRecorder()
	s.handleGetGrantActivityLogs(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("proxy 403 -> handler %d, want 403 (passthrough)", w.Code)
	}
	// The viewer's JWT must be forwarded as a Bearer token (privacy_handlers.go:213).
	if gotAuth == "" || !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Fatalf("Authorization forwarded = %q, want a Bearer token", gotAuth)
	}
}

func TestHandleGetGrantActivityLogs_Success(t *testing.T) {
	const proxyBody = `{"logs":[],"total":0,"limit":20,"offset":0}`
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(proxyBody))
	}, &tcStubProvider{})

	req := tcChiReq(
		tcWithViewer(httptest.NewRequest(http.MethodGet, "/x", nil), "did:example:viewer"),
		map[string]string{"grantId": "g1"},
	)
	w := httptest.NewRecorder()
	s.handleGetGrantActivityLogs(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != proxyBody {
		t.Fatalf("body = %q, want proxy body forwarded", w.Body.String())
	}
}

// =============================================================================
// handleGetViewableAddresses — upstream non-200 -> 500; happy path shape
// =============================================================================

func TestHandleGetViewableAddresses_UpstreamError_MapsTo500(t *testing.T) {
	// Contract: any upstream non-200 makes GetViewableAddressesWithIdentity
	// return an error -> handler 500 (privacy_handlers.go:39-43).
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden) // non-200
	}, &tcStubProvider{})

	req := tcWithViewer(httptest.NewRequest(http.MethodGet, "/api/privacy/viewable-addresses", nil), "did:example:viewer")
	w := httptest.NewRecorder()
	s.handleGetViewableAddresses(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("upstream 403 -> handler %d, want 500", w.Code)
	}
}

func TestHandleGetViewableAddresses_Unauthenticated(t *testing.T) {
	// Contract: no viewer DID -> 400 (this handler uses 400, not 401).
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("proxy must not be called when unauthenticated")
	}, &tcStubProvider{})

	req := httptest.NewRequest(http.MethodGet, "/api/privacy/viewable-addresses", nil)
	w := httptest.NewRecorder()
	s.handleGetViewableAddresses(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("no-cookie -> %d, want 400", w.Code)
	}
}

func TestHandleGetViewableAddresses_HappyPath(t *testing.T) {
	// Contract: a 200 from the proxy is decoded and re-emitted; the viewer DID
	// is forwarded as a Bearer token.
	resp := privacy.ViewableAddressesResponse{
		ViewerDID:    "did:example:viewer",
		OwnAddresses: []privacy.OwnAddress{{Address: "0xaaa"}},
		DisclosedAddresses: []privacy.DisclosedAddress{
			{Address: "0xbbb", DisclosureLevel: "full", GrantID: "g1"},
		},
	}
	var gotAuth string
	s := tcServerWithProxy(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if !strings.Contains(r.URL.Path, "/viewable-addresses") {
			t.Errorf("unexpected proxy path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}, &tcStubProvider{})

	req := tcWithViewer(httptest.NewRequest(http.MethodGet, "/api/privacy/viewable-addresses", nil), "did:example:viewer")
	w := httptest.NewRecorder()
	s.handleGetViewableAddresses(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var got privacy.ViewableAddressesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.OwnAddresses) != 1 || got.OwnAddresses[0].Address != "0xaaa" {
		t.Errorf("own_addresses = %+v, want one entry 0xaaa", got.OwnAddresses)
	}
	if len(got.DisclosedAddresses) != 1 || got.DisclosedAddresses[0].GrantID != "g1" {
		t.Errorf("disclosed_addresses = %+v, want one entry grant g1", got.DisclosedAddresses)
	}
	if gotAuth != "Bearer "+tcViewerJWT("did:example:viewer") {
		// The token forwarded is the raw cookie value (GetAuthToken), which is
		// exactly the JWT we minted.
		if !strings.HasPrefix(gotAuth, "Bearer ") {
			t.Errorf("Authorization = %q, want a forwarded Bearer token", gotAuth)
		}
	}
}

// =============================================================================
// viewer identity extraction
// =============================================================================

func TestGetViewerIdentity(t *testing.T) {
	s := &Server{}

	// With a valid unsigned JWT cookie -> DID + raw token.
	const did = "did:example:abc123"
	req := tcWithViewer(httptest.NewRequest(http.MethodGet, "/x", nil), did)
	v := s.getViewerIdentity(req)
	if v.DID != did {
		t.Errorf("DID = %q, want %q (ExtractClaims does not verify signature)", v.DID, did)
	}
	if v.JWTToken == "" {
		t.Errorf("JWTToken empty, want the raw cookie value forwarded to the proxy")
	}

	// No cookie -> empty identity.
	v = s.getViewerIdentity(httptest.NewRequest(http.MethodGet, "/x", nil))
	if v.DID != "" || v.JWTToken != "" {
		t.Errorf("no-cookie identity = %+v, want empty", v)
	}

	// Expired token -> empty DID (GetAuthDID checks IsExpired).
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	expClaims := fmt.Sprintf(`{"sub":%q,"exp":%d}`, did, time.Now().Add(-time.Hour).Unix())
	expired := header + "." + base64.RawURLEncoding.EncodeToString([]byte(expClaims)) + ".sig"
	reqExp := httptest.NewRequest(http.MethodGet, "/x", nil)
	reqExp.AddCookie(&http.Cookie{Name: AuthCookieName, Value: expired})
	v = s.getViewerIdentity(reqExp)
	if v.DID != "" {
		t.Errorf("expired-token DID = %q, want empty", v.DID)
	}
}
