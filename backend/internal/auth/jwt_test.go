package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// --- JWKS test harness -------------------------------------------------------

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use,omitempty"`
	Alg string `json:"alg,omitempty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

func rsaPublicJWK(pub *rsa.PublicKey, kid string) jwk {
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	return jwk{
		Kty: "RSA",
		Kid: kid,
		Use: "sig",
		Alg: "RS256",
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
	}
}

// jwksServer serves a JWKS document and counts how many times it was fetched
// (to assert TTL caching + single rate-limited refetch behavior).
type jwksServer struct {
	srv    *httptest.Server
	hits   atomic.Int64
	docPtr atomic.Value // jwks
}

func newJWKSServer(doc jwks) *jwksServer {
	js := &jwksServer{}
	js.docPtr.Store(doc)
	js.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		js.hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		cur := js.docPtr.Load().(jwks)
		_ = json.NewEncoder(w).Encode(cur)
	}))
	return js
}

func (js *jwksServer) URL() string  { return js.srv.URL }
func (js *jwksServer) Close()        { js.srv.Close() }
func (js *jwksServer) Hits() int64   { return js.hits.Load() }
func (js *jwksServer) setDoc(d jwks) { js.docPtr.Store(d) }

func signRS256(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	if kid != "" {
		tok.Header["kid"] = kid
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign RS256: %v", err)
	}
	return s
}

// publicKeyBytes returns the PKIX/DER-encoded public key — the bytes a server
// publishes and that an attacker would use as the HMAC secret in the RSA->HMAC
// algorithm-confusion attack.
func publicKeyBytes(t *testing.T, pub *rsa.PublicKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return der
}

// makeHS256TestToken mirrors the api package's display-only makeTestJWT: an
// HS256-"signed" JWT with a fake signature, used to prove VerifyToken rejects
// the existing fixture against an RSA JWKS.
func makeHS256TestToken(sub string, exp time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(
		`{"sub":"` + sub + `","exp":` + itoa(exp.Unix()) + `}`))
	return header + "." + claims + ".fakesig"
}

func itoa(n int64) string { return big.NewInt(n).String() }

func baseClaims() jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"sub": "did:privado:subject-1",
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}
}

// --- A-2 tests ---------------------------------------------------------------

// A-2: a correctly RS256-signed token with a kid present in the JWKS must be
// ACCEPTED and its claims returned (PROD_READINESS_AUDIT §A-2).
func TestVerifyToken_AcceptsValidRS256(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	const kid = "kid-1"
	js := newJWKSServer(jwks{Keys: []jwk{rsaPublicJWK(&key.PublicKey, kid)}})
	defer js.Close()

	v := NewVerifier(VerifierConfig{JWKSURL: js.URL()})
	tokenStr := signRS256(t, key, kid, baseClaims())

	claims, err := v.VerifyToken(tokenStr)
	if err != nil {
		t.Fatalf("expected valid RS256 token to verify, got error: %v", err)
	}
	if claims == nil || claims.GetDID() != "did:privado:subject-1" {
		t.Fatalf("expected sub did:privado:subject-1, got %#v", claims)
	}
}

// A-2 (the headline alg-confusion case): a token with alg=HS256 signed using
// the RSA PUBLIC key bytes as the HMAC secret must be REJECTED. VerifyToken
// must never treat an asymmetric public key as an HMAC secret.
func TestVerifyToken_RejectsAlgConfusionHS256(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	const kid = "kid-1"
	js := newJWKSServer(jwks{Keys: []jwk{rsaPublicJWK(&key.PublicKey, kid)}})
	defer js.Close()

	v := NewVerifier(VerifierConfig{JWKSURL: js.URL()})

	// Forge an HS256 token signed with the public key's PKIX bytes — the
	// classic RSA->HMAC algorithm-confusion attack.
	pubPEM := publicKeyBytes(t, &key.PublicKey)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, baseClaims())
	tok.Header["kid"] = kid
	forged, err := tok.SignedString(pubPEM)
	if err != nil {
		t.Fatalf("forge HS256: %v", err)
	}

	if _, err := v.VerifyToken(forged); err == nil {
		t.Fatal("SECURITY: HS256 token signed with the RSA public key was ACCEPTED — algorithm-confusion not prevented")
	}
}

// A-2: the existing display-only fixture makeTestJWT signs HS256 with a fake
// signature. Verified against an RSA JWKS it MUST be rejected.
func TestVerifyToken_RejectsHS256Fixture(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	js := newJWKSServer(jwks{Keys: []jwk{rsaPublicJWK(&key.PublicKey, "kid-1")}})
	defer js.Close()

	v := NewVerifier(VerifierConfig{JWKSURL: js.URL()})
	hs := makeHS256TestToken("did:privado:x", time.Now().Add(time.Hour))

	if _, err := v.VerifyToken(hs); err == nil {
		t.Fatal("SECURITY: HS256 display-only fixture was accepted by VerifyToken against an RSA key")
	}
}

// A-2: alg=none must be rejected.
func TestVerifyToken_RejectsAlgNone(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	js := newJWKSServer(jwks{Keys: []jwk{rsaPublicJWK(&key.PublicKey, "kid-1")}})
	defer js.Close()

	v := NewVerifier(VerifierConfig{JWKSURL: js.URL()})

	tok := jwt.NewWithClaims(jwt.SigningMethodNone, baseClaims())
	tok.Header["kid"] = "kid-1"
	noneTok, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}
	if _, err := v.VerifyToken(noneTok); err == nil {
		t.Fatal("SECURITY: alg=none token was accepted")
	}
}

// A-2: a token whose signature does not match the JWKS key must be rejected.
func TestVerifyToken_RejectsWrongSignature(t *testing.T) {
	keyA, _ := rsa.GenerateKey(rand.Reader, 2048)
	keyB, _ := rsa.GenerateKey(rand.Reader, 2048) // not in JWKS
	const kid = "kid-1"
	js := newJWKSServer(jwks{Keys: []jwk{rsaPublicJWK(&keyA.PublicKey, kid)}})
	defer js.Close()

	v := NewVerifier(VerifierConfig{JWKSURL: js.URL()})
	// Sign with keyB but claim kid-1 (which maps to keyA in the JWKS).
	bad := signRS256(t, keyB, kid, baseClaims())

	if _, err := v.VerifyToken(bad); err == nil {
		t.Fatal("expected token signed by a non-JWKS key to be rejected")
	}
}

// A-2: a token with no kid header must be rejected (kid is mandatory).
func TestVerifyToken_RejectsMissingKid(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	js := newJWKSServer(jwks{Keys: []jwk{rsaPublicJWK(&key.PublicKey, "kid-1")}})
	defer js.Close()

	v := NewVerifier(VerifierConfig{JWKSURL: js.URL()})
	noKid := signRS256(t, key, "" /* no kid */, baseClaims())

	if _, err := v.VerifyToken(noKid); err == nil {
		t.Fatal("expected a token with no kid to be rejected")
	}
}

// A-2: an unknown kid triggers a single rate-limited JWKS refetch; if still
// unknown the token is rejected.
func TestVerifyToken_RejectsUnknownKid(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	js := newJWKSServer(jwks{Keys: []jwk{rsaPublicJWK(&key.PublicKey, "kid-1")}})
	defer js.Close()

	v := NewVerifier(VerifierConfig{JWKSURL: js.URL()})
	unknown := signRS256(t, key, "kid-DOES-NOT-EXIST", baseClaims())

	if _, err := v.VerifyToken(unknown); err == nil {
		t.Fatal("expected a token with an unknown kid to be rejected")
	}
}

// A-2: an expired token (beyond the 30s leeway) must be rejected; mandatory exp.
func TestVerifyToken_RejectsExpired(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	const kid = "kid-1"
	js := newJWKSServer(jwks{Keys: []jwk{rsaPublicJWK(&key.PublicKey, kid)}})
	defer js.Close()

	v := NewVerifier(VerifierConfig{JWKSURL: js.URL()})
	claims := baseClaims()
	claims["exp"] = time.Now().Add(-5 * time.Minute).Unix() // well past 30s leeway
	expired := signRS256(t, key, kid, claims)

	if _, err := v.VerifyToken(expired); err == nil {
		t.Fatal("expected an expired token (beyond leeway) to be rejected")
	}
}

// A-2: a token missing exp must be rejected (exp is mandatory).
func TestVerifyToken_RejectsMissingExp(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	const kid = "kid-1"
	js := newJWKSServer(jwks{Keys: []jwk{rsaPublicJWK(&key.PublicKey, kid)}})
	defer js.Close()

	v := NewVerifier(VerifierConfig{JWKSURL: js.URL()})
	claims := jwt.MapClaims{"sub": "did:privado:x", "iat": time.Now().Unix()} // no exp
	noExp := signRS256(t, key, kid, claims)

	if _, err := v.VerifyToken(noExp); err == nil {
		t.Fatal("expected a token with no exp to be rejected (exp mandatory)")
	}
}

// A-2: iss/aud are validated only when configured. When configured, a
// mismatching issuer must be rejected and a matching one accepted.
func TestVerifyToken_IssuerAudienceWhenConfigured(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	const kid = "kid-1"
	js := newJWKSServer(jwks{Keys: []jwk{rsaPublicJWK(&key.PublicKey, kid)}})
	defer js.Close()

	v := NewVerifier(VerifierConfig{
		JWKSURL:  js.URL(),
		Issuer:   "https://issuer.example.com",
		Audience: "explorer",
	})

	good := baseClaims()
	good["iss"] = "https://issuer.example.com"
	good["aud"] = "explorer"
	if _, err := v.VerifyToken(signRS256(t, key, kid, good)); err != nil {
		t.Fatalf("expected matching iss/aud to verify: %v", err)
	}

	bad := baseClaims()
	bad["iss"] = "https://evil.example.com"
	bad["aud"] = "explorer"
	if _, err := v.VerifyToken(signRS256(t, key, kid, bad)); err == nil {
		t.Fatal("expected a mismatching issuer to be rejected")
	}
}

// A-2: JWKS is cached (TTL) — repeated verifications of valid tokens must not
// refetch the JWKS every call; a key rotation (new kid) triggers exactly one
// extra refetch.
func TestVerifyToken_JWKSCachingAndSingleRefetch(t *testing.T) {
	keyA, _ := rsa.GenerateKey(rand.Reader, 2048)
	js := newJWKSServer(jwks{Keys: []jwk{rsaPublicJWK(&keyA.PublicKey, "kid-A")}})
	defer js.Close()

	v := NewVerifier(VerifierConfig{JWKSURL: js.URL()})

	// Two verifications with the same known kid: at most one fetch (cached).
	tokA := signRS256(t, keyA, "kid-A", baseClaims())
	if _, err := v.VerifyToken(tokA); err != nil {
		t.Fatalf("verify A #1: %v", err)
	}
	if _, err := v.VerifyToken(tokA); err != nil {
		t.Fatalf("verify A #2: %v", err)
	}
	if h := js.Hits(); h != 1 {
		t.Errorf("expected JWKS fetched once for a known kid (TTL cache), got %d fetches", h)
	}

	// Rotate: add kid-B. An incoming kid-B token is unknown -> single refetch.
	keyB, _ := rsa.GenerateKey(rand.Reader, 2048)
	js.setDoc(jwks{Keys: []jwk{
		rsaPublicJWK(&keyA.PublicKey, "kid-A"),
		rsaPublicJWK(&keyB.PublicKey, "kid-B"),
	}})
	tokB := signRS256(t, keyB, "kid-B", baseClaims())
	if _, err := v.VerifyToken(tokB); err != nil {
		t.Fatalf("verify B after rotation: %v", err)
	}
	if h := js.Hits(); h != 2 {
		t.Errorf("expected exactly one refetch on unknown kid (total 2), got %d fetches", h)
	}
}
