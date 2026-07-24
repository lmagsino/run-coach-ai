package strava

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
)

// ErrNoToken is returned when no token is stored for a provider yet.
var ErrNoToken = errors.New("no stored token")

// TokenStore persists OAuth tokens in the oauth_tokens table.
type TokenStore struct {
	pool *pgxpool.Pool
}

// NewTokenStore creates a TokenStore backed by the given pool.
func NewTokenStore(pool *pgxpool.Pool) *TokenStore {
	return &TokenStore{pool: pool}
}

// Save upserts the token for a provider. It best-effort extracts the Strava
// athlete id from the token's extra fields for convenience.
func (s *TokenStore) Save(ctx context.Context, provider string, tok *oauth2.Token) error {
	var athleteID string
	if athlete, ok := tok.Extra("athlete").(map[string]any); ok {
		if id, ok := athlete["id"].(float64); ok {
			athleteID = strconv.FormatInt(int64(id), 10)
		}
	}

	raw, _ := json.Marshal(map[string]any{
		"token_type": tok.TokenType,
		"expiry":     tok.Expiry,
		"athlete":    tok.Extra("athlete"),
	})

	var expiresAt *time.Time
	if !tok.Expiry.IsZero() {
		expiresAt = &tok.Expiry
	}

	const q = `
		INSERT INTO oauth_tokens
			(provider, access_token, refresh_token, token_type, expires_at, athlete_id, raw, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (provider) DO UPDATE SET
			access_token  = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_type    = EXCLUDED.token_type,
			expires_at    = EXCLUDED.expires_at,
			athlete_id    = EXCLUDED.athlete_id,
			raw           = EXCLUDED.raw,
			updated_at    = now()`
	if _, err := s.pool.Exec(ctx, q, provider, tok.AccessToken, tok.RefreshToken,
		tok.TokenType, expiresAt, nullIfEmpty(athleteID), raw); err != nil {
		return fmt.Errorf("upsert oauth token: %w", err)
	}
	return nil
}

// Load returns the stored token for a provider, or ErrNoToken if none exists.
func (s *TokenStore) Load(ctx context.Context, provider string) (*oauth2.Token, error) {
	const q = `
		SELECT access_token, refresh_token, token_type, expires_at
		FROM oauth_tokens WHERE provider = $1`

	var accessToken, refreshToken, tokenType string
	var expiresAt *time.Time
	err := s.pool.QueryRow(ctx, q, provider).Scan(&accessToken, &refreshToken, &tokenType, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoToken
	}
	if err != nil {
		return nil, fmt.Errorf("load oauth token: %w", err)
	}

	tok := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    tokenType,
	}
	if expiresAt != nil {
		tok.Expiry = *expiresAt
	}
	return tok, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
