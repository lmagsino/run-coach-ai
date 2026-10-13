package mockchat

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
)

// upstreamTools reads deploy/garmin-mcp/tools.txt, the tool names the pinned
// upstream server exposes. Every tool a scenario names must be in it: a mock
// that invents tool names still "works", but the UI's step labels are keyed on
// those names, so a live reply would silently fall back to generic copy and the
// mock would stop being a rehearsal for anything.
func upstreamTools(t *testing.T) map[string]bool {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "deploy", "garmin-mcp", "tools.txt")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open upstream tool list: %v", err)
	}
	defer f.Close()

	tools := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tools[line] = true
	}
	if len(tools) < 100 {
		t.Fatalf("upstream tool list looks truncated: %d names", len(tools))
	}
	return tools
}

func TestScenarioToolsExistUpstream(t *testing.T) {
	garminTools := upstreamTools(t)

	all := append([]Scenario{}, scenarios...)
	all = append(all, fallback)

	for _, s := range all {
		for _, step := range s.Steps {
			if step.Source != mcpclient.SourceGarmin {
				t.Errorf("scenario %q uses unknown source %q", s.Match, step.Source)
				continue
			}
			if !garminTools[step.Tool] {
				t.Errorf("scenario %q calls garmin tool %q, which upstream does not expose",
					s.Match, step.Tool)
			}
			if !mcpclient.ReadOnly(step.Tool) {
				t.Errorf("scenario %q calls %q, which the read surface denies", s.Match, step.Tool)
			}
		}
	}
}

// Each of the five example questions must resolve to a real scenario, not the
// fallback — those five are what the mock walkthrough and the live pass use.
func TestSpecQuestionsAllMatchAScenario(t *testing.T) {
	questions := []string{
		"Does my sleep actually affect my pace? Show me the pattern.",
		"Am I overtraining right now?",
		"How did my last three training blocks compare?",
		"I have a half marathon in 10 weeks. Am I on track, given how recovered I've been?",
		"Why has my body battery been so low this week?",
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
		{Source: "gateway", Tool: "get_activities"},
		{Source: "garmin", Tool: "get_hrv_trend"},
	}}
	got := s.Sources()
	want := []string{"garmin", "gateway"}
	if len(got) != len(want) {
		t.Fatalf("sources: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sources: got %v, want %v", got, want)
		}
	}
}
