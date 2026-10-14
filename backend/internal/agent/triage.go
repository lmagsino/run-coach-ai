package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// Triage is the cheap first look at a question, taken by a small model before
// any Garmin spawn or coach call: is it about this runner's training at all,
// and if so, exactly what should be read. It keeps off-topic questions from
// costing a reply and turns a vague one ("how's my training") into a bounded
// read, which is what keeps payloads and tool calls small.
type Triage struct {
	OnTopic bool   `json:"on_topic"`
	Scope   string `json:"scope"`   // what to read: metrics and a date range
	Decline string `json:"decline"` // one sentence, when off topic
}

const triagePrompt = `You screen questions for RunCoach, a coach that reads one runner's Garmin data (runs, sleep, HRV, stress, body battery, training load and status, readiness, VO2 max, race predictions, steps, body composition). Reply with JSON only, no prose:
{"on_topic": true|false, "scope": "...", "decline": "..."}

on_topic is true when the question is about this runner's training, recovery, health metrics, racing, or how the coach works. Anything else (general knowledge, other people, unrelated tasks) is false.

scope, when on topic: one line naming exactly what to read from Garmin and the date range, concrete and small. Resolve relative dates against today. A vague question gets a sensible default scope, for example "runs and training status for the last 14 days". A follow-up refers to the previous exchange, so keep its scope. Prefer summaries over ranges (one sleep summary call, not one call per night).

decline, when off topic: one friendly sentence saying the coach only talks about the runner's training and recovery.`

// triageTurns bounds how much history the screener sees: the last exchange is
// what a follow-up refers to.
const triageTurns = 1

// RunTriage asks the small model to screen and scope a question. It fails
// open: on any error the question is treated as on topic with no scope, so a
// screener outage never blocks the runner.
func RunTriage(ctx context.Context, client anthropic.Client, model anthropic.Model, prior []PriorTurn, question string, now time.Time) (Triage, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Today is %s.\n", now.UTC().Format("Monday 2 January 2006"))
	if len(prior) > triageTurns {
		prior = prior[len(prior)-triageTurns:]
	}
	for _, p := range prior {
		if p.Question != "" && p.Answer != "" {
			fmt.Fprintf(&b, "\nPrevious question: %s\nPrevious answer (start): %s\n", p.Question, firstN(p.Answer, 300))
		}
	}
	fmt.Fprintf(&b, "\nQuestion: %s", question)

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 300,
		System:    []anthropic.TextBlockParam{{Text: triagePrompt}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(b.String()))},
	})
	if err != nil {
		return Triage{OnTopic: true}, fmt.Errorf("triage: %w", err)
	}
	t, err := parseTriage(textOf(resp))
	if err != nil {
		return Triage{OnTopic: true}, err
	}
	return t, nil
}

// parseTriage reads the JSON object out of the model's text, tolerating
// anything around it.
func parseTriage(text string) (Triage, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return Triage{}, fmt.Errorf("triage: no JSON in %q", firstN(text, 80))
	}
	var t Triage
	if err := json.Unmarshal([]byte(text[start:end+1]), &t); err != nil {
		return Triage{}, fmt.Errorf("triage: %w", err)
	}
	t.Scope = strings.TrimSpace(t.Scope)
	t.Decline = strings.TrimSpace(t.Decline)
	if !t.OnTopic && t.Decline == "" {
		t.Decline = "I can only talk about your own training and recovery: your runs, sleep, HRV, stress, load and races."
	}
	return t, nil
}

func firstN(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
