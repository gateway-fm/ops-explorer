package authv2

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	uuidlib "github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaximumStateCookieAndSkewDate(t *testing.T) {
	s, key, b := testSource(t, "https://app.test", "https://ops.test")
	c, e := newCodec(s, b)
	if e != nil {
		t.Fatal(e)
	}
	state, _ := randomToken()
	verifier, _ := randomToken()
	now := MaxTime - 330
	p := pendingCapture{sha256.Sum256([]byte("browser")), uuidlib.NewString(), handleHash("oauth-state", state), now, now + 300}
	payload := statePayload{1, p.revision, state, verifier, "/?x=" + strings.Repeat(`"`, 1020), "renew", testClaims(s, now).Subject, strings.Repeat("a", 128), now, now + 300}
	if len(payload.ReturnPath) != 1024 || !returnPath(payload.ReturnPath) {
		t.Fatal("maximum approved relative return path invalid")
	}
	frame, e := c.sealState(p, payload)
	if e != nil {
		t.Fatal("maximum state", e)
	}
	w := httptest.NewRecorder()
	setCookie(w, oauthCookie, frame, p.expires)
	if len(w.Header().Get("Set-Cookie")) > 4096 {
		t.Fatal("maximum OAuth cookie exceeds browser bound")
	}
	if _, e = c.openState(p, frame, now); e != nil {
		t.Fatal(e)
	}
	tokenClaims := testClaims(s, now+30)
	raw := sign(t, key, tokenClaims)
	if _, e = s.verify(raw, now); e != nil {
		t.Fatal("maximum permitted original +30s skew", e)
	}
	tokenClaims.Issued++
	if _, e = s.verify(sign(t, key, tokenClaims), now); e == nil {
		t.Fatal("+31s skew accepted")
	}
	w = httptest.NewRecorder()
	setCookie(w, browserCookie, state, min(now+7*86400, MaxTime))
	for _, cookie := range w.Result().Cookies() {
		if cookie.Expires.Year() != 9999 || cookie.Expires.Unix() != MaxTime {
			t.Fatal("browser maxdate clamp")
		}
	}
	for _, field := range []string{"iat", "exp", "claims_as_of"} {
		var v map[string]any
		encoded, _ := json.Marshal(testClaims(s, 1700000000))
		_ = json.Unmarshal(encoded, &v)
		v[field] = 1.5
		data, _ := json.Marshal(v)
		var parsed claims
		if decode(data, 12<<10, &parsed) == nil {
			t.Fatal("fractional timestamp accepted", field)
		}
	}
}
func TestSourceNumericAliasesAndActualSetupCap(t *testing.T) {
	s, _, _ := testSource(t, "https://app.test", "https://ops.test")
	callback:=s.spec;callback.CallbackURI=callback.AppOrigin+"/prefix/api/auth/callback";encoded,_:=json.Marshal(callback);if _,e:=ParseSource(encoded,sha256.Sum256(encoded));e==nil{t.Fatal("callback path not mounted by selected V2 was approved")}
	for _, host := range []string{"127.1", "2130706433", "0x7f000001", "0177.0.0.1", "example.123", "[ops.test]"} {
		v := s.spec
		v.OPSOrigin = "https://" + host
		raw, _ := json.Marshal(v)
		if _, e := ParseSource(raw, sha256.Sum256(raw)); e == nil {
			t.Fatal("browser-different origin accepted", host)
		}
	}
	v := s.spec
	v.Network.ChainID = "115792089237316195423570985008687907853269984665640564039457584007913129639935"
	v.OPSOrigin = "https://" + strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61)
	raw, _ := json.Marshal(v)
	max, e := ParseSource(raw, sha256.Sum256(raw))
	if e != nil || len(max.SetupJSON()) > 2048 {
		t.Fatal("maximum legal approved setup", e)
	}
}
func TestRetentionDoesNotSlideOnReadAndChargesExact(t *testing.T) {
	s, owner, clock, key := fixture(t)
	browser, auth, selected := install(t, s, key, clock.Load())
	for i := 0; i < 3; i++ {
		if _, e := s.session(context.Background(), browser, auth); e != nil {
			t.Fatal(e)
		}
	}
	var changed, retain int64
	if e := owner.QueryRow(context.Background(), `SELECT last_change,retain_until FROM explorer_sso_v2.contexts`).Scan(&changed, &retain); e != nil || changed != clock.Load() || retain != selected.capture.expires {
		t.Fatal("reads extended retained authority")
	}
	clock.Store(retain)
	if e := s.store.retain(context.Background()); e != nil {
		t.Fatal(e)
	}
	var count int
	var charged int64
	if e := owner.QueryRow(context.Background(), `SELECT context_count,cipher_bytes FROM explorer_sso_v2.store_state`).Scan(&count, &charged); e != nil || count != 0 || charged != 0 {
		t.Fatal("janitor charge differs from deleted exact rows")
	}
	if _, e := s.session(context.Background(), browser, auth); e != ErrUnauthenticated {
		t.Fatal("expired retained handle revived")
	}
}

func TestBodyAndSessionDuplicatesClosed(t *testing.T) {
	s, _, _, _ := fixture(t)
	for _, raw := range []string{`{"mode":"login","mode":"renew","returnPath":"/"}`, `{"mode":"login","returnPath":"/","extra":1}`, `null`, `{"mode":"login","returnPath":"//evil.test/"}`} {
		out := authCall(s, req("POST", "/api/auth/login", strings.NewReader(raw), nil))
		if out.Code != 400 || len(out.Result().Cookies()) != 0 {
			t.Fatal("malformed login emitted output", out.Code)
		}
	}
	r := req("GET", "/api/auth/status", nil, []*http.Cookie{{Name: browserCookie, Value: strings.Repeat("A", 43)}, {Name: browserCookie, Value: strings.Repeat("A", 43)}})
	if authCall(s, r).Code != 400 {
		t.Fatal("duplicate browser cookie accepted")
	}
	r = req("GET", "/api/auth/status", nil, nil)
	r.Header.Add(SessionHeader, uuidlib.NewString())
	r.Header.Add(SessionHeader, uuidlib.NewString())
	if authCall(s, r).Code != 400 {
		t.Fatal("duplicate app session accepted")
	}
}
