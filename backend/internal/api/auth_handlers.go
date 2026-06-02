package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"explorer/internal/auth"
	"explorer/pkg/log"
)

const (
	AuthCookieName    = "explorer_auth"
	RefreshCookieName = "explorer_refresh"
	StateCookieName   = "explorer_oauth_state"

	// CookieMaxAge is the browser lifetime of the access-token cookie.
	// This should stay in sync with the access token TTL on privacy-proxy
	// (AccessTokenTTL = 5 min), but we extend the cookie on every successful
	// token refresh so active sessions stay alive as a sliding window.
	CookieMaxAge = 30 * 60 // 30 minutes

	// RefreshCookieMaxAge is the lifetime of the refresh-token cookie.
	// Matches RefreshTokenTTL on privacy-proxy (7 days).
	RefreshCookieMaxAge = 7 * 24 * 60 * 60 // 7 days

	// refreshThreshold is how close to access-token expiry we trigger a silent
	// refresh. Per block-explorer REQ-5.3 it tracks AccessTokenTTL (5 min): the
	// access token is refreshed once it has ≤ this long remaining. Because a
	// freshly minted token already has < TTL left, an active session refreshes on
	// use and slides forward, maximizing how long a user stays logged in. Many
	// concurrent requests can qualify at once, but they are collapsed into a
	// single privacy-proxy /refresh by refreshGroup (single-flight), so this does
	// NOT reopen the rotation race. An idle session lapses when its current access
	// token expires (the documented ban-enforcement window).
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
		log.Error("sso login: failed to generate oauth state", "error", err)
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
		log.Error("sso callback: failed to exchange authorization code", "error", err)
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

	// When the explorer is embedded in another app (e.g. iframed under a
	// parent dashboard at a different origin), the relative returnURL
	// would land the user on the standalone explorer host with no way
	// back to the parent. PostLoginRedirectURL overrides the target so
	// the user lands on the embedding app's URL instead.
	finalURL := returnURL
	if s.postLoginRedirectURL != "" {
		finalURL = s.postLoginRedirectURL
	}
	http.Redirect(w, r, finalURL, http.StatusFound)
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

func handleAuthStatusDisabled(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"enabled":       false,
		"authenticated": false,
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	// Revoke server-side so the token cannot be reused even if captured from
	// the network before the cookie was cleared.
	if s.ssoClient != nil && s.ssoClient.IsEnabled() {
		if refreshCookie, err := r.Cookie(RefreshCookieName); err == nil && refreshCookie.Value != "" {
			if err := s.ssoClient.RevokeToken(r.Context(), refreshCookie.Value); err != nil {
				log.Warn("sso logout: refresh token revocation failed", "error", err)
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
//     with the refresh token cookie to obtain a new access + refresh token pair
//     and write both updated cookies. privacy-proxy rotates the refresh token
//     single-use, so concurrent requests for the same session are collapsed into
//     ONE /refresh call (see refreshGroup) — otherwise the losers would present
//     an already-rotated token and be told it was revoked.
//  3. If /refresh is explicitly rejected (ErrRefreshRevoked) we clear both
//     cookies ONLY when the current access token has also expired — that combo
//     means the session is genuinely dead. A revoke while the access token is
//     still valid is treated as a lost rotation race (a sibling already rotated
//     the pair): we keep the session and ride the existing access token until it
//     expires. A missing refresh cookie or a transient error also passes through
//     without clearing.
//
// Security note: access tokens are intentionally short-lived (5 min). The ban
// enforcement window is at most AccessTokenTTL — a banned user keeps acting until
// their current access token expires, because privacy-proxy validates that token
// on every call (the explorer cookie is not the security boundary). Not clearing
// the cookie on a revoke-while-valid does NOT widen this window. Do not increase
// the TTL without reconsidering the ban enforcement strategy.
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

		// Single-flight the refresh. A browser page load fires many API requests
		// in parallel, all carrying the SAME refresh-token cookie. privacy-proxy
		// rotates refresh tokens single-use: the first /refresh call mints a new
		// pair and REVOKES the presented token, so any sibling that races in with
		// the same (now-revoked) token gets a 401. Collapsing the burst into one
		// /refresh call — keyed by the refresh token — means exactly one rotation
		// happens and every waiter receives the same freshly-minted pair, instead
		// of N-1 of them being told "revoked" and tearing the session down.
		res, refreshErr, _ := s.refreshGroup.Do(refreshKey(refreshCookie.Value), func() (any, error) {
			return s.ssoClient.RefreshTokens(r.Context(), refreshCookie.Value)
		})
		if refreshErr != nil {
			// Only end the session when the refresh token is explicitly rejected
			// AND our access token has now expired too — that combination means
			// the session is genuinely dead (banned user, expired refresh token).
			//
			// An ErrRefreshRevoked while the access token is STILL valid is almost
			// always a lost rotation race: a sibling request already rotated the
			// pair and installed the new cookies, and this request's revoked token
			// is simply the old one. Clearing here would destroy a healthy session
			// (the original "logged out after a page refresh" bug). Instead we ride
			// the still-valid access token; the session lapses on its own when that
			// token expires (bounded by AccessTokenTTL — the designed ban window).
			// Transient errors (network, 5xx) fall through the same way and retry
			// on the next request.
			if errors.Is(refreshErr, auth.ErrRefreshRevoked) && claims.IsExpired() {
				log.Info("sso refresh: token rejected by server and access token expired, clearing session")
				clearAuthCookies(w)
			}
			next.ServeHTTP(w, r)
			return
		}

		newTokens := res.(*auth.RefreshResponse)
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

// refreshKey derives a stable single-flight key from a refresh token. We hash
// it so the raw secret never lives as a map key; the key only needs to be equal
// across the concurrent requests that share one refresh-token cookie.
func refreshKey(refreshToken string) string {
	sum := sha256.Sum256([]byte(refreshToken))
	return hex.EncodeToString(sum[:])
}
