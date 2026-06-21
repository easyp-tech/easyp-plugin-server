// Package goosemigrate provides database migration using embedded SQL files and goose.
package goosemigrate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Up applies all pending migrations to the database at the given DSN.
// It uses PostgreSQL advisory locking to prevent concurrent migration execution.
// Migrations are embedded via embed.FS and applied in a transaction per migration.
func Up(ctx context.Context, dsn string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("sql.Open: %w", err)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			slog.Error("db.Close", "error", err)
		}
	}()

	err = db.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("db.PingContext: %w", err)
	}

	migrations, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("fs.Sub: %w", err)
	}

	sessionLocker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("lock.NewPostgresSessionLocker: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations,
		goose.WithSessionLocker(sessionLocker),
	)
	if err != nil {
		return fmt.Errorf("goose.NewProvider: %w", err)
	}

	_, err = provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("provider.Up: %w", err)
	}

	return nil
}
