package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmagsino/run-coach-ai/backend/internal/db"
)

// The behavioural suite runs against the in-memory store always, and against
// Postgres when RUNCOACH_TEST_DATABASE_URL names a database the test may
// migrate up and down (it drops every table at the end).
func stores(t *testing.T) map[string]Store {
	t.Helper()
	out := map[string]Store{"memory": NewMemory()}
	if url := os.Getenv("RUNCOACH_TEST_DATABASE_URL"); url != "" {
		_ = db.MigrateDown(url)
		if err := db.MigrateUp(url); err != nil {
			t.Fatalf("migrate up: %v", err)
		}
		pool, err := pgxpool.New(context.Background(), url)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			pool.Close()
			_ = db.MigrateDown(url)
		})
		out["postgres"] = NewPG(pool)
	}
	return out
}

func TestOwnerIsRowOne(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			owner, err := s.Owner(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if owner.ID != 1 || owner.Kind != KindOwner || owner.DailyQuestionCap != 30 || owner.Status != StatusActive {
				t.Errorf("owner: %+v", owner)
			}
		})
	}
}

func TestSessionsExpireAndDelete(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			id, err := s.CreateSession(ctx, 1, time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := s.LookupSession(ctx, id); err != nil || got != 1 {
				t.Fatalf("lookup: %d, %v", got, err)
			}
			if _, err := s.LookupSession(ctx, "nope"); err != ErrNoSession {
				t.Errorf("unknown session: %v", err)
			}
			expired, _ := s.CreateSession(ctx, 1, -time.Minute)
			if _, err := s.LookupSession(ctx, expired); err != ErrNoSession {
				t.Errorf("expired session was accepted: %v", err)
			}
			if err := s.DeleteSession(ctx, id); err != nil {
				t.Fatal(err)
			}
			if _, err := s.LookupSession(ctx, id); err != ErrNoSession {
				t.Errorf("deleted session was accepted: %v", err)
			}
		})
	}
}

// Chats and messages belong to one runner: another runner id sees nothing,
// which is the tenancy seam the roadmap relies on (ADR 0002).
func TestChatsAreScopedToTheRunner(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			c, err := s.CreateChat(ctx, 1, "Sleep vs pace")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.AppendMessage(ctx, 1, Message{ChatID: c.ID, Role: "user", Content: "q"}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.AppendMessage(ctx, 1, Message{ChatID: c.ID, Role: "assistant", Content: "a",
				ToolCalls: json.RawMessage(`[{"source":"garmin","tool":"get_sleep_data"}]`),
				Chart:     json.RawMessage(`{"kind":"bar"}`)}); err != nil {
				t.Fatal(err)
			}

			msgs, err := s.ListMessages(ctx, 1, c.ID)
			if err != nil || len(msgs) != 2 || msgs[1].Role != "assistant" {
				t.Fatalf("messages: %+v, %v", msgs, err)
			}
			// Postgres re-serialises JSONB with its own spacing, so compare the
			// value, not the bytes.
			if compact(t, msgs[1].Chart) != `{"kind":"bar"}` || !strings.Contains(compact(t, msgs[1].ToolCalls), `"get_sleep_data"`) {
				t.Errorf("stored JSON: chart %s, tool_calls %s", msgs[1].Chart, msgs[1].ToolCalls)
			}
			chats, _ := s.ListChats(ctx, 1)
			if len(chats) != 1 || chats[0].Title != "Sleep vs pace" {
				t.Errorf("chats: %+v", chats)
			}

			// Runner 2 (the roadmap's second runner; only runner 1 exists as a row,
			// but scoping is by id regardless).
			if _, err := s.GetChat(ctx, 2, c.ID); err != ErrNotFound {
				t.Errorf("another runner could read the chat: %v", err)
			}
			if _, err := s.ListMessages(ctx, 2, c.ID); err != ErrNotFound {
				t.Errorf("another runner could list messages: %v", err)
			}
			if err := s.RenameChat(ctx, 2, c.ID, "x"); err != ErrNotFound {
				t.Errorf("another runner could rename: %v", err)
			}
			if err := s.DeleteChat(ctx, 2, c.ID); err != ErrNotFound {
				t.Errorf("another runner could delete: %v", err)
			}
			if _, err := s.GetChat(ctx, 1, "not-a-uuid"); err != ErrNotFound {
				t.Errorf("malformed id: %v", err)
			}

			if err := s.RenameChat(ctx, 1, c.ID, "Renamed"); err != nil {
				t.Fatal(err)
			}
			if err := s.DeleteChat(ctx, 1, c.ID); err != nil {
				t.Fatal(err)
			}
			if chats, _ := s.ListChats(ctx, 1); len(chats) != 0 {
				t.Errorf("deleted chat still listed: %+v", chats)
			}
			if _, err := s.ListMessages(ctx, 1, c.ID); err != ErrNotFound {
				t.Errorf("deleted chat's messages still readable: %v", err)
			}
		})
	}
}

func TestGarminConnectionLifecycle(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if c, err := s.GarminConnection(ctx, 1); err != nil || c != nil {
				t.Fatalf("before upsert: %+v, %v", c, err)
			}
			exp := time.Now().Add(180 * 24 * time.Hour)
			err := s.UpsertGarminConnection(ctx, GarminConnection{
				RunnerID: 1, Kind: ConnectionLocalTokens, Blob: []byte("sealed"), KeyVersion: 1,
				Status: GarminConnected, ImportedAt: time.Now(), ExpiresEstAt: &exp,
			})
			if err != nil {
				t.Fatal(err)
			}
			c, err := s.GarminConnection(ctx, 1)
			if err != nil || c == nil || string(c.Blob) != "sealed" || c.Status != GarminConnected || c.ExpiresEstAt == nil {
				t.Fatalf("after upsert: %+v, %v", c, err)
			}
			if err := s.SetGarminStatus(ctx, 1, GarminExpired, "401", false); err != nil {
				t.Fatal(err)
			}
			c, _ = s.GarminConnection(ctx, 1)
			if c.Status != GarminExpired || c.LastError != "401" || c.LastOKAt != nil {
				t.Errorf("after expiry: %+v", c)
			}
			if err := s.SetGarminStatus(ctx, 1, GarminConnected, "", true); err != nil {
				t.Fatal(err)
			}
			c, _ = s.GarminConnection(ctx, 1)
			if c.Status != GarminConnected || c.LastOKAt == nil {
				t.Errorf("after ok: %+v", c)
			}
			if err := s.DeleteGarminConnection(ctx, 1); err != nil {
				t.Fatal(err)
			}
			if c, _ := s.GarminConnection(ctx, 1); c != nil {
				t.Errorf("after delete: %+v", c)
			}
		})
	}
}

func TestUsageAccumulatesPerDay(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			day := time.Date(2026, 10, 13, 15, 0, 0, 0, time.UTC)
			if err := s.AddUsage(ctx, 1, day, Usage{Questions: 1, InputTokens: 100, CachedInputTokens: 80, OutputTokens: 20}); err != nil {
				t.Fatal(err)
			}
			if err := s.AddUsage(ctx, 1, day.Add(2*time.Hour), Usage{Questions: 1, InputTokens: 50}); err != nil {
				t.Fatal(err)
			}
			u, err := s.Usage(ctx, 1, day)
			if err != nil || u.Questions != 2 || u.InputTokens != 150 || u.CachedInputTokens != 80 || u.OutputTokens != 20 {
				t.Errorf("usage: %+v, %v", u, err)
			}
			if u, _ := s.Usage(ctx, 1, day.Add(24*time.Hour)); u.Questions != 0 {
				t.Errorf("next day: %+v", u)
			}
			if err := s.Audit(ctx, 1, "tokens_imported", map[string]any{"kind": "local_tokens"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func compact(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("stored JSON is not JSON: %s", raw)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// Members: create, find by email (any case), refuse a second sign-up with the
// same address, update, and sign-in only while active.
func TestRunnersLifecycle(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			m, err := s.CreateRunner(ctx, Runner{Kind: KindMember, Email: "  Ana@Example.com ", PasswordHash: "h", DisplayName: "Ana", DailyQuestionCap: 10})
			if err != nil {
				t.Fatal(err)
			}
			if m.ID <= 1 || m.Email != "ana@example.com" || m.Status != StatusActive {
				t.Errorf("created: %+v", m)
			}
			if _, err := s.CreateRunner(ctx, Runner{Kind: KindMember, Email: "ANA@example.com", DisplayName: "x"}); err != ErrEmailTaken {
				t.Errorf("duplicate email: %v", err)
			}
			got, err := s.RunnerByEmail(ctx, "ana@EXAMPLE.com")
			if err != nil || got.ID != m.ID || got.PasswordHash != "h" {
				t.Errorf("by email: %+v %v", got, err)
			}
			if _, err := s.RunnerByEmail(ctx, "nobody@example.com"); err != ErrNotFound {
				t.Errorf("unknown email: %v", err)
			}
			got.DisplayName, got.PasswordHash = "Ana R", "h2"
			if err := s.UpdateRunner(ctx, got); err != nil {
				t.Fatal(err)
			}
			if got, _ = s.RunnerByID(ctx, m.ID); got.DisplayName != "Ana R" || got.PasswordHash != "h2" {
				t.Errorf("after update: %+v", got)
			}
			if err := s.TouchLogin(ctx, m.ID); err != nil {
				t.Fatal(err)
			}
			if got, _ = s.RunnerByID(ctx, m.ID); got.LastLoginAt == nil {
				t.Error("last login not recorded")
			}

			// Sessions: a disabled runner's sessions stop working, and a
			// password change can sign out everywhere else.
			a, _ := s.CreateSession(ctx, m.ID, time.Hour)
			b, _ := s.CreateSession(ctx, m.ID, time.Hour)
			if err := s.DeleteSessionsFor(ctx, m.ID, a); err != nil {
				t.Fatal(err)
			}
			if _, err := s.LookupSession(ctx, b); err != ErrNoSession {
				t.Errorf("other session survived: %v", err)
			}
			if _, err := s.LookupSession(ctx, a); err != nil {
				t.Errorf("kept session gone: %v", err)
			}
			got.Status = StatusDisabled
			_ = s.UpdateRunner(ctx, got)
			if _, err := s.LookupSession(ctx, a); err != ErrNoSession {
				t.Errorf("disabled runner's session still works: %v", err)
			}
		})
	}
}

// Deleting a runner takes everything it owns and nothing anyone else owns.
func TestDeleteRunnerCascades(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			m, err := s.CreateRunner(ctx, Runner{Kind: KindMember, Email: "del@example.com", DisplayName: "Del", DailyQuestionCap: 10})
			if err != nil {
				t.Fatal(err)
			}
			mine, _ := s.CreateChat(ctx, m.ID, "mine")
			_, _ = s.AppendMessage(ctx, m.ID, Message{ChatID: mine.ID, Role: "user", Content: "q"})
			ownerChat, _ := s.CreateChat(ctx, 1, "owner's")
			sid, _ := s.CreateSession(ctx, m.ID, time.Hour)
			_ = s.UpsertGarminConnection(ctx, GarminConnection{RunnerID: m.ID, Kind: ConnectionLocalTokens, Blob: []byte("b"), KeyVersion: 1, Status: GarminConnected, ImportedAt: time.Now()})
			day := time.Now()
			_ = s.AddUsage(ctx, m.ID, day, Usage{Questions: 2})
			_ = s.AddUsage(ctx, 1, day, Usage{Questions: 3})
			if g, _ := s.GlobalUsage(ctx, day); g.Questions < 5 {
				t.Errorf("global usage before delete: %+v", g)
			}

			if err := s.DeleteRunner(ctx, m.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.RunnerByID(ctx, m.ID); err != ErrNotFound {
				t.Errorf("runner survived: %v", err)
			}
			if _, err := s.GetChat(ctx, m.ID, mine.ID); err != ErrNotFound {
				t.Errorf("chat survived: %v", err)
			}
			if _, err := s.LookupSession(ctx, sid); err != ErrNoSession {
				t.Errorf("session survived: %v", err)
			}
			if c, _ := s.GarminConnection(ctx, m.ID); c != nil {
				t.Error("garmin connection survived")
			}
			if u, _ := s.Usage(ctx, m.ID, day); u.Questions != 0 {
				t.Errorf("usage survived: %+v", u)
			}
			if _, err := s.GetChat(ctx, 1, ownerChat.ID); err != nil {
				t.Errorf("the owner's chat went too: %v", err)
			}
			if err := s.DeleteRunner(ctx, m.ID); err != ErrNotFound {
				t.Errorf("second delete: %v", err)
			}
		})
	}
}

// Every Postgres statement that touches a runner's table must name runner_id,
// and nothing outside this package may run SQL: the scoped store is the only
// data access (ADR 0002).
func TestEveryRunnerTableQueryIsScoped(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "pg.go"))
	if err != nil {
		t.Fatal(err)
	}
	stmts := regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(string(src), -1)
	tables := regexp.MustCompile(`(?i)\b(FROM|INTO|UPDATE)\s+(chat_sessions|chat_messages|garmin_connections|usage_days)\b`)
	var checked int
	for _, m := range stmts {
		sql := m[1]
		if !tables.MatchString(sql) {
			continue
		}
		checked++
		// The global cap reads every runner's usage on purpose and says so.
		if strings.Contains(sql, "across all runners") {
			continue
		}
		if !strings.Contains(sql, "runner_id") {
			t.Errorf("statement touches a runner table without runner_id:\n%s", sql)
		}
	}
	if checked < 10 {
		t.Errorf("only %d runner-table statements found; the scan looks broken", checked)
	}

	internal := filepath.Join(filepath.Dir(thisFile), "..")
	offenders := regexp.MustCompile(`\.(Query|QueryRow|Exec)\(`)
	_ = filepath.Walk(internal, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"store"+string(filepath.Separator)) {
			return nil
		}
		b, _ := os.ReadFile(path)
		if offenders.Match(b) {
			t.Errorf("%s runs SQL outside internal/store", path)
		}
		return nil
	})
}
