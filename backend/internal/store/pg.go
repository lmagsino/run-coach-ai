package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PG is the Postgres store. Every statement below that reads or writes a
// runner's rows names runner_id in its WHERE or VALUES; store_test.go scans
// this file to keep it that way.
type PG struct{ pool *pgxpool.Pool }

func NewPG(pool *pgxpool.Pool) *PG { return &PG{pool: pool} }

const runnerColumns = `id, kind, COALESCE(email, ''), password_hash, status, display_name, daily_question_cap, created_at, last_login_at`

func scanRunner(row pgx.Row) (Runner, error) {
	var r Runner
	err := row.Scan(&r.ID, &r.Kind, &r.Email, &r.PasswordHash, &r.Status, &r.DisplayName, &r.DailyQuestionCap, &r.CreatedAt, &r.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

func (s *PG) Owner(ctx context.Context) (Runner, error) {
	r, err := scanRunner(s.pool.QueryRow(ctx,
		`SELECT `+runnerColumns+` FROM runners WHERE kind = 'owner' ORDER BY id LIMIT 1`))
	if errors.Is(err, ErrNotFound) {
		return r, fmt.Errorf("owner row missing: %w", ErrNotFound)
	}
	return r, err
}

func (s *PG) RunnerByID(ctx context.Context, id int64) (Runner, error) {
	return scanRunner(s.pool.QueryRow(ctx, `SELECT `+runnerColumns+` FROM runners WHERE id = $1`, id))
}

func (s *PG) RunnerByEmail(ctx context.Context, email string) (Runner, error) {
	return scanRunner(s.pool.QueryRow(ctx,
		`SELECT `+runnerColumns+` FROM runners WHERE email = $1`, NormalizeEmail(email)))
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *PG) CreateRunner(ctx context.Context, r Runner) (Runner, error) {
	if r.Status == "" {
		r.Status = StatusActive
	}
	created, err := scanRunner(s.pool.QueryRow(ctx,
		`INSERT INTO runners (kind, email, password_hash, status, display_name, daily_question_cap)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING `+runnerColumns,
		r.Kind, nullable(NormalizeEmail(r.Email)), r.PasswordHash, r.Status, r.DisplayName, r.DailyQuestionCap))
	if isUniqueViolation(err) {
		return Runner{}, ErrEmailTaken
	}
	return created, err
}

func (s *PG) UpdateRunner(ctx context.Context, r Runner) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE runners SET email = $2, password_hash = $3, status = $4, display_name = $5, daily_question_cap = $6
		 WHERE id = $1`,
		r.ID, nullable(NormalizeEmail(r.Email)), r.PasswordHash, r.Status, r.DisplayName, r.DailyQuestionCap)
	if isUniqueViolation(err) {
		return ErrEmailTaken
	}
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *PG) TouchLogin(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE runners SET last_login_at = now() WHERE id = $1`, id)
	return err
}

func (s *PG) DeleteRunner(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM runners WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// isUniqueViolation reports a Postgres unique_violation (23505).
func isUniqueViolation(err error) bool {
	return err != nil && contains(err.Error(), "SQLSTATE 23505")
}

func newSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *PG) CreateSession(ctx context.Context, runnerID int64, ttl time.Duration) (string, error) {
	id, err := newSessionID()
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO sessions (id, runner_id, expires_at) VALUES ($1, $2, $3)`,
		id, runnerID, time.Now().Add(ttl))
	return id, err
}

// LookupSession resolves a live session of an active runner; a disabled or
// deleted runner's sessions stop working at once.
func (s *PG) LookupSession(ctx context.Context, id string) (int64, error) {
	var runnerID int64
	err := s.pool.QueryRow(ctx,
		`UPDATE sessions AS se SET last_seen_at = now() FROM runners AS r
		 WHERE se.id = $1 AND se.expires_at > now() AND r.id = se.runner_id AND r.status = 'active'
		 RETURNING se.runner_id`, id,
	).Scan(&runnerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoSession
	}
	return runnerID, err
}

func (s *PG) DeleteSession(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

func (s *PG) DeleteSessionsFor(ctx context.Context, runnerID int64, keep string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE runner_id = $1 AND id <> $2`, runnerID, keep)
	return err
}

func (s *PG) ListChats(ctx context.Context, runnerID int64) ([]Chat, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, runner_id, title, created_at, updated_at FROM chat_sessions
		 WHERE runner_id = $1 AND deleted_at IS NULL ORDER BY updated_at DESC`, runnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Chat
	for rows.Next() {
		var c Chat
		if err := rows.Scan(&c.ID, &c.RunnerID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *PG) CreateChat(ctx context.Context, runnerID int64, title string) (Chat, error) {
	var c Chat
	err := s.pool.QueryRow(ctx,
		`INSERT INTO chat_sessions (runner_id, title) VALUES ($1, $2)
		 RETURNING id, runner_id, title, created_at, updated_at`, runnerID, title,
	).Scan(&c.ID, &c.RunnerID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *PG) GetChat(ctx context.Context, runnerID int64, chatID string) (Chat, error) {
	var c Chat
	err := s.pool.QueryRow(ctx,
		`SELECT id, runner_id, title, created_at, updated_at FROM chat_sessions
		 WHERE runner_id = $1 AND id = $2 AND deleted_at IS NULL`, runnerID, chatID,
	).Scan(&c.ID, &c.RunnerID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		return c, ErrNotFound
	}
	return c, err
}

func (s *PG) RenameChat(ctx context.Context, runnerID int64, chatID, title string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE chat_sessions SET title = $3, updated_at = now()
		 WHERE runner_id = $1 AND id = $2 AND deleted_at IS NULL`, runnerID, chatID, title)
	if isInvalidUUID(err) || (err == nil && tag.RowsAffected() == 0) {
		return ErrNotFound
	}
	return err
}

func (s *PG) DeleteChat(ctx context.Context, runnerID int64, chatID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE chat_sessions SET deleted_at = now()
		 WHERE runner_id = $1 AND id = $2 AND deleted_at IS NULL`, runnerID, chatID)
	if isInvalidUUID(err) || (err == nil && tag.RowsAffected() == 0) {
		return ErrNotFound
	}
	return err
}

func (s *PG) ListMessages(ctx context.Context, runnerID int64, chatID string) ([]Message, error) {
	if _, err := s.GetChat(ctx, runnerID, chatID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, chat_session_id, role, content, tool_calls, chart, created_at FROM chat_messages
		 WHERE runner_id = $1 AND chat_session_id = $2 ORDER BY created_at, id`, runnerID, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.Role, &m.Content, &m.ToolCalls, &m.Chart, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *PG) AppendMessage(ctx context.Context, runnerID int64, msg Message) (Message, error) {
	if _, err := s.GetChat(ctx, runnerID, msg.ChatID); err != nil {
		return msg, err
	}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO chat_messages (chat_session_id, runner_id, role, content, tool_calls, chart)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		msg.ChatID, runnerID, msg.Role, msg.Content, nullJSON(msg.ToolCalls), nullJSON(msg.Chart),
	).Scan(&msg.ID, &msg.CreatedAt)
	if err != nil {
		return msg, err
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE chat_sessions SET updated_at = now() WHERE runner_id = $1 AND id = $2`, runnerID, msg.ChatID)
	return msg, err
}

func (s *PG) GarminConnection(ctx context.Context, runnerID int64) (*GarminConnection, error) {
	var c GarminConnection
	err := s.pool.QueryRow(ctx,
		`SELECT runner_id, kind, blob, key_version, status, imported_at, last_ok_at, expires_est_at, last_error
		 FROM garmin_connections WHERE runner_id = $1`, runnerID,
	).Scan(&c.RunnerID, &c.Kind, &c.Blob, &c.KeyVersion, &c.Status, &c.ImportedAt, &c.LastOKAt, &c.ExpiresEstAt, &c.LastError)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *PG) UpsertGarminConnection(ctx context.Context, c GarminConnection) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO garmin_connections (runner_id, kind, blob, key_version, status, imported_at, last_ok_at, expires_est_at, last_error)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (runner_id) DO UPDATE SET kind = EXCLUDED.kind, blob = EXCLUDED.blob,
		   key_version = EXCLUDED.key_version, status = EXCLUDED.status, imported_at = EXCLUDED.imported_at,
		   last_ok_at = EXCLUDED.last_ok_at, expires_est_at = EXCLUDED.expires_est_at, last_error = EXCLUDED.last_error`,
		c.RunnerID, c.Kind, c.Blob, c.KeyVersion, c.Status, c.ImportedAt, c.LastOKAt, c.ExpiresEstAt, c.LastError)
	return err
}

func (s *PG) SetGarminStatus(ctx context.Context, runnerID int64, status, lastError string, ok bool) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE garmin_connections SET status = $2, last_error = $3,
		   last_ok_at = CASE WHEN $4 THEN now() ELSE last_ok_at END
		 WHERE runner_id = $1`, runnerID, status, lastError, ok)
	return err
}

func (s *PG) DeleteGarminConnection(ctx context.Context, runnerID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM garmin_connections WHERE runner_id = $1`, runnerID)
	return err
}

func (s *PG) Usage(ctx context.Context, runnerID int64, day time.Time) (Usage, error) {
	var u Usage
	err := s.pool.QueryRow(ctx,
		`SELECT questions, input_tokens, cached_input_tokens, output_tokens FROM usage_days
		 WHERE runner_id = $1 AND day = $2`, runnerID, Day(day),
	).Scan(&u.Questions, &u.InputTokens, &u.CachedInputTokens, &u.OutputTokens)
	if errors.Is(err, pgx.ErrNoRows) {
		return Usage{}, nil
	}
	return u, err
}

func (s *PG) GlobalUsage(ctx context.Context, day time.Time) (Usage, error) {
	var u Usage
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(questions), 0), COALESCE(SUM(input_tokens), 0),
		        COALESCE(SUM(cached_input_tokens), 0), COALESCE(SUM(output_tokens), 0)
		 FROM usage_days WHERE day = $1 -- across all runners, for the global cap`, Day(day),
	).Scan(&u.Questions, &u.InputTokens, &u.CachedInputTokens, &u.OutputTokens)
	return u, err
}

func (s *PG) AddUsage(ctx context.Context, runnerID int64, day time.Time, add Usage) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO usage_days (runner_id, day, questions, input_tokens, cached_input_tokens, output_tokens)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (runner_id, day) DO UPDATE SET
		   questions = usage_days.questions + EXCLUDED.questions,
		   input_tokens = usage_days.input_tokens + EXCLUDED.input_tokens,
		   cached_input_tokens = usage_days.cached_input_tokens + EXCLUDED.cached_input_tokens,
		   output_tokens = usage_days.output_tokens + EXCLUDED.output_tokens`,
		runnerID, Day(day), add.Questions, add.InputTokens, add.CachedInputTokens, add.OutputTokens)
	return err
}

func (s *PG) Audit(ctx context.Context, runnerID int64, action string, detail any) error {
	var raw []byte
	if detail != nil {
		var err error
		if raw, err = json.Marshal(detail); err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO audit_log (runner_id, action, detail) VALUES ($1, $2, $3)`, runnerID, action, nullJSON(raw))
	return err
}

func nullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

// A malformed chat id in the URL is "not found", not a 500: pgx reports it as
// an encoding error for the uuid parameter.
func isInvalidUUID(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "invalid input syntax for type uuid") || contains(msg, "cannot parse UUID") || contains(msg, "invalid UUID")
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
