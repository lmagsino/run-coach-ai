package chart

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

func points(n int) []Point {
	out := make([]Point, n)
	for i := range out {
		out[i] = Point{X: "2026-07-" + string(rune('0'+i%10)), Y: float64(i)}
	}
	return out
}

func valid(kind string) Spec {
	s := Spec{
		Kind: kind, Title: "Sleep vs next-day pace", XLabel: "Sleep", YLabel: "Pace",
		XUnit: "h", YUnit: "min/km", Data: "sleep", Caption: "Shorter nights, slower runs.",
		Series: []Series{{Label: "Long runs", Points: points(8)}},
	}
	return s
}

func TestValidSpecPerKind(t *testing.T) {
	for _, kind := range Kinds {
		s := valid(kind)
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", kind, err)
		}
	}
	two := valid("line")
	two.Series = append(two.Series, Series{Label: "Easy runs", Points: points(8)})
	if err := two.Validate(); err != nil {
		t.Errorf("two-series line: %v", err)
	}
	bar := valid("bar")
	bar.Series[0].Points[7].Partial = true
	if err := bar.Validate(); err != nil {
		t.Errorf("partial bar: %v", err)
	}
}

// Every rejection names its field, so the tool result tells the model what to fix.
func TestRejectionsNameTheField(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*Spec)
		field string
	}{
		{"unknown kind", func(s *Spec) { s.Kind = "pie" }, "kind"},
		{"no title", func(s *Spec) { s.Title = "" }, "title"},
		{"long title", func(s *Spec) { s.Title = strings.Repeat("x", MaxTitleRunes+1) }, "title"},
		{"no caption", func(s *Spec) { s.Caption = "" }, "caption"},
		{"no x label", func(s *Spec) { s.XLabel = "" }, "x_label"},
		{"mixed units", func(s *Spec) { s.YUnit = "mixed" }, "y_unit"},
		{"unknown data", func(s *Spec) { s.Data = "weather" }, "data"},
		{"too few points", func(s *Spec) { s.Series[0].Points = points(MinPoints - 1) }, "series[0].points"},
		{"too many points", func(s *Spec) { s.Series[0].Points = points(MaxPoints + 1) }, "series[0].points"},
		{"no series label", func(s *Spec) { s.Series[0].Label = "" }, "series[0].label"},
		{"empty x", func(s *Spec) { s.Series[0].Points[2].X = "" }, "series[0].points[2].x"},
		{"nan y", func(s *Spec) { s.Series[0].Points[3].Y = math.NaN() }, "series[0].points[3].y"},
		{"inf y", func(s *Spec) { s.Series[0].Points[3].Y = math.Inf(1) }, "series[0].points[3].y"},
		{"three line series", func(s *Spec) {
			s.Series = append(s.Series, Series{Label: "b", Points: points(6)}, Series{Label: "c", Points: points(6)})
		}, "series"},
		{"no series", func(s *Spec) { s.Series = nil }, "series"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := valid("line")
			tc.mut(&s)
			err := s.Validate()
			var ce *Error
			if !errors.As(err, &ce) {
				t.Fatalf("got %v, want a field error", err)
			}
			if ce.Field != tc.field {
				t.Errorf("field: got %q, want %q (%v)", ce.Field, tc.field, err)
			}
		})
	}

	for _, kind := range []string{"bar", "scatter"} {
		s := valid(kind)
		s.Series = append(s.Series, Series{Label: "second", Points: points(6)})
		if err := s.Validate(); err == nil || !strings.HasPrefix(err.Error(), "series:") {
			t.Errorf("%s with two series: got %v, want a series error", kind, err)
		}
	}
	line := valid("line")
	line.Series[0].Points[1].Partial = true
	if err := line.Validate(); err == nil || !strings.Contains(err.Error(), "partial") {
		t.Errorf("partial on a line point: got %v", err)
	}
}

// The spec round-trips through the JSON the model sends and the browser reads.
func TestSpecJSONRoundTrip(t *testing.T) {
	in := valid("bar")
	in.Series[0].Points[0].Partial = true
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Spec
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if err := out.Validate(); err != nil {
		t.Fatalf("round-tripped spec invalid: %v", err)
	}
	if !out.Series[0].Points[0].Partial || out.Series[0].Points[1].Partial {
		t.Error("partial flag did not round-trip")
	}
	if !strings.Contains(string(raw), `"x_label"`) {
		t.Errorf("wire shape uses snake_case fields: %s", raw)
	}
}

func TestToolDefinitionCarriesTheRules(t *testing.T) {
	def := ToolDefinition()
	if def.Name != ToolName {
		t.Errorf("name: %q", def.Name)
	}
	desc := def.Description.Value
	for _, want := range []string{"One chart per reply", "never for a single number", "seconds per km", "5 to 120"} {
		if !strings.Contains(desc, want) {
			t.Errorf("description is missing %q", want)
		}
	}
	if len(def.InputSchema.Required) != 9 {
		t.Errorf("required fields: got %v", def.InputSchema.Required)
	}
}
