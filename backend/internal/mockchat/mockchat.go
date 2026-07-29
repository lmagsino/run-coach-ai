// Package mockchat serves canned answers so the frontend can be built and
// demoed before any credentials exist. It stands in for the whole agent —
// Claude and both MCP sources — and is reachable only when RUNCOACH_MOCK is set.
//
// It deliberately mimics the *shape* of real work rather than just returning a
// string: each scenario names the tools a real answer would have called, in the
// order the agent would have called them, with a delay between so the UI's
// status steps advance the way they will in Phase 5. What it cannot mimic is the
// model's judgment — which sources a question actually needs is the model's
// decision, and verifying that is Phase 5's job.
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

const (
	strava = mcpclient.SourceStrava
	garmin = mcpclient.SourceGarmin
)

// scenarios covers the five example interactions in the feature spec (§4). The
// source plans intentionally differ — Garmin-only, Strava-only, both — so the
// status UI is exercised across every plan rather than only the two-source case.
var scenarios = []Scenario{
	{
		// Spec §4: "Does my sleep actually affect my pace? Show me the pattern."
		// Both sources: the correlation is the product's whole premise.
		Match: []string{"sleep", "hrv"},
		Steps: []Step{
			{Source: garmin, Tool: "get_sleep_data"},
			{Source: garmin, Tool: "get_hrv_trend"},
			{Source: strava, Tool: "list_activities"},
		},
		Answer: `Yes — and the pattern is consistent enough to plan around.

Across your last eight weeks, the nights you slept under six hours were followed by runs that came in slower almost every time. Your HRV tells the same story: the four lowest readings all landed the morning after a short night, and each was followed by a session you rated harder than the pace justified.

[figure: 25s | slower per km after nights under 6 hours, averaged over 8 weeks]

The effect shows up the next day, not two days later, so the lever you have is the night before a quality session — not the whole week.`,
	},
	{
		// Spec §4: "Am I overtraining right now?"
		// Both sources: training load alone is a Garmin-only signal (spec §3).
		Match: []string{"overtrain", "overreach", "burnt out", "burned out"},
		Steps: []Step{
			{Source: garmin, Tool: "get_training_load_trend"},
			{Source: garmin, Tool: "get_hrv_trend"},
			{Source: garmin, Tool: "get_stress_summary"},
			{Source: strava, Tool: "list_activities"},
		},
		Answer: `Not overtraining — but you're closer to the edge than last month.

Your Garmin training load has climbed steadily for three weeks without a down week, and your HRV trend is flat rather than falling. What has changed is variability: your last three easy runs drifted faster than easy, and stress scores have been climbing on the days after them.

[figure: 3 | consecutive easy runs faster than your easy pace ceiling]

Nothing here says stop. It says make the easy days genuinely easy for a week and watch whether stress settles.`,
	},
	{
		// Spec §4: "How did my last 3 marathon training blocks compare?"
		// Strava-only: this is a training-log question with no recovery component.
		Match: []string{"training block", "blocks compare", "marathon block", "last 3", "last three"},
		Steps: []Step{
			{Source: strava, Tool: "list_activities"},
		},
		Answer: `The most recent block was your most consistent, though not your highest volume.

Your spring block peaked highest — one 58km week — but it also had the two weeks you missed entirely. The block before it was steadier but capped out lower. This one has no missed weeks at all, and the long runs held their pace deep into the block rather than fading in the last fortnight.

[figure: 0 | missed weeks this block, against 2 and 1 in the previous two]

Consistency is the difference, not volume. That's usually the one that shows up on race day.`,
	},
	{
		// Spec §4: "I have a half marathon in 10 weeks — am I on track,
		// factoring in how recovered I've been?" Both sources: the readiness
		// half is Garmin, the pace half is Strava.
		// Note: the Garmin allowlist has no "readiness" tool, so recovery here is
		// the HRV trend and training load — the signals we can actually fetch.
		// A mock that cited a number no tool returns would set the UI up to show
		// a claim Phase 5 could never reproduce.
		Match: []string{"on track", "half marathon", "10 weeks", "race", "2:30"},
		Steps: []Step{
			{Source: strava, Tool: "list_activities"},
			{Source: garmin, Tool: "get_hrv_trend"},
			{Source: garmin, Tool: "get_training_load_trend"},
		},
		Answer: `Yes — you're trending a touch ahead of pace.

Your last four long runs have all come in faster than target, and the reason it's sticking is recovery: your HRV has held steady through the whole build while load rose, so you haven't been paying for the effort.

[figure: 6:44 | avg long-run pace per mile — eight seconds ahead of the pace you need]

Hold this and don't chase more volume. The gap you have now is the kind that recovery built, not fitness you have to force.`,
	},
	{
		// Spec §4: "My watch and Strava show different HR for yesterday's run
		// — which is right?" The reconciliation case from spec §3, and the only
		// one that requires listing activities from BOTH sources.
		Match: []string{"different hr", "differ", "disagree", "which is right", "conflict"},
		Steps: []Step{
			{Source: strava, Tool: "list_activities"},
			{Source: garmin, Tool: "get_activities"},
		},
		Answer: `They're both right about different things — the watch is the one to trust for average HR.

Strava has yesterday's run at 8.2km with an average of 154bpm; Garmin has the same session at 8.24km and 149bpm. The gap is in what each averaged over: Strava's figure starts when the activity does, and yours includes about ninety seconds of standing around at the start with an elevated HR from the warm-up.

[figure: 5bpm | the gap, entirely explained by the first 90 seconds]

Distance and pace agree to within a rounding error, so nothing else here is in question.`,
	},
}

// fallback answers anything that matches no scenario. It stays honest about
// being a mock rather than inventing a plausible-sounding reply, so a demo can
// never mistake an unhandled question for a real answer.
var fallback = Scenario{
	Steps: []Step{{Source: strava, Tool: "list_activities"}},
	Answer: `I don't have a canned answer for that one.

The backend is running in mock mode, so nothing here reaches Strava, Garmin or Claude — I only have scripted replies for the five example questions in the feature spec. Ask one of those, or turn mock mode off once credentials are in place.`,
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
