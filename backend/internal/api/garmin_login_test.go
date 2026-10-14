package api

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/crypto"
	"github.com/lmagsino/run-coach-ai/backend/internal/garmin"
	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

// stubLogins answers like Garmin would: the right password connects, or asks
// for a code first when mfa is set; anything else is refused.
type stubLogins struct {
	bundle string
	mfa    bool
	got    *[]string // passwords seen, to prove they are not kept elsewhere
}

type stubLoginSession struct{ bundle string }

func (st stubLogins) Start(email, password string) (string, garmin.LoginSession, error) {
	if st.got != nil {
		*st.got = append(*st.got, password)
	}
	if password != "garmin-pass" {
		return "", nil, &garmin.LoginError{Kind: garmin.LoginBadCredentials, Message: "401"}
	}
	if st.mfa {
		return "", stubLoginSession{st.bundle}, nil
	}
	return st.bundle, nil, nil
}

func (ss stubLoginSession) Code(code string) (string, error) {
	if code != "123456" {
		return "", &garmin.LoginError{Kind: garmin.LoginBadCode, Message: "wrong"}
	}
	return ss.bundle, nil
}

func (stubLoginSession) Close() {}

func loginServer(t *testing.T, logins garmin.Logins) (*Server, *http.Cookie, string) {
	t.Helper()
	bundle := base64.StdEncoding.EncodeToString([]byte(`{"di_token":"` + strings.Repeat("x", 600) + `","di_refresh_token":"r","di_client_id":"c"}`))
	s := liveServer(t, "pw")
	key, _ := crypto.NewKey()
	s.cfg.TokenKey = key
	s.cfg.GarminLoginCommand = "runcoach-garmin-login"
	s = New(s.cfg, nil)
	s.garmin = stubSource{accept: bundle}
	s.garminLogins = logins
	rec := do(t, s, http.MethodPost, "/api/login", `{"password":"pw"}`)
	return s, rec.Result().Cookies()[0], bundle
}

func connected(t *testing.T, s *Server) bool {
	t.Helper()
	conn, _ := s.store.GarminConnection(t.Context(), 1)
	return conn != nil && conn.Status == store.GarminConnected
}

func TestGarminLoginWithoutMFAConnects(t *testing.T) {
	var seen []string
	s, cookie, _ := loginServer(t, nil)
	s.garminLogins = stubLogins{bundle: mustBundle(t, s), got: &seen}
	rec := do(t, s, http.MethodPost, "/api/garmin/login", `{"email":"leo@example.com","password":"garmin-pass"}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"connected"`) {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	if !connected(t, s) {
		t.Fatal("not connected after signing in")
	}
	if strings.Contains(rec.Body.String(), "garmin-pass") {
		t.Error("the reply carries the password")
	}
	if len(seen) != 1 {
		t.Errorf("the helper saw %d passwords, want 1", len(seen))
	}
}

func TestGarminLoginWithMFA(t *testing.T) {
	s, cookie, _ := loginServer(t, nil)
	s.garminLogins = stubLogins{bundle: mustBundle(t, s), mfa: true}
	rec := do(t, s, http.MethodPost, "/api/garmin/login", `{"email":"leo@example.com","password":"garmin-pass"}`, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"needs_mfa"`) {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	if connected(t, s) {
		t.Fatal("connected before the code")
	}
	rec = do(t, s, http.MethodPost, "/api/garmin/login/code", `{"code":" 123456 "}`, cookie)
	if rec.Code != http.StatusOK || !connected(t, s) {
		t.Fatalf("code: %d %s", rec.Code, rec.Body)
	}
	// The sign-in is used up: a second code has nothing to finish.
	if rec = do(t, s, http.MethodPost, "/api/garmin/login/code", `{"code":"123456"}`, cookie); rec.Code != http.StatusConflict {
		t.Errorf("second code: %d", rec.Code)
	}
}

func TestGarminLoginRefusals(t *testing.T) {
	s, cookie, _ := loginServer(t, nil)
	s.garminLogins = stubLogins{bundle: mustBundle(t, s), mfa: true}

	rec := do(t, s, http.MethodPost, "/api/garmin/login", `{"email":"leo@example.com","password":"nope"}`, cookie)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "did not accept that email and password") {
		t.Errorf("wrong password: %d %s", rec.Code, rec.Body)
	}

	do(t, s, http.MethodPost, "/api/garmin/login", `{"email":"leo@example.com","password":"garmin-pass"}`, cookie)
	rec = do(t, s, http.MethodPost, "/api/garmin/login/code", `{"code":"000000"}`, cookie)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"bad_code"`) {
		t.Errorf("wrong code: %d %s", rec.Code, rec.Body)
	}

	// An MFA wait that outlived its window is refused, even with the right code.
	do(t, s, http.MethodPost, "/api/garmin/login", `{"email":"leo@example.com","password":"garmin-pass"}`, cookie)
	v, _ := s.pendingLogins.Load(int64(1))
	v.(*pendingLogin).expires = time.Now().Add(-time.Second)
	if rec = do(t, s, http.MethodPost, "/api/garmin/login/code", `{"code":"123456"}`, cookie); rec.Code != http.StatusConflict {
		t.Errorf("expired code: %d %s", rec.Code, rec.Body)
	}
	if connected(t, s) {
		t.Error("a refused sign-in connected")
	}
}

// Every attempt counts, not only failures, so a runner cannot trip Garmin's
// own lock on their account through RunCoach.
func TestGarminLoginAttemptsAreLimited(t *testing.T) {
	const allowed = 5
	s, cookie, _ := loginServer(t, nil)
	s.garminLogins = stubLogins{bundle: mustBundle(t, s), mfa: true}
	for i := 0; i < allowed; i++ {
		if rec := do(t, s, http.MethodPost, "/api/garmin/login", `{"email":"leo@example.com","password":"garmin-pass"}`, cookie); rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: %d", i+1, rec.Code)
		}
	}
	if rec := do(t, s, http.MethodPost, "/api/garmin/login", `{"email":"leo@example.com","password":"garmin-pass"}`, cookie); rec.Code != http.StatusTooManyRequests {
		t.Errorf("attempt %d: %d, want 429", allowed+1, rec.Code)
	}
}

func TestGarminLoginOff(t *testing.T) {
	s, cookie, _ := loginServer(t, stubLogins{})
	s.cfg.GarminLoginCommand = "off"
	rec := do(t, s, http.MethodPost, "/api/garmin/login", `{"email":"leo@example.com","password":"garmin-pass"}`, cookie)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "token method") {
		t.Errorf("login off: %d %s", rec.Code, rec.Body)
	}
	if me := do(t, s, http.MethodGet, "/api/me", "", cookie); !strings.Contains(me.Body.String(), `"garmin_login":false`) {
		t.Errorf("/api/me still offers the form: %s", me.Body)
	}
}

// mustBundle is the bundle the stub probe accepts for this server.
func mustBundle(t *testing.T, s *Server) string {
	t.Helper()
	return s.garmin.(stubSource).accept
}
