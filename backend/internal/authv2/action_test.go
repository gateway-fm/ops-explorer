package authv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActionCheckUsesOriginalCaptureAndKnownReadCommit(t *testing.T) {
	s, _, clock, key := fixture(t)
	browser, auth, session := install(t, s, key, clock.Load())
	if e := s.CheckCurrent(context.Background()); e != ErrUnauthenticated {
		t.Fatal("unscoped caller accepted", e)
	}
	ctx := context.WithValue(context.Background(), sessionKey{}, session)
	if e := s.CheckCurrent(ctx); e != nil {
		t.Fatal("valid original capture", e)
	}
	if e := s.store.logout(ctx, session.capture); e != nil {
		t.Fatal(e)
	}
	if e := s.CheckCurrent(ctx); e != ErrChanged {
		t.Fatal("revoked original capture passed action gate", e)
	}
	// No action may start merely because the original request carried cookies.
	r := req("POST", "/api/eth/link/challenge", strings.NewReader(`{}`), []*http.Cookie{{Name: browserCookie, Value: browser}, {Name: sessionCookie, Value: auth}})
	r.Header.Set(SessionHeader, session.capture.activation)
	actions := 0
	w := httptest.NewRecorder()
	s.Boundary(s.Protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.CheckCurrent(r.Context()) == nil {
			actions++
		}
		w.WriteHeader(200)
	})), nil).ServeHTTP(w, r)
	if actions != 0 || w.Code != 401 {
		t.Fatal("revoked request forwarded", actions, w.Code)
	}
}

func TestActionUnknownReadCommitNeverStartsEffect(t *testing.T) {
	s, _, clock, key := fixture(t)
	_, _, session := install(t, s, key, clock.Load())
	relay := relayStore(t, s)
	relay.remaining.Store(1)
	ctx := context.WithValue(context.Background(), sessionKey{}, session)
	actions := 0
	if e := s.CheckCurrent(ctx); e == nil {
		actions++
	} else if e != ErrUnknownCommit {
		t.Fatal("unexpected unknown read category", e)
	}
	if actions != 0 {
		t.Fatal("unknown read commit started action")
	}
}
