package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

// VerifierConfig configures JWT signature verification against a JWKS endpoint
// (A-2). JWKSURL is required to enable verification; Issuer/Audience are
// validated only when non-empty. TTL/MinRefetchInterval/HTTPClient default to
// production-sane values when zero/nil.
type VerifierConfig struct {
	JWKSURL  string
	Issuer   string
	Audience string
	// TTL bounds how long the JWKS cache is trusted before a refresh.
	TTL time.Duration
	// MinRefetchInterval rate-limits the "unknown kid" refetch so a flood of
	// bogus kids cannot hammer the JWKS endpoint.
	MinRefetchInterval time.Duration
	// HTTPClient is used to fetch the JWKS. Tests inject a stub; production
	// leaves it nil and a default client is used.
	HTTPClient *http.Client
	// Leeway tolerates small clock skew on exp/nbf/iat. Defaults to 30s.
	Leeway time.Duration
}

const (
	defaultJWKSTTL          = 10 * time.Minute
	defaultMinRefetch       = 1 * time.Minute
	defaultVerifyLeeway     = 30 * time.Second
	defaultJWKSFetchTimeout = 10 * time.Second
)

// Verifier verifies JWT signatures against a privacy-proxy JWKS (A-2). It is
// alg-confusion-safe: only RS256/ES256 are accepted, and the keyfunc asserts
// the token's signing method matches the JWKS key type — an HS* token can never
// be handed an asymmetric public key as its secret.
type Verifier struct {
	cfg        VerifierConfig
	httpClient *http.Client

	mu          sync.RWMutex
	keys        map[string]crypto.PublicKey // kid -> public key
	fetchedAt   time.Time
	lastRefetch time.Time // last "unknown kid" refetch (rate-limit anchor)
}

// NewVerifier constructs an alg-confusion-safe JWT Verifier.
func NewVerifier(cfg VerifierConfig) *Verifier {
	if cfg.TTL <= 0 {
		cfg.TTL = defaultJWKSTTL
	}
	if cfg.MinRefetchInterval <= 0 {
		cfg.MinRefetchInterval = defaultMinRefetch
	}
	if cfg.Leeway <= 0 {
		cfg.Leeway = defaultVerifyLeeway
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: defaultJWKSFetchTimeout}
	}
	return &Verifier{
		cfg:        cfg,
		httpClient: hc,
		keys:       map[string]crypto.PublicKey{},
	}
}

// VerifyToken verifies the token's signature and registered claims against the
// configured JWKS and returns the parsed claims. It rejects: wrong signatures,
// alg=none, HS* (including the RSA->HMAC algorithm-confusion attack), missing
// or unknown kid, missing/expired exp (beyond leeway), and — when configured —
// a mismatching iss/aud.
func (v *Verifier) VerifyToken(token string) (*JWTClaims, error) {
	parserOpts := []jwt.ParserOption{
		// Layer 1: only asymmetric methods are valid. This alone rejects
		// alg=none and HS*; the keyfunc below is the redundant layer 2.
		jwt.WithValidMethods([]string{"RS256", "ES256"}),
		jwt.WithLeeway(v.cfg.Leeway),
		jwt.WithExpirationRequired(),
	}
	if v.cfg.Issuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(v.cfg.Issuer))
	}
	if v.cfg.Audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(v.cfg.Audience))
	}

	parsed, err := jwt.Parse(token, v.keyfunc, parserOpts...)
	if err != nil {
		return nil, fmt.Errorf("jwt verification failed: %w", err)
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("jwt invalid")
	}

	// Re-extract into the project's display claims shape. (The signature is
	// already verified at this point; this just normalizes the struct.)
	claims, err := ExtractClaims(token)
	if err != nil {
		return nil, fmt.Errorf("verified token but failed to parse claims: %w", err)
	}
	return claims, nil
}

// keyfunc selects the JWKS public key by the token's kid and asserts the
// token's signing method matches the key type. It is the second, redundant
// layer of algorithm-confusion defense: even if WithValidMethods were bypassed,
// an HS* token would never be returned an asymmetric key here.
func (v *Verifier) keyfunc(token *jwt.Token) (interface{}, error) {
	kidVal, ok := token.Header["kid"]
	if !ok {
		return nil, fmt.Errorf("token has no kid header")
	}
	kid, ok := kidVal.(string)
	if !ok || kid == "" {
		return nil, fmt.Errorf("token kid is empty or not a string")
	}

	key, err := v.keyForKid(kid)
	if err != nil {
		return nil, err
	}

	// Assert the signing method matches the key type. Never hand an HMAC method
	// an asymmetric key.
	switch token.Method.(type) {
	case *jwt.SigningMethodRSA:
		if _, isRSA := key.(*rsa.PublicKey); !isRSA {
			return nil, fmt.Errorf("token alg %q does not match key type for kid %q", token.Method.Alg(), kid)
		}
	case *jwt.SigningMethodECDSA:
		if _, isEC := key.(*ecdsa.PublicKey); !isEC {
			return nil, fmt.Errorf("token alg %q does not match key type for kid %q", token.Method.Alg(), kid)
		}
	default:
		// Any other method (HS*, none, ...) is refused outright.
		return nil, fmt.Errorf("unsupported signing method %q (only RS256/ES256 allowed)", token.Method.Alg())
	}
	return key, nil
}

// keyForKid returns the public key for kid, populating/refreshing the JWKS
// cache as needed. On an unknown kid it performs at most one rate-limited
// refetch to handle proxy key rotation.
func (v *Verifier) keyForKid(kid string) (crypto.PublicKey, error) {
	// Fast path: cached and fresh.
	v.mu.RLock()
	if time.Since(v.fetchedAt) <= v.cfg.TTL {
		if k, ok := v.keys[kid]; ok {
			v.mu.RUnlock()
			return k, nil
		}
	}
	v.mu.RUnlock()

	// Ensure the cache is populated/fresh (initial fetch or TTL expiry).
	if err := v.ensureFresh(); err != nil {
		return nil, err
	}
	v.mu.RLock()
	k, ok := v.keys[kid]
	v.mu.RUnlock()
	if ok {
		return k, nil
	}

	// Unknown kid: rate-limited single refetch (handles key rotation).
	if err := v.refetchForUnknownKid(); err != nil {
		return nil, err
	}
	v.mu.RLock()
	k, ok = v.keys[kid]
	v.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no JWKS key matches kid %q", kid)
	}
	return k, nil
}

// ensureFresh fetches the JWKS if the cache is empty or past its TTL.
func (v *Verifier) ensureFresh() error {
	v.mu.RLock()
	fresh := len(v.keys) > 0 && time.Since(v.fetchedAt) <= v.cfg.TTL
	v.mu.RUnlock()
	if fresh {
		return nil
	}
	return v.fetch()
}

// refetchForUnknownKid refetches the JWKS at most once per MinRefetchInterval.
func (v *Verifier) refetchForUnknownKid() error {
	v.mu.Lock()
	if !v.lastRefetch.IsZero() && time.Since(v.lastRefetch) < v.cfg.MinRefetchInterval {
		v.mu.Unlock()
		return nil // rate-limited; do not hammer the JWKS endpoint
	}
	v.lastRefetch = time.Now()
	v.mu.Unlock()
	return v.fetch()
}

// fetch loads and parses the JWKS document, replacing the cache.
func (v *Verifier) fetch() error {
	if v.cfg.JWKSURL == "" {
		return fmt.Errorf("no JWKS URL configured")
	}
	resp, err := v.httpClient.Get(v.cfg.JWKSURL)
	if err != nil {
		return fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch JWKS: status %d", resp.StatusCode)
	}

	var doc struct {
		Keys []jwkEntry `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("decode JWKS: %w", err)
	}

	keys := make(map[string]crypto.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kid == "" {
			continue
		}
		pub, err := k.publicKey()
		if err != nil {
			// Skip unparseable keys rather than failing the whole set.
			continue
		}
		keys[k.Kid] = pub
	}

	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	return nil
}

// jwkEntry is a single JWKS key (RSA or EC).
type jwkEntry struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`   // RSA modulus
	E   string `json:"e"`   // RSA exponent
	Crv string `json:"crv"` // EC curve
	X   string `json:"x"`   // EC x
	Y   string `json:"y"`   // EC y
}

func (k jwkEntry) publicKey() (crypto.PublicKey, error) {
	switch k.Kty {
	case "RSA":
		nb, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(k.N, "="))
		if err != nil {
			return nil, fmt.Errorf("decode RSA n: %w", err)
		}
		eb, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(k.E, "="))
		if err != nil {
			return nil, fmt.Errorf("decode RSA e: %w", err)
		}
		e := 0
		for _, b := range eb {
			e = e<<8 | int(b)
		}
		if e == 0 {
			return nil, fmt.Errorf("invalid RSA exponent")
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}, nil
	case "EC":
		xb, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(k.X, "="))
		if err != nil {
			return nil, fmt.Errorf("decode EC x: %w", err)
		}
		yb, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(k.Y, "="))
		if err != nil {
			return nil, fmt.Errorf("decode EC y: %w", err)
		}
		var curve elliptic.Curve
		switch k.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, fmt.Errorf("unsupported EC curve %q", k.Crv)
		}
		return &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(xb), Y: new(big.Int).SetBytes(yb)}, nil
	default:
		return nil, fmt.Errorf("unsupported kty %q", k.Kty)
	}
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
