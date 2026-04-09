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
		r.Get("/shared-logs", s.handleGetSharedLogs)
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
// Test: handleGetGrantedAddressTransactions — proxy forwarding
// The handler now proxies directly to the privacy proxy. The proxy handles
// all auth, grant validation, and pseudonymization. The explorer forwards
// the proxy's response status and body as-is.
// ============================================================================

func TestHandleGetGrantedAddressTransactions_NoAuth(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("proxy should not be called without auth")
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/grant-123/addr-456/transactions", nil)
	// No auth cookie — should be rejected
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestHandleGetGrantedAddressTransactions_NotFound(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"grant or address not found"}`))
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/missing-grant/addr-456/transactions", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestHandleGetGrantedAddressTransactions_ExpiredGrant(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"grant has expired"}`))
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/expired-grant/addr-456/transactions", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Proxy returns 403 for expired grants — explorer forwards as-is
	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d (forwarded from proxy), got %d", http.StatusForbidden, w.Code)
	}
}

func TestHandleGetGrantedAddressTransactions_RevokedGrant(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"grant has been revoked"}`))
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/revoked-grant/addr-456/transactions", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Proxy returns 403 for revoked grants — explorer forwards as-is
	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d (forwarded from proxy), got %d", http.StatusForbidden, w.Code)
	}
}

func TestHandleGetGrantedAddressTransactions_ProxiesToPrivacyProxy(t *testing.T) {
	// The handler now proxies directly to the privacy proxy's grant transactions
	// endpoint. The proxy handles all auth, grant validation, and pseudonymization.
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify the request goes to the correct proxy endpoint
		if r.URL.Path != "/api/v1/explorer/grant/grant-123/addr-456/transactions" {
			t.Errorf("unexpected proxy path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"transactions":    []any{},
			"disclosure_level": "pseudonymous",
			"address_labels":  map[string]string{},
			"has_more":        false,
		})
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/grant-123/addr-456/transactions", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandleGetGrantedAddressTransactions_ProxyErrorForwarded(t *testing.T) {
	// When the proxy returns 403 (revoked/expired grant), the explorer forwards it
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"grant has been revoked"}`))
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/grant/bad-grant/addr-456/transactions", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// ============================================================================
// Test: handleGetSharedLogs
// ============================================================================

func TestHandleGetSharedLogs_NoAuth(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("proxy should not be called without auth")
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/shared-logs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("authentication required")) {
		t.Errorf("expected authentication error, got %s", w.Body.String())
	}
}

func TestHandleGetSharedLogs_PrivacyNotEnabled(t *testing.T) {
	// nil client = privacy not enabled
	router := setupPrivacyTestServerWithMock(nil)

	req := httptest.NewRequest("GET", "/api/privacy/shared-logs", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp struct {
		Logs   []any `json:"logs"`
		Total  int   `json:"total"`
		Limit  int   `json:"limit"`
		Offset int   `json:"offset"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Logs) != 0 {
		t.Errorf("expected empty logs, got %d", len(resp.Logs))
	}
	if resp.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Total)
	}
}

func TestHandleGetSharedLogs_ProxiesToPrivacyProxy(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify the request goes to the correct proxy endpoint
		if r.URL.Path != "/api/v1/explorer/shared-logs" {
			t.Errorf("unexpected proxy path: %s", r.URL.Path)
		}
		// Verify limit/offset are forwarded
		if r.URL.Query().Get("limit") != "25" {
			t.Errorf("expected limit=25, got %s", r.URL.Query().Get("limit"))
		}
		if r.URL.Query().Get("offset") != "0" {
			t.Errorf("expected offset=0, got %s", r.URL.Query().Get("offset"))
		}
		// Verify auth header is forwarded
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			t.Error("expected Authorization header to be forwarded")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"shared_logs": []map[string]any{
				{
					"tx_hash":      "0xabc123",
					"block_number": 42,
					"log_index":    0,
					"address":      "0xcontract1",
					"topics":       []string{"0xtopic0"},
					"data":         "0xdata",
					"sender_did":   "did:test:sender",
					"created_at":   "2026-04-03T00:00:00Z",
				},
			},
			"total":  1,
			"limit":  25,
			"offset": 0,
		})
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/shared-logs", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp struct {
		Logs []struct {
			TxHash    string `json:"tx_hash"`
			SenderDID string `json:"sender_did"`
		} `json:"shared_logs"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(resp.Logs))
	}
	if resp.Logs[0].TxHash != "0xabc123" {
		t.Errorf("expected tx_hash 0xabc123, got %s", resp.Logs[0].TxHash)
	}
	if resp.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Total)
	}
}

func TestHandleGetSharedLogs_WithPagination(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected limit=10, got %s", r.URL.Query().Get("limit"))
		}
		if r.URL.Query().Get("offset") != "20" {
			t.Errorf("expected offset=20, got %s", r.URL.Query().Get("offset"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"logs":   []any{},
			"total":  30,
			"limit":  10,
			"offset": 20,
		})
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/shared-logs?limit=10&offset=20", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHandleGetSharedLogs_ProxyError(t *testing.T) {
	client := mockPrivacyServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	})
	router := setupPrivacyTestServerWithMock(client)

	req := httptest.NewRequest("GET", "/api/privacy/shared-logs", nil)
	req.AddCookie(mockAuthCookie())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("failed to get shared logs")) {
		t.Errorf("expected error message, got %s", w.Body.String())
	}
}

