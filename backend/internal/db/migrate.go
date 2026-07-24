package db

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the "pgx5" scheme
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// newMigrator builds a migrate.Migrate backed by the embedded SQL files and
// the pgx v5 database driver.
func newMigrator(databaseURL string) (*migrate.Migrate, error) {
	src, err := iofs.New(MigrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("load embedded migrations: %w", err)
	}

	// golang-migrate's pgx/v5 driver is registered under the "pgx5" scheme,
	// so translate the standard postgres:// URL the app otherwise uses.
	migrateURL := databaseURL
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(migrateURL, prefix) {
			migrateURL = "pgx5://" + strings.TrimPrefix(migrateURL, prefix)
			break
		}
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, migrateURL)
	if err != nil {
		return nil, fmt.Errorf("init migrator: %w", err)
	}
	return m, nil
}

// MigrateUp applies all pending migrations. A no-op if already current.
func MigrateUp(databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// MigrateDown rolls back all migrations. A no-op if nothing is applied.
func MigrateDown(databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}
