package api

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lmagsino/run-coach-ai/backend/internal/config"
)

// These tests cover the mock-mode and transport plumbing only: no database, no
// MCP subprocess, no Anthropic call. That is the whole point of mock mode — it
// is the path the frontend is built against before any credential exists.

// mockServer builds a Server wired for mock mode. The nil pool and clients are
// safe because mock mode returns before touching any of them; a test that
// reached them would panic rather than silently pass, which is the behaviour we
// want if that ever stops being true.
func mockServer(t *testing.T) *Server {
	t.Helper()
	return &Server{
		cfg: &config.Config{
			MockMode:      true,
			AllowedOrigin: "http://localhost:5173",
		},
		sources: []string{"strava", "garmin"},
	}
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

func postStream(t *testing.T, s *Server, question string) []sseEvent {
	t.Helper()
	body := strings.NewReader(`{"question":` + quote(question) + `}`)
	req := httptest.NewRequest(http.MethodPost, "/chat/stream", body)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

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
// tool call is announced active before it is announced done, and the answer
// arrives last.
func TestChatStreamEmitsStepsThenAnswer(t *testing.T) {
	events := postStream(t, mockServer(t), "Does my sleep actually affect my pace?")

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

	// Every step must be paired: an unmatched "active" leaves a step pulsing in
	// the UI forever.
	open := map[string]bool{}
	for _, ev := range events[:len(events)-1] {
		if ev.Name != "step" {
			t.Fatalf("unexpected event %q before the answer", ev.Name)
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

// The sleep-vs-pace question is the product's premise (spec §3), so its canned
// scenario must genuinely reach both sources — a mock that quietly answered from
// one would hide the cross-source UI we are building.
func TestChatStreamSleepQuestionUsesBothSources(t *testing.T) {
	events := postStream(t, mockServer(t), "Does my sleep actually affect my pace?")

	seen := map[string]bool{}
	for _, ev := range events {
		if ev.Name == "step" {
			seen[ev.Data["source"].(string)] = true
		}
	}
	for _, want := range []string{"strava", "garmin"} {
		if !seen[want] {
			t.Errorf("no %s step in the stream; got %v", want, seen)
		}
	}

	answer := events[len(events)-1].Data
	sources, _ := answer["sources"].([]any)
	if len(sources) != 2 {
		t.Errorf("answer sources: got %v, want both", sources)
	}
}

// A single-source question must not drag the other source in, or the status
// steps would misreport what was consulted.
func TestChatStreamTrainingBlockQuestionIsStravaOnly(t *testing.T) {
	events := postStream(t, mockServer(t), "How did my last 3 marathon training blocks compare?")

	for _, ev := range events {
		if ev.Name == "step" && ev.Data["source"] != "strava" {
			t.Errorf("unexpected %v step for a training-log question", ev.Data["source"])
		}
	}
}

// An unrecognised question must say it has no answer rather than return a
// plausible-sounding one: mock data presented as real is the failure mode that
// would embarrass a demo.
func TestChatStreamUnknownQuestionAdmitsItIsMocked(t *testing.T) {
	events := postStream(t, mockServer(t), "what is the capital of France?")

	answer, _ := events[len(events)-1].Data["answer"].(string)
	if !strings.Contains(strings.ToLower(answer), "mock mode") {
		t.Errorf("fallback answer should say it is mocked, got %q", answer)
	}
}

// Mock mode must not require the credentials the real path does — that is what
// makes it usable in Phases 2-4.
func TestChatMockModeNeedsNoCredentials(t *testing.T) {
	s := mockServer(t)
	req := httptest.NewRequest(http.MethodPost, "/chat",
		strings.NewReader(`{"question":"Am I overtraining right now?"}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var got chatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Answer == "" {
		t.Error("no answer returned")
	}
	if len(got.Sources) == 0 {
		t.Error("no sources reported")
	}
}

func TestChatRejectsEmptyQuestion(t *testing.T) {
	for _, body := range []string{`{}`, `{"question":""}`, `not json`} {
		for _, path := range []string{"/chat", "/chat/stream"} {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
			rec := httptest.NewRecorder()
			mockServer(t).Routes().ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s with body %q: got %d, want 400", path, body, rec.Code)
			}
		}
	}
}

// The UI describes what it is connected to from this endpoint, so it must report
// the registered sources rather than a hardcoded pair.
func TestSourcesReportsRegisteredSourcesAndMockFlag(t *testing.T) {
	s := mockServer(t)
	s.sources = []string{"strava"} // Garmin not configured

	req := httptest.NewRequest(http.MethodGet, "/sources", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	var got sourcesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Sources) != 1 || got.Sources[0] != "strava" {
		t.Errorf("sources: got %v, want [strava]", got.Sources)
	}
	if !got.Mock {
		t.Error("mock flag should be true")
	}
}

// The browser will not read any response without these, and it sends a preflight
// for the JSON content type first.
func TestCORSAllowsTheConfiguredOriginOnly(t *testing.T) {
	s := mockServer(t)

	tests := []struct {
		name, origin, wantAllow string
	}{
		{"configured origin", "http://localhost:5173", "http://localhost:5173"},
		{"other origin", "http://evil.example", ""},
		{"no origin", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/chat/stream", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			s.Routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Errorf("preflight status: got %d, want 204", rec.Code)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.wantAllow {
				t.Errorf("Allow-Origin: got %q, want %q", got, tc.wantAllow)
			}
		})
	}
}

// The client holds the thread, so history has to survive the HTTP boundary
// intact for follow-ups to work at all. Mock mode ignores it when answering, so
// this asserts the decode rather than the behaviour — the model's use of it is
// covered in internal/agent and, live, in Phase 5.
func TestRequestHistoryDecodesInOrder(t *testing.T) {
	body := `{"question":"and the week before?","history":[
		{"question":"q1","answer":"a1"},
		{"question":"q2","answer":"a2"}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	rec := httptest.NewRecorder()

	var got chatRequest
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.History) != 2 {
		t.Fatalf("history: got %d turns, want 2", len(got.History))
	}
	prior := got.prior()
	if prior[0].Question != "q1" || prior[0].Answer != "a1" {
		t.Errorf("first prior turn: got %+v", prior[0])
	}
	if prior[1].Question != "q2" || prior[1].Answer != "a2" {
		t.Errorf("second prior turn: got %+v", prior[1])
	}

	// And a request carrying history must still be served.
	mockServer(t).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
}

// A missing history field must behave as an empty conversation, not an error —
// /chat is documented as curl-friendly with just a question.
func TestRequestWithoutHistoryIsValid(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/chat",
		strings.NewReader(`{"question":"am I overtraining right now?"}`))
	rec := httptest.NewRecorder()
	mockServer(t).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
}
