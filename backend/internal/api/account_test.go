package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

func TestAccountRename(t *testing.T) {
	s := openServer(t)
	ana := signup(t, s, "ana@example.com", "Ana", "ana's long password", testInvite)
	if rec := do(t, s, http.MethodPatch, "/api/account", `{"display_name":"  Ana   R "}`, ana); rec.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	var me struct {
		Runner struct {
			DisplayName string `json:"display_name"`
		}
	}
	_ = json.Unmarshal(do(t, s, http.MethodGet, "/api/me", "", ana).Body.Bytes(), &me)
	if me.Runner.DisplayName != "Ana R" {
		t.Errorf("name: %q", me.Runner.DisplayName)
	}
	if rec := do(t, s, http.MethodPatch, "/api/account", `{"display_name":"   "}`, ana); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("empty name: %d", rec.Code)
	}
}

// A password change needs the current one, signs out every other session,
// keeps this one, and the new password works from then on.
func TestPasswordChange(t *testing.T) {
	s := openServer(t)
	here := signup(t, s, "ana@example.com", "Ana", "ana's long password", testInvite)
	rec := do(t, s, http.MethodPost, "/api/login", `{"email":"ana@example.com","password":"ana's long password"}`)
	elsewhere := rec.Result().Cookies()[0]

	if rec := do(t, s, http.MethodPost, "/api/account/password", `{"current":"wrong","new":"a brand new password"}`, here); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong current: %d", rec.Code)
	}
	if rec := do(t, s, http.MethodPost, "/api/account/password", `{"current":"ana's long password","new":"short"}`, here); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("short new: %d", rec.Code)
	}
	if rec := do(t, s, http.MethodPost, "/api/account/password", `{"current":"ana's long password","new":"a brand new password"}`, here); rec.Code != http.StatusOK {
		t.Fatalf("change: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, http.MethodGet, "/api/me", "", here); rec.Code != http.StatusOK {
		t.Errorf("this session after the change: %d", rec.Code)
	}
	if rec := do(t, s, http.MethodGet, "/api/me", "", elsewhere); rec.Code != http.StatusUnauthorized {
		t.Errorf("the other session after the change: %d", rec.Code)
	}
	if rec := do(t, s, http.MethodPost, "/api/login", `{"email":"ana@example.com","password":"ana's long password"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("old password still works: %d", rec.Code)
	}
	if rec := do(t, s, http.MethodPost, "/api/login", `{"email":"ana@example.com","password":"a brand new password"}`); rec.Code != http.StatusOK {
		t.Errorf("new password: %d", rec.Code)
	}
}

// Deleting an account takes its chats and Garmin bundle, ends its session,
// frees its email, and is refused for the owner and without the password.
func TestAccountDelete(t *testing.T) {
	s := openServer(t)
	ana := signup(t, s, "ana@example.com", "Ana", "ana's long password", testInvite)
	id := runnerOf(t, s, ana)
	do(t, s, http.MethodPost, "/api/chats", `{"title":"mine"}`, ana)
	_ = s.store.UpsertGarminConnection(context.Background(), store.GarminConnection{
		RunnerID: id, Kind: store.ConnectionLocalTokens, Blob: []byte("sealed"), KeyVersion: 1,
		Status: store.GarminConnected, ImportedAt: time.Now(),
	})

	if rec := do(t, s, http.MethodDelete, "/api/account", `{"password":"wrong"}`, ana); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: %d", rec.Code)
	}
	rec := do(t, s, http.MethodDelete, "/api/account", `{"password":"ana's long password"}`, ana)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, s, http.MethodGet, "/api/me", "", ana); rec.Code != http.StatusUnauthorized {
		t.Errorf("session after delete: %d", rec.Code)
	}
	if _, err := s.store.RunnerByID(context.Background(), id); err != store.ErrNotFound {
		t.Errorf("runner after delete: %v", err)
	}
	if c, _ := s.store.GarminConnection(context.Background(), id); c != nil {
		t.Error("the Garmin bundle survived the account")
	}
	// The address can be used again.
	signup(t, s, "ana@example.com", "Ana again", "another long password", testInvite)

	owner := ownerSession(t, s)
	if rec := do(t, s, http.MethodDelete, "/api/account", `{"password":"correct horse"}`, owner); rec.Code != http.StatusForbidden {
		t.Errorf("owner delete: %d", rec.Code)
	}
}

func ownerSession(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	rec := do(t, s, http.MethodPost, "/api/login", `{"email":"leo@example.com","password":"correct horse"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner login: %d", rec.Code)
	}
	return rec.Result().Cookies()[0]
}
