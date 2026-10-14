package auth

import (
	"context"
	"testing"

	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

func TestEmailPassword(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemory()
	ownerHash, _ := HashPassword("owner password")
	owner, _ := s.Owner(ctx)
	owner.Email, owner.PasswordHash = "leo@example.com", ownerHash
	if err := s.UpdateRunner(ctx, owner); err != nil {
		t.Fatal(err)
	}
	memberHash, _ := HashPassword("member password")
	ana, _ := s.CreateRunner(ctx, store.Runner{Kind: store.KindMember, Email: "ana@example.com", PasswordHash: memberHash, DisplayName: "Ana", DailyQuestionCap: 10})
	p := EmailPassword{Runners: s}

	ok := []struct {
		c    Credentials
		want int64
	}{
		{Credentials{Email: "ana@example.com", Password: "member password"}, ana.ID},
		{Credentials{Email: " ANA@example.com ", Password: "member password"}, ana.ID},
		{Credentials{Email: "leo@example.com", Password: "owner password"}, owner.ID},
		// The sign-in form before accounts sent only a password.
		{Credentials{Password: "owner password"}, owner.ID},
	}
	for _, tc := range ok {
		if id, err := p.Authenticate(ctx, tc.c); err != nil || id != tc.want {
			t.Errorf("%+v: got %d, %v", tc.c, id, err)
		}
	}

	denied := []Credentials{
		{Email: "ana@example.com", Password: "owner password"},
		{Email: "ana@example.com", Password: ""},
		{Email: "nobody@example.com", Password: "member password"},
		{Password: "member password"},
	}
	for _, c := range denied {
		if _, err := p.Authenticate(ctx, c); err != ErrDenied {
			t.Errorf("%+v: got %v, want ErrDenied", c, err)
		}
	}

	ana.Status = store.StatusDisabled
	_ = s.UpdateRunner(ctx, ana)
	if _, err := p.Authenticate(ctx, Credentials{Email: "ana@example.com", Password: "member password"}); err != ErrDenied {
		t.Errorf("disabled runner signed in: %v", err)
	}
	// A runner with no password can never sign in, whatever is typed.
	nopw, _ := s.CreateRunner(ctx, store.Runner{Kind: store.KindMember, Email: "nopw@example.com", DisplayName: "x"})
	if _, err := p.Authenticate(ctx, Credentials{Email: nopw.Email, Password: "anything at all"}); err != ErrDenied {
		t.Errorf("runner without a password signed in: %v", err)
	}
}
