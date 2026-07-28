package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// fakeModel is a scripted stand-in for the Anthropic Messages API. It lets the
// tests drive the tool-calling loop through a chosen plan — Strava only, Garmin
// only, or both — and then inspect what the agent actually did with it, without
// any live API call.
//
// What this can and cannot prove: it verifies the agent correctly *executes* a
// multi-source plan (routing, parallel tool_use in one turn, feeding results
// back, reporting sources). Whether the real model *chooses* the right plan for
// a given question is the model's judgment; that is checked by the opt-in live
// test and, for real, in Phase 5.

// fakeToolUse is one tool the scripted model asks for.
type fakeToolUse struct {
	Name  string
	Input map[string]any
}

// fakeTurn is one scripted model response: a set of tool_use blocks, or — when
// ToolUses is empty — a final text answer that ends the loop.
type fakeTurn struct {
	ToolUses []fakeToolUse
	Text     string
}

// fakeRequest captures one request the agent sent to the model.
type fakeRequest struct {
	System string   // the system prompt
	Tools  []string // tool names offered, in order
	Body   string   // raw request JSON, for asserting tool results were fed back
}

type fakeModel struct {
	mu       sync.Mutex
	turns    []fakeTurn
	requests []fakeRequest
}

// Requests returns the requests the agent made, in order.
func (m *fakeModel) Requests() []fakeRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]fakeRequest(nil), m.requests...)
}

// LastRequest returns the final request the agent made. It fails the test if the
// agent never called the model.
func (m *fakeModel) LastRequest(t *testing.T) fakeRequest {
	t.Helper()
	reqs := m.Requests()
	if len(reqs) == 0 {
		t.Fatal("agent never called the model")
	}
	return reqs[len(reqs)-1]
}

// next pops the next scripted turn.
func (m *fakeModel) next() (fakeTurn, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.turns) == 0 {
		return fakeTurn{}, false
	}
	turn := m.turns[0]
	m.turns = m.turns[1:]
	return turn, true
}

func (m *fakeModel) record(r fakeRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, r)
}

// startFakeModel serves the scripted turns over HTTP and returns a client
// pointed at it. Environment defaults are skipped so a real ANTHROPIC_API_KEY or
// ANTHROPIC_BASE_URL on the machine cannot leak into the test.
func startFakeModel(t *testing.T, turns ...fakeTurn) (anthropic.Client, *fakeModel) {
	t.Helper()
	model := &fakeModel{turns: turns}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read model request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var parsed struct {
			System []struct{ Text string } `json:"system"`
			Tools  []struct{ Name string } `json:"tools"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Errorf("decode model request: %v\n%s", err, body)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req := fakeRequest{Body: string(body)}
		for _, s := range parsed.System {
			req.System += s.Text
		}
		for _, tool := range parsed.Tools {
			req.Tools = append(req.Tools, tool.Name)
		}
		model.record(req)

		turn, ok := model.next()
		if !ok {
			// The agent asked for one more turn than the test scripted, which
			// almost always means the loop is not terminating where expected.
			t.Errorf("agent made an unscripted model call (request %d)", len(model.Requests()))
			turn = fakeTurn{Text: "unscripted"}
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, turn.responseJSON(t)); err != nil {
			t.Errorf("write model response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := anthropic.NewClient(
		option.WithoutEnvironmentDefaults(),
		option.WithBaseURL(server.URL),
		option.WithAPIKey("test-key"),
		option.WithMaxRetries(0),
	)
	return client, model
}

// responseJSON renders the turn as a Messages API response.
func (turn fakeTurn) responseJSON(t *testing.T) string {
	t.Helper()

	var blocks []string
	stopReason := "end_turn"
	if len(turn.ToolUses) > 0 {
		stopReason = "tool_use"
		for i, use := range turn.ToolUses {
			input, err := json.Marshal(use.Input)
			if err != nil {
				t.Fatalf("marshal tool input for %s: %v", use.Name, err)
			}
			if use.Input == nil {
				input = []byte("{}")
			}
			blocks = append(blocks, fmt.Sprintf(
				`{"type":"tool_use","id":"toolu_%d","name":%q,"input":%s}`, i, use.Name, input))
		}
	} else {
		text, err := json.Marshal(turn.Text)
		if err != nil {
			t.Fatalf("marshal turn text: %v", err)
		}
		blocks = append(blocks, fmt.Sprintf(`{"type":"text","text":%s}`, text))
	}

	return fmt.Sprintf(`{
		"id": "msg_test",
		"type": "message",
		"role": "assistant",
		"model": "test-model",
		"content": [%s],
		"stop_reason": %q,
		"usage": {"input_tokens": 1, "output_tokens": 1}
	}`, strings.Join(blocks, ","), stopReason)
}
