package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

// A member starts with the member cap and is refused at it; the owner keeps
// its own; and the global cap stops everyone once the day's total is reached,
// before any model call.
func TestCaps(t *testing.T) {
	s := openServer(t)
	s.cfg.GlobalDailyCap = 15
	s.cfg.AnthropicAPIKey = "" // a refusal must come from a cap, never from the model
	ana := signup(t, s, "ana@example.com", "Ana", "ana's long password", testInvite)
	ben := signup(t, s, "ben@example.com", "Ben", "ben's long password", testInvite)
	ctx := context.Background()
	anaID, benID := runnerOf(t, s, ana), runnerOf(t, s, ben)

	ask := func(c *http.Cookie) (int, string) {
		rec := do(t, s, http.MethodPost, "/api/chat", `{"question":"how did I sleep?"}`, c)
		return rec.Code, rec.Body.String()
	}

	// Ana at her member cap of 10: refused with her own message.
	_ = s.store.AddUsage(ctx, anaID, time.Now(), store.Usage{Questions: 10})
	if code, body := ask(ana); code != http.StatusTooManyRequests || !strings.Contains(body, "daily cap reached") {
		t.Errorf("Ana at her cap: %d %s", code, body)
	}
	// Ben is under his own cap, so he gets past the caps (and stops at the
	// missing key, which proves no cap refused him).
	if code, body := ask(ben); code != http.StatusServiceUnavailable {
		t.Errorf("Ben under his cap: want the missing-key 503, got %d %s", code, body)
	}
	// The day's total reaches the global cap: Ben is refused too.
	_ = s.store.AddUsage(ctx, benID, time.Now(), store.Usage{Questions: 5})
	if code, body := ask(ben); code != http.StatusTooManyRequests || !strings.Contains(body, "all the questions it can today") {
		t.Errorf("Ben at the global cap: %d %s", code, body)
	}
	// Turning the global cap off lets him through again.
	s.cfg.GlobalDailyCap = 0
	if code, _ := ask(ben); code != http.StatusServiceUnavailable {
		t.Errorf("global cap 0 should mean off; got %d", code)
	}
}
