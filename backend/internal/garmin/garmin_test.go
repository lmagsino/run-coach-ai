package garmin

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
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
