// Command server runs the RunCoach AI backend HTTP API.
package main

import (
	"context"
	"errors"
	"log"
	"net"
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

	// Nothing is stored until chats are persisted (ADR 0008), so the database
	// is optional: mock mode never touches it, and a live server without
	// DATABASE_URL simply runs without one and says so on /healthz.
	var pool *pgxpool.Pool
	switch {
	case cfg.MockMode:
		log.Println("mock mode: skipping the database connection")
	case cfg.DatabaseURL == "":
		log.Println("DATABASE_URL is not set: running without a database")
	default:
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
		Addr:              net.JoinHostPort(cfg.Host, cfg.Port),
		Handler:           api.New(cfg, pool).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Run the server until an interrupt signal, then shut down gracefully.
	go func() {
		log.Printf("listening on http://%s", srv.Addr)
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
