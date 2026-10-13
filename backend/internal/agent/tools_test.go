package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
)

// twoSourceAgent wires an Agent to a gateway stub and a Garmin stub. Both expose
// a get_activities tool, which is the real collision: a second source serving
// the same Garmin data would list the same sessions under the same names.
func twoSourceAgent(t *testing.T) (*Agent, *callLog, *callLog) {
	t.Helper()
	gatewaySession, gatewayLog := startStub(t, "gateway-mcp",
		stubTool{name: "get_activities", description: "List Gateway activities.", result: `{"activities":[]}`},
	)
	garminSession, garminLog := startStub(t, "garmin-mcp",
		stubTool{name: "get_activities", description: "List Garmin activities.", result: `{"activities":[]}`},
		stubTool{name: "get_sleep_data", description: "Nightly sleep.", result: `{"sleep":[]}`},
	)
	ag := New(anthropic.Client{}, "test-model",
		Source{Name: gatewaySource, Session: gatewaySession},
		Source{Name: mcpclient.SourceGarmin, Session: garminSession},
	)
	return ag, gatewayLog, garminLog
}

func TestBuildToolsNamespacesBothSources(t *testing.T) {
	ag, _, _ := twoSourceAgent(t)

	tools, router, err := ag.buildTools(context.Background())
	if err != nil {
		t.Fatalf("buildTools: %v", err)
	}

	// The built-in chart tool is always last, under the coach source.
	want := []string{"gateway__get_activities", "garmin__get_activities", "garmin__get_sleep_data", "coach__show_chart"}
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
// would silently answer one runner's question with another source's data.
func TestCallToolRoutesToOwningSource(t *testing.T) {
	ag, gatewayLog, garminLog := twoSourceAgent(t)
	ctx := context.Background()

	_, router, err := ag.buildTools(ctx)
	if err != nil {
		t.Fatalf("buildTools: %v", err)
	}

	if _, err := callTool(ctx, router["garmin__get_sleep_data"], `{"days":7}`); err != nil {
		t.Fatalf("call garmin__get_sleep_data: %v", err)
	}
	if _, err := callTool(ctx, router["gateway__get_activities"], ""); err != nil {
		t.Fatalf("call gateway__get_activities: %v", err)
	}

	if got := gatewayLog.Tools(); len(got) != 1 || got[0] != "get_activities" {
		t.Errorf("gateway stub calls: got %v, want [get_activities]", got)
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

// A source's Allow filter decides what the model is offered: a denied tool is
// neither listed nor routable, so even a hallucinated call to it fails as an
// unknown tool (ADR 0004).
func TestBuildToolsAppliesTheSourceFilter(t *testing.T) {
	session, log := startStub(t, "garmin-mcp",
		stubTool{name: "get_sleep_data", description: "Nightly sleep.", result: `{}`},
		stubTool{name: "delete_activity", description: "Delete an activity.", result: `{}`},
		stubTool{name: "set_activity_name", description: "Rename an activity.", result: `{}`},
	)
	ag := New(anthropic.Client{}, "test-model",
		Source{Name: mcpclient.SourceGarmin, Session: session, Allow: mcpclient.ReadOnly})

	tools, router, err := ag.buildTools(context.Background())
	if err != nil {
		t.Fatalf("buildTools: %v", err)
	}
	if got := toolNames(tools); len(got) != 2 || got[0] != "garmin__get_sleep_data" || got[1] != "coach__show_chart" {
		t.Errorf("offered tools: got %v, want garmin__get_sleep_data and the chart tool only", got)
	}
	if _, ok := router["garmin__delete_activity"]; ok {
		t.Error("a denied tool is routable")
	}

	// And the loop treats a call to the denied tool as unknown, never reaching
	// the server.
	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__delete_activity"}}},
		fakeTurn{Text: "I cannot change your data."},
	)
	ag = New(client, "test-model",
		Source{Name: mcpclient.SourceGarmin, Session: session, Allow: mcpclient.ReadOnly})
	if _, err := ag.Answer(context.Background(), "delete my last run"); err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if calls := log.Tools(); len(calls) != 0 {
		t.Errorf("denied tool reached the server: %v", calls)
	}
	if body := model.Requests()[1].Body; !strings.Contains(body, "no such tool") {
		t.Errorf("model was not told the tool does not exist:\n%s", body)
	}
}

func toolNames(tools []anthropic.ToolUnionParam) []string {
	var out []string
	for _, t := range tools {
		out = append(out, t.OfTool.Name)
	}
	return out
}
