package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Memory is the in-memory store for mock mode and tests. It seeds the owner
// as runner 1 exactly like the migration does.
type Memory struct {
	mu       sync.Mutex
	runners  map[int64]Runner
	nextRun  int64
	sessions map[string]memSession
	chats    map[string]Chat
	messages map[string][]Message
	deleted  map[string]bool
	garmin   map[int64]GarminConnection
	usage    map[string]Usage
	audit    []auditRow
	seq      int
}

type memSession struct {
	runnerID  int64
	expiresAt time.Time
}

type auditRow struct {
	runnerID int64
	action   string
	detail   any
}

func NewMemory() *Memory {
	return &Memory{
		runners: map[int64]Runner{1: {ID: 1, Kind: KindOwner, Status: StatusActive, DisplayName: "Owner",
			DailyQuestionCap: 30, CreatedAt: time.Now()}},
		nextRun:  2,
		sessions: map[string]memSession{},
		chats:    map[string]Chat{},
		messages: map[string][]Message{},
		deleted:  map[string]bool{},
		garmin:   map[int64]GarminConnection{},
		usage:    map[string]Usage{},
	}
}

func (m *Memory) nextID(prefix string) string {
	m.seq++
	return fmt.Sprintf("%s-%d", prefix, m.seq)
}

func (m *Memory) Owner(context.Context) (Runner, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var owner *Runner
	for _, r := range m.runners {
		if r.Kind == KindOwner && (owner == nil || r.ID < owner.ID) {
			r := r
			owner = &r
		}
	}
	if owner == nil {
		return Runner{}, fmt.Errorf("owner row missing: %w", ErrNotFound)
	}
	return *owner, nil
}

func (m *Memory) RunnerByID(_ context.Context, id int64) (Runner, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runners[id]
	if !ok {
		return Runner{}, ErrNotFound
	}
	return r, nil
}

func (m *Memory) RunnerByEmail(_ context.Context, email string) (Runner, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	email = NormalizeEmail(email)
	for _, r := range m.runners {
		if email != "" && r.Email == email {
			return r, nil
		}
	}
	return Runner{}, ErrNotFound
}

func (m *Memory) emailTakenLocked(email string, except int64) bool {
	for _, r := range m.runners {
		if email != "" && r.Email == email && r.ID != except {
			return true
		}
	}
	return false
}

func (m *Memory) CreateRunner(_ context.Context, r Runner) (Runner, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.Email = NormalizeEmail(r.Email)
	if m.emailTakenLocked(r.Email, 0) {
		return Runner{}, ErrEmailTaken
	}
	if r.Status == "" {
		r.Status = StatusActive
	}
	r.ID = m.nextRun
	m.nextRun++
	r.CreatedAt = time.Now()
	m.runners[r.ID] = r
	return r, nil
}

func (m *Memory) UpdateRunner(_ context.Context, r Runner) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.runners[r.ID]
	if !ok {
		return ErrNotFound
	}
	r.Email = NormalizeEmail(r.Email)
	if m.emailTakenLocked(r.Email, r.ID) {
		return ErrEmailTaken
	}
	cur.Email, cur.PasswordHash, cur.Status = r.Email, r.PasswordHash, r.Status
	cur.DisplayName, cur.DailyQuestionCap = r.DisplayName, r.DailyQuestionCap
	m.runners[r.ID] = cur
	return nil
}

func (m *Memory) TouchLogin(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.runners[id]; ok {
		now := time.Now()
		r.LastLoginAt = &now
		m.runners[id] = r
	}
	return nil
}

// DeleteRunner mirrors the Postgres cascade by hand.
func (m *Memory) DeleteRunner(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.runners[id]; !ok {
		return ErrNotFound
	}
	delete(m.runners, id)
	for sid, se := range m.sessions {
		if se.runnerID == id {
			delete(m.sessions, sid)
		}
	}
	for cid, c := range m.chats {
		if c.RunnerID == id {
			delete(m.chats, cid)
			delete(m.messages, cid)
			delete(m.deleted, cid)
		}
	}
	delete(m.garmin, id)
	prefix := fmt.Sprintf("%d/", id)
	for k := range m.usage {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			delete(m.usage, k)
		}
	}
	for i := range m.audit {
		if m.audit[i].runnerID == id {
			m.audit[i].runnerID = 0
		}
	}
	return nil
}

func (m *Memory) CreateSession(_ context.Context, runnerID int64, ttl time.Duration) (string, error) {
	id, err := newSessionID()
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[id] = memSession{runnerID: runnerID, expiresAt: time.Now().Add(ttl)}
	return id, nil
}

func (m *Memory) LookupSession(_ context.Context, id string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok || time.Now().After(s.expiresAt) {
		return 0, ErrNoSession
	}
	if r, ok := m.runners[s.runnerID]; !ok || r.Status != StatusActive {
		return 0, ErrNoSession
	}
	return s.runnerID, nil
}

func (m *Memory) DeleteSessionsFor(_ context.Context, runnerID int64, keep string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for sid, se := range m.sessions {
		if se.runnerID == runnerID && sid != keep {
			delete(m.sessions, sid)
		}
	}
	return nil
}

func (m *Memory) DeleteSession(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
	return nil
}

func (m *Memory) ListChats(_ context.Context, runnerID int64) ([]Chat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Chat
	for id, c := range m.chats {
		if c.RunnerID == runnerID && !m.deleted[id] {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (m *Memory) CreateChat(_ context.Context, runnerID int64, title string) (Chat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	c := Chat{ID: m.nextID("chat"), RunnerID: runnerID, Title: title, CreatedAt: now, UpdatedAt: now}
	m.chats[c.ID] = c
	return c, nil
}

func (m *Memory) getLocked(runnerID int64, chatID string) (Chat, error) {
	c, ok := m.chats[chatID]
	if !ok || c.RunnerID != runnerID || m.deleted[chatID] {
		return Chat{}, ErrNotFound
	}
	return c, nil
}

func (m *Memory) GetChat(_ context.Context, runnerID int64, chatID string) (Chat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getLocked(runnerID, chatID)
}

func (m *Memory) RenameChat(_ context.Context, runnerID int64, chatID, title string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.getLocked(runnerID, chatID)
	if err != nil {
		return err
	}
	c.Title = title
	c.UpdatedAt = time.Now()
	m.chats[chatID] = c
	return nil
}

func (m *Memory) DeleteChat(_ context.Context, runnerID int64, chatID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.getLocked(runnerID, chatID); err != nil {
		return err
	}
	m.deleted[chatID] = true
	return nil
}

func (m *Memory) ListMessages(_ context.Context, runnerID int64, chatID string) ([]Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.getLocked(runnerID, chatID); err != nil {
		return nil, err
	}
	return append([]Message(nil), m.messages[chatID]...), nil
}

func (m *Memory) AppendMessage(_ context.Context, runnerID int64, msg Message) (Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.getLocked(runnerID, msg.ChatID)
	if err != nil {
		return msg, err
	}
	msg.ID = m.nextID("msg")
	msg.CreatedAt = time.Now()
	m.messages[msg.ChatID] = append(m.messages[msg.ChatID], msg)
	c.UpdatedAt = msg.CreatedAt
	m.chats[msg.ChatID] = c
	return msg, nil
}

func (m *Memory) GarminConnection(_ context.Context, runnerID int64) (*GarminConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.garmin[runnerID]
	if !ok {
		return nil, nil
	}
	return &c, nil
}

func (m *Memory) UpsertGarminConnection(_ context.Context, c GarminConnection) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.garmin[c.RunnerID] = c
	return nil
}

func (m *Memory) SetGarminStatus(_ context.Context, runnerID int64, status, lastError string, ok bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, found := m.garmin[runnerID]
	if !found {
		return nil
	}
	c.Status, c.LastError = status, lastError
	if ok {
		now := time.Now()
		c.LastOKAt = &now
	}
	m.garmin[runnerID] = c
	return nil
}

func (m *Memory) DeleteGarminConnection(_ context.Context, runnerID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.garmin, runnerID)
	return nil
}

func usageKey(runnerID int64, day time.Time) string {
	return fmt.Sprintf("%d/%s", runnerID, Day(day).Format("2006-01-02"))
}

func (m *Memory) Usage(_ context.Context, runnerID int64, day time.Time) (Usage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.usage[usageKey(runnerID, day)], nil
}

func (m *Memory) GlobalUsage(_ context.Context, day time.Time) (Usage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	suffix := "/" + Day(day).Format("2006-01-02")
	var total Usage
	for k, u := range m.usage {
		if len(k) > len(suffix) && k[len(k)-len(suffix):] == suffix {
			total.Questions += u.Questions
			total.InputTokens += u.InputTokens
			total.CachedInputTokens += u.CachedInputTokens
			total.OutputTokens += u.OutputTokens
		}
	}
	return total, nil
}

func (m *Memory) AddUsage(_ context.Context, runnerID int64, day time.Time, add Usage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := usageKey(runnerID, day)
	u := m.usage[k]
	u.Questions += add.Questions
	u.InputTokens += add.InputTokens
	u.CachedInputTokens += add.CachedInputTokens
	u.OutputTokens += add.OutputTokens
	m.usage[k] = u
	return nil
}

func (m *Memory) Audit(_ context.Context, runnerID int64, action string, detail any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audit = append(m.audit, auditRow{runnerID: runnerID, action: action, detail: detail})
	return nil
}
