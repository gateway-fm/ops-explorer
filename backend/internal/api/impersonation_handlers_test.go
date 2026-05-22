package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// newImpersonationTestServer builds a minimally-wired Server with the
// impersonation feature on and an httptest privacy-proxy stub that lets the
// test drive the probe's response status.
func newImpersonationTestServer(t *testing.T, probeStatus int) (*Server, *httptest.Server) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only the admin-impersonate probe path is expected.
		if !strings.HasPrefix(r.URL.Path, "/api/v1/admin/impersonate/") {
			http.Error(w, "unexpected upstream path: "+r.URL.Path, http.StatusBadRequest)
			return
		}
		w.WriteHeader(probeStatus)
	}))
	t.Cleanup(upstream.Close)

	store := NewMemoryImpersonationStoreNoGC()
	t.Cleanup(store.Stop)

	s := &Server{
		impersonations:    store,
		privacyProxyURL:   upstream.URL,
		impersonationHTTP: upstream.Client(),
		router:            chi.NewRouter(),
	}
	s.router.Route("/api/impersonation", func(r chi.Router) {
		r.Post("/start", s.handleStartImpersonation)
		r.Delete("/{token}", s.handleStopImpersonation)
		r.Get("/{token}", s.handleGetImpersonation)
	})
	return s, upstream
}

// addAuthCookie attaches a forged but parseable JWT cookie to the request so
// GetAuthDID / GetAuthToken on the Server can read the admin DID. The
// fakeJWT/signature is fine because ExtractClaims does not verify signatures.
func addAuthCookie(req *http.Request, sub string) {
	exp := time.Now().Add(1 * time.Hour)
	jwt := makeTestJWT(sub, exp)
	req.AddCookie(&http.Cookie{Name: AuthCookieName, Value: jwt})
}

func TestHandleStartImpersonation_OK(t *testing.T) {
	s, _ := newImpersonationTestServer(t, http.StatusOK)

	body := strings.NewReader(`{"target_did":"did:p:target"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/impersonation/start", body)
	addAuthCookie(req, "did:p:admin")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp startImpersonationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected token in response")
	}
	if resp.TargetDID != "did:p:target" {
		t.Fatalf("unexpected target_did echo: %s", resp.TargetDID)
	}

	// The store should now hold a session for this token bound to the admin.
	session, err := s.impersonations.Lookup(context.Background(), resp.Token)
	if err != nil {
		t.Fatalf("Lookup after mint: %v", err)
	}
	if session.AdminDID != "did:p:admin" || session.TargetDID != "did:p:target" {
		t.Fatalf("unexpected session: %+v", session)
	}
}

func TestHandleStartImpersonation_RejectsSelf(t *testing.T) {
	s, _ := newImpersonationTestServer(t, http.StatusOK)

	body := strings.NewReader(`{"target_did":"did:p:admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/impersonation/start", body)
	addAuthCookie(req, "did:p:admin")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for self-impersonation, got %d", w.Code)
	}
}

func TestHandleStartImpersonation_NoAuth(t *testing.T) {
	s, _ := newImpersonationTestServer(t, http.StatusOK)
	body := strings.NewReader(`{"target_did":"did:p:target"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/impersonation/start", body)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth cookie, got %d", w.Code)
	}
}

func TestHandleStartImpersonation_PropagatesProxyStatus(t *testing.T) {
	cases := []struct {
		probe int
		want  int
	}{
		{http.StatusForbidden, http.StatusForbidden},
		{http.StatusUnauthorized, http.StatusUnauthorized},
		{http.StatusNotFound, http.StatusNotFound},
		{http.StatusInternalServerError, http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.probe), func(t *testing.T) {
			s, _ := newImpersonationTestServer(t, tc.probe)
			body := strings.NewReader(`{"target_did":"did:p:target"}`)
			req := httptest.NewRequest(http.MethodPost, "/api/impersonation/start", body)
			addAuthCookie(req, "did:p:admin")
			w := httptest.NewRecorder()
			s.router.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("probe=%d want=%d got=%d body=%s",
					tc.probe, tc.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandleStartImpersonation_BadBody(t *testing.T) {
	s, _ := newImpersonationTestServer(t, http.StatusOK)
	req := httptest.NewRequest(http.MethodPost, "/api/impersonation/start", bytes.NewBufferString("{not json}"))
	addAuthCookie(req, "did:p:admin")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad body, got %d", w.Code)
	}
}

func TestHandleStopImpersonation_Idempotent(t *testing.T) {
	s, _ := newImpersonationTestServer(t, http.StatusOK)

	// Stop unknown token → 204.
	req := httptest.NewRequest(http.MethodDelete, "/api/impersonation/does-not-exist", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for unknown token, got %d", w.Code)
	}

	// Mint then stop → 204, and lookup returns missing.
	tok, _, err := s.impersonations.Mint(context.Background(), ImpersonationSession{
		AdminDID: "a", TargetDID: "b",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	req = httptest.NewRequest(http.MethodDelete, "/api/impersonation/"+tok, nil)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 after revoke, got %d", w.Code)
	}
	if _, err := s.impersonations.Lookup(context.Background(), tok); err != ErrImpersonationNotFound {
		t.Fatalf("expected token gone after delete, got %v", err)
	}
}

func TestHandleGetImpersonation_OK(t *testing.T) {
	s, _ := newImpersonationTestServer(t, http.StatusOK)
	tok, _, err := s.impersonations.Mint(context.Background(), ImpersonationSession{
		AdminDID:  "did:p:admin",
		TargetDID: "did:p:target",
	}, time.Hour)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/impersonation/"+tok, nil)
	addAuthCookie(req, "did:p:admin")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp startImpersonationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TargetDID != "did:p:target" {
		t.Fatalf("unexpected target: %s", resp.TargetDID)
	}
}

func TestHandleGetImpersonation_WrongAdmin(t *testing.T) {
	s, _ := newImpersonationTestServer(t, http.StatusOK)
	tok, _, err := s.impersonations.Mint(context.Background(), ImpersonationSession{
		AdminDID:  "did:p:alice",
		TargetDID: "did:p:target",
	}, time.Hour)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/impersonation/"+tok, nil)
	addAuthCookie(req, "did:p:mallory")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	// 404 (not 403): we never leak that the token exists.
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-admin token, got %d", w.Code)
	}
}

func TestHandleGetImpersonation_NoAuth(t *testing.T) {
	s, _ := newImpersonationTestServer(t, http.StatusOK)
	req := httptest.NewRequest(http.MethodGet, "/api/impersonation/some-token", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", w.Code)
	}
}

func TestApplyImpersonationPath_NoSession(t *testing.T) {
	if got := applyImpersonationPath(context.Background(), "/api/v1/explorer/blocks"); got != "/api/v1/explorer/blocks" {
		t.Fatalf("expected pass-through, got %s", got)
	}
}

func TestApplyImpersonationPath_RewriteAndEscape(t *testing.T) {
	ctx := WithImpersonation(context.Background(), ImpersonationSession{
		AdminDID:  "did:p:admin",
		TargetDID: "did:p:target",
	})
	got := applyImpersonationPath(ctx, "/api/v1/explorer/blocks?limit=10")
	want := "/api/v1/admin/impersonate/did:p:target/api/v1/explorer/blocks?limit=10"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	// DID with a slash → escaped.
	ctx = WithImpersonation(context.Background(), ImpersonationSession{
		AdminDID:  "did:p:admin",
		TargetDID: "did:method:has/slash",
	})
	got = applyImpersonationPath(ctx, "/api/v1/explorer/blocks")
	if !strings.Contains(got, "did:method:has%2Fslash") {
		t.Fatalf("expected slash to be escaped, got %s", got)
	}
}
