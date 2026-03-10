package api

import (
	"explorer/internal/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleAuthLogin_RedirectsToSSO(t *testing.T) {
	ssoClient := auth.NewSSOClient("https://proxy.example.com", "test-client", "https://explorer.example.com/api/auth/callback")

	s := &Server{
		ssoClient: ssoClient,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login?return_url=/dashboard", nil)
	w := httptest.NewRecorder()

	s.handleAuthLogin(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header")
	}

	// Should redirect to privacy-proxy
	if !strings.Contains(location, "proxy.example.com/oauth/authorize") {
		t.Fatalf("expected redirect to proxy, got %s", location)
	}
	if !strings.Contains(location, "response_mode=redirect") {
		t.Fatalf("expected response_mode=redirect in URL, got %s", location)
	}
}

func TestHandleAuthLogin_NoSSO(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	w := httptest.NewRecorder()

	s.handleAuthLogin(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHandleAuthLogin_DefaultReturnURL(t *testing.T) {
	ssoClient := auth.NewSSOClient("https://proxy.example.com", "test-client", "https://explorer.example.com/api/auth/callback")

	s := &Server{
		ssoClient: ssoClient,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	w := httptest.NewRecorder()

	s.handleAuthLogin(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
}
