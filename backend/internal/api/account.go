package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lmagsino/run-coach-ai/backend/internal/auth"
	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

// handleAccountUpdate changes what a runner can change about themselves
// without their password: the display name.
func (s *Server) handleAccountUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be JSON"})
		return
	}
	name := strings.Join(strings.Fields(body.DisplayName), " ")
	if name == "" || len([]rune(name)) > maxDisplayName {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "a name of up to 60 characters is required"})
		return
	}
	runner, err := s.store.RunnerByID(r.Context(), runnerID(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	runner.DisplayName = name
	if err := s.store.UpdateRunner(r.Context(), runner); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.Audit(r.Context(), runner.ID, "profile_updated", nil)
	writeJSON(w, http.StatusOK, map[string]any{"display_name": name})
}

// handlePasswordChange needs the current password, and signs the runner out
// of every other session: a password change is how a runner takes back an
// account they think someone else is using.
func (s *Server) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be JSON"})
		return
	}
	runner, ok := s.reauthenticate(w, r, body.Current)
	if !ok {
		return
	}
	if msg := passwordProblem(body.New); msg != "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": msg})
		return
	}
	hash, err := auth.HashPassword(body.New)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	runner.PasswordHash = hash
	if err := s.store.UpdateRunner(r.Context(), runner); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	keep := ""
	if c, err := r.Cookie(sessionCookie); err == nil {
		keep = c.Value
	}
	_ = s.store.DeleteSessionsFor(r.Context(), runner.ID, keep)
	_ = s.store.Audit(r.Context(), runner.ID, "password_changed", nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleAccountDelete removes the runner and everything it owns: sessions,
// chats, usage and the Garmin token bundle (ADR 0010). The owner cannot
// delete itself; the service would have nobody left to sign in.
func (s *Server) handleAccountDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be JSON with your \"password\""})
		return
	}
	runner, ok := s.reauthenticate(w, r, body.Password)
	if !ok {
		return
	}
	if runner.Kind == store.KindOwner {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "the owner account cannot be deleted"})
		return
	}
	// Written first: the row it names is about to go, and the foreign key
	// then keeps the audit row with a null runner.
	_ = s.store.Audit(r.Context(), runner.ID, "account_deleted", map[string]any{"email": runner.Email})
	if err := s.store.DeleteRunner(r.Context(), runner.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// reauthenticate checks the signed-in runner's password before a sensitive
// change, writing the refusal itself.
func (s *Server) reauthenticate(w http.ResponseWriter, r *http.Request, password string) (store.Runner, bool) {
	runner, err := s.store.RunnerByID(r.Context(), runnerID(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return runner, false
	}
	if !auth.CheckPassword(runner.PasswordHash, password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "that password is not right"})
		return runner, false
	}
	return runner, true
}
