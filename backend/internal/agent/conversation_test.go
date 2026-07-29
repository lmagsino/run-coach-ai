package agent

import (
	"context"
	"strings"
	"testing"
)

// Spec §5 requires follow-up questions to retain context within a session. The
// server keeps no session state, so the only way that can work is the prior
// exchanges reaching the model on the next request — verified here by inspecting
// what was actually sent.
func TestPriorTurnsReachTheModel(t *testing.T) {
	strava, garmin, _, _ := stubbedSources(t)

	client, model := startFakeModel(t, fakeTurn{Text: "the week before that you ran 42km."})
	ag := New(client, "test-model", strava, garmin)

	prior := []PriorTurn{
		{Question: "how far did I run last week?", Answer: "You ran 48km last week."},
	}
	if _, err := ag.AnswerInConversation(context.Background(), prior, "what about the week before that?"); err != nil {
		t.Fatalf("AnswerInConversation: %v", err)
	}

	body := model.Requests()[0].Body
	for _, want := range []string{
		"how far did I run last week?", // the earlier question
		"You ran 48km last week.",      // and the answer it got
		"what about the week before that?",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("request body is missing %q — a follow-up could not resolve against it", want)
		}
	}
}

// Order matters: a conversation replayed out of sequence would have the model
// resolving "that" against the wrong exchange.
func TestPriorTurnsArriveInOrder(t *testing.T) {
	strava, _, _, _ := stubbedSources(t)
	client, model := startFakeModel(t, fakeTurn{Text: "final answer"})
	ag := New(client, "test-model", strava)

	prior := []PriorTurn{
		{Question: "FIRST_QUESTION", Answer: "FIRST_ANSWER"},
		{Question: "SECOND_QUESTION", Answer: "SECOND_ANSWER"},
	}
	if _, err := ag.AnswerInConversation(context.Background(), prior, "THIRD_QUESTION"); err != nil {
		t.Fatalf("AnswerInConversation: %v", err)
	}

	body := model.Requests()[0].Body
	var last int
	for _, marker := range []string{
		"FIRST_QUESTION", "FIRST_ANSWER", "SECOND_QUESTION", "SECOND_ANSWER", "THIRD_QUESTION",
	} {
		at := strings.Index(body, marker)
		if at < 0 {
			t.Fatalf("%s missing from request", marker)
		}
		if at < last {
			t.Errorf("%s appears out of order in the replayed conversation", marker)
		}
		last = at
	}
}

// Every turn is re-sent on every request, so an uncapped conversation would grow
// the prompt without bound.
func TestPriorTurnsAreCapped(t *testing.T) {
	strava, _, _, _ := stubbedSources(t)
	client, model := startFakeModel(t, fakeTurn{Text: "final answer"})
	ag := New(client, "test-model", strava)

	var prior []PriorTurn
	for i := 0; i < maxPriorTurns+4; i++ {
		prior = append(prior, PriorTurn{
			Question: "OLD_Q", Answer: "OLD_A",
		})
	}
	// Mark the oldest turn distinctly so we can assert it was dropped.
	prior[0] = PriorTurn{Question: "DROPPED_QUESTION", Answer: "DROPPED_ANSWER"}

	if _, err := ag.AnswerInConversation(context.Background(), prior, "current question"); err != nil {
		t.Fatalf("AnswerInConversation: %v", err)
	}

	body := model.Requests()[0].Body
	if strings.Contains(body, "DROPPED_QUESTION") {
		t.Errorf("history beyond maxPriorTurns=%d was replayed", maxPriorTurns)
	}
	if strings.Count(body, "OLD_Q") != maxPriorTurns {
		t.Errorf("replayed %d turns, want %d", strings.Count(body, "OLD_Q"), maxPriorTurns)
	}
}

// A turn that never produced an answer (an error, or one the user abandoned) must
// not be replayed as a bare question — the model would answer that instead of the
// one just asked.
func TestIncompletePriorTurnsAreSkipped(t *testing.T) {
	strava, _, _, _ := stubbedSources(t)
	client, model := startFakeModel(t, fakeTurn{Text: "final answer"})
	ag := New(client, "test-model", strava)

	prior := []PriorTurn{
		{Question: "ABANDONED_QUESTION", Answer: ""},
		{Question: "answered question", Answer: "its answer"},
	}
	if _, err := ag.AnswerInConversation(context.Background(), prior, "current question"); err != nil {
		t.Fatalf("AnswerInConversation: %v", err)
	}

	body := model.Requests()[0].Body
	if strings.Contains(body, "ABANDONED_QUESTION") {
		t.Error("a prior turn with no answer was replayed")
	}
	if !strings.Contains(body, "answered question") {
		t.Error("the completed prior turn was dropped too")
	}
}

// Answer must stay equivalent to a no-history conversation, since /chat and the
// existing tests rely on it.
func TestAnswerIsAConversationWithNoHistory(t *testing.T) {
	strava, _, _, _ := stubbedSources(t)
	client, _ := startFakeModel(t, fakeTurn{Text: "final answer"})
	ag := New(client, "test-model", strava)

	result, err := ag.Answer(context.Background(), "how far did I run?")
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if result.Answer != "final answer" {
		t.Errorf("answer: got %q, want %q", result.Answer, "final answer")
	}
}
