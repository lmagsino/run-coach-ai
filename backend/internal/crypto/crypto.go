// Package crypto seals the Garmin token bundle for storage (ADR 0003, 0007).
// AES-256-GCM under a key from the environment; the first byte of a sealed
// blob is the key version so a key can be rotated by re-sealing rows.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// KeyVersion is written into every blob this build seals.
const KeyVersion = 1

var ErrBadKey = errors.New("crypto: RUNCOACH_TOKEN_KEY must be 32 random bytes, base64 encoded")

// Sealer holds the key.
type Sealer struct{ aead cipher.AEAD }

// New parses a base64 32-byte key.
func New(base64Key string) (*Sealer, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil || len(key) != 32 {
		return nil, ErrBadKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

// NewKey returns a fresh key in the form RUNCOACH_TOKEN_KEY takes.
func NewKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// Seal returns version || nonce || ciphertext.
func (s *Sealer) Seal(plain []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 1+len(nonce)+len(plain)+s.aead.Overhead())
	out = append(out, KeyVersion)
	out = append(out, nonce...)
	return s.aead.Seal(out, nonce, plain, nil), nil
}

// Open reverses Seal. A wrong key or a tampered blob fails; it never returns
// garbage.
func (s *Sealer) Open(blob []byte) ([]byte, error) {
	ns := s.aead.NonceSize()
	if len(blob) < 1+ns+s.aead.Overhead() {
		return nil, errors.New("crypto: blob too short")
	}
	if blob[0] != KeyVersion {
		return nil, fmt.Errorf("crypto: blob sealed with key version %d, this build has %d", blob[0], KeyVersion)
	}
	plain, err := s.aead.Open(nil, blob[1:1+ns], blob[1+ns:], nil)
	if err != nil {
		return nil, errors.New("crypto: cannot open blob (wrong key or tampered)")
	}
	return plain, nil
}
