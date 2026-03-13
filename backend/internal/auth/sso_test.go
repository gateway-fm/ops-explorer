package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestGenerateAndValidateState(t *testing.T) {
	c := &SSOClient{
		stateStore: make(map[string]stateEntry),
	}

	state, err := c.GenerateState("/dashboard")
	if err != nil {
		t.Fatalf("GenerateState failed: %v", err)
	}
	if state == "" {
		t.Fatal("expected non-empty state")
	}

	returnURL, valid := c.ValidateState(state)
	if !valid {
		t.Fatal("expected state to be valid")
	}
	if returnURL != "/dashboard" {
		t.Fatalf("expected returnURL /dashboard, got %s", returnURL)
	}

	// Second use should fail (single-use)
	_, valid = c.ValidateState(state)
	if valid {
		t.Fatal("expected state to be invalid on second use")
	}
}

func TestValidateState_Invalid(t *testing.T) {
	c := &SSOClient{
		stateStore: make(map[string]stateEntry),
	}

	_, valid := c.ValidateState("nonexistent")
	if valid {
		t.Fatal("expected invalid state")
	}
}

func TestGetAuthorizationURL(t *testing.T) {
	c := &SSOClient{
		privacyProxyURL:       "https://proxy.example.com",
		privacyProxyPublicURL: "https://proxy.example.com",
		clientID:              "test-client",
		redirectURI:           "https://explorer.example.com/api/auth/callback",
	}

	authURL := c.GetAuthorizationURL("test-state")

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}

	if parsed.Host != "proxy.example.com" {
		t.Fatalf("expected host proxy.example.com, got %s", parsed.Host)
	}
	if parsed.Path != "/oauth/authorize" {
		t.Fatalf("expected path /oauth/authorize, got %s", parsed.Path)
	}

	q := parsed.Query()
	if q.Get("client_id") != "test-client" {
		t.Fatalf("expected client_id test-client, got %s", q.Get("client_id"))
	}
	if q.Get("response_mode") != "redirect" {
		t.Fatalf("expected response_mode redirect, got %s", q.Get("response_mode"))
	}
	if q.Get("response_type") != "code" {
		t.Fatalf("expected response_type code, got %s", q.Get("response_type"))
	}
	if q.Get("state") != "test-state" {
		t.Fatalf("expected state test-state, got %s", q.Get("state"))
	}
	if q.Get("redirect_uri") != "https://explorer.example.com/api/auth/callback" {
		t.Fatalf("expected correct redirect_uri, got %s", q.Get("redirect_uri"))
	}
}

func TestGenerateState_CapacityLimit(t *testing.T) {
	c := &SSOClient{
		stateStore: make(map[string]stateEntry),
	}

	// Fill to capacity
	for i := 0; i < MaxStateEntries; i++ {
		_, err := c.GenerateState("/")
		if err != nil {
			t.Fatalf("GenerateState failed at entry %d: %v", i, err)
		}
	}

	// Next one should fail
	_, err := c.GenerateState("/")
	if err == nil {
		t.Fatal("expected error when state store is at capacity")
	}

	// Consuming a state should free a slot
	var anyState string
	for s := range c.stateStore {
		anyState = s
		break
	}
	c.ValidateState(anyState)

	_, err = c.GenerateState("/")
	if err != nil {
		t.Fatalf("expected success after freeing a slot, got: %v", err)
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		client   *SSOClient
		expected bool
	}{
		{"nil client", nil, false},
		{"empty URL", &SSOClient{privacyProxyURL: ""}, false},
		{"valid", &SSOClient{privacyProxyURL: "https://proxy.example.com"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.client.IsEnabled()
			if got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestRefreshTokens_Success(t *testing.T) {
	wantRefresh := "old-refresh-token"
	respPayload := RefreshResponse{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    300,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/refresh" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		var reqBody map[string]string
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}
		if reqBody["refresh_token"] != wantRefresh {
			t.Errorf("expected refresh_token %q, got %q", wantRefresh, reqBody["refresh_token"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respPayload)
	}))
	defer srv.Close()

	c := NewSSOClient(srv.URL, srv.URL, "client-id", "http://redirect")
	got, err := c.RefreshTokens(context.Background(), wantRefresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AccessToken != respPayload.AccessToken {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, respPayload.AccessToken)
	}
	if got.RefreshToken != respPayload.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, respPayload.RefreshToken)
	}
	if got.TokenType != respPayload.TokenType {
		t.Errorf("TokenType = %q, want %q", got.TokenType, respPayload.TokenType)
	}
	if got.ExpiresIn != respPayload.ExpiresIn {
		t.Errorf("ExpiresIn = %d, want %d", got.ExpiresIn, respPayload.ExpiresIn)
	}
}

func TestRefreshTokens_Revoked(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"401 Unauthorized", http.StatusUnauthorized},
		{"403 Forbidden", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"error":"token revoked"}`))
			}))
			defer srv.Close()

			c := NewSSOClient(srv.URL, srv.URL, "client-id", "http://redirect")
			_, err := c.RefreshTokens(context.Background(), "revoked-token")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, ErrRefreshRevoked) {
				t.Fatalf("expected ErrRefreshRevoked, got: %v", err)
			}
		})
	}
}

func TestRefreshTokens_ServerError_500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	c := NewSSOClient(srv.URL, srv.URL, "client-id", "http://redirect")
	_, err := c.RefreshTokens(context.Background(), "some-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrRefreshRevoked) {
		t.Fatal("expected generic error, not ErrRefreshRevoked")
	}
}

func TestRefreshTokens_NetworkError(t *testing.T) {
	// Point at a server that is immediately closed so the connection fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	c := NewSSOClient(srv.URL, srv.URL, "client-id", "http://redirect")
	_, err := c.RefreshTokens(context.Background(), "some-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrRefreshRevoked) {
		t.Fatal("expected network error, not ErrRefreshRevoked")
	}
}
