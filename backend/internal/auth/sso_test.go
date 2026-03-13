package auth

import (
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
