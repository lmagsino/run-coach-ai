// Package store is the only data access in the backend (ADR 0002). Every
// method that touches a runner's data takes the runner id and scopes the query
// to it; a test scans the SQL to enforce that. Two implementations: Postgres
// for the real service and an in-memory one for mock mode and tests.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Errors callers branch on.
var (
	ErrNotFound  = errors.New("store: not found")
	ErrNoSession = errors.New("store: no such session")
)

// Garmin connection states shown on the chip (CONTEXT.md).
const (
	GarminConnected   = "connected"
	GarminExpired     = "expired"
	GarminUnavailable = "unavailable"
)

// ConnectionLocalTokens is the one connection kind the MVP uses: a token
// bundle pasted by the runner (ADR 0003). The other kinds are the roadmap's.
const ConnectionLocalTokens = "local_tokens"

type Runner struct {
	ID               int64
	Kind             string
	DisplayName      string
	DailyQuestionCap int
}

type Chat struct {
	ID        string
	RunnerID  int64
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message is one turn. ToolCalls and Chart are raw JSON as stored.
type Message struct {
	ID        string
	ChatID    string
	Role      string // "user" | "assistant"
	Content   string
	ToolCalls json.RawMessage
	Chart     json.RawMessage
	CreatedAt time.Time
}

type GarminConnection struct {
	RunnerID     int64
	Kind         string
	Blob         []byte
	KeyVersion   int
	Status       string
	ImportedAt   time.Time
	LastOKAt     *time.Time
	ExpiresEstAt *time.Time
	LastError    string
}

type Usage struct {
	Questions         int
	InputTokens       int64
	CachedInputTokens int64
	OutputTokens      int64
}

// Store is everything the API needs to remember.
type Store interface {
	Owner(ctx context.Context) (Runner, error)

	CreateSession(ctx context.Context, runnerID int64, ttl time.Duration) (string, error)
	LookupSession(ctx context.Context, id string) (int64, error)
	DeleteSession(ctx context.Context, id string) error

	ListChats(ctx context.Context, runnerID int64) ([]Chat, error)
	CreateChat(ctx context.Context, runnerID int64, title string) (Chat, error)
	GetChat(ctx context.Context, runnerID int64, chatID string) (Chat, error)
	RenameChat(ctx context.Context, runnerID int64, chatID, title string) error
	DeleteChat(ctx context.Context, runnerID int64, chatID string) error

	ListMessages(ctx context.Context, runnerID int64, chatID string) ([]Message, error)
	AppendMessage(ctx context.Context, runnerID int64, msg Message) (Message, error)

	GarminConnection(ctx context.Context, runnerID int64) (*GarminConnection, error)
	UpsertGarminConnection(ctx context.Context, conn GarminConnection) error
	SetGarminStatus(ctx context.Context, runnerID int64, status, lastError string, ok bool) error
	DeleteGarminConnection(ctx context.Context, runnerID int64) error

	Usage(ctx context.Context, runnerID int64, day time.Time) (Usage, error)
	AddUsage(ctx context.Context, runnerID int64, day time.Time, add Usage) error

	Audit(ctx context.Context, runnerID int64, action string, detail any) error
}

// Day truncates a time to the UTC date the usage counters key on. The cap
// resets at midnight UTC, which the 429 message says.
func Day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
