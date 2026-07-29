package mockchat

import (
	"strings"
	"testing"

	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
)

// stravaTools are the tools cmd/strava-mcp actually exposes. Kept as a literal
// because that server registers them inline rather than from a list.
var stravaTools = map[string]bool{"list_activities": true}

// Every tool a scenario names must be one the real servers expose. A mock that
// invents tool names still "works" — but the UI's step labels are keyed on those
// names, so Phase 5 would silently fall back to generic copy for every step, and
// the mock would stop being a rehearsal for anything.
func TestScenarioToolsExistOnTheRealServers(t *testing.T) {
	garminTools := map[string]bool{}
	for _, name := range mcpclient.DefaultGarminTools {
		garminTools[name] = true
	}

	all := append([]Scenario{}, scenarios...)
	all = append(all, fallback)

	for _, s := range all {
		for _, step := range s.Steps {
			switch step.Source {
			case mcpclient.SourceStrava:
				if !stravaTools[step.Tool] {
					t.Errorf("scenario %q calls strava tool %q, which strava-mcp does not expose",
						s.Match, step.Tool)
				}
			case mcpclient.SourceGarmin:
				if !garminTools[step.Tool] {
					t.Errorf("scenario %q calls garmin tool %q, which is not in DefaultGarminTools",
						s.Match, step.Tool)
				}
			default:
				t.Errorf("scenario %q uses unknown source %q", s.Match, step.Source)
			}
		}
	}
}

// Each of the five spec §4 questions must resolve to a real scenario, not the
// fallback — those five are what the Phase 4 walkthrough and the Phase 5 live
// test both use.
func TestSpecQuestionsAllMatchAScenario(t *testing.T) {
	questions := []string{
		"Does my sleep actually affect my pace? Show me the pattern.",
		"Am I overtraining right now?",
		"How did my last 3 marathon training blocks compare?",
		"I have a half marathon in 10 weeks — am I on track, factoring in how recovered I've been?",
		"My watch and Strava show different HR for yesterday's run — which is right?",
	}
	for _, q := range questions {
		got := Match(q)
		if got.Answer == fallback.Answer {
			t.Errorf("question %q fell through to the fallback scenario", q)
		}
		if len(got.Steps) == 0 {
			t.Errorf("question %q matched a scenario with no steps", q)
		}
	}
}

// The fallback must admit to being mocked rather than answer plausibly, so an
// unhandled question in a demo can't be mistaken for real analysis.
func TestFallbackAdmitsItIsMocked(t *testing.T) {
	got := Match("what is the capital of France?")
	if !strings.Contains(strings.ToLower(got.Answer), "mock mode") {
		t.Errorf("fallback answer should say it is mocked, got %q", got.Answer)
	}
}

// Sources must report each source once, in call order — it stands in for
// agent.Result.Sources, which the UI uses to describe what was consulted.
func TestSourcesDedupesInCallOrder(t *testing.T) {
	s := Scenario{Steps: []Step{
		{Source: "garmin", Tool: "get_sleep_data"},
		{Source: "strava", Tool: "list_activities"},
		{Source: "garmin", Tool: "get_hrv_trend"},
	}}
	got := s.Sources()
	want := []string{"garmin", "strava"}
	if len(got) != len(want) {
		t.Fatalf("sources: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sources: got %v, want %v", got, want)
		}
	}
}
