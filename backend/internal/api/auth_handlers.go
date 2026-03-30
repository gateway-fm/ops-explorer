package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"explorer/internal/auth"
)

const (
	AuthCookieName  = "explorer_auth"
	RefreshCookieName = "explorer_refresh"
	StateCookieName = "explorer_oauth_state"

	// CookieMaxAge is the browser lifetime of the access-token cookie.
	// This should stay in sync with the access token TTL on privacy-proxy
	// (AccessTokenTTL = 5 min), but we extend the cookie on every successful
	// token refresh so active sessions stay alive as a sliding window.
	CookieMaxAge = 30 * 60 // 30 minutes

	// RefreshCookieMaxAge is the lifetime of the refresh-token cookie.
	// Matches RefreshTokenTTL on privacy-proxy (7 days).
	RefreshCookieMaxAge = 7 * 24 * 60 * 60 // 7 days

	// refreshThreshold is how close to JWT expiry we trigger a token refresh.
	// Must be > AccessTokenTTL so we always refresh before the token expires.
	refreshThreshold = 5 * time.Minute
)

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if s.ssoClient == nil || !s.ssoClient.IsEnabled() {
		writeError(w, http.StatusServiceUnavailable, "SSO is not configured")
		return
	}

	returnURL := r.URL.Query().Get("return_url")
	if returnURL == "" {
		returnURL = "/"
	}

	// Prevent open redirect: return_url must be a relative path
	if !strings.HasPrefix(returnURL, "/") || strings.HasPrefix(returnURL, "//") {
		returnURL = "/"
	}

	state, err := s.ssoClient.GenerateState(returnURL)
	if err != nil {
		log.Printf("Failed to generate state: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to initiate login")
		return
	}

	authURL := s.ssoClient.GetAuthorizationURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.ssoClient == nil || !s.ssoClient.IsEnabled() {
		writeError(w, http.StatusServiceUnavailable, "SSO is not configured")
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		writeError(w, http.StatusBadRequest, "Missing authorization code")
		return
	}

	if state == "" {
		writeError(w, http.StatusBadRequest, "Missing state parameter")
		return
	}

	returnURL, valid := s.ssoClient.ValidateState(state)
	if !valid {
		writeError(w, http.StatusBadRequest, "Invalid or expired state parameter")
		return
	}

	// Prevent open redirect: return_url must be a relative path
	if !strings.HasPrefix(returnURL, "/") || strings.HasPrefix(returnURL, "//") {
		returnURL = "/"
	}

	tokenResp, err := s.ssoClient.ExchangeCode(r.Context(), code)
	if err != nil {
		log.Printf("Failed to exchange code: %v", err)
		writeError(w, http.StatusBadGateway, "Failed to exchange authorization code")
		return
	}

	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"

	http.SetCookie(w, &http.Cookie{
		Name:     AuthCookieName,
		Value:    tokenResp.AccessToken,
		Path:     "/",
		MaxAge:   CookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})

	if tokenResp.RefreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     RefreshCookieName,
			Value:    tokenResp.RefreshToken,
			Path:     "/",
			MaxAge:   RefreshCookieMaxAge,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
		})
	}

	http.Redirect(w, r, returnURL, http.StatusFound)
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	status := auth.AuthStatus{
		Authenticated: false,
	}

	cookie, err := r.Cookie(AuthCookieName)
	if err != nil || cookie.Value == "" {
		writeJSON(w, status)
		return
	}

	claims, err := auth.ExtractClaims(cookie.Value)
	if err != nil {
		clearAuthCookies(w)
		writeJSON(w, status)
		return
	}

	if claims.IsExpired() {
		clearAuthCookies(w)
		writeJSON(w, status)
		return
	}

	status.Authenticated = true
	status.DID = claims.GetDID()
	status.ExpiresAt = claims.ExpiresAt

	writeJSON(w, status)
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	// Revoke server-side so the token cannot be reused even if captured from
	// the network before the cookie was cleared.
	if s.ssoClient != nil && s.ssoClient.IsEnabled() {
		if refreshCookie, err := r.Cookie(RefreshCookieName); err == nil && refreshCookie.Value != "" {
			if err := s.ssoClient.RevokeToken(r.Context(), refreshCookie.Value); err != nil {
				log.Printf("failed to revoke refresh token on logout: %v", err)
				// Continue — local logout proceeds regardless.
			}
		}
	}

	clearAuthCookies(w)
	writeJSON(w, map[string]bool{"logged_out": true})
}

func (s *Server) GetAuthDID(r *http.Request) string {
	cookie, err := r.Cookie(AuthCookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}

	claims, err := auth.ExtractClaims(cookie.Value)
	if err != nil || claims.IsExpired() {
		return ""
	}

	return claims.GetDID()
}

func (s *Server) GetAuthToken(r *http.Request) string {
	cookie, err := r.Cookie(AuthCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// refreshAuthMiddleware transparently refreshes the access token when it is
// close to expiry. The flow:
//
//  1. If the JWT has > refreshThreshold remaining — pass through unchanged.
//  2. If the JWT has ≤ refreshThreshold remaining — call privacy-proxy /refresh
//     with the refresh token cookie to obtain a new access + refresh token pair.
//     The server rotates the refresh token on every call, so we must write back
//     both updated cookies.
//  3. If the refresh token is missing, invalid, or explicitly revoked (banned
//     user) — clear both cookies immediately. The frontend will detect the
//     missing auth cookie at the next /api/auth/status poll and show "Sign In".
//
// Security note: access tokens are intentionally short-lived (5 min). This
// creates a bounded window between when a user is banned in the admin panel and
// when they are actually locked out. The window is at most AccessTokenTTL.
// Extending the TTL would widen that window; do not increase it without
// reconsidering the ban enforcement strategy.
func (s *Server) refreshAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accessCookie, err := r.Cookie(AuthCookieName)
		if err != nil || accessCookie.Value == "" {
			next.ServeHTTP(w, r)
			return
		}

		claims, err := auth.ExtractClaims(accessCookie.Value)
		if err != nil || claims.IsExpired() {
			// Malformed or already-expired token — clear cookies so the
			// frontend knows to show the login screen.
			clearAuthCookies(w)
			next.ServeHTTP(w, r)
			return
		}

		if claims.TimeToExpiry() > refreshThreshold {
			// Token is fresh enough — nothing to do.
			next.ServeHTTP(w, r)
			return
		}

		// Token is within the refresh window. Attempt a silent refresh.
		refreshCookie, err := r.Cookie(RefreshCookieName)
		if err != nil || refreshCookie.Value == "" {
			// No refresh token — cannot renew. Let the access token expire
			// naturally; the next /api/auth/status call will clear it.
			next.ServeHTTP(w, r)
			return
		}

		if s.ssoClient == nil || !s.ssoClient.IsEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		newTokens, err := s.ssoClient.RefreshTokens(r.Context(), refreshCookie.Value)
		if err != nil {
			if errors.Is(err, auth.ErrRefreshRevoked) {
				// Explicitly rejected: banned user, revoked token, or expired.
				// Clear cookies immediately — the session is dead.
				log.Printf("refresh token rejected by server, clearing session")
				clearAuthCookies(w)
			}
			// For transient errors (network, 5xx) do NOT clear cookies.
			// The existing access token (still valid for a few more minutes)
			// carries the request through. The next request retries the refresh.
			next.ServeHTTP(w, r)
			return
		}

		secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
		http.SetCookie(w, &http.Cookie{
			Name:     AuthCookieName,
			Value:    newTokens.AccessToken,
			Path:     "/",
			MaxAge:   CookieMaxAge,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     RefreshCookieName,
			Value:    newTokens.RefreshToken,
			Path:     "/",
			MaxAge:   RefreshCookieMaxAge,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
		})

		next.ServeHTTP(w, r)
	})
}

// clearAuthCookies removes both the access and refresh cookies.
func clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: AuthCookieName, Value: "", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: RefreshCookieName, Value: "", Path: "/", MaxAge: -1})
}
