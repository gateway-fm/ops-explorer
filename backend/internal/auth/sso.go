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
	privacyProxyURL       string // backend-to-backend (Docker internal)
	privacyProxyPublicURL string // browser-facing (for OAuth redirects)
	clientID              string
	clientSecret          string // RD-1006: presented at /oauth/token via HTTP Basic; empty in dev/local until operators set it
	redirectURI           string
	httpClient            *http.Client

	// CSRF protection
	stateMu    sync.RWMutex
	stateStore map[string]stateEntry
}

type stateEntry struct {
	returnURL string
	expiresAt time.Time
}

type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// RefreshResponse is the response from POST /api/v1/refresh.
// The server rotates the refresh token on every call, so both tokens must be
// stored — the old refresh token is immediately revoked.
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
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
	MaxStateEntries      = 1000
)

func NewSSOClient(privacyProxyURL, privacyProxyPublicURL, clientID, clientSecret, redirectURI string) *SSOClient {
	if privacyProxyPublicURL == "" {
		privacyProxyPublicURL = privacyProxyURL
	}
	c := &SSOClient{
		privacyProxyURL:       strings.TrimSuffix(privacyProxyURL, "/"),
		privacyProxyPublicURL: strings.TrimSuffix(privacyProxyPublicURL, "/"),
		clientID:              clientID,
		clientSecret:          clientSecret,
		redirectURI:           redirectURI,
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
	if len(c.stateStore) >= MaxStateEntries {
		c.stateMu.Unlock()
		return "", fmt.Errorf("state store at capacity (%d entries)", MaxStateEntries)
	}
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
	params.Set("response_mode", "redirect")
	params.Set("state", state)

	return fmt.Sprintf("%s/oauth/authorize?%s", c.privacyProxyPublicURL, params.Encode())
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
	// RD-1006: authenticate to the proxy at the token endpoint via RFC 6749
	// client_secret_basic. Omitted in dev when the secret is unset — the
	// proxy only enforces the gate when the client is on its first-party
	// allowlist, so unconfigured local stacks keep working unchanged.
	if c.clientSecret != "" {
		req.SetBasicAuth(c.clientID, c.clientSecret)
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

// RefreshTokens exchanges a refresh token for a new access + refresh token pair.
// Privacy-proxy rotates the refresh token on every call, so callers must
// persist the new refresh token and discard the old one immediately.
//
// Returns ErrRefreshRevoked if the token has been revoked (e.g. the user was
// banned). The caller should clear all auth cookies and treat the user as
// logged out.
func (c *SSOClient) RefreshTokens(ctx context.Context, refreshToken string) (*RefreshResponse, error) {
	refreshURL := fmt.Sprintf("%s/api/v1/refresh", c.privacyProxyURL)

	body, err := json.Marshal(map[string]string{"refresh_token": refreshToken})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal refresh request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call refresh endpoint: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refresh response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrRefreshRevoked
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh endpoint returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result RefreshResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	return &result, nil
}

// ErrRefreshRevoked is returned by RefreshTokens when the server explicitly
// rejects the token (revoked, banned user, or expired). The caller should
// clear all auth state and redirect to login.
var ErrRefreshRevoked = fmt.Errorf("refresh token revoked or invalid")

// RevokeToken tells privacy-proxy to revoke a refresh token server-side.
// Should be called on explicit logout so the token cannot be reused even if
// captured from network traffic or logs before the cookie is cleared.
// Errors are non-fatal: local cookies are cleared regardless.
func (c *SSOClient) RevokeToken(ctx context.Context, refreshToken string) error {
	revokeURL := fmt.Sprintf("%s/api/v1/revoke", c.privacyProxyURL)

	body, err := json.Marshal(map[string]string{"refresh_token": refreshToken})
	if err != nil {
		return fmt.Errorf("failed to marshal revoke request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, revokeURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create revoke request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call revoke endpoint: %w", err)
	}
	defer resp.Body.Close()

	// RFC 7009: revocation endpoint returns 200 even for unknown tokens.
	// Any non-200 is a server error but we don't fail the logout over it.
	return nil
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
