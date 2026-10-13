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
			if owner.ID != 1 || owner.Kind != "owner" || owner.DailyQuestionCap != 30 {
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
			if err != nil || len(msgs) != 2 || msgs[1].Role != "assistant" || string(msgs[1].Chart) != `{"kind":"bar"}` {
				t.Fatalf("messages: %+v, %v", msgs, err)
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
