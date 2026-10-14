// Command set-password sets a runner's password in the database, for the
// owner to reset a member who forgot theirs, or their own: there is no reset
// mail (ADR 0010). The new password is read from stdin and never echoed.
//
//	echo -n 'new password' | go run ./cmd/set-password ana@example.com
//	echo -n 'new password' | go run ./cmd/set-password owner
//
// Every session of that runner is signed out.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lmagsino/run-coach-ai/backend/internal/auth"
	"github.com/lmagsino/run-coach-ai/backend/internal/config"
	"github.com/lmagsino/run-coach-ai/backend/internal/db"
	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: set-password <email|owner>   (password on stdin)")
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is not set: without a database, accounts live in the server's memory; restart it instead")
	}
	in, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	password := strings.TrimRight(in, "\r\n")
	if len([]rune(password)) < auth.MinPasswordLen {
		log.Fatalf("the password needs at least %d characters", auth.MinPasswordLen)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	s := store.NewPG(pool)

	var runner store.Runner
	if os.Args[1] == "owner" {
		runner, err = s.Owner(ctx)
	} else {
		runner, err = s.RunnerByEmail(ctx, os.Args[1])
	}
	if err != nil {
		log.Fatalf("no runner %q: %v", os.Args[1], err)
	}
	if runner.PasswordHash, err = auth.HashPassword(password); err != nil {
		log.Fatal(err)
	}
	if err := s.UpdateRunner(ctx, runner); err != nil {
		log.Fatal(err)
	}
	if err := s.DeleteSessionsFor(ctx, runner.ID, ""); err != nil {
		log.Fatal(err)
	}
	_ = s.Audit(ctx, runner.ID, "password_reset", map[string]any{"by": "cmd/set-password"})
	fmt.Printf("password set for runner %d (%s); every session signed out\n", runner.ID, runner.DisplayName)
}
