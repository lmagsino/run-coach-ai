package agent

import (
	"context"
	"fmt"
	"testing"
)

// The events the UI narrates must describe tool calls that really happened, in
// the order they happened, and each start must be paired with an end. Without
// this the status steps could claim a source was consulted when it wasn't —
// exactly the dishonesty the visible-reasoning feature exists to avoid.
func TestObserverReportsEveryToolCallInOrder(t *testing.T) {
	strava, garmin, _, _ := stubbedSources(t)

	client, _ := startFakeModel(t,
		// Two tools in one turn, then a third on a later turn: covers both the
		// parallel and sequential shapes the prompt allows.
		fakeTurn{ToolUses: []fakeToolUse{
			{Name: "garmin__get_sleep_data"},
			{Name: "strava__get_activities"},
		}},
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_activities"}}},
		fakeTurn{Text: "final answer"},
	)

	ag := New(client, "test-model", strava, garmin)

	var got []string
	ag.Observe(func(ev Event) {
		got = append(got, fmt.Sprintf("%s %s/%s failed=%t", ev.Kind, ev.Source, ev.Tool, ev.Failed))
	})

	result, err := ag.Answer(context.Background(), "am I recovered enough to race?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}

	want := []string{
		"tool_start garmin/get_sleep_data failed=false",
		"tool_end garmin/get_sleep_data failed=false",
		"tool_start strava/get_activities failed=false",
		"tool_end strava/get_activities failed=false",
		"tool_start garmin/get_activities failed=false",
		"tool_end garmin/get_activities failed=false",
	}
	assertEqualStrings(t, "events", got, want)

	// The event trail and the reported sources describe the same work, so they
	// must not be able to disagree.
	assertEqualStrings(t, "sources", result.Sources(), []string{"garmin", "strava"})
}

// A failing tool must still close its step, marked failed. If the end event were
// skipped the UI would leave that step pulsing forever, reading as "still
// working" when the call is already dead.
func TestObserverMarksFailedToolCalls(t *testing.T) {
	session, _ := startStub(t, "garmin-mcp",
		stubTool{name: "get_sleep_data", description: "Nightly sleep.", fail: true},
	)
	garmin := Source{Name: "garmin", Session: session}

	client, _ := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_sleep_data"}}},
		fakeTurn{Text: "I could not read your sleep data."},
	)

	ag := New(client, "test-model", garmin)

	var got []string
	ag.Observe(func(ev Event) {
		got = append(got, fmt.Sprintf("%s failed=%t", ev.Kind, ev.Failed))
	})

	if _, err := ag.Answer(context.Background(), "how did I sleep?"); err != nil {
		t.Fatalf("Answer: %v", err)
	}

	assertEqualStrings(t, "events", got, []string{
		"tool_start failed=false",
		"tool_end failed=true",
	})
}

// An agent with no observer must behave exactly as before — the whole existing
// /chat path relies on it.
func TestAnswerWorksWithoutObserver(t *testing.T) {
	strava, garmin, _, _ := stubbedSources(t)
	client, _ := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "strava__get_activities"}}},
		fakeTurn{Text: "final answer"},
	)
	ag := New(client, "test-model", strava, garmin)

	result, err := ag.Answer(context.Background(), "how far did I run?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if result.Answer != "final answer" {
		t.Errorf("answer: got %q, want %q", result.Answer, "final answer")
	}
}
