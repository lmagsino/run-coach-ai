package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/auth"
	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

// emailShape is a sanity check, not validation: there is no mail service, so
// the address is a sign-in name that has to look like an email.
var emailShape = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

const (
	maxDisplayName = 60
	// bcrypt ignores everything past 72 bytes, so a longer password would
	// silently be a shorter one.
	maxPasswordBytes = 72
)

// signupOpen reports whether a new runner can create an account.
func (s *Server) signupOpen() bool {
	return !s.cfg.MockMode && s.cfg.InviteCode != ""
}

// handleSignup creates a member and signs them in (ADR 0010). The invite code
// is the gate: every reply is billed to the owner's API key.
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
		InviteCode  string `json:"invite_code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be JSON"})
		return
	}
	if !s.signupOpen() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "sign-up is closed"})
		return
	}
	key := clientKey(r)
	if s.logins.blocked(key, time.Now()) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts; wait a minute"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(body.InviteCode)), []byte(s.cfg.InviteCode)) != 1 {
		s.logins.fail(key, time.Now())
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "that invite code is not right"})
		return
	}
	email := store.NormalizeEmail(body.Email)
	name := strings.Join(strings.Fields(body.DisplayName), " ")
	switch {
	case !emailShape.MatchString(email):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "that does not look like an email address"})
		return
	case name == "" || len([]rune(name)) > maxDisplayName:
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "a name of up to 60 characters is required"})
		return
	}
	if msg := passwordProblem(body.Password); msg != "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": msg})
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	runner, err := s.store.CreateRunner(r.Context(), store.Runner{
		Kind: store.KindMember, Email: email, PasswordHash: hash, DisplayName: name,
		DailyQuestionCap: s.cfg.MemberCap,
	})
	if errors.Is(err, store.ErrEmailTaken) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "an account with that email already exists"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.startSession(w, r, runner.ID, "sign_up")
}

// passwordProblem says what is wrong with a new password, or "" when nothing is.
func passwordProblem(p string) string {
	switch {
	case len([]rune(p)) < auth.MinPasswordLen:
		return "the password needs at least 10 characters"
	case len(p) > maxPasswordBytes:
		return "the password is too long (72 bytes at most)"
	}
	return ""
}
