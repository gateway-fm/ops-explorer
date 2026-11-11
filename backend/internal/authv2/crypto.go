package authv2

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type SecretBinding struct {
	FromEnv string
	Value   []byte
}
type codec struct {
	source       *Source
	state, token cipher.AEAD
	clientSecret string
}

func EnvironmentBindings(s *Source) ([]SecretBinding, error) {
	if s == nil {
		return nil, ErrInvalid
	}
	out := []SecretBinding{}
	for _, ref := range []string{s.spec.ClientSecret.FromEnv, s.spec.StateKey.FromEnv, s.spec.TokenKey.FromEnv} {
		v, ok := os.LookupEnv(ref)
		if !ok {
			return nil, ErrInvalid
		}
		b := []byte(v)
		if ref != s.spec.ClientSecret.FromEnv {
			var e error
			b, e = base64.RawURLEncoding.Strict().DecodeString(v)
			if e != nil || len(b) != 32 || base64.RawURLEncoding.EncodeToString(b) != v {
				return nil, ErrInvalid
			}
		}
		out = append(out, SecretBinding{ref, b})
	}
	return out, nil
}
func newCodec(s *Source, bindings []SecretBinding) (*codec, error) {
	if s == nil || s.digest == ([32]byte{}) || len(bindings) != 3 {
		return nil, ErrInvalid
	}
	values := map[string][]byte{}
	for _, b := range bindings {
		if _, ok := values[b.FromEnv]; ok {
			return nil, ErrInvalid
		}
		values[b.FromEnv] = b.Value
	}
	client, state, token := values[s.spec.ClientSecret.FromEnv], values[s.spec.StateKey.FromEnv], values[s.spec.TokenKey.FromEnv]
	if len(client) < 32 || len(client) > 256 || len(state) != 32 || len(token) != 32 || allZero(state) || allZero(token) || bytes.Equal(state, token) || bytes.Equal(client, state) || bytes.Equal(client, token) {
		return nil, ErrInvalid
	}
	for _, c := range client {
		if c < 0x21 || c > 0x7e {
			return nil, ErrInvalid
		}
	}
	for _, pub := range s.keys {
		if bytes.Equal(state, pub) || bytes.Equal(token, pub) || bytes.Equal(client, pub) {
			return nil, ErrInvalid
		}
	}
	c := &codec{source: s, clientSecret: string(client)}
	block, e := aes.NewCipher(state)
	if e != nil {
		return nil, ErrInvalid
	}
	c.state, e = cipher.NewGCM(block)
	if e != nil {
		return nil, ErrInvalid
	}
	block, e = aes.NewCipher(token)
	if e != nil {
		return nil, ErrInvalid
	}
	c.token, e = cipher.NewGCM(block)
	if e != nil {
		return nil, ErrInvalid
	}
	return c, nil
}
func aad(parts ...[]byte) []byte {
	var b bytes.Buffer
	for _, p := range parts {
		_ = binary.Write(&b, binary.BigEndian, uint32(len(p)))
		b.Write(p)
	}
	return b.Bytes()
}
func uuidRaw(v string) []byte {
	if !uuid(v) {
		return nil
	}
	b, _ := hex.DecodeString(strings.ReplaceAll(v, "-", ""))
	return b
}
func number(v int64) []byte { var b [8]byte; binary.BigEndian.PutUint64(b[:], uint64(v)); return b[:] }
func randomToken() (string, error) {
	var b [32]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", ErrUnavailable
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
func handleHash(purpose, token string) [32]byte {
	return sha256.Sum256(aad([]byte("ops-explorer/"+purpose+"/v1"), []byte(token)))
}
func seal(a cipher.AEAD, plain, associated []byte) ([]byte, error) {
	nonce := make([]byte, a.NonceSize())
	if _, e := rand.Read(nonce); e != nil {
		return nil, ErrUnavailable
	}
	return a.Seal(nonce, nonce, plain, associated), nil
}
func open(a cipher.AEAD, raw, associated []byte) ([]byte, error) {
	if len(raw) < a.NonceSize()+a.Overhead() {
		return nil, ErrUnauthenticated
	}
	out, e := a.Open(nil, raw[:a.NonceSize()], raw[a.NonceSize():], associated)
	if e != nil {
		return nil, ErrUnauthenticated
	}
	return out, nil
}

type pendingCapture struct {
	browser         [32]byte
	revision        string
	hash            [32]byte
	issued, expires int64
}
type activeCapture struct {
	browser, auth      [32]byte
	activation         string
	activated, expires int64
	key                string
	cipher             []byte
}

func (c *codec) pendingAAD(p pendingCapture) []byte {
	return aad([]byte("ops-explorer/sso-state/v1"), c.source.digest[:], uuidRaw(c.source.spec.RecoveryEpoch), p.browser[:], uuidRaw(p.revision), p.hash[:], number(p.issued), number(p.expires), []byte(c.source.spec.StateKey.ID))
}
func (c *codec) tokenAAD(a activeCapture) []byte {
	return aad([]byte("ops-explorer/sso-token/v1"), c.source.digest[:], uuidRaw(c.source.spec.RecoveryEpoch), a.browser[:], a.auth[:], uuidRaw(a.activation), number(a.activated), number(a.expires), []byte(a.key))
}

type statePayload struct {
	Version         int    `json:"v"`
	Revision        string `json:"revision"`
	State           string `json:"state"`
	Verifier        string `json:"verifier"`
	ReturnPath      string `json:"returnPath"`
	Mode            string `json:"mode"`
	ExpectedActor   string `json:"expectedActor"`
	ExpectedProfile string `json:"expectedProfile"`
	IssuedAt        int64  `json:"issuedAt"`
	ExpiresAt       int64  `json:"expiresAt"`
}

func (c *codec) sealState(p pendingCapture, s statePayload) (string, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if e.Encode(s) != nil || b.Len() > 2642 {
		return "", ErrInvalid
	}
	raw, err := seal(c.state, b.Bytes(), c.pendingAAD(p))
	if err != nil {
		return "", err
	}
	frame := "v1." + c.source.spec.StateKey.ID + "." + base64.RawURLEncoding.EncodeToString(raw)
	if len(frame) > 3596 {
		return "", ErrInvalid
	}
	return frame, nil
}
func (c *codec) openState(p pendingCapture, frame string, now int64) (statePayload, error) {
	var s statePayload
	parts := strings.Split(frame, ".")
	if len(frame) > 3596 || len(parts) != 3 || parts[0] != "v1" || parts[1] != c.source.spec.StateKey.ID {
		return s, ErrUnauthenticated
	}
	raw, e := base64.RawURLEncoding.Strict().DecodeString(parts[2])
	if e != nil || base64.RawURLEncoding.EncodeToString(raw) != parts[2] {
		return s, ErrUnauthenticated
	}
	plain, e := open(c.state, raw, c.pendingAAD(p))
	if e != nil {
		return s, e
	}
	if decode(plain, 2642, &s, "v", "revision", "state", "verifier", "returnPath", "mode", "expectedActor", "expectedProfile", "issuedAt", "expiresAt") != nil || s.Version != 1 || s.Revision != p.revision || s.IssuedAt != p.issued || s.ExpiresAt != p.expires || now < s.IssuedAt || now >= s.ExpiresAt || s.ExpiresAt > s.IssuedAt+300 || !token32(s.State) || !token32(s.Verifier) || handleHash("oauth-state", s.State) != p.hash || !returnPath(s.ReturnPath) || s.Mode != "login" && s.Mode != "renew" || s.Mode == "login" && (s.ExpectedActor != "" || s.ExpectedProfile != "") || s.Mode == "renew" && (!actor(s.ExpectedActor) || !visible(s.ExpectedProfile, 128)) {
		return statePayload{}, ErrUnauthenticated
	}
	return s, nil
}
func (c *codec) sealToken(a activeCapture, raw string) ([]byte, error) {
	if len(raw) == 0 || len(raw) > MaxTokenBytes || a.key != c.source.spec.TokenKey.ID || !uuid(a.activation) || a.expires <= a.activated {
		return nil, ErrInvalid
	}
	return seal(c.token, []byte(raw), c.tokenAAD(a))
}
func (c *codec) openToken(a activeCapture, now int64) (string, *claims, error) {
	if a.key != c.source.spec.TokenKey.ID || !uuid(a.activation) || a.activated < 0 || a.expires <= a.activated || a.expires > a.activated+330 || a.expires <= now || len(a.cipher) > MaxCipherBytes {
		return "", nil, ErrUnauthenticated
	}
	raw, e := open(c.token, a.cipher, c.tokenAAD(a))
	if e != nil {
		return "", nil, e
	}
	v, e := c.source.verify(string(raw), now)
	if e != nil || v.Expires != a.expires {
		return "", nil, ErrUnauthenticated
	}
	return string(raw), v, nil
}

type fact struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}
type claims struct {
	Issuer         string  `json:"iss"`
	Audience       string  `json:"aud"`
	Subject        string  `json:"sub"`
	ID             string  `json:"jti"`
	Issued         int64   `json:"iat"`
	NotBefore      int64   `json:"nbf"`
	Expires        int64   `json:"exp"`
	Kind           string  `json:"principal_kind"`
	Profile        string  `json:"source_profile"`
	ProfileDigest  string  `json:"source_profile_digest"`
	Authority      string  `json:"identity_authority"`
	SourceSubject  string  `json:"source_subject"`
	Tenant         *string `json:"source_tenant,omitempty"`
	Verified       int64   `json:"source_verified_at"`
	AsOf           int64   `json:"claims_as_of"`
	SourceNotAfter *int64  `json:"source_not_after,omitempty"`
	Facts          []fact  `json:"facts"`
}

func (s *Source) verify(raw string, now int64) (*claims, error) {
	parts := strings.Split(raw, ".")
	if len(raw) > MaxTokenBytes || len(parts) != 3 || now < 0 || now > MaxTime-330 {
		return nil, ErrUnauthenticated
	}
	decoded := make([][]byte, 3)
	for i, p := range parts {
		b, e := base64.RawURLEncoding.Strict().DecodeString(p)
		if e != nil || base64.RawURLEncoding.EncodeToString(b) != p {
			return nil, ErrUnauthenticated
		}
		decoded[i] = b
	}
	var h struct {
		Type      string `json:"typ"`
		Algorithm string `json:"alg"`
		Kid       string `json:"kid"`
	}
	if decode(decoded[0], 2048, &h, "typ", "alg", "kid") != nil || h.Type != "ops-access+jwt" || h.Algorithm != "EdDSA" || s.keys[h.Kid] == nil || len(decoded[2]) != 64 {
		return nil, ErrUnauthenticated
	}
	var v claims
	if decode(decoded[1], 12<<10, &v, "iss", "aud", "sub", "jti", "iat", "nbf", "exp", "principal_kind", "source_profile", "source_profile_digest", "identity_authority", "source_subject", "source_verified_at", "claims_as_of", "facts") != nil || v.Issuer != s.spec.Issuer || v.Audience != s.spec.Audience || v.Kind != "human" || !actor(v.Subject) || !visible(v.ID, 128) || !visible(v.Profile, 128) || s.profiles[v.Profile] != v.ProfileDigest || !text(v.Authority, 2048) || !text(v.SourceSubject, 1024) || v.Facts == nil || len(v.Facts) > 256 {
		return nil, ErrUnauthenticated
	}
	for _, t := range []int64{v.Issued, v.NotBefore, v.Verified, v.AsOf} {
		if t < 0 || t > now+30 {
			return nil, ErrUnauthenticated
		}
	}
	if v.NotBefore > v.Issued || v.AsOf > v.Verified || v.Verified > v.Issued || v.Expires <= now || v.Expires > MaxTime || v.Expires <= v.Issued || v.Expires > v.Issued+300 || v.Expires > v.AsOf+300 || v.SourceNotAfter != nil && (v.Expires > *v.SourceNotAfter || *v.SourceNotAfter < 0) {
		return nil, ErrUnauthenticated
	}
	seen := map[fact]bool{}
	for _, f := range v.Facts {
		if !visible(f.Kind, 64) || !text(f.Value, 256) || seen[f] {
			return nil, ErrUnauthenticated
		}
		seen[f] = true
	}
	if v.Tenant != nil && !uuid(*v.Tenant) {
		return nil, ErrUnauthenticated
	}
	if jwt.SigningMethodEdDSA.Verify(parts[0]+"."+parts[1], decoded[2], s.keys[h.Kid]) != nil {
		return nil, ErrUnauthenticated
	}
	return &v, nil
}
