package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
)

// gatewaySource stands in for a second source (the roadmap's gateway, ADR
// 0002). It exposes a get_activities tool too, which is the real collision the
// namespacing exists for.
const gatewaySource = "gateway"

// stubbedSources returns a gateway stub and a Garmin stub with distinguishable
// canned payloads, plus their call logs.
func stubbedSources(t *testing.T) (gateway, garmin Source, gatewayLog, garminLog *callLog) {
	t.Helper()
	gatewaySession, gatewayLog := startStub(t, "gateway-mcp",
		stubTool{name: "get_activities", description: "List Gateway activities.",
			result: `{"activities":[{"date":"2026-07-20","distance_km":21.1,"pace":"4:45/km"}]}`},
	)
	garminSession, garminLog := startStub(t, "garmin-mcp",
		stubTool{name: "get_activities", description: "List Garmin activities.",
			result: `{"activities":[{"date":"2026-07-20","distance_km":21.0}]}`},
		stubTool{name: "get_sleep_data", description: "Nightly sleep.",
			result: `{"nights":[{"date":"2026-07-19","hours":6.2}]}`},
	)
	return Source{Name: gatewaySource, Session: gatewaySession},
		Source{Name: mcpclient.SourceGarmin, Session: garminSession},
		gatewayLog, garminLog
}

// Each scripted plan must execute end to end and be reported accurately: the
// sources a caller is told about come from tool calls that really happened.
func TestAnswerExecutesEachSourcePlan(t *testing.T) {
	tests := []struct {
		name        string
		toolUses    []fakeToolUse
		wantSources []string
		wantGateway []string
		wantGarmin  []string
	}{
		{
			name:        "gateway only",
			toolUses:    []fakeToolUse{{Name: "gateway__get_activities", Input: map[string]any{"per_page": 10}}},
			wantSources: []string{"gateway"},
			wantGateway: []string{"get_activities"},
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
				{Name: "gateway__get_activities"},
				{Name: "garmin__get_sleep_data", Input: map[string]any{"days": 30}},
			},
			wantSources: []string{"gateway", "garmin"},
			wantGateway: []string{"get_activities"},
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
			gateway, garmin, gatewayLog, garminLog := stubbedSources(t)

			var turns []fakeTurn
			if len(tc.toolUses) > 0 {
				turns = append(turns, fakeTurn{ToolUses: tc.toolUses})
			}
			turns = append(turns, fakeTurn{Text: "final answer"})

			client, _ := startFakeModel(t, turns...)
			ag := New(client, "test-model", gateway, garmin)

			result, err := ag.Answer(context.Background(), "how is my training going?")
			if err != nil {
				t.Fatalf("Answer: %v", err)
			}
			if result.Answer != "final answer" {
				t.Errorf("answer: got %q, want %q", result.Answer, "final answer")
			}
			assertEqualStrings(t, "sources", result.Sources(), tc.wantSources)
			assertEqualStrings(t, "gateway stub calls", gatewayLog.Tools(), tc.wantGateway)
			assertEqualStrings(t, "garmin stub calls", garminLog.Tools(), tc.wantGarmin)
		})
	}
}

// A source chosen on a later turn, after seeing the first source's data, is the
// sequential half of cross-source reasoning.
func TestAnswerFollowsUpWithSecondSource(t *testing.T) {
	gateway, garmin, gatewayLog, garminLog := stubbedSources(t)

	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_sleep_data", Input: map[string]any{"days": 30}}}},
		fakeTurn{ToolUses: []fakeToolUse{{Name: "gateway__get_activities"}}},
		fakeTurn{Text: "6.2h sleep on 2026-07-19, and the 2026-07-20 half was run at 4:45/km."},
	)
	ag := New(client, "test-model", gateway, garmin)

	result, err := ag.Answer(context.Background(), "does my sleep affect my pace?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}

	assertEqualStrings(t, "sources", result.Sources(), []string{"garmin", "gateway"})
	assertEqualStrings(t, "garmin stub calls", garminLog.Tools(), []string{"get_sleep_data"})
	assertEqualStrings(t, "gateway stub calls", gatewayLog.Tools(), []string{"get_activities"})

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
	gateway, garmin, gatewayLog, garminLog := stubbedSources(t)

	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_readiness"}}},
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_sleep_data"}}},
		fakeTurn{Text: "recovered"},
	)
	ag := New(client, "test-model", gateway, garmin)

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
	assertEqualStrings(t, "gateway stub calls", gatewayLog.Tools(), nil)

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
	gateway, garmin, _, _ := stubbedSources(t)

	turns := make([]fakeTurn, maxTurns)
	for i := range turns {
		turns[i] = fakeTurn{ToolUses: []fakeToolUse{{Name: "gateway__get_activities"}}}
	}
	client, _ := startFakeModel(t, turns...)
	ag := New(client, "test-model", gateway, garmin)

	if _, err := ag.Answer(context.Background(), "loop forever"); err == nil {
		t.Fatal("expected an error after maxTurns tool-use turns")
	}
}

// The prompt is what actually makes tool selection the model's decision, so its
// contents are worth asserting: every registered source described, the Garmin
// category map present only when Garmin is registered, and nothing promised
// that is not registered.
func TestSystemPromptDescribesRegisteredSourcesOnly(t *testing.T) {
	gateway, garmin, _, _ := stubbedSources(t)

	both := New(anthropic.Client{}, "test-model", gateway, garmin).systemPrompt()
	for _, want := range []string{"- gateway —", "- garmin —", "sleep", "Finding the right Garmin tool"} {
		if !strings.Contains(both, want) {
			t.Errorf("two-source prompt is missing %q:\n%s", want, both)
		}
	}

	garminOnly := New(anthropic.Client{}, "test-model", garmin).systemPrompt()
	if strings.Contains(garminOnly, "gateway") {
		t.Errorf("garmin-only prompt should not mention the gateway:\n%s", garminOnly)
	}
	gatewayOnly := New(anthropic.Client{}, "test-model", gateway).systemPrompt()
	for _, unwanted := range []string{"- garmin —", "Finding the right Garmin tool"} {
		if strings.Contains(gatewayOnly, unwanted) {
			t.Errorf("prompt without Garmin should not contain %q:\n%s", unwanted, gatewayOnly)
		}
	}
}

// Every tool the category map names must exist upstream at the pinned commit,
// or the model is pointed at a tool it can never call.
func TestCategoryMapNamesRealTools(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "deploy", "garmin-mcp", "tools.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read upstream tool list: %v", err)
	}
	upstream := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			upstream[line] = true
		}
	}

	named := regexp.MustCompile(`\b(get_[a-z0-9_]+|count_activities)\b`).FindAllString(garminCategoryMap, -1)
	if len(named) < 20 {
		t.Fatalf("category map names only %d tools; expected the twelve-line index", len(named))
	}
	for _, name := range named {
		if !upstream[name] {
			t.Errorf("category map names %q, which upstream does not expose", name)
		}
		if !mcpclient.ReadOnly(name) {
			t.Errorf("category map names %q, which the read surface denies", name)
		}
	}
}

// Every request carries the model, the explicit effort and the two cache
// breakpoints (system prompt and last tool), so the shared block is one cache
// prefix (ADR 0006).
func TestRequestsCarryEffortAndCacheControl(t *testing.T) {
	_, garmin, _, _ := stubbedSources(t)
	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_sleep_data"}}},
		fakeTurn{Text: "final answer"},
	)
	ag := New(client, "test-model", garmin).WithEffort("high")
	if _, err := ag.Answer(context.Background(), "how did I sleep?"); err != nil {
		t.Fatalf("Answer: %v", err)
	}
	for i, req := range model.Requests() {
		var parsed struct {
			Model        string `json:"model"`
			OutputConfig struct {
				Effort string `json:"effort"`
			} `json:"output_config"`
			System []struct {
				CacheControl map[string]string `json:"cache_control"`
			} `json:"system"`
			Tools []struct {
				Name         string            `json:"name"`
				CacheControl map[string]string `json:"cache_control"`
			} `json:"tools"`
		}
		if err := json.Unmarshal([]byte(req.Body), &parsed); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if parsed.Model != "test-model" {
			t.Errorf("request %d model: got %q", i, parsed.Model)
		}
		if parsed.OutputConfig.Effort != "high" {
			t.Errorf("request %d effort: got %q, want high", i, parsed.OutputConfig.Effort)
		}
		if len(parsed.System) != 1 || parsed.System[0].CacheControl["type"] != "ephemeral" {
			t.Errorf("request %d: system prompt is not a cache breakpoint", i)
		}
		last := parsed.Tools[len(parsed.Tools)-1]
		if last.CacheControl["type"] != "ephemeral" {
			t.Errorf("request %d: last tool %q is not a cache breakpoint", i, last.Name)
		}
		for _, tool := range parsed.Tools[:len(parsed.Tools)-1] {
			if tool.CacheControl != nil {
				t.Errorf("request %d: tool %q has a cache breakpoint; only the last should", i, tool.Name)
			}
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
