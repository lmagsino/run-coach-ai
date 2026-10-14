package agent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTriageScopesAnOnTopicQuestion(t *testing.T) {
	client, model := startFakeModel(t,
		fakeTurn{Text: `{"on_topic": true, "scope": "runs and training status for 1-14 October 2026", "decline": ""}`},
	)
	got, err := RunTriage(context.Background(), client, "triage-model", nil, "how's my training", time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !got.OnTopic || got.Scope != "runs and training status for 1-14 October 2026" {
		t.Errorf("triage: %+v", got)
	}
	body := model.Requests()[0].Body
	for _, want := range []string{"Wednesday 14 October 2026", "how's my training", "triage-model"} {
		if !strings.Contains(body, want) {
			t.Errorf("request is missing %q", want)
		}
	}
	if strings.Contains(body, `"tools"`) {
		t.Error("triage offered tools; it must stay cheap")
	}
}

func TestTriageDeclinesOffTopic(t *testing.T) {
	client, _ := startFakeModel(t,
		fakeTurn{Text: "Sure:\n```json\n{\"on_topic\": false, \"scope\": \"\", \"decline\": \"I only talk about your training.\"}\n```"},
	)
	got, err := RunTriage(context.Background(), client, "m", nil, "what is the capital of France?", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got.OnTopic || got.Decline != "I only talk about your training." {
		t.Errorf("triage: %+v", got)
	}
}

func TestTriageCarriesTheLastExchange(t *testing.T) {
	client, model := startFakeModel(t, fakeTurn{Text: `{"on_topic": true, "scope": "same runs, the week before"}`})
	prior := []PriorTurn{{Question: "old", Answer: "old answer"}, {Question: "how far did I run last week?", Answer: "48 km."}}
	if _, err := RunTriage(context.Background(), client, "m", prior, "and the week before that?", time.Now()); err != nil {
		t.Fatal(err)
	}
	body := model.Requests()[0].Body
	if !strings.Contains(body, "how far did I run last week?") || strings.Contains(body, "old answer") {
		t.Errorf("history: want only the last exchange, got:\n%s", body)
	}
}

// A broken screener must never block the runner.
func TestTriageFailsOpen(t *testing.T) {
	client, _ := startFakeModel(t, fakeTurn{Text: "I cannot help with that."})
	got, err := RunTriage(context.Background(), client, "m", nil, "how did I sleep?", time.Now())
	if err == nil {
		t.Error("expected a parse error")
	}
	if !got.OnTopic || got.Scope != "" {
		t.Errorf("fail-open: %+v", got)
	}
	if _, err := parseTriage(`{"on_topic": false}`); err != nil {
		t.Fatal(err)
	}
	if tr, _ := parseTriage(`{"on_topic": false}`); tr.Decline == "" {
		t.Error("off topic without a decline sentence got no default")
	}
}
