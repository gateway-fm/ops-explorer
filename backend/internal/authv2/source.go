// Package authv2 owns Explorer's original-token confidential app session. It
// has no user directory, provider refresh credential or legacy auth fallback.
package authv2

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalid         = errors.New("invalid v2 app configuration or request")
	ErrUnavailable     = errors.New("v2 app unavailable")
	ErrUnauthenticated = errors.New("v2 app authentication required")
	ErrChanged         = errors.New("v2 app session changed")
	ErrUnknownCommit   = errors.New("v2 app commit outcome unknown")
)

const AppRole = "explorer_sso_app"
const MaxTokenBytes = 16384
const MaxCipherBytes = MaxTokenBytes + 28
const MaxTime = int64(253402300799)

type Clock func() int64
type KeyRef struct {
	ID      string `json:"id"`
	FromEnv string `json:"fromEnv"`
}
type SecretRef struct {
	FromEnv string `json:"fromEnv"`
}
type PublicKey struct {
	ID string `json:"id"`
	X  string `json:"x"`
}
type Profile struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}
type Limits struct {
	MaxContexts      int   `json:"maxContexts"`
	MaxCipherBytes   int64 `json:"maxCipherBytes"`
	MaxMetadataBytes int64 `json:"maxMetadataBytes"`
}

func DefaultLimits() Limits { return Limits{200000, 1 << 30, 512 << 20} }
func (l Limits) valid() bool {
	return l.MaxContexts >= 1 && l.MaxContexts <= 200000 && l.MaxCipherBytes >= 1 && l.MaxCipherBytes <= 1<<30 && l.MaxMetadataBytes >= 2048 && l.MaxMetadataBytes <= 512<<20
}

type Network struct {
	ChainID     string `json:"chainId"`
	GenesisHash string `json:"genesisHash"`
}
type Retention struct {
	Enabled        bool `json:"enabled"`
	BatchSize      int  `json:"batchSize"`
	IntervalMillis int  `json:"intervalMillis"`
}
type SourceSpec struct {
	APIVersion    string      `json:"apiVersion"`
	Kind          string      `json:"kind"`
	AppOrigin     string      `json:"appOrigin"`
	OPSOrigin     string      `json:"opsOrigin"`
	CallbackURI   string      `json:"callbackURI"`
	ClientID      string      `json:"clientId"`
	ClientSecret  SecretRef   `json:"clientSecret"`
	Issuer        string      `json:"issuer"`
	Audience      string      `json:"audience"`
	Keys          []PublicKey `json:"keys"`
	Profiles      []Profile   `json:"profiles"`
	StateKey      KeyRef      `json:"stateKey"`
	TokenKey      KeyRef      `json:"tokenKey"`
	RecoveryEpoch string      `json:"recoveryEpoch"`
	Limits        Limits      `json:"limits"`
	Network       Network     `json:"network"`
	Retention     Retention   `json:"retention"`
}
type Setup struct {
	Protocol              string `json:"protocol"`
	Source                string `json:"source"`
	ChainID               string `json:"chainId"`
	ChainIDDecimal        string `json:"chainIdDecimal"`
	GenesisHash           string `json:"genesisHash"`
	PrivacyProxyPublicURL string `json:"privacyProxyPublicUrl"`
}
type Source struct {
	spec     SourceSpec
	digest   [32]byte
	keys     map[string]ed25519.PublicKey
	profiles map[string]string
	host     string
	setup    []byte
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var envName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,127}$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var hashPattern = regexp.MustCompile(`^0x[0-9a-f]{64}$`)

func uuid(v string) bool {
	return uuidPattern.MatchString(v) && v != "00000000-0000-0000-0000-000000000000"
}
func visible(v string, n int) bool {
	if len(v) == 0 || len(v) > n {
		return false
	}
	for _, c := range []byte(v) {
		if c < 0x21 || c > 0x7e {
			return false
		}
	}
	return true
}
func text(v string, n int) bool {
	return len(v) > 0 && len(v) <= n && utf8.ValidString(v) && !strings.ContainsAny(v, "\x00\r\n\t")
}
func token32(v string) bool {
	if len(v) != 43 {
		return false
	}
	b, e := base64.RawURLEncoding.Strict().DecodeString(v)
	return e == nil && len(b) == 32 && base64.RawURLEncoding.EncodeToString(b) == v
}
func actor(v string) bool {
	return strings.HasPrefix(v, "ops:p1:") && token32(strings.TrimPrefix(v, "ops:p1:"))
}
func hostname(v string) bool {
	if ip, e := netip.ParseAddr(v); e == nil {
		return ip.Zone() == "" && !ip.Is4In6() && ip.String() == v
	}
	if len(v) == 0 || len(v) > 253 || strings.Contains(v, ":") {
		return false
	}
	parts := strings.Split(strings.TrimSuffix(v, "."), ".")
	last := parts[len(parts)-1]
	numeric := true
	for _, c := range last {
		numeric = numeric && c >= '0' && c <= '9'
	}
	if last == "" || numeric || strings.HasPrefix(last, "0x") {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 || len(p) > 63 || p[0] == '-' || p[len(p)-1] == '-' {
			return false
		}
		for _, c := range p {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}
func origin(raw string) (*url.URL, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || u.String() != raw || strings.ToLower(u.Host) != u.Host || strings.ContainsAny(u.Host, "%\\") || u.Port() == "443" || !hostname(u.Hostname()) {
		return nil, ErrInvalid
	}
	h := u.Hostname()
	if strings.Contains(h, ":") {
		h = "[" + h + "]"
	}
	if p := u.Port(); p != "" {
		n, e := strconv.ParseUint(p, 10, 16)
		if e != nil || n == 0 || strconv.FormatUint(n, 10) != p {
			return nil, ErrInvalid
		}
		h += ":" + p
	}
	if h != u.Host {
		return nil, ErrInvalid
	}
	return u, nil
}
func ParseSource(raw []byte, expected [32]byte) (*Source, error) {
	if expected == ([32]byte{}) || sha256.Sum256(raw) != expected {
		return nil, ErrInvalid
	}
	var s SourceSpec
	if decode(raw, 64<<10, &s, "apiVersion", "kind", "appOrigin", "opsOrigin", "callbackURI", "clientId", "clientSecret", "issuer", "audience", "keys", "profiles", "stateKey", "tokenKey", "recoveryEpoch", "limits", "network", "retention") != nil {
		return nil, ErrInvalid
	}
	app, e := origin(s.AppOrigin)
	if e != nil {
		return nil, e
	}
	if _, e = origin(s.OPSOrigin); e != nil {
		return nil, e
	}
	callback, e := url.Parse(s.CallbackURI)
	if e != nil || callback.Scheme+"://"+callback.Host != s.AppOrigin || callback.User != nil || callback.Opaque != "" || callback.RawQuery != "" || callback.ForceQuery || callback.Fragment != "" || callback.String() != s.CallbackURI || len(s.CallbackURI) > 2048 || callback.Path != "/api/auth/callback" || path.Clean(callback.Path) != callback.Path || strings.ContainsAny(s.CallbackURI, "%\\ \t\r\n") || strings.Contains(callback.Path, "//") {
		return nil, ErrInvalid
	}
	if s.APIVersion != "ops.explorer-sso/v2" || s.Kind != "ExplorerSSO" || !namePattern.MatchString(s.ClientID) || !envName.MatchString(s.ClientSecret.FromEnv) || !text(s.Issuer, 2048) || !text(s.Audience, 256) || !uuid(s.RecoveryEpoch) || !s.Limits.valid() || len(s.Keys) < 1 || len(s.Keys) > 8 || len(s.Profiles) < 1 || len(s.Profiles) > 64 {
		return nil, ErrInvalid
	}
	if !identifier.MatchString(s.StateKey.ID) || !identifier.MatchString(s.TokenKey.ID) || s.StateKey.ID == s.TokenKey.ID || !envName.MatchString(s.StateKey.FromEnv) || !envName.MatchString(s.TokenKey.FromEnv) || s.StateKey.FromEnv == s.TokenKey.FromEnv || s.StateKey.FromEnv == s.ClientSecret.FromEnv || s.TokenKey.FromEnv == s.ClientSecret.FromEnv {
		return nil, ErrInvalid
	}
	if s.Retention.BatchSize < 1 || s.Retention.BatchSize > 1000 || s.Retention.IntervalMillis < 1000 || s.Retention.IntervalMillis > 60000 {
		return nil, ErrInvalid
	}
	n, ok := new(big.Int).SetString(s.Network.ChainID, 10)
	if !ok || n.Sign() <= 0 || n.BitLen() > 256 || n.String() != s.Network.ChainID || !hashPattern.MatchString(s.Network.GenesisHash) || s.Network.GenesisHash == "0x"+strings.Repeat("0", 64) {
		return nil, ErrInvalid
	}
	out := &Source{spec: s, digest: expected, host: app.Host, keys: map[string]ed25519.PublicKey{}, profiles: map[string]string{}}
	for _, k := range s.Keys {
		b, e := base64.RawURLEncoding.Strict().DecodeString(k.X)
		if !visible(k.ID, 128) || e != nil || len(b) != 32 || base64.RawURLEncoding.EncodeToString(b) != k.X || out.keys[k.ID] != nil || allZero(b) {
			return nil, ErrInvalid
		}
		out.keys[k.ID] = append(ed25519.PublicKey{}, b...)
	}
	for _, p := range s.Profiles {
		if !namePattern.MatchString(p.ID) || !digestPattern.MatchString(p.Digest) || out.profiles[p.ID] != "" {
			return nil, ErrInvalid
		}
		out.profiles[p.ID] = p.Digest
	}
	setup := Setup{"ops-v2", "approved-config", "0x" + n.Text(16), n.String(), s.Network.GenesisHash, s.OPSOrigin}
	out.setup, e = json.Marshal(setup)
	if e != nil || len(out.setup)+1 > 2048 {
		return nil, ErrInvalid
	}
	out.setup = append(out.setup, '\n')
	return out, nil
}
func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

type StorageMetadata struct {
	ConfigDigest  [32]byte
	RecoveryEpoch string
	Limits        Limits
}

func (s *Source) StorageMetadata() StorageMetadata {
	if s == nil {
		return StorageMetadata{}
	}
	return StorageMetadata{s.digest, s.spec.RecoveryEpoch, s.spec.Limits}
}
func (s *Source) SetupJSON() []byte {
	if s == nil {
		return nil
	}
	return append([]byte(nil), s.setup...)
}
func ExpectedSHA(text string) ([32]byte, error) {
	var out [32]byte
	if len(text) != 64 {
		return out, ErrInvalid
	}
	b, e := hex.DecodeString(text)
	if e != nil || hex.EncodeToString(b) != text {
		return out, ErrInvalid
	}
	copy(out[:], b)
	return out, nil
}
