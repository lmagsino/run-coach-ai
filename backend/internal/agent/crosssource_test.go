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

// Cross-source scenarios: the value proposition from spec §3 is answering
// questions that need Strava and Garmin together. These tests use fabricated
// datasets — no live account, no real numbers — with a signal deliberately
// planted across the two sources so that an answer drawing on only one of them
// is provably incomplete.

// sleepNight is a night in the fake Garmin dataset.
type sleepNight struct {
	Date      string  `json:"date"`
	Hours     float64 `json:"sleep_hours"`
	HRV       int     `json:"hrv_ms"`
	RestingHR int     `json:"resting_hr"`
}

// longRun is a run in the fake Strava dataset.
type longRun struct {
	Date       string  `json:"date"`
	DistanceKM float64 `json:"distance_km"`
	Pace       string  `json:"pace_per_km"`
	AvgHR      int     `json:"average_heartrate"`
}

// The planted pattern: the four nights under 6.2 hours (with HRV in the 40s)
// each precede a long run roughly 25 s/km slower than the well-slept weeks, at
// the same distance. Neither source shows this alone — Strava has the paces but
// no sleep, Garmin has the sleep but not the pace the agent should cite.
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

// crossSourceStubs wires a Strava stub holding the fake runs and a Garmin stub
// holding the fake nights, returning both sources plus the exact payload text
// each tool serves (so tests can assert that text reached the model).
func crossSourceStubs(t *testing.T) (strava, garmin Source, runsPayload, sleepPayload string) {
	t.Helper()

	runs := jsonTool(t, "get_activities",
		"List the athlete's Strava activities with distance, pace and average heart rate.",
		map[string]any{"activities": fakeLongRuns})
	sleep := jsonTool(t, "get_sleep_data",
		"Nightly sleep duration, HRV and resting heart rate from the watch.",
		map[string]any{"nights": fakeSleepNights})

	stravaSession, _ := startStub(t, "strava-mcp", runs)
	garminSession, _ := startStub(t, "garmin-mcp", sleep)

	return Source{Name: mcpclient.SourceStrava, Session: stravaSession},
		Source{Name: mcpclient.SourceGarmin, Session: garminSession},
		runs.result, sleep.result
}

// "Does my sleep actually affect my pace?" — the headline cross-source question.
// Both datasets must land in the same model context, otherwise the agent is
// correlating against data it cannot see.
func TestCrossSourceSleepVersusPace(t *testing.T) {
	strava, garmin, runsPayload, sleepPayload := crossSourceStubs(t)

	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{
			{Name: "garmin__get_sleep_data", Input: map[string]any{"days": 60}},
			{Name: "strava__get_activities", Input: map[string]any{"per_page": 30}},
		}},
		fakeTurn{Text: "Your four nights under 6.2h preceded long runs around 5:00/km, " +
			"about 25s/km slower than the 4:35/km weeks after 8h sleep."},
	)
	ag := New(client, "test-model", strava, garmin)

	result, err := ag.Answer(context.Background(), "Does my sleep actually affect my pace? Show me the pattern.")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}

	assertEqualStrings(t, "sources", result.Sources(), []string{"garmin", "strava"})

	// Both tool results must be present verbatim in the request that produced
	// the answer — the actual definition of "combined in one context".
	final := model.LastRequest(t)
	assertBodyContains(t, final.Body, "Garmin sleep dataset", sleepPayload)
	assertBodyContains(t, final.Body, "Strava run dataset", runsPayload)

	// And the model was offered both tool sets to choose from in the first place.
	first := model.Requests()[0]
	assertEqualStrings(t, "tools offered", first.Tools,
		[]string{"strava__get_activities", "garmin__get_sleep_data"})
}

// "Am I overtraining?" — training load and recovery live in Garmin, the volume
// ramp that caused them lives in Strava. Here the agent reaches for the second
// source only after seeing the first, so the sequential path must combine too.
func TestCrossSourceOvertrainingSequential(t *testing.T) {
	loadPayload := `{"acute_load":412,"chronic_load":268,"acute_chronic_ratio":1.54,"status":"overreaching"}`
	garminSession, _ := startStub(t, "garmin-mcp",
		jsonTool(t, "get_sleep_data", "Nightly sleep.", map[string]any{"nights": fakeSleepNights}),
		stubTool{name: "get_training_load_trend", description: "Acute vs chronic training load.",
			result: loadPayload},
	)
	stravaTool := jsonTool(t, "get_activities", "List Strava activities.",
		map[string]any{"activities": fakeLongRuns})
	stravaSession, _ := startStub(t, "strava-mcp", stravaTool)

	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_training_load_trend"}}},
		fakeTurn{ToolUses: []fakeToolUse{{Name: "strava__get_activities", Input: map[string]any{"per_page": 30}}}},
		fakeTurn{Text: "Acute:chronic load is 1.54 (overreaching) and your last two long runs " +
			"slowed to 5:06/km and 5:01/km at a higher average HR."},
	)
	ag := New(client, "test-model",
		Source{Name: mcpclient.SourceStrava, Session: stravaSession},
		Source{Name: mcpclient.SourceGarmin, Session: garminSession},
	)

	result, err := ag.Answer(context.Background(), "Am I overtraining right now?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}

	assertEqualStrings(t, "sources", result.Sources(), []string{"garmin", "strava"})
	if len(result.ToolCalls) != 2 {
		t.Fatalf("tool calls: got %+v, want 2", result.ToolCalls)
	}

	final := model.LastRequest(t)
	assertBodyContains(t, final.Body, "Garmin training load", loadPayload)
	assertBodyContains(t, final.Body, "Strava run dataset", stravaTool.result)
}

// A single-source question must not drag in the other source: pulling Garmin
// data for a plain distance question wastes tokens and, worse, would make
// Result.Sources() claim the answer rests on recovery data it never used.
func TestSingleSourceQuestionStaysOnOneSource(t *testing.T) {
	strava, garmin, runsPayload, sleepPayload := crossSourceStubs(t)

	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "strava__get_activities", Input: map[string]any{"per_page": 30}}}},
		fakeTurn{Text: "You ran 16 km each week for the last 8 weeks."},
	)
	ag := New(client, "test-model", strava, garmin)

	result, err := ag.Answer(context.Background(), "how far were my long runs?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}

	assertEqualStrings(t, "sources", result.Sources(), []string{"strava"})

	final := model.LastRequest(t)
	assertBodyContains(t, final.Body, "Strava run dataset", runsPayload)
	if bodyContains(final.Body, sleepPayload) {
		t.Error("Garmin sleep data reached the model for a Strava-only question")
	}
}

// TestCrossSourceReasoningWithLiveModel is the one test that asks the real model
// to *choose* its sources; everything above verifies our execution of a plan the
// test scripted. The tool data is still fabricated, so no Garmin or Strava
// account is involved — only the Anthropic API.
//
// Skipped unless explicitly enabled, so the default `go test ./...` makes no
// network call and costs nothing:
//
//	RUNCOACH_LIVE_MODEL_TESTS=1 ANTHROPIC_API_KEY=sk-ant-... go test ./internal/agent/ -run LiveModel -v
func TestCrossSourceReasoningWithLiveModel(t *testing.T) {
	if os.Getenv("RUNCOACH_LIVE_MODEL_TESTS") != "1" {
		t.Skip("set RUNCOACH_LIVE_MODEL_TESTS=1 to run against the real Claude API")
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY is not set")
	}

	strava, garmin, _, _ := crossSourceStubs(t)
	model := anthropic.Model(getenvOr("ANTHROPIC_MODEL", "claude-sonnet-5"))
	ag := New(anthropic.NewClient(), model, strava, garmin)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := ag.Answer(ctx, "Does my sleep actually affect my pace? Show me the pattern.")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	t.Logf("tool calls: %+v\nanswer:\n%s", result.ToolCalls, result.Answer)

	// The question cannot be answered from one source, so the model should have
	// reached for both without being told to.
	for _, want := range []string{mcpclient.SourceStrava, mcpclient.SourceGarmin} {
		if !containsString(result.Sources(), want) {
			t.Errorf("expected the model to consult %s; sources were %v", want, result.Sources())
		}
	}

	// And the answer should cite figures from both datasets rather than
	// describing the correlation in the abstract.
	if !citesAny(result.Answer, "5:06", "5:02", "5:01", "4:35", "4:33", "4:38") {
		t.Errorf("answer cites no pace from the Strava dataset:\n%s", result.Answer)
	}
	if !citesAny(result.Answer, "5.7", "5.9", "6.0", "6.1", "7.8", "8.1", "8.2") {
		t.Errorf("answer cites no sleep duration from the Garmin dataset:\n%s", result.Answer)
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

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
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
