// Command migrate applies or rolls back database migrations.
//
// Usage:
//
//	go run ./cmd/migrate up      # apply all pending migrations
//	go run ./cmd/migrate down    # roll back all migrations
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/lmagsino/run-coach-ai/backend/internal/config"
	"github.com/lmagsino/run-coach-ai/backend/internal/db"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate <up|down>")
	}
	direction := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	switch direction {
	case "up":
		if err := db.MigrateUp(cfg.DatabaseURL); err != nil {
			log.Fatalf("migrate up: %v", err)
		}
		fmt.Println("migrations applied")
	case "down":
		if err := db.MigrateDown(cfg.DatabaseURL); err != nil {
			log.Fatalf("migrate down: %v", err)
		}
		fmt.Println("migrations rolled back")
	default:
		log.Fatalf("unknown direction %q (want up or down)", direction)
	}
}
