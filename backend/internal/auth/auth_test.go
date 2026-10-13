package auth

import (
	"context"
	"testing"
)

func TestOwnerPassword(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	p := OwnerPassword{Hash: []byte(hash), OwnerID: 1}
	ctx := context.Background()

	if id, err := p.Authenticate(ctx, Credentials{Password: "correct horse"}); err != nil || id != 1 {
		t.Errorf("right password: %d, %v", id, err)
	}
	for _, bad := range []string{"wrong", "", "correct horse "} {
		if _, err := p.Authenticate(ctx, Credentials{Password: bad}); err != ErrDenied {
			t.Errorf("password %q: got %v, want ErrDenied", bad, err)
		}
	}
	// No hash configured means nobody can log in, never "anyone can".
	if _, err := (OwnerPassword{OwnerID: 1}).Authenticate(ctx, Credentials{Password: "anything"}); err != ErrDenied {
		t.Errorf("empty hash: got %v, want ErrDenied", err)
	}
}
