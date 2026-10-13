package agent

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
)

// Grounding scenarios: the product's premise is answering questions that need
// several Garmin tools together, with every number in the reply traceable to a
// tool result. These tests use fabricated datasets — no live account, no real
// numbers — with a signal planted across two tools so that an answer drawing on
// only one of them is provably incomplete.

// sleepNight is a night in the fake sleep dataset.
type sleepNight struct {
	Date      string  `json:"date"`
	Hours     float64 `json:"sleep_hours"`
	HRV       int     `json:"hrv_ms"`
	RestingHR int     `json:"resting_hr"`
}

// longRun is a run in the fake activities dataset.
type longRun struct {
	Date       string  `json:"date"`
	DistanceKM float64 `json:"distance_km"`
	Pace       string  `json:"pace_per_km"`
	AvgHR      int     `json:"average_heartrate"`
}

// The planted pattern: the four nights under 6.2 hours (with HRV in the 40s)
// each precede a long run roughly 25 s/km slower than the well-slept weeks, at
// the same distance. Neither tool shows this alone — activities have the paces
// but no sleep, sleep has the nights but not the pace the agent should cite.
var (
	fakeSleepNights = []sleepNight{
		{Date: "2026-06-05", Hours: 7.8, HRV: 62, RestingHR: 44},
		{Date: "2026-06-12", Hours: 8.1, HRV: 65, RestingHR: 43},
		{Date: "2026-06-19", Hours: 5.9, HRV: 48, RestingHR: 52},
		{Date: "2026-06-26", Hours: 6.1, HRV: 51, RestingHR: 50},
		{Date: "2026-07-03", Hours: 7.9, HRV: 63, RestingHR: 44},
		{Date: "2026-07-10", Hours: 8.2, HRV: 66, RestingHR: 42},
		{Date: "2026-07-17", Hours: 5.7, HRV: 45, RestingHR: 53},
		{Date: "2026-07-24", Hours: 6.0, HRV: 49, RestingHR: 51},
	}

	fakeLongRuns = []longRun{
		{Date: "2026-06-06", DistanceKM: 16.0, Pace: "4:38/km", AvgHR: 148},
		{Date: "2026-06-13", DistanceKM: 16.1, Pace: "4:35/km", AvgHR: 146},
		{Date: "2026-06-20", DistanceKM: 16.0, Pace: "5:02/km", AvgHR: 157},
		{Date: "2026-06-27", DistanceKM: 16.2, Pace: "4:58/km", AvgHR: 155},
		{Date: "2026-07-04", DistanceKM: 16.0, Pace: "4:36/km", AvgHR: 147},
		{Date: "2026-07-11", DistanceKM: 16.1, Pace: "4:33/km", AvgHR: 145},
		{Date: "2026-07-18", DistanceKM: 16.0, Pace: "5:06/km", AvgHR: 159},
		{Date: "2026-07-25", DistanceKM: 16.1, Pace: "5:01/km", AvgHR: 156},
	}
)

// garminStub wires one Garmin stub holding the fake runs and the fake nights,
// returning the source plus the exact payload text each tool serves (so tests
// can assert that text reached the model).
func garminStub(t *testing.T) (garmin Source, runsPayload, sleepPayload string) {
	t.Helper()

	runs := jsonTool(t, "get_activities_by_date",
		"List activities in a date range with distance, pace and average heart rate.",
		map[string]any{"activities": fakeLongRuns})
	sleep := jsonTool(t, "get_sleep_summary_range",
		"Nightly sleep duration, HRV and resting heart rate over a date range.",
		map[string]any{"nights": fakeSleepNights})

	session, _ := startStub(t, "garmin-mcp", runs, sleep)
	return Source{Name: mcpclient.SourceGarmin, Session: session, Allow: mcpclient.ReadOnly},
		runs.result, sleep.result
}

// "Does my sleep actually affect my pace?" — the headline question. Both
// datasets must land in the same model context, otherwise the agent is
// correlating against data it cannot see.
func TestSleepVersusPaceCombinesBothTools(t *testing.T) {
	garmin, runsPayload, sleepPayload := garminStub(t)

	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{
			{Name: "garmin__get_sleep_summary_range", Input: map[string]any{"start": "2026-06-01", "end": "2026-07-31"}},
			{Name: "garmin__get_activities_by_date", Input: map[string]any{"start": "2026-06-01", "end": "2026-07-31"}},
		}},
		fakeTurn{Text: "Your four nights under 6.2h preceded long runs around 5:00/km, " +
			"about 25s/km slower than the 4:35/km weeks after 8h sleep."},
	)
	ag := New(client, "test-model", garmin)

	result, err := ag.Answer(context.Background(), "Does my sleep actually affect my pace? Show me the pattern.")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	assertEqualStrings(t, "sources", result.Sources(), []string{"garmin"})
	if len(result.ToolCalls) != 2 {
		t.Fatalf("tool calls: got %+v, want 2", result.ToolCalls)
	}

	// Both tool results must be present verbatim in the request that produced
	// the answer — the actual definition of "combined in one context".
	final := model.LastRequest(t)
	assertBodyContains(t, final.Body, "sleep dataset", sleepPayload)
	assertBodyContains(t, final.Body, "run dataset", runsPayload)

	// And the model was offered both tools in the first place, plus the chart.
	first := model.Requests()[0]
	assertEqualStrings(t, "tools offered", first.Tools,
		[]string{"garmin__get_activities_by_date", "garmin__get_sleep_summary_range", "coach__show_chart"})
}

// "Am I overtraining?" — the agent reaches for the second tool only after
// seeing the first, so the sequential path must combine too.
func TestOvertrainingSequential(t *testing.T) {
	loadPayload := `{"acute_load":412,"chronic_load":268,"acute_chronic_ratio":1.54,"status":"overreaching"}`
	runs := jsonTool(t, "get_activities_by_date", "List activities.", map[string]any{"activities": fakeLongRuns})
	session, _ := startStub(t, "garmin-mcp",
		runs,
		stubTool{name: "get_training_load_trend", description: "Acute vs chronic training load.", result: loadPayload},
	)

	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_training_load_trend"}}},
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_activities_by_date", Input: map[string]any{"start": "2026-06-01", "end": "2026-07-31"}}}},
		fakeTurn{Text: "Acute:chronic load is 1.54 (overreaching) and your last two long runs " +
			"slowed to 5:06/km and 5:01/km at a higher average HR."},
	)
	ag := New(client, "test-model", Source{Name: mcpclient.SourceGarmin, Session: session})

	result, err := ag.Answer(context.Background(), "Am I overtraining right now?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if len(result.ToolCalls) != 2 {
		t.Fatalf("tool calls: got %+v, want 2", result.ToolCalls)
	}
	final := model.LastRequest(t)
	assertBodyContains(t, final.Body, "training load", loadPayload)
	assertBodyContains(t, final.Body, "run dataset", runs.result)
}

// A question about distance alone must not drag in sleep data: it wastes tokens
// and the reply would rest on data it never needed.
func TestSingleToolQuestionStaysOnOneTool(t *testing.T) {
	garmin, runsPayload, sleepPayload := garminStub(t)

	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_activities_by_date", Input: map[string]any{"start": "2026-06-01", "end": "2026-07-31"}}}},
		fakeTurn{Text: "You ran 16 km each week for the last 8 weeks."},
	)
	ag := New(client, "test-model", garmin)

	result, err := ag.Answer(context.Background(), "how far were my long runs?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].Tool != "get_activities_by_date" {
		t.Fatalf("tool calls: got %+v, want one get_activities_by_date", result.ToolCalls)
	}
	final := model.LastRequest(t)
	assertBodyContains(t, final.Body, "run dataset", runsPayload)
	if bodyContains(final.Body, sleepPayload) {
		t.Error("sleep data reached the model for a distance-only question")
	}
}

// TestGroundingWithLiveModel is the one test that asks the real model to
// *choose* its tools; everything above verifies our execution of a plan the
// test scripted. The tool data is still fabricated, so no Garmin account is
// involved — only the Anthropic API.
//
// Skipped unless explicitly enabled, so the default `go test ./...` makes no
// network call and costs nothing:
//
//	RUNCOACH_LIVE_MODEL_TESTS=1 ANTHROPIC_API_KEY=sk-ant-... go test ./internal/agent/ -run LiveModel -v
func TestGroundingWithLiveModel(t *testing.T) {
	if os.Getenv("RUNCOACH_LIVE_MODEL_TESTS") != "1" {
		t.Skip("set RUNCOACH_LIVE_MODEL_TESTS=1 to run against the real Claude API")
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY is not set")
	}

	garmin, _, _ := garminStub(t)
	model := anthropic.Model(getenvOr("ANTHROPIC_MODEL", "claude-opus-5-5"))
	ag := New(anthropic.NewClient(), model, garmin).WithEffort(os.Getenv("ANTHROPIC_EFFORT"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := ag.Answer(ctx, "Does my sleep actually affect my pace? Show me the pattern.")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	t.Logf("tool calls: %+v\nusage: %+v\nanswer:\n%s", result.ToolCalls, result.Usage, result.Answer)

	// The question cannot be answered from one tool, so the model should have
	// reached for both without being told to.
	called := map[string]bool{}
	for _, c := range result.ToolCalls {
		called[c.Tool] = true
	}
	for _, want := range []string{"get_activities_by_date", "get_sleep_summary_range"} {
		if !called[want] {
			t.Errorf("expected the model to call %s; calls were %+v", want, result.ToolCalls)
		}
	}

	// And the answer should cite figures from both datasets rather than
	// describing the correlation in the abstract.
	if !citesAny(result.Answer, "5:06", "5:02", "5:01", "4:35", "4:33", "4:38") {
		t.Errorf("answer cites no pace from the run dataset:\n%s", result.Answer)
	}
	if !citesAny(result.Answer, "5.7", "5.9", "6.0", "6.1", "7.8", "8.1", "8.2") {
		t.Errorf("answer cites no sleep duration from the sleep dataset:\n%s", result.Answer)
	}
}

// assertBodyContains checks that a tool result reached the model. The payload is
// embedded as a JSON string inside the request body, so it is compared in its
// escaped form.
func assertBodyContains(t *testing.T, body, label, payload string) {
	t.Helper()
	if !bodyContains(body, payload) {
		t.Errorf("%s never reached the model.\nwanted: %s\nrequest: %s", label, payload, body)
	}
}

func bodyContains(body, payload string) bool {
	quoted, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	return strings.Contains(body, string(quoted[1:len(quoted)-1]))
}

func citesAny(answer string, figures ...string) bool {
	for _, f := range figures {
		if strings.Contains(answer, f) {
			return true
		}
	}
	return false
}

func getenvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
