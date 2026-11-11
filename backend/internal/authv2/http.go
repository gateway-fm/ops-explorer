package authv2

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	uuidlib "github.com/google/uuid"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const browserCookie = "__Host-explorer-browser"
const oauthCookie = "__Host-explorer-oauth"
const sessionCookie = "__Host-explorer-session"
const SessionHeader = "X-OPS-App-Session"

func cookie(r *http.Request, name string, max int) (string, error) {
	if len(r.Header.Values("Cookie")) > 8 {
		return "", ErrInvalid
	}
	total := 0
	for _, v := range r.Header.Values("Cookie") {
		total += len(v)
	}
	if total > 8192 {
		return "", ErrInvalid
	}
	value := ""
	found := false
	for _, line := range r.Header.Values("Cookie") {
		for _, part := range strings.Split(line, ";") {
			part = strings.TrimSpace(part)
			key, raw, hasValue := strings.Cut(part, "=")
			if strings.TrimSpace(key) != name {
				continue
			}
			if found || key != name || !hasValue || raw == "" || len(raw) > max || raw != strings.TrimSpace(raw) {
				return "", ErrInvalid
			}
			for _, c := range []byte(raw) {
				if c < 0x21 || c > 0x7e || strings.ContainsRune("\"\\,", rune(c)) {
					return "", ErrInvalid
				}
			}
			found = true
			value = raw
		}
	}
	return value, nil
}
func sessionHeader(r *http.Request, required bool) (string, error) {
	v := r.Header.Values(SessionHeader)
	if len(v) == 0 && !required {
		return "", nil
	}
	if len(v) != 1 || !uuid(v[0]) {
		return "", ErrInvalid
	}
	return v[0], nil
}
func setCookie(w http.ResponseWriter, name, value string, expires int64) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: time.Unix(expires, 0).UTC()})
}
func safeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) {
	status, code := 503, "temporarily_unavailable"
	switch {
	case errors.Is(e, ErrInvalid):
		status, code = 400, "invalid_request"
	case errors.Is(e, ErrUnauthenticated):
		status, code = 401, "authentication_required"
	case errors.Is(e, ErrChanged):
		status, code = 409, "session_changed"
	}
	safeJSON(w, status, struct {
		Error string `json:"error"`
	}{code})
}
func body(r *http.Request, max int) ([]byte, error) {
	types := r.Header.Values("Content-Type")
	if len(types) != 1 || len(types[0]) > 128 || len(r.Header.Values("Content-Encoding")) != 0 || r.Body == nil || r.ContentLength > int64(max) {
		return nil, ErrInvalid
	}
	media, params, e := mime.ParseMediaType(types[0])
	if e != nil || media != "application/json" || len(params) > 1 {
		return nil, ErrInvalid
	}
	for k, v := range params {
		if k != "charset" || !strings.EqualFold(v, "utf-8") {
			return nil, ErrInvalid
		}
	}
	raw, e := io.ReadAll(io.LimitReader(r.Body, int64(max)+1))
	if e != nil || len(raw) > max {
		return nil, ErrInvalid
	}
	return raw, nil
}
func (s *Service) Auth(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" && r.URL.Path != "/api/auth/callback" {
		fail(w, ErrInvalid)
		return
	}
	switch r.URL.Path {
	case "/api/auth/login":
		if r.Method == "POST" {
			s.login(w, r)
			return
		}
	case "/api/auth/callback":
		if r.Method == "GET" {
			s.callback(w, r)
			return
		}
	case "/api/auth/status":
		if r.Method == "GET" {
			s.status(w, r)
			return
		}
	case "/api/auth/logout":
		if r.Method == "POST" {
			s.logout(w, r)
			return
		}
	}
	safeJSON(w, 405, map[string]string{"error": "method_not_allowed"})
}

type loginRequest struct {
	Mode            string `json:"mode"`
	ReturnPath      string `json:"returnPath"`
	ExpectedActor   string `json:"expectedActorId,omitempty"`
	ExpectedProfile string `json:"expectedProfile,omitempty"`
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	raw, e := body(r, 4096)
	if e != nil {
		fail(w, e)
		return
	}
	var v loginRequest
	if decode(raw, 4096, &v, "mode", "returnPath") != nil || !returnPath(v.ReturnPath) || v.Mode != "login" && v.Mode != "renew" || v.Mode == "login" && (v.ExpectedActor != "" || v.ExpectedProfile != "") || v.Mode == "renew" && (!actor(v.ExpectedActor) || s.source.profiles[v.ExpectedProfile] == "") {
		fail(w, ErrInvalid)
		return
	}
	browser, e := cookie(r, browserCookie, 43)
	if e != nil {
		fail(w, e)
		return
	}
	fresh := browser == ""
	if fresh {
		browser, e = randomToken()
	} else if !token32(browser) {
		e = ErrInvalid
	}
	if e != nil {
		fail(w, e)
		return
	}
	expected, e := sessionHeader(r, false)
	if e != nil {
		fail(w, e)
		return
	}
	auth, e := cookie(r, sessionCookie, 43)
	if e != nil {
		fail(w, e)
		return
	}
	var guard *activeCapture
	if v.Mode == "renew" && auth != "" {
		session, e := s.session(r.Context(), browser, auth)
		if e == nil {
			if expected == "" || expected != session.capture.activation || v.ExpectedActor != session.claims.Subject || v.ExpectedProfile != session.claims.Profile {
				fail(w, ErrChanged)
				return
			}
			guard = &session.capture
		} else if !errors.Is(e, ErrUnauthenticated) {
			fail(w, e)
			return
		}
	}
	now, e := s.store.now()
	if e != nil {
		fail(w, ErrUnavailable)
		return
	}
	state, e := randomToken()
	if e != nil {
		fail(w, e)
		return
	}
	verifier, e := randomToken()
	if e != nil {
		fail(w, e)
		return
	}
	p := pendingCapture{browser: handleHash("browser", browser), revision: uuidlib.NewString(), hash: handleHash("oauth-state", state), issued: now, expires: now + 300}
	payload := statePayload{1, p.revision, state, verifier, v.ReturnPath, v.Mode, v.ExpectedActor, v.ExpectedProfile, p.issued, p.expires}
	frame, e := s.codec.sealState(p, payload)
	if e != nil {
		fail(w, e)
		return
	}
	challenge := sha256.Sum256([]byte(verifier))
	query := url.Values{"response_type": {"code"}, "client_id": {s.source.spec.ClientID}, "redirect_uri": {s.source.spec.CallbackURI}, "state": {state}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"}}
	if e = s.store.startGuarded(r.Context(), p, guard, v.Mode == "renew"); e != nil {
		fail(w, e)
		return
	}
	if fresh {
		setCookie(w, browserCookie, browser, min(now+7*86400, MaxTime))
	}
	setCookie(w, oauthCookie, frame, p.expires)
	safeJSON(w, 200, struct {
		AuthorizationURL string `json:"authorizationUrl"`
	}{s.source.spec.OPSOrigin + "/api/v2/sso/authorize?" + query.Encode()})
}
func (s *Service) callback(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.RawQuery) > 256 || strings.Contains(r.URL.RawQuery, ";") {
		fail(w, ErrInvalid)
		return
	}
	q, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil || len(q) != 2 || len(q["code"]) != 1 || len(q["state"]) != 1 || !token32(q.Get("code")) || !token32(q.Get("state")) {
		fail(w, ErrInvalid)
		return
	}
	browser, e := cookie(r, browserCookie, 43)
	if e != nil || !token32(browser) {
		fail(w, ErrUnauthenticated)
		return
	}
	frame, e := cookie(r, oauthCookie, 3596)
	if e != nil || frame == "" {
		fail(w, ErrUnauthenticated)
		return
	}
	row, e := s.store.read(r.Context(), handleHash("browser", browser))
	if e != nil {
		fail(w, e)
		return
	}
	p, ok := row.pending()
	if !ok {
		fail(w, ErrUnauthenticated)
		return
	}
	payload, e := s.codec.openState(p, frame, s.clock())
	if e != nil || !equalToken(payload.State, q.Get("state")) {
		fail(w, ErrUnauthenticated)
		return
	}
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {s.source.spec.ClientID}, "code": {q.Get("code")}, "redirect_uri": {s.source.spec.CallbackURI}, "code_verifier": {payload.Verifier}}
	req, e := http.NewRequestWithContext(r.Context(), "POST", s.source.spec.OPSOrigin+"/api/v2/sso/token", strings.NewReader(form.Encode()))
	if e != nil {
		fail(w, ErrUnavailable)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(s.source.spec.ClientID), url.QueryEscape(s.codec.clientSecret))
	resp, e := s.client.Do(req)
	if e != nil {
		fail(w, ErrUnavailable)
		return
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(resp.Body, (24<<10)+1))
	if e != nil || len(raw) > 24<<10 || resp.StatusCode != 200 {
		fail(w, ErrUnauthenticated)
		return
	}
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if decode(raw, 24<<10, &token, "access_token", "token_type", "expires_in") != nil || token.TokenType != "Bearer" || token.ExpiresIn < 1 || token.ExpiresIn > 330 {
		fail(w, ErrUnauthenticated)
		return
	}
	now, e := s.store.now()
	if e != nil {
		fail(w, e)
		return
	}
	claims, e := s.source.verify(token.AccessToken, now)
	if e != nil {
		fail(w, e)
		return
	}
	if payload.Mode == "renew" && (claims.Subject != payload.ExpectedActor || claims.Profile != payload.ExpectedProfile) {
		fail(w, ErrChanged)
		return
	}
	auth, e := randomToken()
	if e != nil {
		fail(w, e)
		return
	}
	a := activeCapture{browser: p.browser, auth: handleHash("auth", auth), activation: uuidlib.NewString(), activated: now, expires: claims.Expires, key: s.source.spec.TokenKey.ID}
	a.cipher, e = s.codec.sealToken(a, token.AccessToken)
	if e != nil {
		fail(w, e)
		return
	}
	if e = s.store.activate(r.Context(), p, a); e != nil {
		fail(w, e)
		return
	}
	if _, e = s.source.verify(token.AccessToken, s.clock()); e != nil || r.Context().Err() != nil {
		fail(w, ErrUnauthenticated)
		return
	}
	setCookie(w, sessionCookie, auth, a.expires)
	w.Header().Set("Location", payload.ReturnPath)
	w.WriteHeader(303)
	// Never clear OAuth state here: a late callback must not erase a newer flow.
}
func (s *Service) status(w http.ResponseWriter, r *http.Request) {
	expected, e := sessionHeader(r, false)
	if e != nil {
		fail(w, e)
		return
	}
	browser, e := cookie(r, browserCookie, 43)
	if e != nil {
		fail(w, e)
		return
	}
	auth, e := cookie(r, sessionCookie, 43)
	if e != nil {
		fail(w, e)
		return
	}
	session, e := s.session(r.Context(), browser, auth)
	if errors.Is(e, ErrUnauthenticated) {
		safeJSON(w, 200, map[string]any{"protocol": "ops-v2", "authenticated": false})
		return
	}
	if e != nil {
		fail(w, e)
		return
	}
	if expected != "" && expected != session.capture.activation {
		fail(w, ErrChanged)
		return
	}
	if e = s.recheck(r.Context(), session); e != nil {
		fail(w, e)
		return
	}
	safeJSON(w, 200, struct {
		Protocol      string `json:"protocol"`
		Authenticated bool   `json:"authenticated"`
		DID           string `json:"did"`
		ExpiresAt     int64  `json:"expires_at"`
		SessionID     string `json:"session_id"`
		Profile       string `json:"source_profile"`
	}{"ops-v2", true, session.claims.Subject, session.claims.Expires, session.capture.activation, session.claims.Profile})
}
func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	raw, e := body(r, 128)
	if e != nil {
		fail(w, e)
		return
	}
	var empty struct{}
	if decode(raw, 128, &empty) != nil {
		fail(w, ErrInvalid)
		return
	}
	expected, e := sessionHeader(r, true)
	if e != nil {
		fail(w, e)
		return
	}
	browser, e := cookie(r, browserCookie, 43)
	if e != nil {
		fail(w, e)
		return
	}
	auth, e := cookie(r, sessionCookie, 43)
	if e != nil {
		fail(w, e)
		return
	}
	session, e := s.session(r.Context(), browser, auth)
	if e != nil {
		fail(w, e)
		return
	}
	if expected != session.capture.activation {
		fail(w, ErrChanged)
		return
	}
	if e = s.store.logout(r.Context(), session.capture); e != nil {
		fail(w, e)
		return
	}
	// The durable capture is revoked. Do not delete the shared browser cookie:
	// this response can arrive after a newer callback has installed its handle.
	safeJSON(w, 200, map[string]bool{"logged_out": true})
}

func (s *Service) Protected(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected, e := sessionHeader(r, true)
		if e != nil {
			fail(w, e)
			return
		}
		browser, e := cookie(r, browserCookie, 43)
		if e != nil {
			fail(w, e)
			return
		}
		auth, e := cookie(r, sessionCookie, 43)
		if e != nil {
			fail(w, e)
			return
		}
		session, e := s.session(r.Context(), browser, auth)
		if e != nil {
			fail(w, e)
			return
		}
		if expected != session.capture.activation {
			fail(w, ErrChanged)
			return
		}
		ctx := context.WithValue(r.Context(), sessionKey{}, session)
		r = r.WithContext(ctx)
		gate := newPrivateWriter(w, r, s, session)
		// No deferred finalization: panic unwinds without releasing staged headers.
		next.ServeHTTP(gate, r)
		gate.finish()
	})
}
