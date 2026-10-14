// Package auth decides who a request belongs to. One implementation today,
// EmailPassword; the roadmap adds GatewayOAuth (ADR 0002). Routes never ask
// "is this the owner": they get a runner id from the session and nothing else.
package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/lmagsino/run-coach-ai/backend/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// ErrDenied is the one error a caller sees for a bad credential, so a response
// cannot hint at which part was wrong.
var ErrDenied = errors.New("auth: denied")

// MinPasswordLen is the shortest password sign-up and a password change accept.
const MinPasswordLen = 10

// Credentials is what a sign-in form sends.
type Credentials struct {
	Email    string
	Password string
}

// Provider turns credentials into a runner id, or ErrDenied.
type Provider interface {
	Authenticate(ctx context.Context, c Credentials) (runnerID int64, err error)
}

// Runners is the slice of the store a provider reads.
type Runners interface {
	Owner(ctx context.Context) (store.Runner, error)
	RunnerByEmail(ctx context.Context, email string) (store.Runner, error)
}

// EmailPassword checks an email and a bcrypt-hashed password against the
// runners table. An empty email means the owner, so the single-runner sign-in
// form that predates accounts keeps working. A disabled runner, a runner with
// no password and an unknown email all come back as ErrDenied.
type EmailPassword struct {
	Runners Runners
}

// dummyHash is compared against when there is no runner, so an unknown email
// takes as long as a wrong password and the timing does not say which it was.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not a real password"), bcrypt.DefaultCost)

func (p EmailPassword) Authenticate(ctx context.Context, c Credentials) (int64, error) {
	if c.Password == "" {
		return 0, ErrDenied
	}
	var (
		r   store.Runner
		err error
	)
	if strings.TrimSpace(c.Email) == "" {
		r, err = p.Runners.Owner(ctx)
	} else {
		r, err = p.Runners.RunnerByEmail(ctx, c.Email)
	}
	if errors.Is(err, store.ErrNotFound) || (err == nil && (r.PasswordHash == "" || r.Status != store.StatusActive)) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(c.Password))
		return 0, ErrDenied
	}
	if err != nil {
		return 0, err
	}
	if bcrypt.CompareHashAndPassword([]byte(r.PasswordHash), []byte(c.Password)) != nil {
		return 0, ErrDenied
	}
	return r.ID, nil
}

// CheckPassword reports whether password matches a stored hash.
func CheckPassword(hash, password string) bool {
	return hash != "" && bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// HashPassword is what cmd/hash-password prints for RUNCOACH_OWNER_PASSWORD_HASH.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}
