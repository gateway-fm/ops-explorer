package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"explorer/internal/privacy"
	"github.com/go-chi/chi/v5"
)

// mockAuthCookie creates a fake JWT cookie for testing.
// ExtractClaims does not verify signatures, so a well-formed JWT is sufficient.
func mockAuthCookie() *http.Cookie {
	claims := map[string]any{
		"sub": "did:test:12345",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	payload, _ := json.Marshal(claims)
	token := "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	return &http.Cookie{
		Name:  AuthCookieName,
		Value: token,
	}
}

func setupPrivacyTestServer(privacyClient *privacy.Client) *chi.Mux {
	s := &Server{
		privacyClient: privacyClient,
	}

	r := chi.NewRouter()
	r.Route("/api/privacy", func(r chi.Router) {
		r.Get("/viewable-addresses", s.handleGetViewableAddresses)
		r.Get("/check-address/{address}", s.handleCheckAddressVisibility)
		r.Post("/check-addresses", s.handleBatchCheckAddresses)
	})

	return r
}

func TestHandleGetViewableAddresses_NoAuth(t *testing.T) {
	router := setupPrivacyTestServer(nil)

	req := httptest.NewRequest("GET", "/api/privacy/viewable-addresses", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("authentication required")) {
		t.Errorf("expected error about authentication, got %s", w.Body.String())
	}
}

func TestHandleCheckAddressVisibility_NoAuth(t *testing.T) {
	router := setupPrivacyTestServer(nil)

	req := httptest.NewRequest("GET", "/api/privacy/check-address/0xabcd", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("authentication required")) {
		t.Errorf("expected error about authentication, got %s", w.Body.String())
	}
}

func TestHandleBatchCheckAddresses_NoAuth(t *testing.T) {
	router := setupPrivacyTestServer(nil)

	body := bytes.NewBufferString(`{"addresses":["0x1234"]}`)
	req := httptest.NewRequest("POST", "/api/privacy/check-addresses", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("authentication required")) {
		t.Errorf("expected error about authentication, got %s", w.Body.String())
	}
}

func TestHandleBatchCheckAddresses_InvalidBody(t *testing.T) {
	router := setupPrivacyTestServer(nil)

	// Set a valid auth cookie to pass the auth check
	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("POST", "/api/privacy/check-addresses", body)
	req.Header.Set("Content-Type", "application/json")
	// Note: Without auth cookie, will fail with auth error first
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Without auth, returns 400 for auth, not for invalid body
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleBatchCheckAddresses_EmptyAddresses(t *testing.T) {
	router := setupPrivacyTestServer(nil)

	body := bytes.NewBufferString(`{"addresses":[]}`)
	req := httptest.NewRequest("POST", "/api/privacy/check-addresses", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Without auth, returns auth error
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleBatchCheckAddresses_TooManyAddresses(t *testing.T) {
	router := setupPrivacyTestServer(nil)

	addresses := make([]string, 101)
	for i := range addresses {
		addresses[i] = "0x1234"
	}
	reqBody := struct {
		Addresses []string `json:"addresses"`
	}{Addresses: addresses}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/privacy/check-addresses", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Without auth, returns auth error first
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}


// ============================================================================
// Test: handleGetGrantedAddress
// ============================================================================

// mockPrivacyServer creates a mock privacy proxy server for testing
func mockPrivacyServer(t *testing.T, handler http.HandlerFunc) *privacy.Client {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return privacy.NewClient(server.URL)
}

func setupPrivacyTestServerWithMock(privacyClient *privacy.Client) *chi.Mux {
	s := &Server{
		privacyClient: privacyClient,
	}

	r := chi.NewRouter()
	r.Route("/api/privacy", func(r chi.Router) {
		r.Get("/viewable-addresses", s.handleGetViewableAddresses)
		r.Get("/check-address/{address}", s.handleCheckAddressVisibility)
		r.Post("/check-addresses", s.handleBatchCheckAddresses)
		r.Get("/grant/{grantId}/{addressId}", s.handleGetGrantedAddress)
		r.Get("/grant/{grantId}/{addressId}/transactions", s.handleGetGrantedAddressTransactions)
	})

	return r
}

func TestHandleGetGrantedAddress_ServiceNotEnabled(t *testing.T) {
	// nil client = service not enabled
	router := setupPrivacyTestServerWithMock(nil)

	req := httptest.NewRequest("GET", "/api/privacy/grant/grant-123/addr-456", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("privacy service not enabled")) {
		t.Errorf("expected error about privacy service, got %s", w.Body.String())
	}
}

func TestHandleGetGrantedAddress_MissingParams(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Should not be called
		t.Error("privacy proxy should not be called")
	})
	router := setupPrivacyTestServerWithMock(client)

	// Missing grantId - route matches but handler returns 400 for empty params
	req := httptest.NewRequest("GET", "/api/privacy/grant//addr-456", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleGetGrantedAddress_NotFound(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "grant not found"}`))
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/invalid-grant/addr-456", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("grant or address not found")) {
		t.Errorf("expected not found error, got %s", w.Body.String())
	}
}

func TestHandleGetGrantedAddress_ExpiredGrant(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": "grant has expired"}`))
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/expired-grant/addr-456", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
	body := w.Body.String()
	if !bytes.Contains(w.Body.Bytes(), []byte("grant or address not found")) {
		t.Errorf("expected opaque not found message, got %s", body)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("expired")) {
		t.Errorf("response must not leak expiry details, got %s", body)
	}
}

func TestHandleGetGrantedAddress_RevokedGrant(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": "grant has been revoked"}`))
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/revoked-grant/addr-456", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
	body := w.Body.String()
	if !bytes.Contains(w.Body.Bytes(), []byte("grant or address not found")) {
		t.Errorf("expected opaque not found message, got %s", body)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("revoked")) {
		t.Errorf("response must not leak revocation details, got %s", body)
	}
}


// ============================================================================
// Test: checkAddressVisibility helper
// ============================================================================

func TestCheckAddressVisibility_NoPrivacyClient(t *testing.T) {
	s := &Server{privacyClient: nil}
	req := httptest.NewRequest("GET", "/test", nil)

	result := s.checkAddressVisibility(req, "0x2222222222222222222222222222222222222222")
	if result != nil {
		t.Error("expected nil when privacy client is not enabled")
	}
}

func TestCheckAddressVisibility_NoIdentity(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Should not be called when no identity
		t.Error("Should not call privacy proxy without identity")
	})

	s := &Server{privacyClient: client}
	// No auth cookie
	req := httptest.NewRequest("GET", "/test", nil)

	result := s.checkAddressVisibility(req, "0x2222222222222222222222222222222222222222")
	if result != nil {
		t.Error("expected nil without identity")
	}
}

// ============================================================================
// Test: handleGetGrantedAddressTransactions opaque error handling
// ============================================================================

func TestHandleGetGrantedAddressTransactions_NotFound(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/missing-grant/addr-456/transactions", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("grant or address not found")) {
		t.Errorf("expected not found message, got %s", w.Body.String())
	}
}

func TestHandleGetGrantedAddressTransactions_ExpiredGrant(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": "grant has expired"}`))
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/expired-grant/addr-456/transactions", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d (opaque denial), got %d", http.StatusNotFound, w.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("expired")) {
		t.Errorf("response must not leak expiry details, got %s", w.Body.String())
	}
}

func TestHandleGetGrantedAddressTransactions_RevokedGrant(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": "grant has been revoked"}`))
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/revoked-grant/addr-456/transactions", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d (opaque denial), got %d", http.StatusNotFound, w.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("revoked")) {
		t.Errorf("response must not leak revocation details, got %s", w.Body.String())
	}
}

func TestHandleGetGrantedAddressTransactions_NoAuth(t *testing.T) {
	router := setupPrivacyTestServerWithMock(nil)

	req := httptest.NewRequest("GET", "/api/privacy/grant/grant-123/addr-456/transactions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestHandleGetGrantedAddressTransactions_NoErrorLeakage(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`secret internal reason that should not leak`))
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/bad-grant/addr-456/transactions", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body := w.Body.String()
	if bytes.Contains([]byte(body), []byte("secret")) {
		t.Errorf("internal error details leaked to client: %s", body)
	}
	if bytes.Contains([]byte(body), []byte("internal reason")) {
		t.Errorf("internal error details leaked to client: %s", body)
	}
}

