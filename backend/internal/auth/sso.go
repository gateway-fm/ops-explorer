package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type SSOClient struct {
	privacyProxyURL string
	clientID        string
	redirectURI     string
	httpClient      *http.Client

	// CSRF protection
	stateMu    sync.RWMutex
	stateStore map[string]stateEntry
}

type stateEntry struct {
	returnURL string
	expiresAt time.Time
}

type OAuthAuthorizeResponse struct {
	OAuthSessionID string      `json:"oauth_session_id"`
	AuthSessionID  string      `json:"auth_session_id"`
	AuthRequest    interface{} `json:"auth_request"`
}

type OAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type OAuthSessionStatusResponse struct {
	Completed   bool   `json:"completed"`
	RedirectURL string `json:"redirect_url,omitempty"`
}

type OAuthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

type AuthStatus struct {
	Authenticated bool   `json:"authenticated"`
	DID           string `json:"did,omitempty"`
	ExpiresAt     int64  `json:"expires_at,omitempty"`
}

const (
	StateExpiration      = 10 * time.Minute
	StateCleanupInterval = 5 * time.Minute
)

func NewSSOClient(privacyProxyURL, clientID, redirectURI string) *SSOClient {
	c := &SSOClient{
		privacyProxyURL: strings.TrimSuffix(privacyProxyURL, "/"),
		clientID:        clientID,
		redirectURI:     redirectURI,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		stateStore: make(map[string]stateEntry),
	}

	go c.cleanupStates()

	return c
}

func (c *SSOClient) IsEnabled() bool {
	return c != nil && c.privacyProxyURL != ""
}

func (c *SSOClient) GenerateState(returnURL string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}
	state := base64.URLEncoding.EncodeToString(b)

	c.stateMu.Lock()
	c.stateStore[state] = stateEntry{
		returnURL: returnURL,
		expiresAt: time.Now().Add(StateExpiration),
	}
	c.stateMu.Unlock()

	return state, nil
}

func (c *SSOClient) ValidateState(state string) (string, bool) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	entry, exists := c.stateStore[state]
	if !exists {
		return "", false
	}

	// Single-use: delete after retrieval
	delete(c.stateStore, state)

	if time.Now().After(entry.expiresAt) {
		return "", false
	}

	return entry.returnURL, true
}

func (c *SSOClient) GetAuthorizationURL(state string) string {
	params := url.Values{}
	params.Set("client_id", c.clientID)
	params.Set("redirect_uri", c.redirectURI)
	params.Set("response_type", "code")
	params.Set("state", state)

	return fmt.Sprintf("%s/oauth/authorize?%s", c.privacyProxyURL, params.Encode())
}

// InitiateAuthorization calls the authorization endpoint.
// Used for browser-based flows where we need the auth request for QR code display.
func (c *SSOClient) InitiateAuthorization(ctx context.Context, state string) (*OAuthAuthorizeResponse, error) {
	authURL := c.GetAuthorizationURL(state)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var oauthErr OAuthErrorResponse
		if err := json.Unmarshal(body, &oauthErr); err == nil && oauthErr.Error != "" {
			return nil, fmt.Errorf("OAuth error: %s - %s", oauthErr.Error, oauthErr.ErrorDescription)
		}
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result OAuthAuthorizeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *SSOClient) CheckSessionStatus(ctx context.Context, sessionID string) (*OAuthSessionStatusResponse, error) {
	statusURL := fmt.Sprintf("%s/oauth/session/%s/status", c.privacyProxyURL, sessionID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("session not found or expired")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result OAuthSessionStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *SSOClient) ExchangeCode(ctx context.Context, code string) (*OAuthTokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/oauth/token", c.privacyProxyURL)

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", c.redirectURI)
	data.Set("client_id", c.clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var oauthErr OAuthErrorResponse
		if err := json.Unmarshal(body, &oauthErr); err == nil && oauthErr.Error != "" {
			return nil, fmt.Errorf("token exchange failed: %s - %s", oauthErr.Error, oauthErr.ErrorDescription)
		}
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result OAuthTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *SSOClient) cleanupStates() {
	ticker := time.NewTicker(StateCleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.stateMu.Lock()
		now := time.Now()
		for state, entry := range c.stateStore {
			if now.After(entry.expiresAt) {
				delete(c.stateStore, state)
			}
		}
		c.stateMu.Unlock()
	}
}
