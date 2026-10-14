package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

// Two runners on one server see only their own chats, messages, Garmin state
// and usage, and get 404 (never 403, which would confirm the chat exists) for
// anything of the other's. This is the tenancy seam ADR 0002 promised.
func TestTwoRunnersAreIsolated(t *testing.T) {
	s := openServer(t)
	ana := signup(t, s, "ana@example.com", "Ana", "ana's long password", testInvite)
	ben := signup(t, s, "ben@example.com", "Ben", "ben's long password", testInvite)

	rec := do(t, s, http.MethodPost, "/api/chats", `{"title":"Ana's long run"}`, ana)
	var anaChat chatJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &anaChat)
	if anaChat.ID == "" {
		t.Fatalf("create chat: %s", rec.Body.String())
	}
	anaID := runnerOf(t, s, ana)
	if _, err := s.store.AppendMessage(context.Background(), anaID, store.Message{ChatID: anaChat.ID, Role: "user", Content: "how far did I run?"}); err != nil {
		t.Fatal(err)
	}
	_ = s.store.UpsertGarminConnection(context.Background(), store.GarminConnection{
		RunnerID: anaID, Kind: store.ConnectionLocalTokens, Blob: []byte("sealed"), KeyVersion: 1,
		Status: store.GarminConnected, ImportedAt: time.Now(),
	})
	_ = s.store.AddUsage(context.Background(), anaID, time.Now(), store.Usage{Questions: 3})

	// Ben's list is empty and none of Ana's chat routes answer him.
	var list struct{ Chats []chatJSON }
	_ = json.Unmarshal(do(t, s, http.MethodGet, "/api/chats", "", ben).Body.Bytes(), &list)
	if len(list.Chats) != 0 {
		t.Errorf("Ben sees chats: %+v", list.Chats)
	}
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/chats/" + anaChat.ID + "/messages", ""},
		{http.MethodPatch, "/api/chats/" + anaChat.ID, `{"title":"mine now"}`},
		{http.MethodDelete, "/api/chats/" + anaChat.ID, ""},
		{http.MethodPost, "/api/chats/" + anaChat.ID + "/stream", `{"question":"hi"}`},
	} {
		if rec := do(t, s, c.method, c.path, c.body, ben); rec.Code != http.StatusNotFound {
			t.Errorf("Ben %s %s: got %d, want 404", c.method, c.path, rec.Code)
		}
	}

	// Ben's /me has his own Garmin state and usage, not Ana's.
	var me struct {
		Runner struct{ Email string }
		Garmin struct{ Status string }
		Usage  struct{ Today int }
	}
	_ = json.Unmarshal(do(t, s, http.MethodGet, "/api/me", "", ben).Body.Bytes(), &me)
	if me.Runner.Email != "ben@example.com" || me.Garmin.Status != "none" || me.Usage.Today != 0 {
		t.Errorf("Ben's /me: %+v", me)
	}

	// Ben disconnecting Garmin does not touch Ana's connection.
	if rec := do(t, s, http.MethodDelete, "/api/garmin/tokens", "", ben); rec.Code != http.StatusOK {
		t.Fatalf("Ben's delete: %d", rec.Code)
	}
	if c, _ := s.store.GarminConnection(context.Background(), anaID); c == nil {
		t.Error("Ben's disconnect removed Ana's Garmin connection")
	}

	// And Ana still has everything.
	_ = json.Unmarshal(do(t, s, http.MethodGet, "/api/chats", "", ana).Body.Bytes(), &list)
	if len(list.Chats) != 1 || list.Chats[0].Title != "Ana's long run" {
		t.Errorf("Ana's chats: %+v", list.Chats)
	}
	_ = json.Unmarshal(do(t, s, http.MethodGet, "/api/me", "", ana).Body.Bytes(), &me)
	if me.Garmin.Status != "connected" || me.Usage.Today != 3 {
		t.Errorf("Ana's /me: %+v", me)
	}
}

// A runner who has not connected Garmin gets the fix sentence, never a reply
// built from whatever tokens the configured command could reach.
func TestRunnerWithoutBundleIsNotConnected(t *testing.T) {
	s := openServer(t)
	s.cfg.AnthropicAPIKey = "sk-test" // past the key check; no model call is reached
	ben := signup(t, s, "ben@example.com", "Ben", "ben's long password", testInvite)
	rec := do(t, s, http.MethodPost, "/api/chats", `{}`, ben)
	var chat chatJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &chat)
	// Triage needs a model; give it a scripted one that says on topic.
	s.anthropic = scriptedTriage(t)
	rec = do(t, s, http.MethodPost, "/api/chats/"+chat.ID+"/stream", `{"question":"how did I sleep?"}`, ben)
	events := readSSE(t, rec.Body.String())
	last := events[len(events)-1]
	if last.Name != "error" || !strings.Contains(last.Data["error"].(string), "Garmin is not connected") {
		t.Errorf("stream for a runner with no bundle ended with %+v", last)
	}
}

// runnerOf resolves a session cookie to its runner id through the store.
func runnerOf(t *testing.T, s *Server, c *http.Cookie) int64 {
	t.Helper()
	id, err := s.store.LookupSession(context.Background(), c.Value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// scriptedTriage is a Messages API stand-in that answers every call with an
// on-topic triage verdict, so a stream gets past screening without a network.
func scriptedTriage(t *testing.T) anthropic.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"m","type":"message","role":"assistant","model":"x",
			"content":[{"type":"text","text":"{\"on_topic\": true, \"scope\": \"sleep, last 7 days\"}"}],
			"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	t.Cleanup(srv.Close)
	return anthropic.NewClient(option.WithoutEnvironmentDefaults(), option.WithBaseURL(srv.URL), option.WithAPIKey("test"), option.WithMaxRetries(0))
}
