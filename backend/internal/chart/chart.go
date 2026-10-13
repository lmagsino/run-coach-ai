// Package chart defines the one chart a reply may carry: the schema of the
// virtual coach__show_chart tool the model calls, and the validation the
// backend applies before anything reaches the browser (ADR 0005).
//
// The model reshapes the data itself and passes ready points; the backend
// checks the shape and refuses anything the renderer could not draw honestly.
// Every error names the field, so the model can correct the call or drop it.
package chart

import (
	"errors"
	"fmt"
	"math"

	"github.com/anthropics/anthropic-sdk-go"
)

// ToolName is the tool's name before source namespacing (coach__show_chart).
const ToolName = "show_chart"

// Limits that keep a card readable (artifact Design §7, §8).
const (
	MinPoints      = 5
	MaxPoints      = 120
	MaxLineSeries  = 2
	MaxTitleRunes  = 80
	MaxLabelRunes  = 40
	MaxCaptionRune = 280
)

// Kinds the renderer can draw.
var Kinds = []string{"line", "bar", "scatter"}

// DataNames describe which Garmin data a chart was built from; the card shows
// it as "Garmin data used".
var DataNames = []string{
	"activities", "sleep", "hrv", "stress", "body_battery",
	"training_load", "readiness", "vo2max", "steps", "body",
}

// Point is one x/y pair. X is a string so dates ("2026-07-18") and numbers
// ("6.2") both travel as given; the renderer decides the scale from XUnit.
// Partial flags a bar for an incomplete period, drawn faint.
type Point struct {
	X       string  `json:"x"`
	Y       float64 `json:"y"`
	Partial bool    `json:"partial,omitempty"`
}

// Series is one labelled set of points.
type Series struct {
	Label  string  `json:"label"`
	Points []Point `json:"points"`
}

// Spec is a validated chart.
type Spec struct {
	Kind    string   `json:"kind"`
	Title   string   `json:"title"`
	XLabel  string   `json:"x_label"`
	YLabel  string   `json:"y_label"`
	XUnit   string   `json:"x_unit"`
	YUnit   string   `json:"y_unit"`
	Data    string   `json:"data"`
	Series  []Series `json:"series"`
	Caption string   `json:"caption"`
}

// Error names the field a call failed on, so the tool result tells the model
// what to fix.
type Error struct {
	Field string
	Msg   string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Msg) }

func fail(field, format string, args ...any) error {
	return &Error{Field: field, Msg: fmt.Sprintf(format, args...)}
}

// Validate checks a decoded spec and returns the first problem, naming its
// field. A nil error means the renderer can draw it.
func (s *Spec) Validate() error {
	if s == nil {
		return errors.New("chart: empty spec")
	}
	if !contains(Kinds, s.Kind) {
		return fail("kind", "must be one of %v", Kinds)
	}
	if s.Title == "" || runeLen(s.Title) > MaxTitleRunes {
		return fail("title", "required, at most %d characters", MaxTitleRunes)
	}
	if s.Caption == "" || runeLen(s.Caption) > MaxCaptionRune {
		return fail("caption", "required, at most %d characters", MaxCaptionRune)
	}
	for field, v := range map[string]string{"x_label": s.XLabel, "y_label": s.YLabel} {
		if v == "" || runeLen(v) > MaxLabelRunes {
			return fail(field, "required, at most %d characters", MaxLabelRunes)
		}
	}
	if s.YUnit == "" || s.YUnit == "mixed" {
		return fail("y_unit", "required and single: two measures on different units need two charts, never a dual axis")
	}
	if !contains(DataNames, s.Data) {
		return fail("data", "must be one of %v", DataNames)
	}

	switch s.Kind {
	case "line":
		if len(s.Series) < 1 || len(s.Series) > MaxLineSeries {
			return fail("series", "a line chart takes 1 or %d series", MaxLineSeries)
		}
	default:
		if len(s.Series) != 1 {
			return fail("series", "a %s chart takes exactly one series", s.Kind)
		}
	}

	for i, ser := range s.Series {
		field := fmt.Sprintf("series[%d]", i)
		if ser.Label == "" || runeLen(ser.Label) > MaxLabelRunes {
			return fail(field+".label", "required, at most %d characters", MaxLabelRunes)
		}
		if n := len(ser.Points); n < MinPoints || n > MaxPoints {
			return fail(field+".points", "between %d and %d points, got %d", MinPoints, MaxPoints, n)
		}
		for j, p := range ser.Points {
			if p.X == "" {
				return fail(fmt.Sprintf("%s.points[%d].x", field, j), "required")
			}
			if math.IsNaN(p.Y) || math.IsInf(p.Y, 0) {
				return fail(fmt.Sprintf("%s.points[%d].y", field, j), "must be a finite number")
			}
			if p.Partial && s.Kind != "bar" {
				return fail(fmt.Sprintf("%s.points[%d].partial", field, j), "only a bar can be partial")
			}
		}
	}
	return nil
}

// ToolDefinition is the tool offered to the model, with the rules in the
// description so a model that reads nothing else still calls it correctly.
func ToolDefinition() anthropic.ToolParam {
	return anthropic.ToolParam{
		Name: ToolName,
		Description: anthropic.String("Show one chart with your reply when the answer is a pattern over time " +
			"or a relationship between two measures, never for a single number. Reshape the data " +
			"yourself into 5 to 120 points. kind: line (1 or 2 series on the same unit), bar (1 series; " +
			"mark an incomplete period with partial:true) or scatter (1 series, x and y numeric). " +
			"Pace goes in seconds per km with y_unit \"min/km\". One chart per reply; the prose must " +
			"still stand without it."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"kind":    map[string]any{"type": "string", "enum": Kinds},
				"title":   map[string]any{"type": "string", "description": "Short title, what the chart shows"},
				"x_label": map[string]any{"type": "string"},
				"y_label": map[string]any{"type": "string"},
				"x_unit":  map[string]any{"type": "string", "description": "e.g. date, h, km, week"},
				"y_unit":  map[string]any{"type": "string", "description": "e.g. min/km, h, km, ms, score; one unit for all series"},
				"data":    map[string]any{"type": "string", "enum": DataNames, "description": "Which Garmin data the points came from"},
				"series": map[string]any{
					"type":     "array",
					"minItems": 1,
					"maxItems": MaxLineSeries,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"label": map[string]any{"type": "string"},
							"points": map[string]any{
								"type":     "array",
								"minItems": MinPoints,
								"maxItems": MaxPoints,
								"items": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"x":       map[string]any{"type": "string", "description": "ISO date or a number as text"},
										"y":       map[string]any{"type": "number"},
										"partial": map[string]any{"type": "boolean", "description": "bar only: period not finished"},
									},
									"required": []string{"x", "y"},
								},
							},
						},
						"required": []string{"label", "points"},
					},
				},
				"caption": map[string]any{"type": "string", "description": "One or two sentences on what the pattern means"},
			},
			Required: []string{"kind", "title", "x_label", "y_label", "x_unit", "y_unit", "data", "series", "caption"},
		},
	}
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func runeLen(s string) int { return len([]rune(s)) }
