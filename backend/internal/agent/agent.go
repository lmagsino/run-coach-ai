// Package agent runs a Claude tool-calling loop whose tools are backed by MCP
// sessions. Claude decides which tools to call; the agent routes each call to
// the session that owns it and feeds results back until Claude answers.
//
// There is one source today, Garmin. The loop stays written for N sources
// because the roadmap adds the same Garmin data through a gateway or a
// runner-supplied URL as further sources (ADR 0002).
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/lmagsino/run-coach-ai/backend/internal/chart"
	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const basePrompt = `You are RunCoach, a running coach who reads the runner's own Garmin data. ` +
	`Answer questions about their training and recovery using ONLY the data returned by the tools. ` +
	`Ground every claim in what the tools return — cite specific numbers and dates (distances, ` +
	`paces, hours of sleep, HRV, load). If the tools return no data for a range, say so plainly; ` +
	`never invent sessions or fall back on generic training advice. If a tool fails, say what ` +
	`you could not read rather than guessing around it.

Read only what the question needs. When a scope is given with the question, read exactly that and nothing more; two or three tool calls answer most questions.

Be brief. The runner is learning to read their own data: answer in under 150 words, one or two short paragraphs, the conclusion first. Leave out numbers that do not change the conclusion. Offer more detail only if there is something worth digging into.`

// answerFormat teaches the one piece of structure the UI needs that prose cannot
// express: which single number deserves the pull-quote treatment (DESIGN.md §5).
// It is a marker rather than a JSON envelope so a malformed response degrades to
// plain text instead of costing the whole answer.
const answerFormat = `Formatting your answer:
- Open with a single short sentence that answers the question directly, then a blank line, then the supporting detail in short paragraphs separated by blank lines.
- If ONE number is the heart of the answer, put it on its own line as: [figure: VALUE | short caption]
  e.g. [figure: 4:44 | avg long-run pace per km — eight seconds ahead of the pace a 1:40 needs]
- At most one such line per answer, and only when a single figure genuinely carries it. Most answers have none. Never use it for a list of numbers, and never emit more than one.
- No markdown, headings, bullet lists or tables. Plain prose only.`

// sourceGuidance tells Claude what a source can answer. Only registered sources
// are described, so the prompt never promises data the agent cannot fetch.
var sourceGuidance = map[string]string{
	mcpclient.SourceGarmin: "the watch's view of the runner: every activity with distance, " +
		"pace, heart rate and splits, plus sleep, HRV, stress, body battery, training load " +
		"and status, readiness, VO2 max, race predictions, steps and body composition.",
}

// garminCategoryMap is the twelve-line index into roughly ninety Garmin tools.
// It encodes upstream's own range advice (one summary call over weeks, not one
// call per night) so the model reaches for the cheap, findable tool first.
// Every tool named here must exist upstream; a test checks the list against
// deploy/garmin-mcp/tools.txt.
const garminCategoryMap = `Finding the right Garmin tool (prefix garmin__):
- Activities over a range: get_activities_by_date (start, end); recent list: get_activities; one session's detail and splits: get_activity, get_activity_splits.
- Sleep over weeks: get_sleep_summary_range; one night in detail: get_sleep_data.
- HRV: get_hrv_trend for weeks, get_hrv_data for one day.
- Stress and body battery: get_stress_summary, get_all_day_stress, get_body_battery.
- Training load, status and recovery: get_training_load_trend, get_training_status, get_training_readiness, get_recovery_time_remaining.
- Fitness markers: get_vo2max_trend, get_race_predictions, get_lactate_threshold, get_endurance_score, get_hill_score, get_heart_rate_zones.
- Daily health: get_heart_rates_summary, get_respiration_summary, get_spo2_data, get_daily_steps, get_weekly_intensity_minutes.
- Body: get_body_composition, get_stats_and_body, get_fitnessage_data.
- Profile and devices: get_user_profile, get_primary_training_device.
- Lap-level detail from the FIT file: get_activity_fit_data, get_activity_fit_messages. Large; use only when the question is about intervals inside one session.
- Prefer one range tool over many single-day calls. Pass return_timeseries=false unless the question needs intra-day detail.
- Make at most eight tool calls per answer; ask for several in one turn when they do not depend on each other.`

// chartRules teaches when a chart earns its place. The tool is offered under
// tool_choice auto (forced choice is rejected on Opus 5.5), so the prompt is
// what decides the chart rate (ADR 0005).
const chartRules = `Charts (tool coach__show_chart):
- The chart is the proof of what you say. After the data tools and before the prose, call coach__show_chart with the series your answer is about. Skip it only when the answer is a single number or a yes/no that no series supports.
- Defaults by question: training or runs over a period → distance per run (bars, x = date, data activities); sleep → hours per night (bars, data sleep); recovery or overtraining → HRV or readiness per day (line); sleep vs pace → scatter of sleep against next-day pace; training blocks → distance per week (bars).
- Put the values in the chart, not in the prose. The prose names the pattern and the one or two numbers that matter.
- One chart at most. Never chart a single number; use the figure line for that.
- Reshape the data yourself into the points: one point per run, night or week. Pace goes in seconds per kilometre with y_unit "min/km".
- Two line series only when they share a unit. Two measures on different units are two charts; pick the one that carries the answer.
- Call the chart tool before writing the prose, after the data tools. The prose must still stand on its own if the chart is dropped.`

// CoachSource names the built-in source whose only tool is the chart. It is
// not a data source: a call is answered in-process and never counts towards
// Result.Sources.
const CoachSource = "coach"

// maxTurns bounds the tool loop; the prompt also tells the model the budget.
const maxTurns = 8

// toolNameSep separates a source name from the tool name in the namespaced tool
// names given to Claude, e.g. "garmin__get_sleep_data". A second source could
// expose the same tool names, so the prefix is what routes a call.
const toolNameSep = "__"

// Source is one MCP-backed tool source available to the agent. Name must be a
// short identifier safe for a tool name (see mcpclient.SourceGarmin).
type Source struct {
	Name    string
	Session *mcp.ClientSession
	// Allow, when set, decides which of the session's tools are offered to the
	// model. Garmin uses mcpclient.ReadOnly so write tools are never offered;
	// a hallucinated call to one hits the unknown-tool error like any other.
	Allow func(tool string) bool
}

// ToolCall records one tool invocation the agent made on Claude's behalf.
// Callers use it to show which sources a given answer actually drew on.
type ToolCall struct {
	Source string `json:"source"`
	Tool   string `json:"tool"`
	Failed bool   `json:"failed"`
}

// EventKind identifies a point in the tool-calling loop worth reporting.
type EventKind string

const (
	// EventToolStart fires immediately before a tool call is dispatched.
	EventToolStart EventKind = "tool_start"
	// EventToolEnd fires once it returns, with Failed set on error.
	EventToolEnd EventKind = "tool_end"
	// EventChart fires when the model's chart has been validated and kept.
	EventChart EventKind = "chart"
)

// Event is a progress report from inside the loop. Chart is set on EventChart.
type Event struct {
	Kind   EventKind
	Source string
	Tool   string
	Failed bool
	Chart  *chart.Spec
}

// Observer receives Events as the loop runs. It is called synchronously from
// Answer's goroutine, so an implementation that blocks stalls the answer — SSE
// writers should hand off rather than do slow work here.
type Observer func(Event)

// Observe registers an Observer so callers can report progress while an answer
// is still being assembled. Without one, Answer's tool-call trail is only
// readable after it returns, which is too late to narrate.
func (a *Agent) Observe(fn Observer) { a.observer = fn }

func (a *Agent) emit(ev Event) {
	if a.observer != nil {
		a.observer(ev)
	}
}

// Usage is the token accounting summed over every model call in one answer.
// Cached input is reported separately because it is what prompt caching on the
// system prompt and tool block saves (ADR 0006).
type Usage struct {
	InputTokens       int64
	CachedInputTokens int64
	OutputTokens      int64
}

// Result is a finished answer plus the trail of tool calls that produced it
// and, when the model asked for one and it validated, the chart.
type Result struct {
	Answer    string
	ToolCalls []ToolCall
	Usage     Usage
	Chart     *chart.Spec
}

// Sources returns the distinct source names the answer drew on, in call order.
func (r *Result) Sources() []string {
	var out []string
	seen := map[string]bool{}
	for _, c := range r.ToolCalls {
		if !seen[c.Source] {
			seen[c.Source] = true
			out = append(out, c.Source)
		}
	}
	return out
}

// Agent bridges the Anthropic Messages API to one or more MCP tool sessions.
type Agent struct {
	anthropic anthropic.Client
	model     anthropic.Model
	effort    anthropic.OutputConfigEffort
	sources   []Source
	observer  Observer
}

// New builds an Agent over already-connected MCP sessions. Order is preserved
// in the tool list handed to Claude. Effort defaults to medium.
func New(client anthropic.Client, model anthropic.Model, sources ...Source) *Agent {
	return &Agent{anthropic: client, model: model, effort: anthropic.OutputConfigEffortMedium, sources: sources}
}

// WithEffort sets the effort level sent with every request. Thinking cannot be
// switched off on Opus 5.5, so this is the one dial on reasoning cost.
func (a *Agent) WithEffort(effort string) *Agent {
	if effort != "" {
		a.effort = anthropic.OutputConfigEffort(effort)
	}
	return a
}

// toolRef points a namespaced tool name back at the session that owns it.
type toolRef struct {
	source  string
	tool    string
	session *mcp.ClientSession
}

// PriorTurn is one completed exchange earlier in the same conversation, used to
// resolve follow-up questions like "what about the week before that?".
type PriorTurn struct {
	Question string
	Answer   string
}

// maxPriorTurns caps how much history is replayed. Every turn is re-sent on each
// request, so an uncapped conversation grows the prompt without bound; the recent
// exchanges are the ones a follow-up actually refers back to.
const maxPriorTurns = 6

// maxToolResultRunes caps what one tool result feeds back to the model. A
// month of activities can run to tens of thousands of tokens; past this size
// the result is cut and the model is told to ask for a narrower range.
const maxToolResultRunes = 12000

// Answer runs the tool-calling loop with no prior context.
func (a *Agent) Answer(ctx context.Context, question string) (*Result, error) {
	return a.AnswerInConversation(ctx, nil, question)
}

// AnswerScoped is AnswerInConversation with the triage scope attached to the
// question, so the model reads exactly what the screener named.
func (a *Agent) AnswerScoped(ctx context.Context, prior []PriorTurn, question, scope string) (*Result, error) {
	if scope != "" {
		question = question + "\n\n(Scope: read " + scope + ". Nothing else.)"
	}
	return a.AnswerInConversation(ctx, prior, question)
}

// AnswerInConversation runs the tool-calling loop with earlier exchanges
// replayed first, so pronouns and relative ranges in a follow-up resolve against
// what was already discussed.
//
// Only the final text of each prior answer is replayed, not the tool_use blocks
// that produced it: the conclusions are what a follow-up refers to, and replaying
// whole tool payloads would cost far more tokens for no added context.
func (a *Agent) AnswerInConversation(ctx context.Context, prior []PriorTurn, question string) (*Result, error) {
	tools, router, err := a.buildTools(ctx)
	if err != nil {
		return nil, err
	}

	// The system prompt and the tool block are identical for every request, so
	// they are marked as one cache prefix: the breakpoint on the last tool
	// covers everything before it. Only the messages differ per request.
	system := []anthropic.TextBlockParam{{
		Text:         a.systemPrompt(),
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
	}}
	if len(tools) > 0 {
		tools[len(tools)-1].OfTool.CacheControl = anthropic.NewCacheControlEphemeralParam()
	}

	if len(prior) > maxPriorTurns {
		prior = prior[len(prior)-maxPriorTurns:]
	}
	var messages []anthropic.MessageParam
	for _, turn := range prior {
		// A turn with no answer never completed (an error, or a request the user
		// abandoned). Replaying the question alone would leave the model looking
		// at an unanswered prompt and re-answering it instead of the new one.
		if turn.Question == "" || turn.Answer == "" {
			continue
		}
		messages = append(messages,
			anthropic.NewUserMessage(anthropic.NewTextBlock(turn.Question)),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock(turn.Answer)),
		)
	}
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(question)))
	result := &Result{}

	for turn := 0; turn < maxTurns; turn++ {
		resp, err := a.anthropic.Messages.New(ctx, anthropic.MessageNewParams{
			Model:        a.model,
			MaxTokens:    2048,
			System:       system,
			Messages:     messages,
			Tools:        tools,
			OutputConfig: anthropic.OutputConfigParam{Effort: a.effort},
		})
		if err != nil {
			return nil, fmt.Errorf("claude message: %w", err)
		}
		result.Usage.InputTokens += resp.Usage.InputTokens
		result.Usage.CachedInputTokens += resp.Usage.CacheReadInputTokens
		result.Usage.OutputTokens += resp.Usage.OutputTokens
		messages = append(messages, resp.ToParam())

		if resp.StopReason != anthropic.StopReasonToolUse {
			result.Answer = textOf(resp)
			return result, nil
		}

		var results []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			tu, ok := block.AsAny().(anthropic.ToolUseBlock)
			if !ok {
				continue
			}
			ref, known := router[tu.Name]
			if !known {
				results = append(results, anthropic.NewToolResultBlock(tu.ID,
					fmt.Sprintf("no such tool %q", tu.Name), true))
				continue
			}
			if ref.source == CoachSource {
				out, isErr := a.showChart(result, tu.JSON.Input.Raw())
				results = append(results, anthropic.NewToolResultBlock(tu.ID, out, isErr))
				continue
			}
			a.emit(Event{Kind: EventToolStart, Source: ref.source, Tool: ref.tool})
			out, callErr := callTool(ctx, ref, tu.JSON.Input.Raw())
			a.emit(Event{
				Kind: EventToolEnd, Source: ref.source, Tool: ref.tool, Failed: callErr != nil,
			})
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				Source: ref.source, Tool: ref.tool, Failed: callErr != nil,
			})
			results = append(results, anthropic.NewToolResultBlock(tu.ID, out, callErr != nil))
		}
		messages = append(messages, anthropic.NewUserMessage(results...))
	}
	return nil, fmt.Errorf("gave up after %d tool-use turns without a final answer", maxTurns)
}

// showChart answers the virtual chart tool in-process: the first valid spec in
// a reply is kept and announced; a second call is refused with the first kept;
// an invalid one returns the validator's message, naming the field, so the
// model can fix it or carry on without a chart. Nothing here reaches a source.
func (a *Agent) showChart(result *Result, rawArgs string) (string, bool) {
	if result.Chart != nil {
		return "One chart per reply; the first one is kept. Continue with the prose.", true
	}
	var spec chart.Spec
	if err := json.Unmarshal([]byte(rawArgs), &spec); err != nil {
		return fmt.Sprintf("invalid chart arguments: %v", err), true
	}
	if err := spec.Validate(); err != nil {
		return fmt.Sprintf("chart rejected, %v. Fix it or continue without a chart.", err), true
	}
	result.Chart = &spec
	a.emit(Event{Kind: EventChart, Source: CoachSource, Tool: chart.ToolName, Chart: &spec})
	return "Chart accepted; it will be shown with your reply. Continue with the prose.", false
}

// systemPrompt assembles the instructions for this agent's actual source set:
// the grounding rules, what each registered source is good for, the Garmin
// category map when Garmin is registered, the answer format, and the current
// date so relative ranges like "last week" can be resolved.
func (a *Agent) systemPrompt() string {
	var b strings.Builder
	b.WriteString(basePrompt)

	b.WriteString("\n\nAvailable data sources, one tool set each. Tool names are\n")
	b.WriteString("prefixed with the source they belong to (e.g. garmin" + toolNameSep + "get_sleep_data):\n")
	hasGarmin := false
	for _, src := range a.sources {
		guidance, ok := sourceGuidance[src.Name]
		if !ok {
			guidance = "an additional data source; rely on its tool descriptions."
		}
		fmt.Fprintf(&b, "- %s — %s\n", src.Name, guidance)
		if src.Name == mcpclient.SourceGarmin {
			hasGarmin = true
		}
	}

	if hasGarmin {
		b.WriteString("\n")
		b.WriteString(garminCategoryMap)
	}

	b.WriteString("\n\n")
	b.WriteString(chartRules)

	b.WriteString("\n\n")
	b.WriteString(answerFormat)

	fmt.Fprintf(&b, "\n\nThe current date and time is %s. Use it to resolve relative "+
		"dates (e.g. \"last week\", \"this month\").",
		time.Now().UTC().Format(time.RFC3339))

	return b.String()
}

// buildTools converts every source's MCP tools into Anthropic tool definitions
// with source-namespaced names, plus the routing table to undo the namespacing.
// A source's Allow filter is applied here, so a denied tool is never offered
// and never routable. The built-in chart tool is appended last, under the
// coach source, and routes to no session.
func (a *Agent) buildTools(ctx context.Context) ([]anthropic.ToolUnionParam, map[string]toolRef, error) {
	var tools []anthropic.ToolUnionParam
	router := map[string]toolRef{}

	for _, src := range a.sources {
		list, err := src.Session.ListTools(ctx, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("list %s mcp tools: %w", src.Name, err)
		}
		for _, t := range list.Tools {
			if src.Allow != nil && !src.Allow(t.Name) {
				continue
			}
			name := src.Name + toolNameSep + t.Name
			router[name] = toolRef{source: src.Name, tool: t.Name, session: src.Session}
			tools = append(tools, anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
				Name:        name,
				Description: anthropic.String(fmt.Sprintf("[%s] %s", src.Name, t.Description)),
				InputSchema: inputSchema(t),
			}})
		}
	}

	chartTool := chart.ToolDefinition()
	chartTool.Name = CoachSource + toolNameSep + chart.ToolName
	router[chartTool.Name] = toolRef{source: CoachSource, tool: chart.ToolName}
	tools = append(tools, anthropic.ToolUnionParam{OfTool: &chartTool})
	return tools, router, nil
}

// inputSchema translates an MCP tool's JSON Schema into the Anthropic shape.
func inputSchema(t *mcp.Tool) anthropic.ToolInputSchemaParam {
	schema := anthropic.ToolInputSchemaParam{}
	m, ok := t.InputSchema.(map[string]any)
	if !ok {
		return schema
	}
	schema.Properties = m["properties"]
	if req, ok := m["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}
	return schema
}

// callTool invokes an MCP tool on its owning session and returns its textual result.
func callTool(ctx context.Context, ref toolRef, rawArgs string) (string, error) {
	var args map[string]any
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return fmt.Sprintf("invalid tool arguments: %v", err), err
		}
	}
	res, err := ref.session.CallTool(ctx, &mcp.CallToolParams{Name: ref.tool, Arguments: args})
	if err != nil {
		return fmt.Sprintf("tool error: %v", err), err
	}
	text := contentText(res)
	if r := []rune(text); len(r) > maxToolResultRunes {
		text = string(r[:maxToolResultRunes]) + fmt.Sprintf(
			"\n[cut: %d more characters. Ask for a narrower range or a summary tool.]", len(r)-maxToolResultRunes)
	}
	if res.IsError || mcpclient.LooksLikeError(text) {
		// The model still sees the text, so it can say what it could not
		// read; the step and the Garmin state record the failure.
		return text, fmt.Errorf("%s tool %q reported an error", ref.source, ref.tool)
	}
	return text, nil
}

func contentText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func textOf(resp *anthropic.Message) string {
	var b strings.Builder
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(tb.Text)
		}
	}
	return b.String()
}
