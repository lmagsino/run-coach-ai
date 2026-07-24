// Package strava handles Strava OAuth2: building the authorize URL, exchanging
// the authorization code for tokens, persisting them, and returning a valid
// (auto-refreshed) access token for API/MCP calls.
package strava

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
)

// provider is the key under which Strava tokens are stored.
const provider = "strava"

// Strava OAuth endpoints.
var endpoint = oauth2.Endpoint{
	AuthURL:  "https://www.strava.com/oauth/authorize",
	TokenURL: "https://www.strava.com/oauth/token",
}

// Client wraps the OAuth2 config and token persistence for Strava.
type Client struct {
	cfg   *oauth2.Config
	store *TokenStore
}

// NewClient builds a Strava OAuth client. The scope is passed as a single
// comma-joined value because Strava expects comma-separated scopes (not the
// space-separated form oauth2 would otherwise produce).
func NewClient(clientID, clientSecret, redirectURI string, store *TokenStore) *Client {
	return &Client{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURI,
			Scopes:       []string{"read,activity:read_all"},
			Endpoint:     endpoint,
		},
		store: store,
	}
}

// Configured reports whether client credentials are present.
func (c *Client) Configured() bool {
	return c.cfg.ClientID != "" && c.cfg.ClientSecret != ""
}

// AuthCodeURL returns the Strava authorize URL to redirect the user to.
func (c *Client) AuthCodeURL(state string) string {
	return c.cfg.AuthCodeURL(state, oauth2.SetAuthURLParam("approval_prompt", "auto"))
}

// Exchange trades an authorization code for tokens and persists them.
func (c *Client) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	tok, err := c.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("strava token exchange: %w", err)
	}
	if err := c.store.Save(ctx, provider, tok); err != nil {
		return nil, fmt.Errorf("persist strava token: %w", err)
	}
	return tok, nil
}

// ValidToken loads the stored token and returns a non-expired one, refreshing
// and persisting it if the access token has expired.
func (c *Client) ValidToken(ctx context.Context) (*oauth2.Token, error) {
	stored, err := c.store.Load(ctx, provider)
	if err != nil {
		return nil, err
	}
	fresh, err := c.cfg.TokenSource(ctx, stored).Token()
	if err != nil {
		return nil, fmt.Errorf("refresh strava token: %w", err)
	}
	if fresh.AccessToken != stored.AccessToken {
		if err := c.store.Save(ctx, provider, fresh); err != nil {
			return nil, fmt.Errorf("persist refreshed strava token: %w", err)
		}
	}
	return fresh, nil
}
