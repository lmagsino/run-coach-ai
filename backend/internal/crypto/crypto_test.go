package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("eyJvYXV0aDEiOiAi...the bundle...")
	blob, err := s.Seal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, plain[:12]) {
		t.Error("blob contains the plaintext")
	}
	if blob[0] != KeyVersion {
		t.Errorf("version byte: %d", blob[0])
	}
	got, err := s.Open(blob)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("open: %q, %v", got, err)
	}

	other, _ := New(mustKey(t))
	if _, err := other.Open(blob); err == nil {
		t.Error("another key opened the blob")
	}
	blob[len(blob)-1] ^= 1
	if _, err := s.Open(blob); err == nil {
		t.Error("a tampered blob opened")
	}
}

func TestBadKeys(t *testing.T) {
	for _, k := range []string{"", "short", "not base64!!", "AAAA"} {
		if _, err := New(k); err != ErrBadKey {
			t.Errorf("key %q: got %v, want ErrBadKey", k, err)
		}
	}
}

func mustKey(t *testing.T) string {
	t.Helper()
	k, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestKeyInEveryBase64Alphabet(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(250 + i) // high bytes, so the URL and standard alphabets differ
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if _, err := New(enc.EncodeToString(raw)); err != nil {
			t.Errorf("%q: %v", enc.EncodeToString(raw), err)
		}
	}
}
