package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

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

// runnerOf resolves a session cookie to its runner id through the store.
func runnerOf(t *testing.T, s *Server, c *http.Cookie) int64 {
	t.Helper()
	id, err := s.store.LookupSession(context.Background(), c.Value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
