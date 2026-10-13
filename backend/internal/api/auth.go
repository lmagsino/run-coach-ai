package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/auth"
	"github.com/lmagsino/run-coach-ai/backend/internal/garmin"
	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be JSON with a \"password\""})
		return
	}
	if !s.logins.allow(clientKey(r), time.Now()) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts; wait a minute"})
		return
	}
	id, err := s.auth.Authenticate(r.Context(), auth.Credentials{Password: body.Password})
	if errors.Is(err, auth.ErrDenied) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "wrong password"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sid, err := s.store.CreateSession(r.Context(), id, sessionTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	setSessionCookie(w, r, sid)
	_ = s.store.Audit(r.Context(), id, "sign_in", nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		_ = s.store.DeleteSession(r.Context(), c.Value)
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// garminState is the chip and the fix sentence, derived from the stored row
// and never from a live Garmin call (that would make every page load a
// Garmin request).
type garminState struct {
	Status       string     `json:"status"` // none | connected | expired | unavailable
	Fix          string     `json:"fix,omitempty"`
	ImportedAt   *time.Time `json:"imported_at,omitempty"`
	LastOKAt     *time.Time `json:"last_ok_at,omitempty"`
	ExpiresEstAt *time.Time `json:"expires_est_at,omitempty"`
	LastError    string     `json:"last_error,omitempty"`
}

const (
	fixNone        = "Garmin is not connected. Run garmin-mcp-auth on your laptop and paste the token bundle on the Connection page."
	fixExpired     = "Your Garmin tokens have expired. Run garmin-mcp-auth on your laptop and paste the new bundle on the Connection page."
	fixUnavailable = "Garmin did not answer. Try again in a few minutes; if it keeps failing, upstream may need an update."
)

func (s *Server) garminStateFor(r *http.Request, id int64) (garminState, error) {
	if s.cfg.MockMode {
		return garminState{Status: store.GarminConnected}, nil
	}
	conn, err := s.store.GarminConnection(r.Context(), id)
	if err != nil {
		return garminState{}, err
	}
	if conn == nil {
		return garminState{Status: "none", Fix: fixNone}, nil
	}
	st := garminState{
		Status: conn.Status, ImportedAt: &conn.ImportedAt, LastOKAt: conn.LastOKAt,
		ExpiresEstAt: conn.ExpiresEstAt, LastError: conn.LastError,
	}
	switch conn.Status {
	case store.GarminExpired:
		st.Fix = fixExpired
	case store.GarminUnavailable:
		st.Fix = fixUnavailable
	}
	return st, nil
}

// handleMe tells the UI who is signed in, how Garmin is, and today's usage.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	id := runnerID(r)
	owner, err := s.store.Owner(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	st, err := s.garminStateFor(r, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	usage, err := s.store.Usage(r.Context(), id, time.Now())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"runner": map[string]any{"id": id, "display_name": owner.DisplayName},
		"garmin": st,
		"usage":  map[string]any{"today": usage.Questions, "cap": owner.DailyQuestionCap},
		"mock":   s.cfg.MockMode,
	})
}

// handleGarminTokens takes the pasted bundle, probes it, and stores it sealed.
func (s *Server) handleGarminTokens(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Bundle string `json:"bundle"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be JSON with a \"bundle\""})
		return
	}
	if s.cfg.MockMode {
		writeJSON(w, http.StatusOK, map[string]any{"status": store.GarminConnected, "mock": true})
		return
	}
	if s.sealer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "RUNCOACH_TOKEN_KEY is not set on the server"})
		return
	}
	bundle, err := garmin.NormalizeBundle(body.Bundle)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	name, err := s.garmin.Probe(r.Context(), bundle)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "the bundle did not work: " + err.Error()})
		return
	}
	blob, err := s.sealer.Seal([]byte(bundle))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	id := runnerID(r)
	now := time.Now()
	exp := now.Add(garmin.TokenLifetime)
	err = s.store.UpsertGarminConnection(r.Context(), store.GarminConnection{
		RunnerID: id, Kind: store.ConnectionLocalTokens, Blob: blob, KeyVersion: 1,
		Status: store.GarminConnected, ImportedAt: now, LastOKAt: &now, ExpiresEstAt: &exp,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.Audit(r.Context(), id, "tokens_imported", map[string]any{"garmin_name": name})
	st, _ := s.garminStateFor(r, id)
	writeJSON(w, http.StatusOK, map[string]any{"garmin": st, "garmin_name": name})
}

func (s *Server) handleGarminTokensDelete(w http.ResponseWriter, r *http.Request) {
	id := runnerID(r)
	if err := s.store.DeleteGarminConnection(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.Audit(r.Context(), id, "tokens_removed", nil)
	st, _ := s.garminStateFor(r, id)
	writeJSON(w, http.StatusOK, map[string]any{"garmin": st})
}
