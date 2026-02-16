package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"explorer/internal/auth"

	"github.com/go-chi/chi/v5"
)

const (
	// AuthCookieName is the name of the cookie that stores the JWT
	AuthCookieName = "explorer_auth"
	// StateCookieName is the name of the cookie that stores the OAuth state
	StateCookieName = "explorer_oauth_state"
	// CookieMaxAge is the max age of the auth cookie (30 minutes)
	CookieMaxAge = 30 * 60
)

// InitiateLoginRequest is the request body for POST /api/auth/login
type InitiateLoginRequest struct {
	ReturnURL string `json:"return_url,omitempty"`
}

// InitiateLoginResponse is the response from POST /api/auth/login
type InitiateLoginResponse struct {
	OAuthSessionID string      `json:"oauth_session_id"`
	AuthSessionID  string      `json:"auth_session_id"`
	AuthRequest    interface{} `json:"auth_request"`
	State          string      `json:"state"`
}

// handleAuthLogin initiates the SSO login flow
// POST /api/auth/login
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if s.ssoClient == nil || !s.ssoClient.IsEnabled() {
		writeError(w, http.StatusServiceUnavailable, "SSO is not configured")
		return
	}

	var req InitiateLoginRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
	}

	// Generate state for CSRF protection
	returnURL := req.ReturnURL
	if returnURL == "" {
		returnURL = "/"
	}
	state, err := s.ssoClient.GenerateState(returnURL)
	if err != nil {
		log.Printf("Failed to generate state: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to initiate login")
		return
	}

	// Call privacy-proxy's OAuth authorize endpoint
	authResp, err := s.ssoClient.InitiateAuthorization(r.Context(), state)
	if err != nil {
		log.Printf("Failed to initiate authorization: %v", err)
		writeError(w, http.StatusBadGateway, "Failed to connect to authentication service")
		return
	}

	writeJSON(w, InitiateLoginResponse{
		OAuthSessionID: authResp.OAuthSessionID,
		AuthSessionID:  authResp.AuthSessionID,
		AuthRequest:    authResp.AuthRequest,
		State:          state,
	})
}

// handleAuthCallback handles the OAuth callback
// GET /api/auth/callback?code=xxx&state=yyy
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

	// Validate state
	returnURL, valid := s.ssoClient.ValidateState(state)
	if !valid {
		writeError(w, http.StatusBadRequest, "Invalid or expired state parameter")
		return
	}

	// Exchange code for token
	tokenResp, err := s.ssoClient.ExchangeCode(r.Context(), code)
	if err != nil {
		log.Printf("Failed to exchange code: %v", err)
		writeError(w, http.StatusBadGateway, "Failed to exchange authorization code")
		return
	}

	// Set auth cookie
	http.SetCookie(w, &http.Cookie{
		Name:     AuthCookieName,
		Value:    tokenResp.AccessToken,
		Path:     "/",
		MaxAge:   tokenResp.ExpiresIn,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})

	// Redirect to the original page
	http.Redirect(w, r, returnURL, http.StatusFound)
}

// handleAuthStatus returns the current authentication status
// GET /api/auth/status
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	status := auth.AuthStatus{
		Authenticated: false,
	}

	// Check for auth cookie
	cookie, err := r.Cookie(AuthCookieName)
	if err != nil || cookie.Value == "" {
		writeJSON(w, status)
		return
	}

	// Extract claims from JWT
	claims, err := auth.ExtractClaims(cookie.Value)
	if err != nil {
		// Invalid token - clear cookie
		http.SetCookie(w, &http.Cookie{
			Name:   AuthCookieName,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		writeJSON(w, status)
		return
	}

	// Check if expired
	if claims.IsExpired() {
		// Clear expired cookie
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

// handleAuthLogout clears the authentication cookie
// POST /api/auth/logout
func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   AuthCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	writeJSON(w, map[string]bool{"logged_out": true})
}

// handleAuthSessionStatus checks the status of an OAuth session
// GET /api/auth/session/{id}/status
func (s *Server) handleAuthSessionStatus(w http.ResponseWriter, r *http.Request) {
	if s.ssoClient == nil || !s.ssoClient.IsEnabled() {
		writeError(w, http.StatusServiceUnavailable, "SSO is not configured")
		return
	}

	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "Missing session ID")
		return
	}

	status, err := s.ssoClient.CheckSessionStatus(r.Context(), sessionID)
	if err != nil {
		log.Printf("Failed to check session status: %v", err)
		writeJSON(w, map[string]bool{"completed": false})
		return
	}

	writeJSON(w, status)
}

// GetAuthDID extracts the DID from the auth cookie
// Returns empty string if not authenticated
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

// GetAuthToken extracts the JWT token from the auth cookie
// Returns empty string if not authenticated
func (s *Server) GetAuthToken(r *http.Request) string {
	cookie, err := r.Cookie(AuthCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// refreshAuthMiddleware refreshes the auth cookie if it's close to expiring
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
						Secure:   r.TLS != nil,
					})
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
