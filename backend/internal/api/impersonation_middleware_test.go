package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newMiddlewareTestServer(t *testing.T) (*Server, *memoryImpersonationStore) {
	t.Helper()
	store := NewMemoryImpersonationStoreNoGC()
	t.Cleanup(store.Stop)
	return &Server{impersonations: store}, store
}

// nextSawSession captures whether the inner handler observed an
// impersonation session on its request context.
type nextSawSession struct {
	session ImpersonationSession
	saw     bool
	headers http.Header
}

func (n *nextSawSession) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := ImpersonationFromContext(r.Context())
		n.session = sess
		n.saw = ok
		n.headers = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	})
}

func TestImpersonationMiddleware_NoHeader(t *testing.T) {
	s, _ := newMiddlewareTestServer(t)
	captured := &nextSawSession{}
	h := s.impersonationMiddleware(captured.handler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/blocks", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if captured.saw {
		t.Fatal("expected no session attached")
	}
}

func TestImpersonationMiddleware_InvalidToken(t *testing.T) {
	s, _ := newMiddlewareTestServer(t)
	captured := &nextSawSession{}
	h := s.impersonationMiddleware(captured.handler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/blocks", nil)
	req.Header.Set(ImpersonateHeader, "garbage")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestImpersonationMiddleware_BoundAdminMismatch(t *testing.T) {
	s, store := newMiddlewareTestServer(t)
	captured := &nextSawSession{}
	h := s.impersonationMiddleware(captured.handler())

	tok, _, err := store.Mint(context.Background(), ImpersonationSession{
		AdminDID: "did:p:alice", TargetDID: "did:p:target", OrgID: "org-7",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/blocks", nil)
	req.Header.Set(ImpersonateHeader, tok)
	// Auth cookie carries a DIFFERENT admin DID — must reject.
	addAuthCookie(req, "did:p:mallory")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on admin mismatch, got %d", w.Code)
	}
	if captured.saw {
		t.Fatal("inner handler should not have run")
	}
}

func TestImpersonationMiddleware_StripsHeaderAndAttachesSession(t *testing.T) {
	s, store := newMiddlewareTestServer(t)
	captured := &nextSawSession{}
	h := s.impersonationMiddleware(captured.handler())

	tok, _, err := store.Mint(context.Background(), ImpersonationSession{
		AdminDID: "did:p:alice", TargetDID: "did:p:target", OrgID: "org-7",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/blocks", nil)
	req.Header.Set(ImpersonateHeader, tok)
	addAuthCookie(req, "did:p:alice")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !captured.saw {
		t.Fatal("expected impersonation session on context")
	}
	if captured.session.TargetDID != "did:p:target" {
		t.Fatalf("unexpected target: %s", captured.session.TargetDID)
	}
	// Header must be stripped before reaching the inner handler.
	if got := captured.headers.Get(ImpersonateHeader); got != "" {
		t.Fatalf("expected header to be stripped, got %q", got)
	}
}

func TestImpersonationMiddleware_RejectsWriteMethods(t *testing.T) {
	s, store := newMiddlewareTestServer(t)
	captured := &nextSawSession{}
	h := s.impersonationMiddleware(captured.handler())

	tok, _, err := store.Mint(context.Background(), ImpersonationSession{
		AdminDID: "did:p:alice", TargetDID: "did:p:target", OrgID: "org-7",
	}, time.Minute)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/blocks", strings.NewReader(""))
			req.Header.Set(ImpersonateHeader, tok)
			addAuthCookie(req, "did:p:alice")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s: expected 405 in view-as mode, got %d", method, w.Code)
			}
		})
	}
}

func TestImpersonationMiddleware_FeatureDisabled(t *testing.T) {
	// No store on Server → middleware silently passes through.
	s := &Server{}
	captured := &nextSawSession{}
	h := s.impersonationMiddleware(captured.handler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/blocks", nil)
	req.Header.Set(ImpersonateHeader, "anything")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected pass-through 200 when feature disabled, got %d", w.Code)
	}
	if captured.saw {
		t.Fatal("expected no session when feature disabled")
	}
}
