package agent

import (
	"context"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
)

// twoSourceAgent wires an Agent to a Strava stub and a Garmin stub. Both expose
// a get_activities tool, which is the real collision: Strava lists rides/runs
// and Garmin lists the same sessions from the watch.
func twoSourceAgent(t *testing.T) (*Agent, *callLog, *callLog) {
	t.Helper()
	stravaSession, stravaLog := startStub(t, "strava-mcp",
		stubTool{name: "get_activities", description: "List Strava activities.", result: `{"activities":[]}`},
	)
	garminSession, garminLog := startStub(t, "garmin-mcp",
		stubTool{name: "get_activities", description: "List Garmin activities.", result: `{"activities":[]}`},
		stubTool{name: "get_sleep_data", description: "Nightly sleep.", result: `{"sleep":[]}`},
	)
	ag := New(anthropic.Client{}, "test-model",
		Source{Name: mcpclient.SourceStrava, Session: stravaSession},
		Source{Name: mcpclient.SourceGarmin, Session: garminSession},
	)
	return ag, stravaLog, garminLog
}

func TestBuildToolsNamespacesBothSources(t *testing.T) {
	ag, _, _ := twoSourceAgent(t)

	tools, router, err := ag.buildTools(context.Background())
	if err != nil {
		t.Fatalf("buildTools: %v", err)
	}

	want := []string{"strava__get_activities", "garmin__get_activities", "garmin__get_sleep_data"}
	if len(tools) != len(want) {
		t.Fatalf("got %d tools, want %d: %+v", len(tools), len(want), toolNames(tools))
	}
	got := toolNames(tools)
	for i, name := range want {
		if got[i] != name {
			t.Errorf("tool %d: got %q, want %q (full list %v)", i, got[i], name, got)
		}
		if _, ok := router[name]; !ok {
			t.Errorf("router has no entry for %q", name)
		}
	}

	// The source is named in the description too, so tool selection does not
	// hinge on the model parsing the name prefix.
	desc := tools[2].OfTool.Description.Value
	if want := "[garmin] Nightly sleep."; desc != want {
		t.Errorf("description: got %q, want %q", desc, want)
	}
}

// The two get_activities tools must reach different servers — a routing bug here
// would silently answer Strava questions with Garmin data, or vice versa.
func TestCallToolRoutesToOwningSource(t *testing.T) {
	ag, stravaLog, garminLog := twoSourceAgent(t)
	ctx := context.Background()

	_, router, err := ag.buildTools(ctx)
	if err != nil {
		t.Fatalf("buildTools: %v", err)
	}

	if _, err := callTool(ctx, router["garmin__get_sleep_data"], `{"days":7}`); err != nil {
		t.Fatalf("call garmin__get_sleep_data: %v", err)
	}
	if _, err := callTool(ctx, router["strava__get_activities"], ""); err != nil {
		t.Fatalf("call strava__get_activities: %v", err)
	}

	if got := stravaLog.Tools(); len(got) != 1 || got[0] != "get_activities" {
		t.Errorf("strava stub calls: got %v, want [get_activities]", got)
	}
	garminCalls := garminLog.Calls()
	if len(garminCalls) != 1 || garminCalls[0].Tool != "get_sleep_data" {
		t.Fatalf("garmin stub calls: got %+v, want one get_sleep_data", garminCalls)
	}
	// Arguments must survive the round trip unmangled.
	if days, ok := garminCalls[0].Args["days"].(float64); !ok || days != 7 {
		t.Errorf("garmin get_sleep_data args: got %+v, want days=7", garminCalls[0].Args)
	}
}

func toolNames(tools []anthropic.ToolUnionParam) []string {
	var out []string
	for _, t := range tools {
		out = append(out, t.OfTool.Name)
	}
	return out
}
