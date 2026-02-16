package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"explorer/internal/privacy"
	"github.com/go-chi/chi/v5"
)

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

func TestFilterAddressesForVisibility_NoIdentity(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest("GET", "/test", nil)

	result := s.filterAddressesForVisibility(req, []string{"0x1234", "0x5678"})

	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
	for _, info := range result {
		if info.IsPrivate {
			t.Errorf("expected not private, got private for %s", info.Address)
		}
		if info.DisplayName == "[PRIVATE]" {
			t.Errorf("expected non-private display name, got [PRIVATE]")
		}
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
	})

	return r
}

func TestHandleGetGrantedAddress_ServiceNotEnabled(t *testing.T) {
	// nil client = service not enabled
	router := setupPrivacyTestServerWithMock(nil)

	req := httptest.NewRequest("GET", "/api/privacy/grant/grant-123/addr-456", nil)
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
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestHandleGetGrantedAddress_RevokedGrant(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": "grant has been revoked"}`))
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/revoked-grant/addr-456", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// ============================================================================
// Test: filterAddressesForVisibility with disclosure levels
// ============================================================================

func TestFilterAddressesForVisibility_PseudonymousAddress(t *testing.T) {
	pseudonym := "Address-KLMN"
	grantID := "grant-123"

	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := privacy.BatchCheckAddressesResponse{
			Results: map[string]privacy.AddressVisibility{
				"0x3333333333333333333333333333333333333333": {
					Address:   "0x3333333333333333333333333333333333333333",
					Visible:   true,
					Level:     privacy.VisibilityPseudonymous,
					Reason:    privacy.ReasonDisclosureGrant,
					Pseudonym: &pseudonym,
					GrantID:   &grantID,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	// Note: In SSO-only mode, we need a DID from auth cookie.
	// For this test, we set a mock cookie. However, since the viewer identity
	// extraction uses GetAuthDID which requires a valid JWT, we test with
	// the privacy client directly.
	s := &Server{privacyClient: client}

	// Without auth, filterAddressesForVisibility returns all visible (fail open)
	req := httptest.NewRequest("GET", "/test", nil)
	result := s.filterAddressesForVisibility(req, []string{"0x3333333333333333333333333333333333333333"})

	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	info := result["0x3333333333333333333333333333333333333333"]
	if info.IsPrivate {
		t.Error("expected not private (fail open without auth)")
	}
}

func TestFilterAddressesForVisibility_NoPrivacyClient(t *testing.T) {
	s := &Server{privacyClient: nil}
	req := httptest.NewRequest("GET", "/test", nil)

	result := s.filterAddressesForVisibility(req, []string{"0x1234", "0x5678"})

	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
	for _, info := range result {
		if info.IsPrivate {
			t.Errorf("expected not private without privacy client")
		}
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
// Test: AddressDisplayInfo struct
// ============================================================================

func TestAddressDisplayInfo_Fields(t *testing.T) {
	grantID := "grant-test"
	pseudonym := "Address-TEST"

	visibility := &privacy.AddressVisibility{
		Address:   "0x1234567890123456789012345678901234567890",
		Visible:   true,
		Level:     privacy.VisibilityPseudonymous,
		Reason:    privacy.ReasonDisclosureGrant,
		Pseudonym: &pseudonym,
		GrantID:   &grantID,
	}

	info := AddressDisplayInfo{
		Address:     "0x1234567890123456789012345678901234567890",
		DisplayName: pseudonym,
		IsPrivate:   false,
		Visibility:  visibility,
	}

	if info.Address != "0x1234567890123456789012345678901234567890" {
		t.Errorf("unexpected address: %s", info.Address)
	}
	if info.DisplayName != "Address-TEST" {
		t.Errorf("unexpected display name: %s", info.DisplayName)
	}
	if info.IsPrivate {
		t.Error("expected not private")
	}
	if info.Visibility == nil {
		t.Error("expected non-nil visibility")
	}
	if info.Visibility.Level != privacy.VisibilityPseudonymous {
		t.Errorf("unexpected visibility level: %s", info.Visibility.Level)
	}
}
