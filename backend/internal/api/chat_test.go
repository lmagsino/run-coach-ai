package api

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmagsino/run-coach-ai/backend/internal/auth"
	"github.com/lmagsino/run-coach-ai/backend/internal/config"
	"github.com/lmagsino/run-coach-ai/backend/internal/crypto"
	"github.com/lmagsino/run-coach-ai/backend/internal/garmin"
	"github.com/lmagsino/run-coach-ai/backend/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// These tests cover mock mode and the transport plumbing: no database, no MCP
// subprocess, no Anthropic call. Mock mode is the path the frontend is built
// against before any credential exists, and it goes through the same routes,
// store and stream writer as the live path.

func mockServer(t *testing.T) *Server {
	t.Helper()
	return New(&config.Config{MockMode: true}, nil)
}

// liveServer is a non-mock server with an in-memory store and no Garmin or
// model: enough to test login, sessions and the guards that fire before any
// of those are reached.
func liveServer(t *testing.T, password string) *Server {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	return New(&config.Config{OwnerEmail: "leo@example.com", OwnerPasswordHash: hash}, nil)
}

// sseEvent is one parsed frame off the stream.
type sseEvent struct {
	Name string
	Data map[string]any
}

// readSSE parses an SSE body into frames. Deliberately hand-rolled: asserting on
// the wire format is part of the contract the browser depends on.
func readSSE(t *testing.T, body string) []sseEvent {
	t.Helper()
	var events []sseEvent
	var current sseEvent
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, ":"):
			// keepalive comment
		case strings.HasPrefix(line, "event: "):
			current.Name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			raw := strings.TrimPrefix(line, "data: ")
			if err := json.Unmarshal([]byte(raw), &current.Data); err != nil {
				t.Fatalf("event %q has non-JSON data %q: %v", current.Name, raw, err)
			}
		case line == "":
			if current.Name != "" {
				events = append(events, current)
				current = sseEvent{}
			}
		}
	}
	return events
}

func do(t *testing.T, s *Server, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var rd *strings.Reader
	if body != "" {
		rd = strings.NewReader(body)
	} else {
		rd = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	return rec
}

func newChat(t *testing.T, s *Server, cookies ...*http.Cookie) string {
	t.Helper()
	rec := do(t, s, http.MethodPost, "/api/chats", `{}`, cookies...)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create chat: %d %s", rec.Code, rec.Body.String())
	}
	var c chatJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func postStream(t *testing.T, s *Server, chatID, question string) []sseEvent {
	t.Helper()
	rec := do(t, s, http.MethodPost, "/api/chats/"+chatID+"/stream", `{"question":`+quote(question)+`}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type: got %q, want text/event-stream", ct)
	}
	return readSSE(t, rec.Body.String())
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// The stream's shape is the contract the UI's status steps are built on: every
// tool call is announced active before it is announced done, a chart may follow
// the steps, and the answer arrives last.
func TestChatStreamEmitsStepsThenAnswer(t *testing.T) {
	s := mockServer(t)
	events := postStream(t, s, newChat(t, s), "Does my sleep actually affect my pace?")

	if len(events) < 3 {
		t.Fatalf("expected several events, got %d: %+v", len(events), events)
	}
	last := events[len(events)-1]
	if last.Name != "answer" {
		t.Fatalf("last event: got %q, want %q", last.Name, "answer")
	}
	if answer, _ := last.Data["answer"].(string); answer == "" {
		t.Error("answer event carried no answer text")
	}

	open := map[string]bool{}
	sawChart := false
	for _, ev := range events[:len(events)-1] {
		if ev.Name == "chart" {
			sawChart = true
			continue
		}
		if ev.Name != "step" {
			t.Fatalf("unexpected event %q before the answer", ev.Name)
		}
		if sawChart {
			t.Fatalf("step %v arrived after the chart", ev.Data)
		}
		if ev.Data["source"] != "garmin" {
			t.Errorf("unexpected %v step", ev.Data["source"])
		}
		key, _ := ev.Data["source"].(string)
		key += "/" + ev.Data["tool"].(string)
		switch ev.Data["state"] {
		case "active":
			if open[key] {
				t.Errorf("%s went active twice without finishing", key)
			}
			open[key] = true
		case "done", "failed":
			if !open[key] {
				t.Errorf("%s finished without starting", key)
			}
			delete(open, key)
		default:
			t.Errorf("%s has unknown state %v", key, ev.Data["state"])
		}
	}
	for key := range open {
		t.Errorf("step %s never finished", key)
	}
}

// The chart frame contract: one at most, after the steps and before the
// answer, carrying the validated spec; an invalid spec emits none while the
// prose still arrives.
func TestChatStreamChartFrames(t *testing.T) {
	cases := []struct {
		question  string
		wantChart bool
		kind      string
	}{
		{"Does my sleep actually affect my pace?", true, "scatter"},
		{"How did my last three training blocks compare?", true, "bar"},
		{"Am I overtraining right now?", false, ""},
		{"I have a half marathon in 10 weeks. Am I on track?", false, ""}, // invalid spec: dropped
	}
	for _, tc := range cases {
		s := mockServer(t)
		events := postStream(t, s, newChat(t, s), tc.question)
		var charts []sseEvent
		for _, ev := range events {
			if ev.Name == "chart" {
				charts = append(charts, ev)
			}
		}
		answer := events[len(events)-1]
		if tc.wantChart {
			if len(charts) != 1 || charts[0].Data["kind"] != tc.kind || events[len(events)-2].Name != "chart" || answer.Data["chart"] == nil {
				t.Errorf("%q: charts %+v, answer chart %v", tc.question, charts, answer.Data["chart"])
			}
		} else if len(charts) != 0 || answer.Data["chart"] != nil || answer.Data["answer"] == "" {
			t.Errorf("%q: charts %+v, answer %v", tc.question, charts, answer.Data)
		}
	}
}

// A saved chat remembers its turns: the question and the reply (with its
// chart) are stored, the title comes from the first question, and the list
// shows it.
func TestChatIsPersistedWithTitleAndChart(t *testing.T) {
	s := mockServer(t)
	id := newChat(t, s)
	postStream(t, s, id, "Does my sleep actually affect my pace? Show me the pattern.")

	rec := do(t, s, http.MethodGet, "/api/chats/"+id+"/messages", "")
	var got struct{ Messages []messageJSON }
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 2 || got.Messages[0].Role != "user" || got.Messages[1].Role != "assistant" {
		t.Fatalf("messages: %+v", got.Messages)
	}
	if len(got.Messages[1].Chart) == 0 || !strings.Contains(string(got.Messages[1].Chart), `"scatter"`) {
		t.Errorf("assistant message lost its chart: %s", got.Messages[1].Chart)
	}

	rec = do(t, s, http.MethodGet, "/api/chats", "")
	var list struct{ Chats []chatJSON }
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Chats) != 1 || !strings.HasPrefix(list.Chats[0].Title, "Does my sleep actually affect my pace?") {
		t.Errorf("chats: %+v", list.Chats)
	}

	if rec := do(t, s, http.MethodPatch, "/api/chats/"+id, `{"title":"Sleep"}`); rec.Code != http.StatusOK {
		t.Errorf("rename: %d", rec.Code)
	}
	if rec := do(t, s, http.MethodDelete, "/api/chats/"+id, ""); rec.Code != http.StatusOK {
		t.Errorf("delete: %d", rec.Code)
	}
	if rec := do(t, s, http.MethodGet, "/api/chats/"+id+"/messages", ""); rec.Code != http.StatusNotFound {
		t.Errorf("messages of a deleted chat: %d", rec.Code)
	}
	if rec := do(t, s, http.MethodPost, "/api/chats/nope/stream", `{"question":"hi"}`); rec.Code != http.StatusNotFound {
		t.Errorf("stream to an unknown chat: %d", rec.Code)
	}
}

// An unrecognised question must say it has no answer rather than return a
// plausible-sounding one.
func TestChatStreamUnknownQuestionAdmitsItIsMocked(t *testing.T) {
	s := mockServer(t)
	events := postStream(t, s, newChat(t, s), "what is the capital of France?")
	answer, _ := events[len(events)-1].Data["answer"].(string)
	if !strings.Contains(strings.ToLower(answer), "mock mode") {
		t.Errorf("fallback answer should say it is mocked, got %q", answer)
	}
}

func TestStatelessChatInMockMode(t *testing.T) {
	rec := do(t, mockServer(t), http.MethodPost, "/api/chat", `{"question":"Am I overtraining right now?"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var got chatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Answer == "" || len(got.Sources) == 0 {
		t.Errorf("response: %+v", got)
	}
}

func TestChatRejectsEmptyQuestion(t *testing.T) {
	s := mockServer(t)
	id := newChat(t, s)
	for _, body := range []string{`{}`, `{"question":""}`, `not json`} {
		for _, path := range []string{"/api/chat", "/api/chats/" + id + "/stream"} {
			if rec := do(t, s, http.MethodPost, path, body); rec.Code != http.StatusBadRequest {
				t.Errorf("%s with body %q: got %d, want 400", path, body, rec.Code)
			}
		}
	}
}

// The daily cap is a cost guard: once reached, no reply is attempted.
func TestDailyCapReturns429(t *testing.T) {
	s := mockServer(t)
	id := newChat(t, s)
	owner, _ := s.store.Owner(httptest.NewRequest("GET", "/", nil).Context())
	// The stateless route counts too and has no step delay.
	for i := 0; i < owner.DailyQuestionCap; i++ {
		if rec := do(t, s, http.MethodPost, "/api/chat", `{"question":"Am I overtraining right now?"}`); rec.Code != http.StatusOK {
			t.Fatalf("question %d: %d", i, rec.Code)
		}
	}
	rec := do(t, s, http.MethodPost, "/api/chats/"+id+"/stream", `{"question":"one more"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("over the cap: got %d, want 429 (%s)", rec.Code, rec.Body.String())
	}
	var me struct {
		Usage struct{ Today, Cap int }
	}
	rec = do(t, s, http.MethodGet, "/api/me", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Usage.Today != owner.DailyQuestionCap || me.Usage.Cap != owner.DailyQuestionCap {
		t.Errorf("/me usage: %+v", me.Usage)
	}
}

// Outside mock mode every /api route needs a session, login hands one out,
// a wrong password gets nothing, and logout ends it.
func TestLoginSessionAndLogout(t *testing.T) {
	s := liveServer(t, "correct horse")

	if rec := do(t, s, http.MethodGet, "/api/me", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("/me without a session: %d", rec.Code)
	}
	if rec := do(t, s, http.MethodPost, "/api/login", `{"email":"leo@example.com","password":"wrong"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: %d", rec.Code)
	}
	rec := do(t, s, http.MethodPost, "/api/login", `{"email":"Leo@Example.com","password":"correct horse"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	if cookie == nil || cookie.Value == "" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie: %+v", cookie)
	}

	rec = do(t, s, http.MethodGet, "/api/me", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("/me with a session: %d %s", rec.Code, rec.Body.String())
	}
	var me struct {
		Runner struct {
			Email, Kind string
		}
		Garmin garminState
		Mock   bool
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Garmin.Status != "none" || me.Garmin.Fix == "" || me.Mock || me.Runner.Email != "leo@example.com" || me.Runner.Kind != "owner" {
		t.Errorf("/me: %+v", me)
	}
	// The password-only form from before accounts still signs the owner in.
	if rec := do(t, s, http.MethodPost, "/api/login", `{"password":"correct horse"}`); rec.Code != http.StatusOK {
		t.Errorf("password-only owner login: %d", rec.Code)
	}

	id := newChat(t, s, cookie)
	// No API key: refused with a status before the stream opens, so nothing
	// is spent on screening a question that cannot be answered.
	rec = do(t, s, http.MethodPost, "/api/chats/"+id+"/stream", `{"question":"hi"}`, cookie)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "ANTHROPIC_API_KEY") {
		t.Errorf("stream without a key: %d %s", rec.Code, rec.Body.String())
	}

	if rec := do(t, s, http.MethodPost, "/api/logout", "", cookie); rec.Code != http.StatusOK {
		t.Errorf("logout: %d", rec.Code)
	}
	if rec := do(t, s, http.MethodGet, "/api/me", "", cookie); rec.Code != http.StatusUnauthorized {
		t.Errorf("/me after logout: %d", rec.Code)
	}
}

// Failures are limited; successes are not counted, so signing in and out
// repeatedly never locks a runner out.
func TestLoginIsRateLimited(t *testing.T) {
	s := liveServer(t, "correct horse")
	for i := 0; i < 8; i++ {
		if rec := do(t, s, http.MethodPost, "/api/login", `{"password":"correct horse"}`); rec.Code != http.StatusOK {
			t.Fatalf("successful sign-in %d: %d", i+1, rec.Code)
		}
	}
	var last int
	for i := 0; i < 6; i++ {
		last = do(t, s, http.MethodPost, "/api/login", `{"password":"nope"}`).Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("sixth failure: got %d, want 429", last)
	}
	// Once blocked, even the right password waits out the window.
	if rec := do(t, s, http.MethodPost, "/api/login", `{"password":"correct horse"}`); rec.Code != http.StatusTooManyRequests {
		t.Errorf("right password while blocked: %d", rec.Code)
	}
}

// A pasted bundle that is not a bundle is refused before anything is spawned.
func TestGarminTokensRejectsGarbage(t *testing.T) {
	s := liveServer(t, "pw")
	s.cfg.TokenKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	s = New(s.cfg, nil)
	rec := do(t, s, http.MethodPost, "/api/login", `{"password":"pw"}`)
	cookie := rec.Result().Cookies()[0]
	rec = do(t, s, http.MethodPost, "/api/garmin/tokens", `{"bundle":"abc"}`, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("garbage bundle: %d %s", rec.Code, rec.Body.String())
	}
}

// stubSource stands in for the Garmin spawn: it accepts one bundle.
type stubSource struct{ accept string }

func (st stubSource) Connect(context.Context, int64) (*mcp.ClientSession, func(), error) {
	return nil, nil, garmin.ErrUnavailable
}
func (st stubSource) Probe(_ context.Context, bundle string) (string, error) {
	if bundle == st.accept {
		return "Leo", nil
	}
	return "", garmin.ErrAuth
}

// A bundle file written by the laptop sign-in connects the owner on start,
// with no paste; a bundle the probe rejects stores nothing; the same bundle
// again is left alone.
func TestGarminBundleImportedOnStart(t *testing.T) {
	good := base64.StdEncoding.EncodeToString([]byte(`{"di_token":"` + strings.Repeat("x", 600) + `","di_refresh_token":"r","di_client_id":"c"}`))
	bad := base64.StdEncoding.EncodeToString([]byte(`{"di_token":"` + strings.Repeat("y", 600) + `","di_refresh_token":"r","di_client_id":"c"}`))
	dir := t.TempDir()
	file := filepath.Join(dir, "bundle.b64")
	if err := os.WriteFile(file, []byte(good+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	key, _ := crypto.NewKey()
	hash, _ := auth.HashPassword("pw")
	cfg := &config.Config{OwnerPasswordHash: hash, TokenKey: key, GarminBundleFile: file, GarminMCPCommand: "false"}

	s := New(cfg, nil) // the real spawn fails, so the import logs and stores nothing
	s.garmin = stubSource{accept: good}
	if err := s.importGarminBundle(context.Background()); err != nil {
		t.Fatalf("import: %v", err)
	}
	conn, _ := s.store.GarminConnection(context.Background(), 1)
	if conn == nil || conn.Status != store.GarminConnected {
		t.Fatalf("not connected after import: %+v", conn)
	}
	first := conn.ImportedAt
	if err := s.importGarminBundle(context.Background()); err != nil {
		t.Fatalf("second import: %v", err)
	}
	if conn, _ = s.store.GarminConnection(context.Background(), 1); !conn.ImportedAt.Equal(first) {
		t.Error("an identical bundle was re-imported")
	}

	s.cfg.GarminBundle = bad
	if err := s.importGarminBundle(context.Background()); err == nil {
		t.Error("a rejected bundle imported without error")
	}
	if conn, _ = s.store.GarminConnection(context.Background(), 1); !conn.ImportedAt.Equal(first) {
		t.Error("a rejected bundle replaced the stored one")
	}

	// A missing file is simply nothing to import.
	s.cfg.GarminBundle, s.cfg.GarminBundleFile = "", filepath.Join(dir, "absent")
	if err := s.importGarminBundle(context.Background()); err != nil {
		t.Errorf("missing file: %v", err)
	}
}

// Health never needs a session, reports db off without a pool, and the Garmin
// state from the row.
func TestHealthz(t *testing.T) {
	for _, path := range []string{"/healthz", "/api/healthz"} {
		rec := do(t, liveServer(t, "pw"), http.MethodGet, path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		var got map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got["status"] != "ok" || got["db"] != "off" || got["garmin"] != "none" {
			t.Errorf("%s: %v", path, got)
		}
	}
}

// The UI is served from the same origin for every non-API path, with the
// index for unknown routes (history mode).
func TestEmbeddedUIFallsBackToIndex(t *testing.T) {
	for _, path := range []string{"/", "/c/abc", "/login"} {
		rec := do(t, mockServer(t), http.MethodGet, path, "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "RunCoach") {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
}

// CORS stays off unless an origin is configured.
func TestCORSOffByDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/chats", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	mockServer(t).Routes().ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS header set with no ALLOWED_ORIGIN")
	}
}

var _ = store.ErrNotFound
