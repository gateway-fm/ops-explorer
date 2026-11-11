package authv2

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"
)

func TestResealedSameOriginalTokenInvalidatesCapturedResponse(t *testing.T) {
	s, owner, clock, key := fixture(t)
	_, _, original := install(t, s, key, clock.Load())
	replacement := original.capture
	var e error
	replacement.cipher, e = s.codec.sealToken(replacement, original.raw)
	if e != nil || bytes.Equal(replacement.cipher, original.capture.cipher) {
		t.Fatal("independent same-token encryption control failed")
	}
	if raw, _, e := s.codec.openToken(replacement, clock.Load()); e != nil || raw != original.raw {
		t.Fatal("replacement did not preserve the original verified identity")
	}
	if _, e = owner.Exec(context.Background(), `UPDATE explorer_sso_v2.contexts SET token_cipher=$1 WHERE browser_hash=$2`, replacement.cipher, replacement.browser[:]); e != nil {
		t.Fatal(e)
	}
	if e = s.recheck(context.Background(), original); e != ErrChanged {
		t.Fatal("final gate compared claimed identity/hash instead of actual captured cipher bytes", e)
	}
}

func TestKeyMaterialAndEnvironmentReferencesSeparated(t *testing.T) {
	s, _, valid := testSource(t, "https://app.test", "https://ops.test")
	for _, which := range []string{"zero", "same-aes", "client-reuse", "public-reuse", "missing", "extra"} {
		t.Run(which, func(t *testing.T) {
			b := make([]SecretBinding, len(valid))
			for i, v := range valid {
				b[i] = SecretBinding{v.FromEnv, append([]byte(nil), v.Value...)}
			}
			switch which {
			case "zero":
				b[1].Value = make([]byte, 32)
			case "same-aes":
				b[2].Value = b[1].Value
			case "client-reuse":
				b[1].Value = b[0].Value
			case "public-reuse":
				for _, pub := range s.keys {
					b[1].Value = pub
					break
				}
			case "missing":
				b = b[:2]
			case "extra":
				b = append(b, SecretBinding{"UNSELECTED", make([]byte, 32)})
			}
			if _, e := newCodec(s, b); e == nil {
				t.Fatal("cross-role or malformed binding accepted")
			}
		})
	}
	for _, v := range valid {
		value := string(v.Value)
		if v.FromEnv != s.spec.ClientSecret.FromEnv {
			value = base64.RawURLEncoding.EncodeToString(v.Value)
		}
		t.Setenv(v.FromEnv, value)
	}
	resolved, e := EnvironmentBindings(s)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = newCodec(s, resolved); e != nil {
		t.Fatal(e)
	}
	t.Setenv(s.spec.TokenKey.FromEnv, "not canonical base64")
	if _, e = EnvironmentBindings(s); e == nil {
		t.Fatal("malformed key environment was accepted")
	}
}
