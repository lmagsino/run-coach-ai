package garmin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

func TestNormalizeBundle(t *testing.T) {
	long := base64.StdEncoding.EncodeToString([]byte(`{"oauth1_token":{"x":"` + strings.Repeat("a", 600) + `"}}`))
	got, err := NormalizeBundle("  " + long[:100] + "\n" + long[100:] + "\n")
	if err != nil || got != long {
		t.Errorf("whitespace-wrapped bundle: %v", err)
	}
	for name, s := range map[string]string{
		"short":      "abc",
		"not base64": strings.Repeat("!", 600),
		"not json":   base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 600))),
	} {
		if _, err := NormalizeBundle(s); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

// What reaches the child is the JSON garmin-mcp-auth encoded, never the
// base64: python-garminconnect parses an inline store with json.loads.
func TestTokenStoreIsDecodedJSON(t *testing.T) {
	tokens := `{"di_token":"` + strings.Repeat("t", 700) + `","di_refresh_token":"r","di_client_id":"c"}`
	bundle := base64.StdEncoding.EncodeToString([]byte(" " + tokens + "\n"))
	got, err := TokenStore(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if got != tokens {
		t.Errorf("token store is not the compact JSON:\n%s", got[:80])
	}
	if strings.Contains(got, "\n") {
		t.Error("compact store carries a newline")
	}

	// A bundle whose JSON is short would be read as a path; it is indented
	// past the inline threshold and stays valid JSON.
	short := base64.StdEncoding.EncodeToString([]byte(`{"di_token":"abc","di_refresh_token":"r","di_client_id":"c"}`))
	got, err = TokenStore(short)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) <= 512 {
		t.Errorf("short store not padded past 512: %d", len(got))
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(got), &v); err != nil || v["di_token"] != "abc" {
		t.Errorf("padded store is not the same JSON: %v", err)
	}

	if _, err := TokenStore("not base64!"); err == nil {
		t.Error("garbage accepted")
	}
}

func TestClassify(t *testing.T) {
	for msg, want := range map[string]error{
		"OAuth tokens not found":    ErrAuth,
		"HTTP 401 Unauthorized":     ErrAuth,
		"Please login again":        ErrAuth,
		"connection reset by peer":  ErrUnavailable,
		"HTTP 503 from connectapi":  ErrUnavailable,
		"Cloudflare 429 rate limit": ErrUnavailable,
	} {
		if got := Classify(errors.New(msg)); !errors.Is(got, want) {
			t.Errorf("%q: got %v, want %v", msg, got, want)
		}
	}
	if Classify(nil) != nil {
		t.Error("nil classified as an error")
	}
}

func TestDisplayName(t *testing.T) {
	if got := displayName(`{"displayName":"leo","id":1}`); got != "leo" {
		t.Errorf("got %q", got)
	}
	if got := displayName(`not json`); got != "" {
		t.Errorf("got %q", got)
	}
}

// A runner with no stored bundle is not connected, and nothing is spawned:
// the configured command could find another runner's tokens on its own.
func TestConnectWithoutABundleSpawnsNothing(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "spawned")
	l := &LocalSpawn{Command: "touch " + marker, Store: store.NewMemory()}
	_, _, err := l.Connect(context.Background(), 1)
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("got %v, want ErrNotConnected", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Error("the command ran for a runner with no bundle")
	}
}
