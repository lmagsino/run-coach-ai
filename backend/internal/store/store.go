// Package store is the only data access in the backend (ADR 0002). Every
// method that touches a runner's data takes the runner id and scopes the query
// to it; a test scans the SQL to enforce that. Two implementations: Postgres
// for the real service and an in-memory one for mock mode and tests.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Errors callers branch on.
var (
	ErrNotFound   = errors.New("store: not found")
	ErrNoSession  = errors.New("store: no such session")
	ErrEmailTaken = errors.New("store: email already registered")
)

// Runner kinds and statuses (CONTEXT.md).
const (
	KindOwner  = "owner"
	KindMember = "member"

	StatusActive   = "active"
	StatusDisabled = "disabled"
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
	Email            string // lowercased; empty for an owner without one
	PasswordHash     string // bcrypt; empty means this runner cannot sign in
	Status           string
	DisplayName      string
	DailyQuestionCap int
	CreatedAt        time.Time
	LastLoginAt      *time.Time
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
	RunnerByID(ctx context.Context, id int64) (Runner, error)
	RunnerByEmail(ctx context.Context, email string) (Runner, error)
	// CreateRunner inserts a runner and returns it with its id; ErrEmailTaken
	// when the email is already registered.
	CreateRunner(ctx context.Context, r Runner) (Runner, error)
	// UpdateRunner writes email, password hash, status, display name and cap
	// for the runner with r.ID.
	UpdateRunner(ctx context.Context, r Runner) error
	TouchLogin(ctx context.Context, id int64) error
	// DeleteRunner removes the runner and, by cascade, everything it owns:
	// sessions, chats, messages, usage and the Garmin connection.
	DeleteRunner(ctx context.Context, id int64) error

	CreateSession(ctx context.Context, runnerID int64, ttl time.Duration) (string, error)
	LookupSession(ctx context.Context, id string) (int64, error)
	DeleteSession(ctx context.Context, id string) error
	// DeleteSessionsFor signs a runner out everywhere except keep.
	DeleteSessionsFor(ctx context.Context, runnerID int64, keep string) error

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
	// GlobalUsage sums a day's usage across every runner, for the global cap.
	GlobalUsage(ctx context.Context, day time.Time) (Usage, error)

	Audit(ctx context.Context, runnerID int64, action string, detail any) error
}

// NormalizeEmail is the one form an email is stored and looked up in.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Day truncates a time to the UTC date the usage counters key on. The cap
// resets at midnight UTC, which the 429 message says.
func Day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
