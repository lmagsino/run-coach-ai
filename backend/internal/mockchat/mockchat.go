// Package mockchat serves canned answers so the frontend can be built and
// demoed before any credentials exist. It stands in for the whole agent —
// Claude and the Garmin MCP server — and is reachable only when RUNCOACH_MOCK
// is set.
//
// It deliberately mimics the *shape* of real work rather than just returning a
// string: each scenario names the tools a real answer would have called, in the
// order the agent would have called them, with a delay between so the UI's
// status steps advance the way they do live. What it cannot mimic is the
// model's judgment — which tools a question actually needs is the model's
// decision, and verifying that is the live pass's job.
package mockchat

import (
	"strings"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
)

// Step is one tool call a scenario pretends to make.
type Step struct {
	Source string
	Tool   string
	// Failed marks a step that reports an error, so the UI's failure path is
	// exercisable without breaking a real integration.
	Failed bool
}

// Scenario is a canned question/answer pair plus the tool calls behind it.
type Scenario struct {
	// Match is a set of lowercase substrings; a question matching ANY of them
	// selects this scenario.
	Match  []string
	Steps  []Step
	Answer string
}

// StepDelay is how long each pretend tool call takes. Long enough that the
// status steps are legible rather than a flicker, short enough not to feel
// broken — roughly what a real MCP round-trip costs.
const StepDelay = 700 * time.Millisecond

const garmin = mcpclient.SourceGarmin

// scenarios covers the five example questions in the design spec. Every tool
// named here exists upstream (deploy/garmin-mcp/tools.txt); a test checks.
var scenarios = []Scenario{
	{
		// "Does my sleep actually affect my pace? Show me the pattern."
		Match: []string{"sleep", "hrv"},
		Steps: []Step{
			{Source: garmin, Tool: "get_sleep_summary_range"},
			{Source: garmin, Tool: "get_hrv_trend"},
			{Source: garmin, Tool: "get_activities_by_date"},
		},
		Answer: `Yes — and the pattern is consistent enough to plan around.

Across your last eight weeks, the nights you slept under six hours were followed by runs that came in slower almost every time. Your HRV tells the same story: the four lowest readings all landed the morning after a short night, and each was followed by a session you rated harder than the pace justified.

[figure: 25s | slower per km after nights under 6 hours, averaged over 8 weeks]

The effect shows up the next day, not two days later, so the lever you have is the night before a quality session — not the whole week.`,
	},
	{
		// "Am I overtraining right now?"
		Match: []string{"overtrain", "overreach", "burnt out", "burned out"},
		Steps: []Step{
			{Source: garmin, Tool: "get_training_load_trend"},
			{Source: garmin, Tool: "get_hrv_trend"},
			{Source: garmin, Tool: "get_stress_summary"},
			{Source: garmin, Tool: "get_training_status"},
		},
		Answer: `Not overtraining — but you're closer to the edge than last month.

Your training load has climbed steadily for three weeks without a down week, and your HRV trend is flat rather than falling. What has changed is variability: your last three easy runs drifted faster than easy, and stress scores have been climbing on the days after them.

[figure: 3 | consecutive easy runs faster than your easy pace ceiling]

Nothing here says stop. It says make the easy days genuinely easy for a week and watch whether stress settles.`,
	},
	{
		// "How did my last three training blocks compare?"
		Match: []string{"training block", "blocks compare", "marathon block", "last 3", "last three"},
		Steps: []Step{
			{Source: garmin, Tool: "get_activities_by_date"},
			{Source: garmin, Tool: "get_training_load_trend"},
		},
		Answer: `The most recent block was your most consistent, though not your highest volume.

Your spring block peaked highest — one 58km week — but it also had the two weeks you missed entirely. The block before it was steadier but capped out lower. This one has no missed weeks at all, and the long runs held their pace deep into the block rather than fading in the last fortnight.

[figure: 0 | missed weeks this block, against 2 and 1 in the previous two]

Consistency is the difference, not volume. That's usually the one that shows up on race day.`,
	},
	{
		// "I have a half marathon in 10 weeks. Am I on track, given how
		// recovered I've been?"
		Match: []string{"on track", "half marathon", "10 weeks", "race", "2:30"},
		Steps: []Step{
			{Source: garmin, Tool: "get_activities_by_date"},
			{Source: garmin, Tool: "get_training_readiness"},
			{Source: garmin, Tool: "get_race_predictions"},
		},
		Answer: `Yes — you're trending a touch ahead of pace.

Your last four long runs have all come in faster than target, and the reason it's sticking is recovery: your readiness score has held in the green through the whole build while load rose, so you haven't been paying for the effort.

[figure: 6:44 | avg long-run pace per mile — eight seconds ahead of the pace you need]

Hold this and don't chase more volume. The gap you have now is the kind that recovery built, not fitness you have to force.`,
	},
	{
		// "Why has my body battery been so low this week?"
		Match: []string{"body battery", "battery", "so low", "drained"},
		Steps: []Step{
			{Source: garmin, Tool: "get_body_battery"},
			{Source: garmin, Tool: "get_sleep_summary_range"},
			{Source: garmin, Tool: "get_stress_summary"},
		},
		Answer: `It's the nights, not the runs.

Your body battery has started each day this week between 38 and 51, against the 70s you were waking up with a fortnight ago. The charge you get overnight is what fell: four of the last six nights were under six and a half hours, and your stress stayed elevated past midnight on three of them. The training load itself is no higher than last month.

[figure: 44 | average morning body battery this week, down from 74]

Protect the next three nights and the number comes back on its own. Cutting the runs would not fix it.`,
	},
}

// fallback answers anything that matches no scenario. It stays honest about
// being a mock rather than inventing a plausible-sounding reply, so a demo can
// never mistake an unhandled question for a real answer.
var fallback = Scenario{
	Steps: []Step{{Source: garmin, Tool: "get_activities"}},
	Answer: `I don't have a canned answer for that one.

The backend is running in mock mode, so nothing here reaches Garmin or Claude — I only have scripted replies for the five example questions in the design spec. Ask one of those, or turn mock mode off once credentials are in place.`,
}

// Match picks the scenario for a question. Substring matching is crude on
// purpose: the point is to demo the UI, and anything smarter would be pretending
// to be the model's tool-selection judgment, which is exactly what mock mode
// cannot stand in for.
func Match(question string) Scenario {
	q := strings.ToLower(question)
	for _, s := range scenarios {
		for _, m := range s.Match {
			if strings.Contains(q, m) {
				return s
			}
		}
	}
	return fallback
}

// Sources returns the distinct sources a scenario touches, in call order —
// the mock equivalent of agent.Result.Sources.
func (s Scenario) Sources() []string {
	var out []string
	seen := map[string]bool{}
	for _, step := range s.Steps {
		if !seen[step.Source] {
			seen[step.Source] = true
			out = append(out, step.Source)
		}
	}
	return out
}
