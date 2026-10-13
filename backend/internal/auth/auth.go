// Package auth decides who a request belongs to. One implementation today,
// OwnerPassword; the roadmap adds GatewayOAuth (ADR 0002). Routes never ask
// "is this the owner": they get a runner id from the session and nothing else.
package auth

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// ErrDenied is the one error a caller sees for a bad credential, so a response
// cannot hint at which part was wrong.
var ErrDenied = errors.New("auth: denied")

// Credentials is what a login form sends. Only Password is used today.
type Credentials struct {
	Password string
}

// Provider turns credentials into a runner id, or ErrDenied.
type Provider interface {
	Authenticate(ctx context.Context, c Credentials) (runnerID int64, err error)
}

// OwnerPassword accepts one password, stored as a bcrypt hash in config, and
// always answers with the owner's runner id.
type OwnerPassword struct {
	Hash    []byte
	OwnerID int64
}

func (p OwnerPassword) Authenticate(_ context.Context, c Credentials) (int64, error) {
	if len(p.Hash) == 0 || c.Password == "" {
		return 0, ErrDenied
	}
	if err := bcrypt.CompareHashAndPassword(p.Hash, []byte(c.Password)); err != nil {
		return 0, ErrDenied
	}
	return p.OwnerID, nil
}

// HashPassword is what cmd/hash-password prints for RUNCOACH_OWNER_PASSWORD_HASH.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}
