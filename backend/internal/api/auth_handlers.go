package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"explorer/internal/auth"
)

const (
	AuthCookieName  = "explorer_auth"
	StateCookieName = "explorer_oauth_state"
	CookieMaxAge    = 30 * 60 // 30 minutes
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

	http.SetCookie(w, &http.Cookie{
		Name:     AuthCookieName,
		Value:    tokenResp.AccessToken,
		Path:     "/",
		MaxAge:   tokenResp.ExpiresIn,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})

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
		http.SetCookie(w, &http.Cookie{
			Name:   AuthCookieName,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		writeJSON(w, status)
		return
	}

	if claims.IsExpired() {
		http.SetCookie(w, &http.Cookie{
			Name:   AuthCookieName,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		writeJSON(w, status)
		return
	}

	status.Authenticated = true
	status.DID = claims.GetDID()
	status.ExpiresAt = claims.ExpiresAt

	writeJSON(w, status)
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   AuthCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

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

func (s *Server) refreshAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(AuthCookieName)
		if err == nil && cookie.Value != "" {
			claims, err := auth.ExtractClaims(cookie.Value)
			if err == nil && !claims.IsExpired() {
				// Refresh cookie if less than 5 minutes remaining
				if claims.TimeToExpiry() < 5*time.Minute {
					http.SetCookie(w, &http.Cookie{
						Name:     AuthCookieName,
						Value:    cookie.Value,
						Path:     "/",
						MaxAge:   CookieMaxAge,
						HttpOnly: true,
						SameSite: http.SameSiteLaxMode,
						Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
					})
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
