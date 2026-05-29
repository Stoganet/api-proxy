package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	*sql.DB
}

func Open(ctx context.Context, path string) (*DB, error) {
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := sqldb.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	if err := applyMigrations(ctx, sqldb); err != nil {
		return nil, err
	}
	return &DB{sqldb}, nil
}

func applyMigrations(ctx context.Context, sqldb *sql.DB) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, e := range entries {
		b, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		if _, err := sqldb.ExecContext(ctx, string(b)); err != nil {
			return fmt.Errorf("migration %s: %w", e.Name(), err)
		}
	}
	return nil
}
