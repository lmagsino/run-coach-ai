package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/lmagsino/run-coach-ai/backend/internal/auth"
	"github.com/lmagsino/run-coach-ai/backend/internal/config"
)

const testInvite = "run-with-leo"

// openServer is a non-mock server with sign-up open and no Garmin or model.
func openServer(t *testing.T) *Server {
	t.Helper()
	hash, err := auth.HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	return New(&config.Config{
		OwnerEmail: "leo@example.com", OwnerPasswordHash: hash,
		InviteCode: testInvite, MemberCap: 10,
	}, nil)
}

func signup(t *testing.T, s *Server, email, name, password, code string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "display_name": name, "password": password, "invite_code": code})
	rec := do(t, s, http.MethodPost, "/api/signup", string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("sign-up %s: %d %s", email, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("sign-up set no session cookie")
	return nil
}

// A new runner signs up with the invite code, lands signed in as a member
// with the member cap, and can sign in again later with the same email.
func TestSignupCreatesASignedInMember(t *testing.T) {
	s := openServer(t)
	cookie := signup(t, s, " Ana@Example.com ", "  Ana   Reyes ", "a long enough password", testInvite)

	rec := do(t, s, http.MethodGet, "/api/me", "", cookie)
	var me struct {
		Runner struct {
			DisplayName string `json:"display_name"`
			Email, Kind string
		}
		Usage struct{ Cap int }
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Runner.Kind != "member" || me.Runner.Email != "ana@example.com" || me.Runner.DisplayName != "Ana Reyes" || me.Usage.Cap != 10 {
		t.Errorf("/me after sign-up: %+v", me)
	}

	if rec := do(t, s, http.MethodPost, "/api/login", `{"email":"ana@example.com","password":"a long enough password"}`); rec.Code != http.StatusOK {
		t.Errorf("sign-in after sign-up: %d", rec.Code)
	}
}

func TestSignupRefusals(t *testing.T) {
	s := openServer(t)
	signup(t, s, "taken@example.com", "Taken", "a long enough password", testInvite)

	cases := []struct {
		name, email, display, password, code string
		want                                 int
	}{
		{"wrong code", "b@example.com", "B", "a long enough password", "guess", http.StatusForbidden},
		{"no code", "b@example.com", "B", "a long enough password", "", http.StatusForbidden},
		{"email taken, any case", "TAKEN@example.com", "B", "a long enough password", testInvite, http.StatusConflict},
		{"not an email", "b-at-example", "B", "a long enough password", testInvite, http.StatusUnprocessableEntity},
		{"no name", "b@example.com", "   ", "a long enough password", testInvite, http.StatusUnprocessableEntity},
		{"short password", "b@example.com", "B", "short", testInvite, http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		// A fresh limiter per case so the refusals are about the input.
		s.logins = newLoginLimiter(100, 60e9)
		body, _ := json.Marshal(map[string]string{"email": tc.email, "display_name": tc.display, "password": tc.password, "invite_code": tc.code})
		if rec := do(t, s, http.MethodPost, "/api/signup", string(body)); rec.Code != tc.want {
			t.Errorf("%s: got %d, want %d (%s)", tc.name, rec.Code, tc.want, rec.Body.String())
		}
	}
}

// Without an invite code configured, sign-up is closed and health says so.
func TestSignupClosedWithoutACode(t *testing.T) {
	s := liveServer(t, "pw")
	body := `{"email":"b@example.com","display_name":"B","password":"a long enough password","invite_code":""}`
	if rec := do(t, s, http.MethodPost, "/api/signup", body); rec.Code != http.StatusForbidden {
		t.Errorf("closed sign-up: %d", rec.Code)
	}
	var h map[string]any
	_ = json.Unmarshal(do(t, s, http.MethodGet, "/api/healthz", "").Body.Bytes(), &h)
	if h["signup"] != false {
		t.Errorf("health signup: %v", h["signup"])
	}
	_ = json.Unmarshal(do(t, openServer(t), http.MethodGet, "/api/healthz", "").Body.Bytes(), &h)
	if h["signup"] != true {
		t.Errorf("health signup when open: %v", h["signup"])
	}
}
