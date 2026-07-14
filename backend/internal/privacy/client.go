package privacy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrNotFound = errors.New("privacy: grant or address not found")

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func NewClientWithHTTP(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

type OwnAddress struct {
	Address string  `json:"address"`
	ENSName *string `json:"ens_name,omitempty"`
}

type DisclosedAddress struct {
	Address         string     `json:"address"`
	AddressID       string     `json:"address_id"`
	OwnerDID        string     `json:"owner_did"`
	DisclosureLevel string     `json:"disclosure_level"`
	GrantID         string     `json:"grant_id"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	ENSName         *string    `json:"ens_name,omitempty"`
}

type ViewableAddressesResponse struct {
	ViewerWallet       string             `json:"viewer_wallet"`
	ViewerDID          string             `json:"viewer_did,omitempty"`
	OwnAddresses       []OwnAddress       `json:"own_addresses"`
	DisclosedAddresses []DisclosedAddress `json:"disclosed_addresses"`
}

type VisibilityLevel string

const (
	VisibilityFull         VisibilityLevel = "full"
	VisibilityPseudonymous VisibilityLevel = "pseudonymous"
	VisibilityRedacted     VisibilityLevel = "redacted"
	VisibilityHidden       VisibilityLevel = "hidden"
)

type VisibilityReason string

const (
	ReasonOwnAddress       VisibilityReason = "own_address"
	ReasonDisclosureGrant  VisibilityReason = "disclosure_grant"
	ReasonPublicAddress    VisibilityReason = "public_address"
	ReasonNoAccess         VisibilityReason = "no_access"
	ReasonRBACGroupMember  VisibilityReason = "rbac_group_member"
)

type AddressVisibility struct {
	Address   string           `json:"address"`
	Visible   bool             `json:"visible"`
	Level     VisibilityLevel  `json:"level"`
	Reason    VisibilityReason `json:"reason"`
	Pseudonym *string          `json:"pseudonym,omitempty"`
	GrantID   *string          `json:"grant_id,omitempty"`
	ExpiresAt *time.Time       `json:"expires_at,omitempty"`
}

type ViewerIdentity struct {
	Wallet   string // Wallet address (used for DB-verified DID lookup)
	DID      string // DID extracted locally (for display only, NOT sent to proxy)
	JWTToken string // Raw JWT token forwarded to privacy-proxy for authenticated requests
}

func (c *Client) GetViewableAddresses(ctx context.Context, wallet string) (*ViewableAddressesResponse, error) {
	return c.GetViewableAddressesWithIdentity(ctx, ViewerIdentity{Wallet: wallet})
}

func (c *Client) GetViewableAddressesWithIdentity(ctx context.Context, viewer ViewerIdentity) (*ViewableAddressesResponse, error) {
	u, err := url.Parse(c.baseURL + "/api/v1/explorer/viewable-addresses")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	if viewer.Wallet != "" {
		q.Set("wallet", viewer.Wallet)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if viewer.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+viewer.JWTToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result ViewableAddressesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (v *AddressVisibility) IsVisible() bool {
	return v.Visible
}

func (v *AddressVisibility) IsOwnAddress() bool {
	return v.Reason == ReasonOwnAddress
}

func (v *AddressVisibility) IsDisclosedAddress() bool {
	return v.Reason == ReasonDisclosureGrant
}

func (v *AddressVisibility) IsPublicAddress() bool {
	return v.Reason == ReasonPublicAddress
}

func (c *Client) IsEnabled() bool {
	return c != nil && c.baseURL != ""
}

// BaseURL returns the configured privacy-proxy base URL (with no trailing slash).
// Used by the "View as user" (RD-928) flow to build impersonation URLs without
// needing a second copy of the URL in the call site.
func (c *Client) BaseURL() string {
	if c == nil {
		return ""
	}
	return strings.TrimSuffix(c.baseURL, "/")
}

// ETH address linking types

type LinkedAddress struct {
	Address    string  `json:"address"`
	VerifiedAt string  `json:"verified_at"`
	LinkType   string  `json:"link_type"`
	ENSName    *string `json:"ens_name,omitempty"`
}

type LinkedAddressesResponse struct {
	Addresses []LinkedAddress `json:"addresses"`
}

type LinkChallengeResponse struct {
	Nonce   string `json:"nonce"`
	Message string `json:"message"`
}

type VerifyLinkRequest struct {
	Nonce     string `json:"nonce"`
	Address   string `json:"address"`
	Signature string `json:"signature"`
}

type VerifyLinkResponse struct {
	Message string `json:"message"`
	Address string `json:"address"`
}

// GetLinkedAddresses fetches the user's linked ETH addresses from the privacy proxy.
// Requires a valid JWT token for authentication.
func (c *Client) GetLinkedAddresses(ctx context.Context, token string) (*LinkedAddressesResponse, error) {
	u, err := url.Parse(c.baseURL + "/eth/addresses")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result LinkedAddressesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CreateLinkChallenge creates a challenge nonce/message for linking an ETH address.
func (c *Client) CreateLinkChallenge(ctx context.Context, token string) (*LinkChallengeResponse, error) {
	u, err := url.Parse(c.baseURL + "/eth/link/challenge")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result LinkChallengeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// VerifyLink verifies the signature for an ETH address link.
func (c *Client) VerifyLink(ctx context.Context, token string, verifyReq VerifyLinkRequest) (*VerifyLinkResponse, error) {
	u, err := url.Parse(c.baseURL + "/eth/link/verify")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	body, err := json.Marshal(verifyReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var result VerifyLinkResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UnlinkAddress removes a linked ETH address.
func (c *Client) UnlinkAddress(ctx context.Context, token string, address string) error {
	normalizedAddress := strings.ToLower(address)
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("failed to parse base URL: %w", err)
	}
	base.Path = base.Path + "/eth/addresses/" + url.PathEscape(normalizedAddress)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, base.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

type ResolveAddressResponse struct {
	RealAddress     string   `json:"real_address"`
	DisclosureLevel string   `json:"disclosure_level"`
	GrantID         string   `json:"grant_id"`
	Pseudonym       string   `json:"pseudonym,omitempty"`
	ScopeMethods    []string `json:"scope_methods,omitempty"` // Methods from grant scope (e.g. "transaction_history", "activity_logs")
}

// ResolveAddressID resolves an opaque address_id back to real address information.
// This is for explorer backend internal use only - the real address should NOT be sent to frontend.
// SECURITY: The caller must apply appropriate redaction before sending data to the frontend.
func (c *Client) ResolveAddressID(ctx context.Context, grantID, addressID string) (*ResolveAddressResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/explorer/grant/%s/resolve/%s", grantID, addressID)
	u, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("grant=%s address=%s: %w", grantID, addressID, ErrNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("grant=%s address=%s: privacy proxy status %d", grantID, addressID, resp.StatusCode)
	}

	var result ResolveAddressResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetGrantTransactions fetches pseudonymized transactions for a disclosure grant
// directly from the privacy proxy. The proxy handles all pseudonymization —
// the explorer just forwards the response as raw JSON.
func (c *Client) GetGrantTransactions(ctx context.Context, grantID, addressID string, limit int, cursor string, beforeBlock *uint64) ([]byte, int, error) {
	endpoint := fmt.Sprintf("/api/v1/explorer/grant/%s/%s/transactions?limit=%d", grantID, addressID, limit)
	// Opaque cursor (RD-1149) takes precedence over the legacy ?before= bound; the
	// proxy returns next_cursor in the JSON body, which we forward as-is.
	if cursor != "" {
		endpoint += "&cursor=" + url.QueryEscape(cursor)
	} else if beforeBlock != nil {
		endpoint += fmt.Sprintf("&before=%d", *beforeBlock)
	}

	u, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return body, resp.StatusCode, nil
}

// ActivityLogsResponse matches the privacy proxy's grant activity logs response.
type ActivityLogsResponse struct {
	Logs   []ActivityLogEntry `json:"logs"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

// ActivityLogEntry is a stripped-down activity log entry (no IP, no params).
type ActivityLogEntry struct {
	Method     string `json:"method"`
	StatusCode int    `json:"status_code"`
	Timestamp  string `json:"timestamp"`
}

// GetGrantActivityLogs fetches activity logs for a disclosure grant from the privacy proxy.
// The caller's JWT is forwarded so the proxy can verify grant holder identity.
func (c *Client) GetGrantActivityLogs(ctx context.Context, grantID string, token string, limit, offset int) ([]byte, int, error) {
	endpoint := fmt.Sprintf("/api/v1/explorer/grant/%s/activity?limit=%d&offset=%d", grantID, limit, offset)

	u, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return body, resp.StatusCode, nil
}
