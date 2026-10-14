package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/garmin"
)

// What the account page says for each way a sign-in fails. "blocked" and
// "unavailable" point at the paste below, which works from any computer.
var loginSentences = map[string]string{
	garmin.LoginBadCredentials: "Garmin did not accept that email and password.",
	garmin.LoginBadCode:        "Garmin did not accept that code. Sign in again to get a new one.",
	garmin.LoginBlocked:        "Garmin refused a sign-in from RunCoach's server. Connect with the token method below instead.",
	garmin.LoginUnavailable:    "Signing in to Garmin is not working on this server right now. Connect with the token method below instead.",
	garmin.LoginFailed:         "Signing in to Garmin failed. Try again, or connect with the token method below.",
}

var loginStatus = map[string]int{
	garmin.LoginBadCredentials: http.StatusUnprocessableEntity,
	garmin.LoginBadCode:        http.StatusUnprocessableEntity,
	garmin.LoginBlocked:        http.StatusBadGateway,
	garmin.LoginUnavailable:    http.StatusServiceUnavailable,
	garmin.LoginFailed:         http.StatusBadGateway,
}

type pendingLogin struct {
	session garmin.LoginSession
	expires time.Time
}

// handleGarminLogin signs the runner in to Garmin with their email and
// password. The password is passed to the sign-in helper once and kept nowhere:
// not in the store, the log or the reply. Garmin either returns tokens, which
// are probed and stored like a pasted bundle, or asks for an MFA code, which
// handleGarminLoginCode takes.
func (s *Server) handleGarminLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil ||
		strings.TrimSpace(body.Email) == "" || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enter your Garmin email and password"})
		return
	}
	if !s.loginEnabled() {
		s.loginFailed(w, &garmin.LoginError{Kind: garmin.LoginUnavailable, Message: "sign-in is turned off"})
		return
	}
	id := runnerID(r)
	key := "garmin:" + strconv.FormatInt(id, 10)
	if s.garminAttempts.blocked(key, time.Now()) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{
			"error": "Too many Garmin sign-ins in a short time. Wait 15 minutes, so Garmin does not lock your account.",
			"kind":  "too_many",
		})
		return
	}
	s.garminAttempts.fail(key, time.Now())
	s.dropPendingLogin(id)

	bundle, session, err := s.garminLogins.Start(strings.TrimSpace(body.Email), body.Password)
	if err != nil {
		s.loginFailed(w, err)
		return
	}
	if session != nil {
		s.pendingLogins.Store(id, &pendingLogin{session: session, expires: time.Now().Add(garmin.LoginTTL)})
		writeJSON(w, http.StatusOK, map[string]string{"status": "needs_mfa"})
		return
	}
	s.connectBundle(w, r, bundle)
}

// handleGarminLoginCode finishes a sign-in that is waiting for its MFA code.
func (s *Server) handleGarminLoginCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil || strings.TrimSpace(body.Code) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enter the code Garmin sent you"})
		return
	}
	id := runnerID(r)
	v, ok := s.pendingLogins.LoadAndDelete(id)
	if !ok || time.Now().After(v.(*pendingLogin).expires) {
		if ok {
			v.(*pendingLogin).session.Close()
		}
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "That sign-in has expired. Sign in again to get a new code.", "kind": "expired",
		})
		return
	}
	bundle, err := v.(*pendingLogin).session.Code(strings.TrimSpace(body.Code))
	if err != nil {
		s.loginFailed(w, err)
		return
	}
	s.connectBundle(w, r, bundle)
}

func (s *Server) loginEnabled() bool {
	c := strings.TrimSpace(s.cfg.GarminLoginCommand)
	return s.cfg.MockMode || (c != "" && c != "off")
}

func (s *Server) dropPendingLogin(id int64) {
	if v, ok := s.pendingLogins.LoadAndDelete(id); ok {
		v.(*pendingLogin).session.Close()
	}
}

// loginFailed answers with the sentence for the failure. Garmin's own message
// goes to the log, never the password, which the helper scrubs from it.
func (s *Server) loginFailed(w http.ResponseWriter, err error) {
	kind, message := garmin.IsLoginError(err)
	sentence, ok := loginSentences[kind]
	if !ok {
		kind, sentence = garmin.LoginFailed, loginSentences[garmin.LoginFailed]
	}
	if kind != garmin.LoginBadCredentials && kind != garmin.LoginBadCode {
		log.Printf("garmin sign-in: %s: %s", kind, message)
	}
	writeJSON(w, loginStatus[kind], map[string]string{"error": sentence, "kind": kind})
}

// mockLogins stands in for Garmin in mock mode, so the walkthrough can drive
// the form: "wrong" is a refused password, anything else asks for a code, and
// 123456 is the right code.
type mockLogins struct{}

type mockLoginSession struct{}

func (mockLogins) Start(_, password string) (string, garmin.LoginSession, error) {
	if password == "wrong" {
		return "", nil, &garmin.LoginError{Kind: garmin.LoginBadCredentials, Message: "mock"}
	}
	return "", mockLoginSession{}, nil
}

func (mockLoginSession) Code(code string) (string, error) {
	if code != "123456" {
		return "", &garmin.LoginError{Kind: garmin.LoginBadCode, Message: "mock"}
	}
	return "mock-bundle", nil
}

func (mockLoginSession) Close() {}
