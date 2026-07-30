// Command server runs the RunCoach AI backend HTTP API.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmagsino/run-coach-ai/backend/internal/api"
	"github.com/lmagsino/run-coach-ai/backend/internal/config"
	"github.com/lmagsino/run-coach-ai/backend/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Mock mode answers without touching Postgres — no token lookup, no history
	// persistence — so requiring a running database would make frontend work
	// depend on infrastructure it never uses.
	var pool *pgxpool.Pool
	if cfg.MockMode {
		log.Println("mock mode: skipping the database connection")
	} else {
		ctx := context.Background()
		p, err := db.NewPool(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("connect database: %v", err)
		}
		pool = p
		defer pool.Close()
		log.Println("connected to database")
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.New(cfg, pool).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Run the server until an interrupt signal, then shut down gracefully.
	go func() {
		log.Printf("listening on http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("graceful shutdown: %v", err)
	}
}
