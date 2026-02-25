package privacy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a client for the privacy-proxy explorer API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new privacy-proxy client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientWithHTTP creates a new privacy-proxy client with a custom HTTP client
func NewClientWithHTTP(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// OwnAddress represents an address owned by the viewer
type OwnAddress struct {
	Address string  `json:"address"`
	ENSName *string `json:"ens_name,omitempty"`
}

// DisclosedAddress represents an address disclosed to the viewer via a grant
type DisclosedAddress struct {
	Address         string     `json:"address"`
	AddressID       string     `json:"address_id"`
	OwnerDID        string     `json:"owner_did"`
	DisclosureLevel string     `json:"disclosure_level"`
	GrantID         string     `json:"grant_id"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	ENSName         *string    `json:"ens_name,omitempty"`
}

// ViewableAddressesResponse is the response from GetViewableAddresses
type ViewableAddressesResponse struct {
	ViewerWallet       string             `json:"viewer_wallet"`
	ViewerDID          string             `json:"viewer_did,omitempty"`
	OwnAddresses       []OwnAddress       `json:"own_addresses"`
	DisclosedAddresses []DisclosedAddress `json:"disclosed_addresses"`
}

// VisibilityLevel represents how much of an address's data is visible
type VisibilityLevel string

const (
	VisibilityFull         VisibilityLevel = "full"
	VisibilityPseudonymous VisibilityLevel = "pseudonymous"
	VisibilityRedacted     VisibilityLevel = "redacted"
	VisibilityHidden       VisibilityLevel = "hidden"
)

// VisibilityReason explains why an address has certain visibility
type VisibilityReason string

const (
	ReasonOwnAddress       VisibilityReason = "own_address"
	ReasonDisclosureGrant  VisibilityReason = "disclosure_grant"
	ReasonPublicAddress    VisibilityReason = "public_address"
	ReasonNoAccess         VisibilityReason = "no_access"
	ReasonRBACGroupMember  VisibilityReason = "rbac_group_member"
)

// AddressVisibility represents the visibility status of a single address
type AddressVisibility struct {
	Address   string           `json:"address"`
	Visible   bool             `json:"visible"`
	Level     VisibilityLevel  `json:"level"`
	Reason    VisibilityReason `json:"reason"`
	Pseudonym *string          `json:"pseudonym,omitempty"`
	GrantID   *string          `json:"grant_id,omitempty"`
	ExpiresAt *time.Time       `json:"expires_at,omitempty"`
}

// BatchCheckAddressesResponse is the response from CheckAddresses
type BatchCheckAddressesResponse struct {
	Results map[string]AddressVisibility `json:"results"`
}

// ViewerIdentity represents how the viewer is identified (wallet and/or DID)
type ViewerIdentity struct {
	Wallet string // Wallet address (optional if DID is provided)
	DID    string // DID from SSO (optional, takes precedence over wallet)
}

// GetViewableAddresses returns all addresses the wallet owner can view
func (c *Client) GetViewableAddresses(ctx context.Context, wallet string) (*ViewableAddressesResponse, error) {
	return c.GetViewableAddressesWithIdentity(ctx, ViewerIdentity{Wallet: wallet})
}

// GetViewableAddressesWithIdentity returns all addresses the viewer can view using wallet or DID
func (c *Client) GetViewableAddressesWithIdentity(ctx context.Context, viewer ViewerIdentity) (*ViewableAddressesResponse, error) {
	u, err := url.Parse(c.baseURL + "/api/v1/explorer/viewable-addresses")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	if viewer.Wallet != "" {
		q.Set("wallet", viewer.Wallet)
	}
	if viewer.DID != "" {
		q.Set("did", viewer.DID)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
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

// CheckAddress checks if a specific address is visible to the wallet owner
func (c *Client) CheckAddress(ctx context.Context, wallet, address string) (*AddressVisibility, error) {
	return c.CheckAddressWithIdentity(ctx, ViewerIdentity{Wallet: wallet}, address)
}

// CheckAddressWithIdentity checks if a specific address is visible to the viewer using wallet or DID
func (c *Client) CheckAddressWithIdentity(ctx context.Context, viewer ViewerIdentity, address string) (*AddressVisibility, error) {
	// Normalize address to lowercase for consistent API calls
	normalizedAddress := strings.ToLower(address)
	u, err := url.Parse(c.baseURL + "/api/v1/explorer/check-address/" + normalizedAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	if viewer.Wallet != "" {
		q.Set("wallet", viewer.Wallet)
	}
	if viewer.DID != "" {
		q.Set("did", viewer.DID)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
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

	var result AddressVisibility
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CheckAddresses checks visibility of multiple addresses at once
func (c *Client) CheckAddresses(ctx context.Context, wallet string, addresses []string) (map[string]*AddressVisibility, error) {
	return c.CheckAddressesWithIdentity(ctx, ViewerIdentity{Wallet: wallet}, addresses)
}

// CheckAddressesWithIdentity checks visibility of multiple addresses using wallet or DID
func (c *Client) CheckAddressesWithIdentity(ctx context.Context, viewer ViewerIdentity, addresses []string) (map[string]*AddressVisibility, error) {
	u, err := url.Parse(c.baseURL + "/api/v1/explorer/check-addresses")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	if viewer.Wallet != "" {
		q.Set("wallet", viewer.Wallet)
	}
	u.RawQuery = q.Encode()

	// Include DID in the request body
	reqBody := map[string]interface{}{"addresses": addresses}
	if viewer.DID != "" {
		reqBody["did"] = viewer.DID
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
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

	var batchResp BatchCheckAddressesResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to pointer map
	result := make(map[string]*AddressVisibility)
	for addr, vis := range batchResp.Results {
		v := vis // Create a copy to take pointer
		result[addr] = &v
	}

	return result, nil
}

// IsVisible returns true if the address is visible to the wallet owner
func (v *AddressVisibility) IsVisible() bool {
	return v.Visible
}

// IsOwnAddress returns true if this is the viewer's own address
func (v *AddressVisibility) IsOwnAddress() bool {
	return v.Reason == ReasonOwnAddress
}

// IsDisclosedAddress returns true if this address was disclosed via a grant
func (v *AddressVisibility) IsDisclosedAddress() bool {
	return v.Reason == ReasonDisclosureGrant
}

// IsPublicAddress returns true if this is a public address with no owner
func (v *AddressVisibility) IsPublicAddress() bool {
	return v.Reason == ReasonPublicAddress
}

// IsEnabled returns true if the client is configured (has a baseURL)
func (c *Client) IsEnabled() bool {
	return c != nil && c.baseURL != ""
}

// ResolveAddressResponse contains the resolved address information
type ResolveAddressResponse struct {
	RealAddress     string `json:"real_address"`
	DisclosureLevel string `json:"disclosure_level"`
	GrantID         string `json:"grant_id"`
	Pseudonym       string `json:"pseudonym,omitempty"`
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

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("grant or address not found")
	}
	if resp.StatusCode == http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("access denied: %s", string(body))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result ResolveAddressResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
