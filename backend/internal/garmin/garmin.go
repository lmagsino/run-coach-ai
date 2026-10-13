// Package garmin is how the backend reaches a runner's Garmin data. One
// implementation today, LocalSpawn, which runs the upstream garmin-mcp server
// with the runner's stored token bundle (ADR 0003, 0004). The roadmap adds
// GatewayHTTP (a hosted gateway speaking MCP over HTTP) and BYOURL (a URL the
// runner supplies) behind the same interface (ADR 0002).
package garmin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/crypto"
	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
	"github.com/lmagsino/run-coach-ai/backend/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Source opens an MCP session over a runner's Garmin data.
type Source interface {
	// Connect returns a ready session and a func that closes it and cleans up.
	Connect(ctx context.Context, runnerID int64) (*mcp.ClientSession, func(), error)
	// Probe checks a token bundle before it is stored: one cheap profile
	// read. It returns the runner's Garmin display name on success.
	Probe(ctx context.Context, bundle string) (string, error)
}

// Errors the API maps to a state and a fix sentence.
var (
	ErrNotConnected = errors.New("garmin: no token bundle for this runner")
	ErrAuth         = errors.New("garmin: tokens rejected")
	ErrUnavailable  = errors.New("garmin: unavailable")
)

// TokenLifetime is upstream's own figure for a bundle ("~6-month bearer
// credentials"); the chip shows the estimate and a real auth error overrides it.
const TokenLifetime = 182 * 24 * time.Hour

// ProbeTool is the cheapest read that proves a bundle works.
const ProbeTool = "get_user_profile"

// LocalSpawn runs garmin-mcp as a subprocess for each reply.
//
// The decrypted bundle is decoded and handed to the child as GARMINTOKENS.
// Upstream reads that variable as a token store path, but python-garminconnect
// 0.3.2 treats any store longer than 512 characters as the token data itself
// and parses it with json.loads, so the child gets the JSON that
// garmin-mcp-auth base64-encoded, never the base64, and no file is written.
// Without a stored bundle the command runs as configured, which is the
// local-dev case of a named Docker volume holding the tokens.
type LocalSpawn struct {
	Command string // program and args, whitespace separated
	Store   store.Store
	Sealer  *crypto.Sealer
}

func (l *LocalSpawn) client(extraEnv ...string) *mcpclient.Garmin {
	fields := strings.Fields(l.Command)
	if len(fields) == 0 {
		fields = []string{"garmin-mcp"}
	}
	g := mcpclient.NewGarmin(fields[0], fields[1:]...)
	g.ExtraEnv = extraEnv
	return g
}

func (l *LocalSpawn) Connect(ctx context.Context, runnerID int64) (*mcp.ClientSession, func(), error) {
	conn, err := l.Store.GarminConnection(ctx, runnerID)
	if err != nil {
		return nil, nil, err
	}
	var env []string
	if conn != nil {
		if l.Sealer == nil {
			return nil, nil, errors.New("garmin: RUNCOACH_TOKEN_KEY is not set, cannot open the stored bundle")
		}
		bundle, err := l.Sealer.Open(conn.Blob)
		if err != nil {
			return nil, nil, err
		}
		tokens, err := TokenStore(string(bundle))
		if err != nil {
			return nil, nil, err
		}
		env = []string{"GARMINTOKENS=" + tokens}
	}
	session, err := l.client(env...).Connect(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return session, func() { session.Close() }, nil
}

func (l *LocalSpawn) Probe(ctx context.Context, bundle string) (string, error) {
	bundle, err := NormalizeBundle(bundle)
	if err != nil {
		return "", err
	}
	tokens, err := TokenStore(bundle)
	if err != nil {
		return "", err
	}
	session, err := l.client("GARMINTOKENS=" + tokens).Connect(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer session.Close()
	return probe(ctx, session)
}

// probe calls the profile tool and classifies the outcome.
func probe(ctx context.Context, session *mcp.ClientSession) (string, error) {
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: ProbeTool, Arguments: map[string]any{}})
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	// Upstream answers a dead login as ordinary text with is_error false, so
	// the flag alone would store a bundle that does not work.
	if res.IsError || mcpclient.LooksLikeError(text.String()) {
		return "", Classify(errors.New(text.String()))
	}
	return displayName(text.String()), nil
}

// Classify turns an error from the child into the state the chip shows.
// Anything upstream reports about tokens, login or 401/403 is "expired";
// everything else is "unavailable". It never invents a success.
func Classify(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	for _, hint := range []string{"token", "login", "authenticat", "401", "403", "not logged in", "mfa"} {
		if strings.Contains(msg, hint) {
			return fmt.Errorf("%w: %v", ErrAuth, err)
		}
	}
	if errors.Is(err, ErrAuth) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}

// NormalizeBundle trims what a paste brings along and checks it is base64 of
// JSON, which is what garmin-mcp-auth writes. It does not check the tokens;
// Probe does that.
func NormalizeBundle(s string) (string, error) {
	s = strings.Join(strings.Fields(s), "")
	if len(s) <= 512 {
		return "", errors.New("garmin: that is not a token bundle (too short); paste the whole line from ~/.garminconnect_base64")
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", errors.New("garmin: the bundle is not valid base64")
	}
	if !json.Valid(raw) {
		return "", errors.New("garmin: the bundle does not decode to JSON")
	}
	return s, nil
}

// TokenStore turns a normalised bundle into the value GARMINTOKENS carries: the
// decoded JSON on one line. python-garminconnect only treats the value as inline
// token data when it is longer than 512 characters (shorter means a path), so
// a short JSON is indented until it is; indentation is still valid JSON.
func TokenStore(bundle string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(bundle)
	if err != nil {
		return "", errors.New("garmin: the bundle is not valid base64")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return "", errors.New("garmin: the bundle does not decode to JSON")
	}
	out := compact.String()
	if len(out) <= 512 {
		// Each line gets a long prefix, so even a three-key object passes 512.
		var padded bytes.Buffer
		if err := json.Indent(&padded, compact.Bytes(), strings.Repeat(" ", 256), " "); err == nil && padded.Len() > 512 {
			out = padded.String()
		}
	}
	return out, nil
}

// displayName pulls a name out of the profile payload if there is one; the
// probe's success does not depend on it.
func displayName(profileJSON string) string {
	var v map[string]any
	if err := json.Unmarshal([]byte(profileJSON), &v); err != nil {
		return ""
	}
	for _, k := range []string{"displayName", "fullName", "userName", "display_name"} {
		if s, ok := v[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}
