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
	AuthCookieName  = "explorer_auth"
	StateCookieName = "explorer_oauth_state"
	CookieMaxAge    = 30 * 60 // 30 minutes
)

type InitiateLoginRequest struct {
	ReturnURL string `json:"return_url,omitempty"`
}

type InitiateLoginResponse struct {
	OAuthSessionID string      `json:"oauth_session_id"`
	AuthSessionID  string      `json:"auth_session_id"`
	AuthRequest    interface{} `json:"auth_request"`
	State          string      `json:"state"`
}

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
		Secure:   r.TLS != nil,
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
						Secure:   r.TLS != nil,
					})
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
