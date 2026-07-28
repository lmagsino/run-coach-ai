package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
)

// stubbedSources returns a Strava stub and a Garmin stub with distinguishable
// canned payloads, plus their call logs.
func stubbedSources(t *testing.T) (strava, garmin Source, stravaLog, garminLog *callLog) {
	t.Helper()
	stravaSession, stravaLog := startStub(t, "strava-mcp",
		stubTool{name: "get_activities", description: "List Strava activities.",
			result: `{"activities":[{"date":"2026-07-20","distance_km":21.1,"pace":"4:45/km"}]}`},
	)
	garminSession, garminLog := startStub(t, "garmin-mcp",
		stubTool{name: "get_activities", description: "List Garmin activities.",
			result: `{"activities":[{"date":"2026-07-20","distance_km":21.0}]}`},
		stubTool{name: "get_sleep_data", description: "Nightly sleep.",
			result: `{"nights":[{"date":"2026-07-19","hours":6.2}]}`},
	)
	return Source{Name: mcpclient.SourceStrava, Session: stravaSession},
		Source{Name: mcpclient.SourceGarmin, Session: garminSession},
		stravaLog, garminLog
}

// Each scripted plan must execute end to end and be reported accurately: the
// sources a caller is told about come from tool calls that really happened.
func TestAnswerExecutesEachSourcePlan(t *testing.T) {
	tests := []struct {
		name        string
		toolUses    []fakeToolUse
		wantSources []string
		wantStrava  []string
		wantGarmin  []string
	}{
		{
			name:        "strava only",
			toolUses:    []fakeToolUse{{Name: "strava__get_activities", Input: map[string]any{"per_page": 10}}},
			wantSources: []string{"strava"},
			wantStrava:  []string{"get_activities"},
		},
		{
			name:        "garmin only",
			toolUses:    []fakeToolUse{{Name: "garmin__get_sleep_data", Input: map[string]any{"days": 7}}},
			wantSources: []string{"garmin"},
			wantGarmin:  []string{"get_sleep_data"},
		},
		{
			// Both tools requested in a single turn — the parallel case the
			// prompt encourages for cross-source questions.
			name: "both sources in one turn",
			toolUses: []fakeToolUse{
				{Name: "strava__get_activities"},
				{Name: "garmin__get_sleep_data", Input: map[string]any{"days": 30}},
			},
			wantSources: []string{"strava", "garmin"},
			wantStrava:  []string{"get_activities"},
			wantGarmin:  []string{"get_sleep_data"},
		},
		{
			// No tool at all: a question the model can answer without data
			// must not be reported as having consulted a source.
			name:        "no tools",
			wantSources: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			strava, garmin, stravaLog, garminLog := stubbedSources(t)

			var turns []fakeTurn
			if len(tc.toolUses) > 0 {
				turns = append(turns, fakeTurn{ToolUses: tc.toolUses})
			}
			turns = append(turns, fakeTurn{Text: "final answer"})

			client, _ := startFakeModel(t, turns...)
			ag := New(client, "test-model", strava, garmin)

			result, err := ag.Answer(context.Background(), "how is my training going?")
			if err != nil {
				t.Fatalf("Answer: %v", err)
			}
			if result.Answer != "final answer" {
				t.Errorf("answer: got %q, want %q", result.Answer, "final answer")
			}
			assertEqualStrings(t, "sources", result.Sources(), tc.wantSources)
			assertEqualStrings(t, "strava stub calls", stravaLog.Tools(), tc.wantStrava)
			assertEqualStrings(t, "garmin stub calls", garminLog.Tools(), tc.wantGarmin)
		})
	}
}

// A source chosen on a later turn, after seeing the first source's data, is the
// sequential half of cross-source reasoning.
func TestAnswerFollowsUpWithSecondSource(t *testing.T) {
	strava, garmin, stravaLog, garminLog := stubbedSources(t)

	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_sleep_data", Input: map[string]any{"days": 30}}}},
		fakeTurn{ToolUses: []fakeToolUse{{Name: "strava__get_activities"}}},
		fakeTurn{Text: "6.2h sleep on 2026-07-19, and the 2026-07-20 half was run at 4:45/km."},
	)
	ag := New(client, "test-model", strava, garmin)

	result, err := ag.Answer(context.Background(), "does my sleep affect my pace?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}

	assertEqualStrings(t, "sources", result.Sources(), []string{"garmin", "strava"})
	assertEqualStrings(t, "garmin stub calls", garminLog.Tools(), []string{"get_sleep_data"})
	assertEqualStrings(t, "strava stub calls", stravaLog.Tools(), []string{"get_activities"})

	// Both sources' payloads must reach the model, or it would be answering
	// a cross-source question from one side of the data.
	last := model.LastRequest(t)
	for _, want := range []string{`\"hours\":6.2`, `\"pace\":\"4:45/km\"`} {
		if !strings.Contains(last.Body, want) {
			t.Errorf("final request to the model is missing %s", want)
		}
	}
}

// A hallucinated tool name must come back as a tool error the model can recover
// from, not kill the request.
func TestAnswerReportsUnknownToolToModel(t *testing.T) {
	strava, garmin, stravaLog, garminLog := stubbedSources(t)

	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_readiness"}}},
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_sleep_data"}}},
		fakeTurn{Text: "recovered"},
	)
	ag := New(client, "test-model", strava, garmin)

	result, err := ag.Answer(context.Background(), "am I recovered?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if result.Answer != "recovered" {
		t.Errorf("answer: got %q, want %q", result.Answer, "recovered")
	}
	// The bad call reached no server and is not reported as a source.
	assertEqualStrings(t, "sources", result.Sources(), []string{"garmin"})
	assertEqualStrings(t, "garmin stub calls", garminLog.Tools(), []string{"get_sleep_data"})
	assertEqualStrings(t, "strava stub calls", stravaLog.Tools(), nil)

	if body := model.Requests()[1].Body; !strings.Contains(body, "no such tool") {
		t.Errorf("model was not told the tool does not exist; request was:\n%s", body)
	}
}

// A tool that fails must be reported as failed rather than counted as a clean
// read of that source.
func TestAnswerMarksFailedToolCall(t *testing.T) {
	failing, _ := startStub(t, "garmin-mcp",
		stubTool{name: "get_sleep_data", description: "Nightly sleep.", result: "", fail: true},
	)
	client, _ := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_sleep_data"}}},
		fakeTurn{Text: "sleep data unavailable"},
	)
	ag := New(client, "test-model", Source{Name: mcpclient.SourceGarmin, Session: failing})

	result, err := ag.Answer(context.Background(), "how did I sleep?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if len(result.ToolCalls) != 1 || !result.ToolCalls[0].Failed {
		t.Fatalf("tool calls: got %+v, want one failed call", result.ToolCalls)
	}
}

// The loop must not spin forever if the model keeps asking for tools.
func TestAnswerGivesUpAfterMaxTurns(t *testing.T) {
	strava, garmin, _, _ := stubbedSources(t)

	turns := make([]fakeTurn, maxTurns)
	for i := range turns {
		turns[i] = fakeTurn{ToolUses: []fakeToolUse{{Name: "strava__get_activities"}}}
	}
	client, _ := startFakeModel(t, turns...)
	ag := New(client, "test-model", strava, garmin)

	if _, err := ag.Answer(context.Background(), "loop forever"); err == nil {
		t.Fatal("expected an error after maxTurns tool-use turns")
	}
}

// The prompt is what actually makes tool selection the model's decision, so its
// contents are worth asserting: both sources described, and the multi-source
// rules present only when there is a choice to make.
func TestSystemPromptDescribesRegisteredSourcesOnly(t *testing.T) {
	strava, garmin, _, _ := stubbedSources(t)

	both := New(anthropic.Client{}, "test-model", strava, garmin).systemPrompt()
	for _, want := range []string{"- strava —", "- garmin —", "sleep", "Choosing which sources to call"} {
		if !strings.Contains(both, want) {
			t.Errorf("two-source prompt is missing %q:\n%s", want, both)
		}
	}

	// With Garmin unconfigured the prompt must not advertise recovery data the
	// agent has no way to fetch, and the selection rules are just noise.
	stravaOnly := New(anthropic.Client{}, "test-model", strava).systemPrompt()
	for _, unwanted := range []string{"garmin", "Choosing which sources to call"} {
		if strings.Contains(stravaOnly, unwanted) {
			t.Errorf("single-source prompt should not mention %q:\n%s", unwanted, stravaOnly)
		}
	}
}

func assertEqualStrings(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %v, want %v", label, got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s: got %v, want %v", label, got, want)
			return
		}
	}
}
