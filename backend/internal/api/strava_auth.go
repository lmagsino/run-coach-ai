package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

const stateCookie = "strava_oauth_state"

// handleStravaLogin starts the OAuth flow: it sets a CSRF state cookie and
// redirects the user to Strava's authorize page.
func (s *Server) handleStravaLogin(w http.ResponseWriter, r *http.Request) {
	if !s.strava.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "strava not configured: set STRAVA_CLIENT_ID and STRAVA_CLIENT_SECRET in .env",
		})
		return
	}

	state, err := randomState()
	if err != nil {
		http.Error(w, "failed to generate state", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(10 * time.Minute),
	})

	http.Redirect(w, r, s.strava.AuthCodeURL(state), http.StatusFound)
}

// handleStravaCallback completes the OAuth flow: it validates the state cookie,
// exchanges the authorization code for tokens, and persists them.
func (s *Server) handleStravaCallback(w http.ResponseWriter, r *http.Request) {
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "strava denied authorization: " + errMsg})
		return
	}

	// Validate CSRF state against the cookie set at login.
	cookie, err := r.Cookie(stateCookie)
	if err != nil || cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	// State consumed — clear the cookie.
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Value: "", Path: "/", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	tok, err := s.strava.Exchange(r.Context(), code)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "connected",
		"provider":    "strava",
		"token_type":  tok.TokenType,
		"expires_at":  tok.Expiry,
		"has_refresh": tok.RefreshToken != "",
	})
}

// randomState returns a cryptographically random hex string for CSRF protection.
func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
