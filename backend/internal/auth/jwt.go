package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type JWTClaims struct {
	Subject   string `json:"sub"` // DID
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
	KYC       bool   `json:"kyc,omitempty"`
}

// ExtractClaims extracts claims from a JWT without verifying the signature
// This is safe for non-security-critical reads since the token was just issued by privacy-proxy
// and is used only for display purposes (the actual authorization is done by privacy-proxy)
func ExtractClaims(token string) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Try standard base64 with padding
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
		}
	}

	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	return &claims, nil
}

func (c *JWTClaims) IsExpired() bool {
	return time.Now().Unix() > c.ExpiresAt
}

func (c *JWTClaims) GetDID() string {
	return c.Subject
}

func (c *JWTClaims) TimeToExpiry() time.Duration {
	return time.Until(time.Unix(c.ExpiresAt, 0))
}
