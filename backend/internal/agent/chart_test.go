package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
)

func chartInput(points int) map[string]any {
	pts := make([]map[string]any, points)
	for i := range pts {
		pts[i] = map[string]any{"x": "2026-07-0" + string(rune('1'+i%9)), "y": 40 + i}
	}
	return map[string]any{
		"kind": "line", "title": "Weekly km", "x_label": "Week", "y_label": "Distance",
		"x_unit": "date", "y_unit": "km", "data": "activities", "caption": "Steady build.",
		"series": []map[string]any{{"label": "km", "points": pts}},
	}
}

// A valid chart call is answered in-process: the spec is kept on the result,
// announced once through the observer, and never reaches a data source.
func TestChartCallIsKeptAndAnnounced(t *testing.T) {
	garmin, _, _ := garminStub(t)
	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "garmin__get_activities_by_date", Input: map[string]any{"start": "2026-06-01", "end": "2026-07-31"}}}},
		fakeTurn{ToolUses: []fakeToolUse{{Name: "coach__show_chart", Input: chartInput(8)}}},
		fakeTurn{Text: "Your volume climbed from 40 to 47 km a week."},
	)
	ag := New(client, "test-model", garmin)

	var events []Event
	ag.Observe(func(ev Event) { events = append(events, ev) })

	result, err := ag.Answer(context.Background(), "how has my volume gone?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if result.Chart == nil || result.Chart.Kind != "line" || len(result.Chart.Series[0].Points) != 8 {
		t.Fatalf("chart not kept: %+v", result.Chart)
	}
	// The chart is not a data source.
	assertEqualStrings(t, "sources", result.Sources(), []string{"garmin"})
	if len(result.ToolCalls) != 1 {
		t.Errorf("tool calls: got %+v, want the one Garmin call", result.ToolCalls)
	}

	var charts int
	for _, ev := range events {
		if ev.Kind == EventChart {
			charts++
			if ev.Chart == nil || ev.Source != CoachSource {
				t.Errorf("chart event is incomplete: %+v", ev)
			}
		}
	}
	if charts != 1 {
		t.Errorf("chart events: got %d, want 1", charts)
	}
	// And the model was told it worked.
	if body := model.Requests()[2].Body; !strings.Contains(body, "Chart accepted") {
		t.Errorf("model was not told the chart was accepted:\n%s", body)
	}
}

// A second chart in the same reply is refused readably and the first is kept.
func TestSecondChartIsRefused(t *testing.T) {
	garmin, _, _ := garminStub(t)
	first := chartInput(6)
	first["title"] = "First"
	second := chartInput(7)
	second["title"] = "Second"
	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "coach__show_chart", Input: first}}},
		fakeTurn{ToolUses: []fakeToolUse{{Name: "coach__show_chart", Input: second}}},
		fakeTurn{Text: "done"},
	)
	ag := New(client, "test-model", garmin)
	var charts int
	ag.Observe(func(ev Event) {
		if ev.Kind == EventChart {
			charts++
		}
	})

	result, err := ag.Answer(context.Background(), "chart twice")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if result.Chart == nil || result.Chart.Title != "First" {
		t.Errorf("kept chart: %+v, want the first", result.Chart)
	}
	if charts != 1 {
		t.Errorf("chart events: got %d, want 1", charts)
	}
	if body := model.Requests()[2].Body; !strings.Contains(body, "One chart per reply") {
		t.Errorf("model was not told the second chart was refused:\n%s", body)
	}
}

// An invalid chart returns the validator's message naming the field, emits no
// event, and the reply carries on without a chart.
func TestInvalidChartIsRejectedAndProseContinues(t *testing.T) {
	garmin, _, _ := garminStub(t)
	client, model := startFakeModel(t,
		fakeTurn{ToolUses: []fakeToolUse{{Name: "coach__show_chart", Input: chartInput(3)}}},
		fakeTurn{Text: "Here is the answer without a chart."},
	)
	ag := New(client, "test-model", garmin)
	var charts int
	ag.Observe(func(ev Event) {
		if ev.Kind == EventChart {
			charts++
		}
	})

	result, err := ag.Answer(context.Background(), "chart badly")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if result.Chart != nil {
		t.Errorf("invalid chart was kept: %+v", result.Chart)
	}
	if charts != 0 {
		t.Errorf("chart events: got %d, want 0", charts)
	}
	if result.Answer != "Here is the answer without a chart." {
		t.Errorf("answer: %q", result.Answer)
	}
	// The tool result the model saw is an error naming the failing field.
	body := model.Requests()[1].Body
	if !strings.Contains(body, `"is_error":true`) || !strings.Contains(body, "series[0].points") {
		t.Errorf("tool result did not name the failing field:\n%s", body)
	}
}

// The chart tool rides along with any source set, always last and under coach.
func TestChartToolIsOfferedWithoutGarmin(t *testing.T) {
	session, _ := startStub(t, "gateway-mcp",
		stubTool{name: "get_activities", description: "List activities.", result: `{}`})
	ag := New(startFakeModelClient(t), "test-model", Source{Name: "gateway", Session: session})
	tools, router, err := ag.buildTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := toolNames(tools); len(got) != 2 || got[1] != "coach__show_chart" {
		t.Errorf("tools: %v", got)
	}
	if ref := router["coach__show_chart"]; ref.source != CoachSource || ref.session != nil {
		t.Errorf("chart route: %+v", ref)
	}
	_ = mcpclient.SourceGarmin
}
